package plan_test

// catalog_name_collision_test.go verifies the catalog name check: when the recorded and the
// current catalog share an id but differ in name, every item with a manifest entry (agents,
// skill files, hooks) carries a warning naming both names while its action and reasons stay
// those of the plan without the warning. A different id keeps the existing change message and
// no warning; a missing id on either side skips both checks.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	ccRecordedName = "Recorded catalog name"
	ccCurrentName  = "Current catalog name"
)

// ccNamedSupplier returns a provenance table answering the identity for every planned item.
func ccNamedSupplier(id domain.CatalogIdentity) map[domain.CatalogItem]domain.CatalogLayer {
	table := map[domain.CatalogItem]domain.CatalogLayer{}
	for _, it := range ccItems() {
		table[it] = domain.CatalogLayer{Identity: id}
	}
	return table
}

// ccNamedBuild plans the fixture with the recorded identity in every entry and the current
// identity supplying every item.
func ccNamedBuild(t *testing.T, recorded, current domain.CatalogIdentity, tweaks ...func(*plan.Input)) domain.Plan {
	t.Helper()
	o := ccCatalogOptions{recorded: recorded}
	cat := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccNamedSupplier(current)}
	return ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(recorded), tweaks...)
}

// ccWarningsText joins the warnings of an item.
func ccWarningsText(item domain.PlanItem) string { return strings.Join(item.Warnings, " | ") }

func TestBuild_CatalogNameCollision_EveryItemWarnsNamingBothNames(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName, Version: "1.0.0"}
	current := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName, Version: "1.0.0"}

	p := ccNamedBuild(t, recorded, current)

	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)
			warnings := ccWarningsText(item)

			if len(item.Warnings) != 1 {
				t.Fatalf("Warnings = %q, want exactly one", item.Warnings)
			}
			for _, want := range []string{ccRecordedName, ccCurrentName, ccNewCatalogID} {
				if !strings.Contains(warnings, want) {
					t.Errorf("Warnings = %q, want it to name %q", item.Warnings, want)
				}
			}
		})
	}
}

func TestBuild_CatalogNameCollision_PlanningIsExactlyThatOfTheSameCatalogWithoutTheWarning(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName, Version: "1.0.0"}
	current := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName, Version: "1.0.0"}
	baseline := ccNamedBuild(t, current, current)

	p := ccNamedBuild(t, recorded, current)

	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item, want := ccItemAt(t, p, target), ccItemAt(t, baseline, target)

			if item.Action != want.Action || item.Action != domain.ActionUnchanged {
				t.Errorf("Action = %q, want %q (the same as without the warning)", item.Action, domain.ActionUnchanged)
			}
			if reasonText(item) != reasonText(want) {
				t.Errorf("Reasons = %q, want %q", item.Reasons, want.Reasons)
			}
			if len(item.Stale) != 0 || item.Conflict != nil || item.BackupBeforeWrite {
				t.Errorf("Stale %+v Conflict %+v backup %v, want none: the warning changes nothing", item.Stale, item.Conflict, item.BackupBeforeWrite)
			}
		})
	}
	if len(baseline.Gaps) != len(p.Gaps) {
		t.Errorf("Gaps = %+v, want the same gaps as without the warning (%+v)", p.Gaps, baseline.Gaps)
	}
}

func TestBuild_CatalogNameCollision_KeepsAnUpdateCausedByAnotherReason(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName}
	current := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}
	o := ccCatalogOptions{recorded: recorded, workerSourceVersion: "2.0"}
	cat := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccNamedSupplier(current)}

	p := ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(recorded))

	item := ccItemAt(t, p, ccWorkerTarget)
	if item.Action != domain.ActionUpdate || !strings.Contains(reasonText(item), "2.0") {
		t.Errorf("Action %q Reasons %q, want the version update unchanged", item.Action, item.Reasons)
	}
	if strings.Contains(reasonText(item), ccCurrentName) || strings.Contains(reasonText(item), ccRecordedName) {
		t.Errorf("Reasons = %q, want the warning kept out of the reasons", item.Reasons)
	}
	if !strings.Contains(ccWarningsText(item), ccCurrentName) {
		t.Errorf("Warnings = %q, want the collision warning on the updated item", item.Warnings)
	}
}

func TestBuild_CatalogNameCollision_AlsoWarnsWhenUpdatingTheWorkspace(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName}
	current := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}

	p := ccNamedBuild(t, recorded, current, func(in *plan.Input) {
		in.Mode = domain.ModeUpdateWorkspace
		in.ScannedAgentKeys = []string{"worker", "orchestrator", "orchestrator-script"}
	})

	for _, target := range []string{ccWorkerTarget, ccOrchTarget, ccSkillEntryTarget, ccHookTarget} {
		item := ccItemAt(t, p, target)
		if !strings.Contains(ccWarningsText(item), ccRecordedName) || !strings.Contains(ccWarningsText(item), ccCurrentName) {
			t.Errorf("%s: Warnings = %q, want both names", target, item.Warnings)
		}
	}
}

func TestBuild_CatalogNameCollision_DifferentId_KeepsTheChangeMessageAndWarnsNothing(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccOldCatalogID, Name: ccRecordedName}
	current := domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}

	p := ccNamedBuild(t, recorded, current)

	want := `source catalog changed from "` + ccOldCatalogID + `" to "` + ccNewCatalogID + `"`
	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)

			if len(item.Warnings) != 0 {
				t.Errorf("Warnings = %q, want none for a different catalog id", item.Warnings)
			}
			if !hasExactReason(item, want) {
				t.Errorf("Reasons = %q, want the unchanged message %q", item.Reasons, want)
			}
		})
	}
}

func TestBuild_CatalogNameCollision_SkippedWhenEitherSideHasNoIdOrName(t *testing.T) {
	cases := []struct {
		name              string
		recorded, current domain.CatalogIdentity
	}{
		{"recorded without id", domain.CatalogIdentity{Name: ccRecordedName}, domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}},
		{"current without id", domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName}, domain.CatalogIdentity{Name: ccCurrentName}},
		{"neither has an id", domain.CatalogIdentity{Name: ccRecordedName}, domain.CatalogIdentity{Name: ccCurrentName}},
		{"recorded without name", domain.CatalogIdentity{ID: ccNewCatalogID}, domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}},
		{"current without name", domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccRecordedName}, domain.CatalogIdentity{ID: ccNewCatalogID}},
		{"same id and same name", domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName}, domain.CatalogIdentity{ID: ccNewCatalogID, Name: ccCurrentName, Version: "9.0.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := ccNamedBuild(t, tc.recorded, tc.current)

			for _, target := range ccPlannedTargets() {
				item := ccItemAt(t, p, target)
				if len(item.Warnings) != 0 {
					t.Errorf("%s: Warnings = %q, want none", target, item.Warnings)
				}
				if item.Action != domain.ActionUnchanged || len(item.Stale) != 0 {
					t.Errorf("%s: Action %q Stale %+v, want the unchanged outcome", target, item.Action, item.Stale)
				}
			}
		})
	}
}
