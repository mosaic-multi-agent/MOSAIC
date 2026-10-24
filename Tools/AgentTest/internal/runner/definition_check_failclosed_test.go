package runner_test

// Tests that the pre-spawn check on the catalogue path fails closed: a
// deployed agent without a catalog entry, without a declared tier, with a
// tier that has no selected model, or whose key matches several catalog files
// stops the run before launch and is never given a default expectation.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// assertNamesAgentFileAndSituation checks the message content shared by every
// fail-closed situation.
func assertNamesAgentFileAndSituation(t *testing.T, got *runner.DefinitionModelError, agent, path string, problem runner.DefinitionModelProblem) {
	t.Helper()
	if got.Problem != problem {
		t.Errorf("Problem = %q, want %q", got.Problem, problem)
	}
	if got.Agent != agent {
		t.Errorf("Agent = %q, want %q", got.Agent, agent)
	}
	if got.Path != path {
		t.Errorf("Path = %q, want %q", got.Path, path)
	}
	text := got.Error()
	for _, want := range []string{agent, path, string(problem)} {
		if !strings.Contains(text, want) {
			t.Errorf("error text %q does not contain %q", text, want)
		}
	}
}

func TestFailClosed_DeployedAgentWithoutACatalogEntry(t *testing.T) {
	h := newHarness(t)
	folder := newTestCatalog(t, incidentCatalogFiles()...)
	h.useCatalog(folder)
	paths := deployKeys(h, "orchestrator", "ghost")

	got := runAborted(t, h, tierRequest("fc-no-entry"))

	assertNamesAgentFileAndSituation(t, got, "ghost", paths["ghost"], runner.DefinitionModelNoCatalogEntry)
	if got.Expected != "" || got.Tier != "" {
		t.Errorf("Tier/Expected = %q/%q, want none: no entry means nothing is known", got.Tier, got.Expected)
	}
	if !strings.Contains(got.Error(), folder) {
		t.Errorf("error text %q does not name the test catalog %q", got.Error(), folder)
	}
}

func TestFailClosed_DeployedAgentWhoseCatalogFileDeclaresNoTier(t *testing.T) {
	h := newHarness(t)
	files := append(incidentCatalogFiles(), catalogFile{"Subagents/general/tierless.md", ""})
	h.useCatalog(newTestCatalog(t, files...))
	paths := deployKeys(h, "orchestrator", "tierless")

	got := runAborted(t, h, tierRequest("fc-no-tier"))

	assertNamesAgentFileAndSituation(t, got, "tierless", paths["tierless"], runner.DefinitionModelNoDeclaredTier)
	if len(got.CatalogFiles) != 1 || !strings.HasSuffix(filepath.ToSlash(got.CatalogFiles[0]), "Subagents/general/tierless.md") {
		t.Errorf("CatalogFiles = %v, want the one tierless catalog file", got.CatalogFiles)
	}
	if !strings.Contains(got.Error(), "tierless.md") {
		t.Errorf("error text %q does not name the catalog file", got.Error())
	}
}

func TestFailClosed_TierWithoutASelectedModel(t *testing.T) {
	t.Run("tier outside the test tier vocabulary", func(t *testing.T) {
		h := newHarness(t)
		files := append(incidentCatalogFiles(), catalogFile{"Subagents/general/odd.md", "HIGH"})
		h.useCatalog(newTestCatalog(t, files...))
		paths := deployKeys(h, "orchestrator", "odd")

		got := runAborted(t, h, tierRequest("fc-odd-tier"))

		assertNamesAgentFileAndSituation(t, got, "odd", paths["odd"], runner.DefinitionModelUnselectedTier)
		if got.Tier != "HIGH" || got.Expected != "" {
			t.Errorf("Tier/Expected = %q/%q, want the declared tier and no expected model", got.Tier, got.Expected)
		}
		if !strings.Contains(got.Error(), "HIGH") {
			t.Errorf("error text %q does not name the declared tier", got.Error())
		}
	})

	t.Run("subject tier declared while no subject model is selected", func(t *testing.T) {
		h, paths := incidentHarness(t)
		req := catalogueRequestWithModels("fc-empty-subject", "", "", domain.ModelSelection{Stub: haikuModel})

		got := runAborted(t, h, req)

		assertNamesAgentFileAndSituation(t, got, "orchestrator", paths["orchestrator"], runner.DefinitionModelUnselectedTier)
		if got.Tier != domain.TierTestSubject {
			t.Errorf("Tier = %q, want %q", got.Tier, domain.TierTestSubject)
		}
	})
}

func TestFailClosed_UnconfiguredCatalogFolderOnTheCataloguePath(t *testing.T) {
	h := newHarness(t)
	h.Deps.CatalogFolder = ""

	got := runAborted(t, h, tierRequest("fc-unconfigured"))

	if got.Problem != runner.DefinitionModelNoCatalogEntry {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelNoCatalogEntry)
	}
	if got.Agent != "orchestrator" {
		t.Errorf("Agent = %q, want the first deployed agent", got.Agent)
	}
}

