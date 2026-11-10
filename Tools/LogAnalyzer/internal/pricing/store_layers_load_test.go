package pricing_test

// Tests for Store.Load across the global and project levels: built-in (empty) <
// global < project, merged per (provider, model), each entry tagged with its level.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func loadLayered(t *testing.T, l layered) (domain.PricingTable, []domain.Finding) {
	t.Helper()
	table, findings, err := l.store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return table, findings
}

func TestLoadLayers_GlobalOnly_EntriesComeFromGlobalLevel(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("model-g", "3.00"))

	table, findings := loadLayered(t, l)

	e := mustLookup(t, table, "", "model-g")
	if e.Source != domain.LevelGlobal {
		t.Errorf("Source = %v, want global", e.Source)
	}
	if e.Standard.Input != mustRate(t, "3.00") {
		t.Errorf("input rate = %v, want 3.00", e.Standard.Input)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadLayers_ProjectOnly_EntriesComeFromProjectLevel(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, modelYAML("model-p", "4.00"))

	table, _ := loadLayered(t, l)

	e := mustLookup(t, table, "", "model-p")
	if e.Source != domain.LevelProject {
		t.Errorf("Source = %v, want project", e.Source)
	}
}

func TestLoadLayers_NoFiles_ReturnsEmptyTableWithoutErrorOrFindings(t *testing.T) {
	l := newLayered(t)

	table, findings := loadLayered(t, l)

	if table.Len() != 0 || len(findings) != 0 {
		t.Errorf("len = %d, findings = %v, want an empty table and no findings", table.Len(), findings)
	}
}

func TestLoadLayers_CommentsOnlyFilesAtBothLevels_LoadWithoutFailure(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, commentsOnlyYAML)
	writeFileAt(t, l.projFile, commentsOnlyYAML)

	table, findings := loadLayered(t, l)

	if table.Len() != 0 || len(findings) != 0 {
		t.Errorf("len = %d, findings = %v, want an empty table and no findings", table.Len(), findings)
	}
}

func TestLoadLayers_BothLevels_ProjectWinsOnSameModelGlobalKeptOtherwise(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("shared", "3.00")+modelEntry("global-only", "7.00", 2))
	writeFileAt(t, l.projFile, modelYAML("shared", "9.00")+modelEntry("project-only", "8.00", 2))

	table, _ := loadLayered(t, l)

	shared := mustLookup(t, table, "", "shared")
	if shared.Standard.Input != mustRate(t, "9.00") || shared.Source != domain.LevelProject {
		t.Errorf("shared = input %v from %v, want 9.00 from project", shared.Standard.Input, shared.Source)
	}
	if g := mustLookup(t, table, "", "global-only"); g.Source != domain.LevelGlobal || g.Standard.Input != mustRate(t, "7.00") {
		t.Errorf("global-only = input %v from %v, want 7.00 from global", g.Standard.Input, g.Source)
	}
	if p := mustLookup(t, table, "", "project-only"); p.Source != domain.LevelProject {
		t.Errorf("project-only Source = %v, want project", p.Source)
	}
	if table.Len() != 3 {
		t.Errorf("table has %d entries, want 3", table.Len())
	}
}

func TestLoadLayers_ProviderEntry_ProjectReplacesGlobalOnlyForThatProviderAndModel(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, providerModelYAML("anthropic", "m", "3.00"))
	writeFileAt(t, l.projFile, providerModelYAML("github-copilot", "m", "5.00"))

	table, _ := loadLayered(t, l)

	if a := mustLookup(t, table, "anthropic", "m"); a.Source != domain.LevelGlobal || a.Standard.Input != mustRate(t, "3.00") {
		t.Errorf("anthropic/m = %v from %v, want 3.00 from global (project did not price it)", a.Standard.Input, a.Source)
	}
	if c := mustLookup(t, table, "github-copilot", "m"); c.Source != domain.LevelProject {
		t.Errorf("github-copilot/m Source = %v, want project", c.Source)
	}
	if _, ok := table.LookupFor("", "m"); ok {
		t.Error("a provider entry must not price the provider-less model")
	}
}

func TestLoadLayers_ProviderEntryInBoth_ProjectWins(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, providerModelYAML("anthropic", "m", "3.00"))
	writeFileAt(t, l.projFile, providerModelYAML("anthropic", "m", "6.00"))

	table, _ := loadLayered(t, l)

	e := mustLookup(t, table, "anthropic", "m")
	if e.Standard.Input != mustRate(t, "6.00") || e.Source != domain.LevelProject {
		t.Errorf("anthropic/m = %v from %v, want 6.00 from project", e.Standard.Input, e.Source)
	}
}

func TestLoadLayers_InvalidProjectEntry_IsFindingAndGlobalEntryStillApplies(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("shared", "3.00"))
	writeFileAt(t, l.projFile, "models:\n  shared:\n    input: \"9.00\"\n")

	table, findings := loadLayered(t, l)

	if _, ok := findingFor(findings, "shared"); !ok {
		t.Errorf("want an invalid_pricing_entry finding naming shared, got %v", findings)
	}
	e := mustLookup(t, table, "", "shared")
	if e.Source != domain.LevelGlobal || e.Standard.Input != mustRate(t, "3.00") {
		t.Errorf("shared = %v from %v, want the global 3.00 entry", e.Standard.Input, e.Source)
	}
}

func TestLoadLayers_InvalidGlobalEntry_IsFindingAndProjectEntriesStillLoad(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, "models:\n  broken:\n    input: \"1.00\"\n")
	writeFileAt(t, l.projFile, modelYAML("good", "2.00"))

	table, findings := loadLayered(t, l)

	if _, ok := findingFor(findings, "broken"); !ok {
		t.Errorf("want an invalid_pricing_entry finding naming broken, got %v", findings)
	}
	if _, ok := table.Lookup("broken"); ok {
		t.Error("an invalid entry must not be loaded")
	}
	mustLookup(t, table, "", "good")
}

func TestLoadLayers_UnparseableGlobalFile_ErrorNamesThatFile(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, malformedYAML)
	writeFileAt(t, l.projFile, modelYAML("m", "1.00"))

	_, _, err := l.store.Load(context.Background())

	if err == nil {
		t.Fatal("Load must fail on an unparseable global document")
	}
	if !strings.Contains(err.Error(), l.globalFile) {
		t.Errorf("error %q must name the global file %q", err, l.globalFile)
	}
}

func TestLoadLayers_UnparseableProjectFile_ErrorNamesThatFile(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("m", "1.00"))
	writeFileAt(t, l.projFile, malformedYAML)

	_, _, err := l.store.Load(context.Background())

	if err == nil {
		t.Fatal("Load must fail on an unparseable project document")
	}
	if !strings.Contains(err.Error(), l.projFile) {
		t.Errorf("error %q must name the project file %q", err, l.projFile)
	}
}

func TestLoadLayers_Load_NeverCreatesEitherFile(t *testing.T) {
	l := newLayered(t)

	loadLayered(t, l)

	if exists(l.globalFile) || exists(l.projFile) {
		t.Errorf("Load must not write: global exists = %v, project exists = %v", exists(l.globalFile), exists(l.projFile))
	}
}
