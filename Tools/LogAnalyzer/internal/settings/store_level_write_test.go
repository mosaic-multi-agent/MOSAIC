package settings_test

// Tests for Store.AddUserInteractionTool with a target level: the level decides
// which file is written, the other level's file is never created or changed, and
// the returned ConfigFile names the file written.

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

func TestAddUserInteractionToolAtGlobal_WritesTheGlobalFileAndNeverCreatesTheProjectFile(t *testing.T) {
	l := newLayered(t)

	file, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelGlobal, domain.HarnessClaudeCode, "MyAskTool")

	if err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}
	if !hasLine(readAt(t, l.globalFile), "MyAskTool") {
		t.Error("the global file must hold the marked tool")
	}
	if fileExists(l.projFile) {
		t.Error("a global save must not create the project file")
	}
	if file.Level != domain.LevelGlobal || file.Path != l.globalFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing global file %s", file, l.globalFile)
	}
}

func TestAddUserInteractionToolAtProject_CreatesTheProjectFileAndLeavesTheGlobalFileAlone(t *testing.T) {
	l := newLayered(t)

	file, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelProject, domain.HarnessClaudeCode, "MyAskTool")

	if err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}
	if !hasLine(readAt(t, l.projFile), "MyAskTool") {
		t.Error("the project file must be created with the marked tool")
	}
	if fileExists(l.globalFile) {
		t.Error("a project save must not create the global file")
	}
	if file.Level != domain.LevelProject || file.Path != l.projFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing project file %s", file, l.projFile)
	}
}

func TestAddUserInteractionToolAtGlobal_LeavesAnExistingProjectFileUntouched(t *testing.T) {
	l := newLayered(t)
	const project = "detected_compaction:\n  drop_percent: 30\n"
	writeAt(t, l.projFile, project)

	if _, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelGlobal, domain.HarnessClaudeCode, "MyAskTool"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	if got := readAt(t, l.projFile); got != project {
		t.Errorf("the project file changed by a global save:\n%s", got)
	}
}

func TestAddUserInteractionToolAtGlobal_ThenLoad_MergedSettingsHoldTheTool(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, "detected_compaction:\n  drop_percent: 30\n")

	if _, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelGlobal, domain.HarnessClaudeCode, "MyAskTool"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := loadLayered(t, l)
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "MyAskTool") || got.CompactionDropPercent != 30 {
		t.Error("the merged settings must hold the globally marked tool and keep the project drop_percent")
	}
}

func TestAddUserInteractionTool_GlobalLevelUnavailable_ReturnsErrConfigLevelUnavailableAndWritesNothing(t *testing.T) {
	store := settings.NewStore("", t.TempDir())

	_, err := store.AddUserInteractionTool(context.Background(), domain.LevelGlobal, domain.HarnessClaudeCode, "MyAskTool")

	if !errors.Is(err, domain.ErrConfigLevelUnavailable) {
		t.Errorf("err = %v, want ErrConfigLevelUnavailable", err)
	}
	if fileExists(store.Path()) {
		t.Error("nothing may be written when the global level is unavailable")
	}
}

func TestAddUserInteractionTool_BuiltInOrUnknownLevel_ReturnsErrInvalidConfigLevelAndWritesNothing(t *testing.T) {
	for _, level := range []domain.ConfigLevel{domain.LevelBuiltIn, domain.ConfigLevel(99)} {
		l := newLayered(t)

		_, err := l.store.AddUserInteractionTool(context.Background(), level, domain.HarnessClaudeCode, "MyAskTool")

		if !errors.Is(err, domain.ErrInvalidConfigLevel) {
			t.Errorf("level %v: err = %v, want ErrInvalidConfigLevel", level, err)
		}
		if fileExists(l.globalFile) || fileExists(l.projFile) {
			t.Errorf("level %v: no file may be written for an invalid level", level)
		}
	}
}

func TestAddUserInteractionToolAtGlobal_InvalidToolName_WritesNothing(t *testing.T) {
	l := newLayered(t)

	_, err := l.store.AddUserInteractionTool(context.Background(), domain.LevelGlobal, domain.HarnessClaudeCode, "")

	if !errors.Is(err, domain.ErrInvalidToolName) {
		t.Errorf("err = %v, want ErrInvalidToolName", err)
	}
	if fileExists(l.globalFile) {
		t.Error("no file may be written for an invalid tool name")
	}
}

func TestAddUserInteractionToolAtProject_GlobalLevelUnavailable_StillCreatesTheProjectFile(t *testing.T) {
	root := t.TempDir()
	store := settings.NewStore("", root)
	projFile := filepath.Join(root, "MosaicLogAnalyzer", "config", "analysis.yaml")

	file, err := store.AddUserInteractionTool(context.Background(), domain.LevelProject, domain.HarnessClaudeCode, "MyAskTool")

	if err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}
	if !hasLine(readAt(t, projFile), "MyAskTool") {
		t.Error("the project file must be created with the marked tool")
	}
	if file.Level != domain.LevelProject || file.Path != projFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing project file %s", file, projFile)
	}
}