func TestFailClosed_KeyMatchingSeveralCatalogFiles(t *testing.T) {
	h := newHarness(t)
	files := append(incidentCatalogFiles(),
		catalogFile{"Subagents/alpha/dup.md", domain.TierTestStub},
		catalogFile{"Subagents/beta/dup.md", domain.TierTestStub},
	)
	folder := newTestCatalog(t, files...)
	h.useCatalog(folder)
	paths := deployKeys(h, "orchestrator", "dup")

	got := runAborted(t, h, tierRequest("fc-ambiguous"))

	assertNamesAgentFileAndSituation(t, got, "dup", paths["dup"], runner.DefinitionModelAmbiguousCatalogEntry)
	wantFiles := []string{
		filepath.Join(folder, "Subagents", "alpha", "dup.md"),
		filepath.Join(folder, "Subagents", "beta", "dup.md"),
	}
	if len(got.CatalogFiles) != 2 || got.CatalogFiles[0] != wantFiles[0] || got.CatalogFiles[1] != wantFiles[1] {
		t.Errorf("CatalogFiles = %v, want %v", got.CatalogFiles, wantFiles)
	}
	for _, f := range wantFiles {
		if !strings.Contains(got.Error(), f) {
			t.Errorf("error text %q does not name the conflicting file %q", got.Error(), f)
		}
	}
}

func TestFailClosed_SameBaseNameInOrchestratorAndSubagentsIsAmbiguous(t *testing.T) {
	h := newHarness(t)
	files := append(incidentCatalogFiles(), catalogFile{"Subagents/general/orchestrator.md", domain.TierTestStub})
	h.useCatalog(newTestCatalog(t, files...))
	paths := deployKeys(h, "orchestrator")

	got := runAborted(t, h, tierRequest("fc-ambiguous-orchestrator"))

	assertNamesAgentFileAndSituation(t, got, "orchestrator", paths["orchestrator"], runner.DefinitionModelAmbiguousCatalogEntry)
}

func TestFailClosed_FirstAgentWhoseTierFailsToResolveIsReportedEvenBehindADefinitionMismatch(t *testing.T) {
	h := newHarness(t)
	h.useCatalog(newTestCatalog(t, incidentCatalogFiles()...))
	paths := deployKeys(h, "orchestrator", "ghost")
	// The first agent is also wrong, but tier resolution is settled for all
	// agents before any definition is compared.
	overrideDefinition(h, "orchestrator.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)

	got := runAborted(t, h, tierRequest("fc-resolution-first"))

	assertNamesAgentFileAndSituation(t, got, "ghost", paths["ghost"], runner.DefinitionModelNoCatalogEntry)
}

func TestFailClosed_EarlierResolutionFailureOutranksALaterOne(t *testing.T) {
	h := newHarness(t)
	files := append(incidentCatalogFiles(), catalogFile{"Subagents/general/tierless.md", ""})
	h.useCatalog(newTestCatalog(t, files...))
	paths := deployKeys(h, "orchestrator", "tierless", "ghost")

	got := runAborted(t, h, tierRequest("fc-first-resolution"))

	assertNamesAgentFileAndSituation(t, got, "tierless", paths["tierless"], runner.DefinitionModelNoDeclaredTier)
}

func TestFailClosed_ConfiguredCatalogFolderThatDoesNotExistFailsBeforeLaunch(t *testing.T) {
	h := newHarness(t)
	missing := filepath.Join(t.TempDir(), "no-such-catalog")
	h.Deps.CatalogFolder = missing

	_, err := runner.Run(context.Background(), h.Deps, tierRequest("fc-missing-folder"), nil)

	if err == nil {
		t.Fatal("Run returned no error for a catalog folder that does not exist")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the catalog folder %q", err, missing)
	}
	var defErr *runner.DefinitionModelError
	if errors.As(err, &defErr) {
		t.Errorf("error = %v, want a plain catalog read error, not a *runner.DefinitionModelError", err)
	}
	for _, c := range h.Rec.all() {
		if c == "adapter.SpawnPlan" || c == "launcher.Launch" {
			t.Errorf("%s was called after a failed catalog read; calls: %v", c, h.Rec.all())
		}
	}
}

func TestCatalogFilesNoDeployedAgentUsesAreNeverConsulted(t *testing.T) {
	h := newHarness(t)
	files := append(incidentCatalogFiles(),
		catalogFile{"Subagents/README.md", ""},
		catalogFile{"Subagents/alpha/unused.md", domain.TierTestStub},
		catalogFile{"Subagents/beta/unused.md", "HIGH"},
	)
	h.useCatalog(newTestCatalog(t, files...))
	deployKeys(h, "orchestrator", "researcher")

	runProceeds(t, h, tierRequest("fc-unused-files"))
}
