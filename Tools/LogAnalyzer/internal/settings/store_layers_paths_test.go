package settings_test

// Tests for Store.Paths, the unavailable global level and saving to the project file.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

func TestPaths_ReportsBothLevelsWithExistence(t *testing.T) {
	l := newLayered(t)

	absent := l.store.Paths()
	writeAt(t, l.globalFile, "# g\n")
	writeAt(t, l.projFile, "# p\n")
	present := l.store.Paths()

	if absent.Global.Path != l.globalFile || absent.Project.Path != l.projFile {
		t.Errorf("paths = %q / %q, want %q / %q", absent.Global.Path, absent.Project.Path, l.globalFile, l.projFile)
	}
	if absent.Global.Level != domain.LevelGlobal || absent.Project.Level != domain.LevelProject {
		t.Errorf("levels = %v / %v, want global / project", absent.Global.Level, absent.Project.Level)
	}
	if absent.Global.Exists || absent.Project.Exists {
		t.Errorf("absent files reported as existing: %+v", absent)
	}
	if !present.Global.Exists || !present.Project.Exists {
		t.Errorf("present files reported as absent: %+v", present)
	}
}

func TestPaths_NoGlobalFolder_GlobalLevelUnavailable(t *testing.T) {
	s := settings.NewStore("", t.TempDir())

	p := s.Paths()

	if p.Global.Unavailable == "" || p.Global.Path != "" {
		t.Errorf("global = %+v, want an Unavailable reason and no path", p.Global)
	}
	if p.Project.Path == "" || p.Project.Unavailable != "" {
		t.Errorf("project = %+v, want a usable project path", p.Project)
	}
}

func TestLoadLayers_NoGlobalFolder_ProjectLevelStillLoads(t *testing.T) {
	s := settings.NewStore("", t.TempDir())
	writeAt(t, s.Paths().Project.Path, "detected_compaction:\n  drop_percent: 45\n")

	got, _, err := s.Load(context.Background())

	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.CompactionDropPercent != 45 {
		t.Errorf("CompactionDropPercent = %d, want 45", got.CompactionDropPercent)
	}
}

func TestAddUserInteractionTool_WritesProjectFileAndLeavesGlobalUntouched(t *testing.T) {
	l := newLayered(t)
	const global = "detected_compaction:\n  drop_percent: 40\n"
	writeAt(t, l.globalFile, global)

	if _, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelProject, domain.HarnessClaudeCode, "MyAskTool"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	if got := readAt(t, l.globalFile); got != global {
		t.Errorf("global file changed by the save:\n%s", got)
	}
	if !hasLine(readAt(t, l.projFile), "MyAskTool") {
		t.Error("the project file must hold the marked tool")
	}
	got, _ := loadLayered(t, l)
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "MyAskTool") || got.CompactionDropPercent != 40 {
		t.Error("the merged settings must hold the marked tool and keep the global drop_percent")
	}
}

func TestAddUserInteractionTool_WithNoGlobalFile_DoesNotCreateIt(t *testing.T) {
	l := newLayered(t)

	if _, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelProject, domain.HarnessClaudeCode, "MyAskTool"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	if fileExists(l.globalFile) {
		t.Error("marking a tool must not create the global file")
	}
	if !fileExists(l.projFile) {
		t.Error("the project file must be created by the save")
	}
}
