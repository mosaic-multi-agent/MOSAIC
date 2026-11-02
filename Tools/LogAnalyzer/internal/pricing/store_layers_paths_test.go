package pricing_test

// Tests for Store.Paths, the unavailable global level, and saving to the project file.

import (
	"context"
	"os"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

func TestPaths_ReportsBothLevelsWithExistence(t *testing.T) {
	l := newLayered(t)

	absent := l.store.Paths()
	writeFileAt(t, l.globalFile, commentsOnlyYAML)
	writeFileAt(t, l.projFile, commentsOnlyYAML)
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
	store := pricing.NewStore("", t.TempDir())

	p := store.Paths()

	if p.Global.Unavailable == "" || p.Global.Path != "" {
		t.Errorf("global = %+v, want an Unavailable reason and no path", p.Global)
	}
	if p.Project.Path == "" || p.Project.Unavailable != "" {
		t.Errorf("project = %+v, want a usable project path", p.Project)
	}
}

func TestLoadLayers_NoGlobalFolder_ProjectLevelStillLoads(t *testing.T) {
	store := pricing.NewStore("", t.TempDir())
	writeFileAt(t, store.Paths().Project.Path, modelYAML("m", "1.00"))

	table, _, err := store.Load(context.Background())

	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e := mustLookup(t, table, "", "m"); e.Source != domain.LevelProject {
		t.Errorf("Source = %v, want project", e.Source)
	}
}

func TestPut_WithGlobalFilePresent_WritesProjectFileAndLeavesGlobalUntouched(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("global-model", "3.00"))
	before := readFile(t, l.globalFile)

	if _, err := l.store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "saved-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if after := readFile(t, l.globalFile); after != before {
		t.Errorf("global file changed by Put:\n%s", after)
	}
	proj := readFile(t, l.projFile)
	if !contains(proj, "saved-model") || contains(proj, "global-model") {
		t.Errorf("project file must hold the saved entry only, got:\n%s", proj)
	}
}

func TestPut_ThenLoad_SavedEntryIsProjectLevelAndGlobalEntriesRemain(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("global-model", "3.00"))
	if _, err := l.store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "saved-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	table, _ := loadLayered(t, l)

	if e := mustLookup(t, table, "", "saved-model"); e.Source != domain.LevelProject {
		t.Errorf("saved Source = %v, want project", e.Source)
	}
	if e := mustLookup(t, table, "", "global-model"); e.Source != domain.LevelGlobal {
		t.Errorf("global Source = %v, want global", e.Source)
	}
}

func TestPut_WithNoGlobalFile_DoesNotCreateIt(t *testing.T) {
	l := newLayered(t)

	if _, err := l.store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "saved-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if exists(l.globalFile) {
		t.Error("saving a price must not create the global file")
	}
	if _, err := os.Stat(l.projFile); err != nil {
		t.Errorf("project file must be created by the save: %v", err)
	}
}
