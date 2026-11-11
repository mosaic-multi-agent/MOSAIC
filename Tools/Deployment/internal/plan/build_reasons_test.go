package plan_test

// build_reasons_test.go verifies that the planner gives a plan item one reason per cause: an
// update with several causes carries each as its own reason, and the legacy-rename reason
// composes with the item's other reasons instead of being joined into one string.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// reasonsMentioning returns the reasons of item that contain every one of parts.
func reasonsMentioning(item domain.PlanItem, parts ...string) []string {
	var found []string
	for _, r := range item.Reasons {
		ok := true
		for _, p := range parts {
			if !strings.Contains(r, p) {
				ok = false
			}
		}
		if ok {
			found = append(found, r)
		}
	}
	return found
}

// assertNoStalePrefix fails when a reason still carries the old combined "stale: " prefix.
func assertNoStalePrefix(t *testing.T, item domain.PlanItem) {
	t.Helper()
	for _, r := range item.Reasons {
		if strings.HasPrefix(r, "stale:") {
			t.Errorf("reason %q carries the old \"stale: \" prefix; each cause is its own reason", r)
		}
	}
}

func TestBuild_AgentVersionChangeAndCatalogChange_AreSeparateReasons(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{
		recorded:            domain.CatalogIdentity{ID: ccOldCatalogID, Version: "1.0.0"},
		workerSourceVersion: "2.0",
	}

	// Act
	p := ccStackBuild(t, ccNewCatalogID, o)

	// Assert
	item := ccItemAt(t, p, ccWorkerTarget)
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q, want update", item.Action)
	}
	if got := reasonsMentioning(item, `"1.0"`, `"2.0"`); len(got) != 1 {
		t.Errorf("reasons naming the version change = %q, want exactly one of %q", got, item.Reasons)
	}
	if got := reasonsMentioning(item, ccOldCatalogID, ccNewCatalogID); len(got) != 1 {
		t.Errorf("reasons naming the catalog change = %q, want exactly one of %q", got, item.Reasons)
	}
	if len(item.Reasons) != 2 {
		t.Errorf("Reasons = %q, want exactly the version reason and the catalog reason", item.Reasons)
	}
	if len(item.Reasons) == 2 && (!strings.Contains(item.Reasons[0], `"2.0"`) || !strings.Contains(item.Reasons[1], ccNewCatalogID)) {
		t.Errorf("Reasons = %q, want the version delta first, then the catalog change", item.Reasons)
	}
	assertNoStalePrefix(t, item)
}

func TestBuild_AgentWithSeveralVersionDeltas_GivesOneReasonPerDelta(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{
		recorded:            domain.CatalogIdentity{ID: ccOldCatalogID},
		workerSourceVersion: "2.0",
	}
	cat := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccSupplier(ccOldCatalogID)}

	// Act
	p := ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(o.recorded), func(in *plan.Input) {
		in.ToolMappingsVersion = "tm-2"
	})

	// Assert
	item := ccItemAt(t, p, ccWorkerTarget)
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q, want update", item.Action)
	}
	want := []string{
		`version changed from "1.0" to "2.0"`,
		`tool_mappings_version changed from "" to "tm-2"`,
	}
	for _, w := range want {
		found := false
		for _, r := range item.Reasons {
			if r == w {
				found = true
			}
		}
		if !found {
			t.Errorf("Reasons = %q, want the separate reason %q", item.Reasons, w)
		}
	}
	assertNoStalePrefix(t, item)
}

func TestBuild_OrchestratorWithInfrastructureAndCatalogDrift_KeepsEachAsItsOwnReason(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}}
	fake := ccFakeCatalog(o)
	fake.infraAgents = []domain.Agent{infraCatalogAgent("code-review", "1.0")}
	cat := &provenanceCatalog{fakeCatalog: fake, layers: ccSupplier(ccNewCatalogID)}

	// Act
	p := ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(o.recorded), func(in *plan.Input) {
		in.Mode = domain.ModeUpdateWorkspace
		in.ScannedAgentKeys = []string{"worker", "orchestrator", "orchestrator-script"}
		in.InfrastructureDeclarations = ensureIntent("code-review")
	})

	// Assert
	item := ccItemAt(t, p, ccOrchTarget)
	infra := reasonsMentioning(item, "code-review")
	catalogR := reasonsMentioning(item, ccOldCatalogID, ccNewCatalogID)
	if len(infra) != 1 || len(catalogR) != 1 {
		t.Fatalf("Reasons = %q, want one infrastructure reason and one catalog reason", item.Reasons)
	}
	if infra[0] == catalogR[0] {
		t.Errorf("infrastructure and catalog drift share the reason %q, want separate reasons", infra[0])
	}
}

func TestBuild_SkillVersionChangeAndCatalogChange_AreSeparateReasons(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{
		recorded:           domain.CatalogIdentity{ID: ccOldCatalogID, Version: "1.0.0"},
		skillSourceVersion: "2.0",
	}

	// Act
	p := ccStackBuild(t, ccNewCatalogID, o)

	// Assert
	item := ccItemAt(t, p, ccSkillEntryTarget)
	if got := reasonsMentioning(item, `"1.0"`, `"2.0"`); len(got) != 1 {
		t.Errorf("reasons naming the version change = %q, want exactly one of %q", got, item.Reasons)
	}
	if got := reasonsMentioning(item, ccOldCatalogID, ccNewCatalogID); len(got) != 1 {
		t.Errorf("reasons naming the catalog change = %q, want exactly one of %q", got, item.Reasons)
	}
	assertNoStalePrefix(t, item)
}

func TestBuild_HookVersionChangeAndCatalogChange_AreSeparateReasons(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{
		recorded:          domain.CatalogIdentity{ID: ccOldCatalogID, Version: "1.0.0"},
		hookSourceVersion: "2.0",
	}

	// Act
	p := ccStackBuild(t, ccNewCatalogID, o)

	// Assert
	item := ccItemAt(t, p, ccHookTarget)
	if got := reasonsMentioning(item, `"1.0"`, `"2.0"`); len(got) != 1 {
		t.Errorf("reasons naming the version change = %q, want exactly one of %q", got, item.Reasons)
	}
	if got := reasonsMentioning(item, ccOldCatalogID, ccNewCatalogID); len(got) != 1 {
		t.Errorf("reasons naming the catalog change = %q, want exactly one of %q", got, item.Reasons)
	}
	assertNoStalePrefix(t, item)
}

func TestBuild_NewAgent_HasExactlyOneReason(t *testing.T) {
	// Arrange
	item := buildParseFailedItem(t, domain.DeployedArtifactState{})

	// Assert
	if item.Action != domain.ActionCreate {
		t.Fatalf("Action = %q, want create", item.Action)
	}
	if len(item.Reasons) != 1 || item.Reasons[0] == "" {
		t.Errorf("Reasons = %q, want exactly one non-empty reason", item.Reasons)
	}
}

func TestBuild_UnchangedItems_HaveNoReasons(t *testing.T) {
	// Arrange
	o := ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}}

	// Act
	p := ccStackBuild(t, ccOldCatalogID, o)

	// Assert
	unchanged := 0
	for _, target := range ccPlannedTargets() {
		item := ccItemAt(t, p, target)
		if item.Action != domain.ActionUnchanged {
			continue
		}
		unchanged++
		if len(item.Reasons) != 0 {
			t.Errorf("%s: unchanged item has Reasons %q, want none", target, item.Reasons)
		}
	}
	if unchanged == 0 {
		t.Fatal("no planned target is unchanged; the check verified nothing")
	}
}
