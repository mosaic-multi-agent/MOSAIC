package pricing_test

// Tests for Store.Put with a target level: the level decides which file is
// written, the other level's file is never created or changed, and the returned
// ConfigFile names the file written.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

func TestPutAtGlobal_WritesTheGlobalFileAndNeverCreatesTheProjectFile(t *testing.T) {
	l := newLayered(t)

	file, err := l.store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "saved-model"))

	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !contains(readFile(t, l.globalFile), "saved-model") {
		t.Error("the global file must hold the saved entry")
	}
	if _, statErr := os.Stat(l.projFile); !os.IsNotExist(statErr) {
		t.Error("a global save must not create the project file")
	}
	if file.Level != domain.LevelGlobal || file.Path != l.globalFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing global file %s", file, l.globalFile)
	}
}

func TestPutAtProject_CreatesTheProjectFileAndLeavesTheGlobalFileAlone(t *testing.T) {
	l := newLayered(t)

	file, err := l.store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "saved-model"))

	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !contains(readFile(t, l.projFile), "saved-model") {
		t.Error("the project file must be created with the saved entry")
	}
	if exists(l.globalFile) {
		t.Error("a project save must not create the global file")
	}
	if file.Level != domain.LevelProject || file.Path != l.projFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing project file %s", file, l.projFile)
	}
}

func TestPutAtGlobal_LeavesAnExistingProjectFileUntouched(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, modelYAML("project-model", "3.00"))
	before := readFile(t, l.projFile)

	if _, err := l.store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "saved-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if after := readFile(t, l.projFile); after != before {
		t.Errorf("the project file changed by a global save:\n%s", after)
	}
}

func TestPutAtGlobal_KeepsOtherEntriesAndHeaderComments(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, "# my global prices\n"+modelYAML("other-model", "3.00"))

	if _, err := l.store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "saved-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got := readFile(t, l.globalFile)
	for _, want := range []string{"# my global prices", "other-model", "saved-model"} {
		if !contains(got, want) {
			t.Errorf("global file lost or lacks %q:\n%s", want, got)
		}
	}
}

func TestPutAtGlobal_ThenLoad_EntryComesFromTheGlobalLevelAndProjectStillWins(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, modelYAML("shared-model", "9.00"))
	if _, err := l.store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "global-only")); err != nil {
		t.Fatalf("Put global-only: %v", err)
	}
	if _, err := l.store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "shared-model")); err != nil {
		t.Fatalf("Put shared-model: %v", err)
	}

	table, _ := loadLayered(t, l)

	if e := mustLookup(t, table, "", "global-only"); e.Source != domain.LevelGlobal {
		t.Errorf("global-only Source = %v, want global", e.Source)
	}
	if e := mustLookup(t, table, "", "shared-model"); e.Source != domain.LevelProject {
		t.Errorf("shared-model Source = %v, want project (the project entry replaces the global one)", e.Source)
	}
}

func TestPut_GlobalLevelUnavailable_ReturnsErrConfigLevelUnavailableAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	store := pricing.NewStore("", root)

	_, err := store.Put(context.Background(), domain.LevelGlobal, newMinimalEntry(t, "saved-model"))

	if !errors.Is(err, domain.ErrConfigLevelUnavailable) {
		t.Errorf("err = %v, want ErrConfigLevelUnavailable", err)
	}
	if _, statErr := os.Stat(store.Path()); !os.IsNotExist(statErr) {
		t.Error("nothing may be written when the global level is unavailable")
	}
}

func TestPut_BuiltInOrUnknownLevel_ReturnsErrInvalidConfigLevelAndWritesNothing(t *testing.T) {
	for _, level := range []domain.ConfigLevel{domain.LevelBuiltIn, domain.ConfigLevel(99)} {
		l := newLayered(t)

		_, err := l.store.Put(context.Background(), level, newMinimalEntry(t, "saved-model"))

		if !errors.Is(err, domain.ErrInvalidConfigLevel) {
			t.Errorf("level %v: err = %v, want ErrInvalidConfigLevel", level, err)
		}
		if exists(l.globalFile) || exists(l.projFile) {
			t.Errorf("level %v: no file may be written for an invalid level", level)
		}
	}
}

func TestPutAtProject_GlobalLevelUnavailable_StillCreatesTheProjectFile(t *testing.T) {
	root := t.TempDir()
	store := pricing.NewStore("", root)
	projFile := filepath.Join(root, "MosaicLogAnalyzer", "config", "pricing.yaml")

	file, err := store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "saved-model"))

	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !contains(readFile(t, projFile), "saved-model") {
		t.Error("the project file must be created with the saved entry")
	}
	if file.Level != domain.LevelProject || file.Path != projFile || !file.Exists {
		t.Errorf("returned file = %+v, want the existing project file %s", file, projFile)
	}
}
