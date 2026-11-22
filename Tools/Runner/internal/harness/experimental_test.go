package harness_test

// Tests for the Runner's own experimental-harness declaration and how it
// reaches harness selection data.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
)

const testExperimentalReason = "reason-token-4417 not proven in real runs"

// markedHarnessID is a catalog harness the tests mark; unmarkedHarnessID is a
// different catalog harness that stays unmarked.
const (
	markedHarnessID   = commonharness.HarnessIDOpenCode
	unmarkedHarnessID = commonharness.HarnessIDClaudeCode
)

func findSelection(t *testing.T, sels []harness.Selection, id string) harness.Selection {
	t.Helper()
	for _, s := range sels {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("selection %q not offered; got %+v", id, sels)
	return harness.Selection{}
}

func TestExperimentalReason_MarkedHarnessReportsReason(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	defer restore()

	reason, ok := harness.ExperimentalReason(markedHarnessID)

	if !ok || reason != testExperimentalReason {
		t.Errorf("ExperimentalReason(%q) = (%q, %v), want (%q, true)", markedHarnessID, reason, ok, testExperimentalReason)
	}
}

func TestExperimentalReason_OtherHarnessesAreNotMarked(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	defer restore()

	for _, id := range []string{unmarkedHarnessID, harness.FakeHarnessID, "unknown", ""} {
		if reason, ok := harness.ExperimentalReason(id); ok || reason != "" {
			t.Errorf("ExperimentalReason(%q) = (%q, %v), want (\"\", false)", id, reason, ok)
		}
	}
}

func TestMarkExperimentalForTest_RestoreRemovesMarking(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	restore()

	if reason, ok := harness.ExperimentalReason(markedHarnessID); ok {
		t.Errorf("after restore ExperimentalReason(%q) = (%q, true), want unmarked", markedHarnessID, reason)
	}
}

func TestMarkExperimentalForTest_BlankReasonPanics(t *testing.T) {
	for _, reason := range []string{"", "   ", "\t\n"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("MarkExperimentalForTest(%q, %q) did not panic", markedHarnessID, reason)
				}
			}()
			restore := harness.MarkExperimentalForTest(markedHarnessID, reason)
			restore()
		}()
	}
}

// Every marking the Runner declares in production carries a reason.
func TestExperimentalReason_EveryDeclaredMarkingHasNonEmptyReason(t *testing.T) {
	ids := []string{harness.FakeHarnessID}
	for _, e := range commonharness.CLIHarnesses() {
		ids = append(ids, e.ID)
	}
	for _, id := range ids {
		reason, ok := harness.ExperimentalReason(id)
		if ok && strings.TrimSpace(reason) == "" {
			t.Errorf("ExperimentalReason(%q) is marked with an empty reason", id)
		}
	}
}

func TestSelections_MarkedHarnessStaysOfferedAndCarriesReason(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	defer restore()

	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		got := findSelection(t, sels, markedHarnessID)
		if got.ExperimentalReason != testExperimentalReason {
			t.Errorf("%s: ExperimentalReason = %q, want %q", name, got.ExperimentalReason, testExperimentalReason)
		}
		if !got.CLIBacked {
			t.Errorf("%s: marked harness must stay CLIBacked", name)
		}
	}
}

func TestSelections_UnmarkedHarnessesCarryNoReason(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	defer restore()

	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		for _, s := range sels {
			productionReason, _ := harness.ExperimentalReason(s.ID)
			if s.ID != markedHarnessID && s.ExperimentalReason != productionReason {
				t.Errorf("%s: unmarked %q carries reason %q, want its production reason %q", name, s.ID, s.ExperimentalReason, productionReason)
			}
		}
	}
}

func TestAccepts_MarkedHarnessIsAccepted(t *testing.T) {
	restore := harness.MarkExperimentalForTest(markedHarnessID, testExperimentalReason)
	defer restore()

	if !harness.Accepts(markedHarnessID) {
		t.Errorf("Accepts(%q) = false for an experimental harness, want true", markedHarnessID)
	}
	if !strings.Contains(harness.FlagValues(), markedHarnessID) {
		t.Errorf("FlagValues() = %q, want it to contain the experimental harness", harness.FlagValues())
	}
}

// The Runner declaration is independent of Deploy: nothing in the package
// imports a mosaic-deploy package.
func TestExperimentalDeclaration_PackageDoesNotImportDeploy(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("cannot list package sources: %v (%d files)", err, len(files))
	}
	if _, statErr := os.Stat("experimental.go"); statErr != nil {
		t.Fatalf("experimental declaration file missing: %v", statErr)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, perr := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if perr != nil {
			t.Fatalf("parse %s: %v", f, perr)
		}
		for _, imp := range parsed.Imports {
			if strings.Contains(imp.Path.Value, "mosaic-deploy") {
				t.Errorf("%s imports %s; the Runner declaration must be independent of Deploy", f, imp.Path.Value)
			}
		}
	}
}
