package plan_test

// catalog_staleness_test.go verifies that an item whose recorded catalog differs from the catalog
// supplying it now is planned as an update, for every kind of item with a manifest entry:
//
//   - Agents of every role, including both orchestrator files, every file of a skill (entry and
//     non-entry) and hook bundles become updates whose reason names both catalog ids.
//   - The same catalog id stays unchanged, whatever its name or version.
//   - An unknown recorded catalog, a catalog that reports no provenance and a catalog that does
//     not know the item all leave the version-based outcome untouched, without any gap.
//   - A locally modified hook stays a conflict; items without a deployed file or manifest entry
//     keep their create and conflict outcomes.
//   - The catalog change composes with a version change into one update item.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestBuild_CatalogChanged_EveryItemIsUpdatedNamingBothCatalogs verifies agents (both
// orchestrator files too), both files of a skill and a hook become updates when the catalog that
// supplies them is not the recorded one.
func TestBuild_CatalogChanged_EveryItemIsUpdatedNamingBothCatalogs(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID, Version: "1.0.0"}})

	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)

			if item.Action != domain.ActionUpdate {
				t.Fatalf("Action = %q, want %q (reason %q)", item.Action, domain.ActionUpdate, item.Reason)
			}
			if !strings.HasPrefix(item.Reason, "stale: ") {
				t.Errorf("Reason = %q, want it to start with \"stale: \"", item.Reason)
			}
			for _, id := range []string{ccOldCatalogID, ccNewCatalogID} {
				if !strings.Contains(item.Reason, id) {
					t.Errorf("Reason = %q, want it to name catalog %q", item.Reason, id)
				}
			}
		})
	}
}

// TestBuild_CatalogChanged_DeltaCarriesBothCatalogIds verifies the catalog delta records the
// recorded id as deployed and the current id as source, as the last delta of the item.
func TestBuild_CatalogChanged_DeltaCarriesBothCatalogIds(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}})

	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)

			d, ok := ccCatalogDelta(item)
			if !ok {
				t.Fatalf("Stale = %+v, want a %q delta", item.Stale, ccCatalogDeltaField)
			}
			if d.Deployed != ccOldCatalogID || d.Source != ccNewCatalogID {
				t.Errorf("catalog delta = %+v, want Deployed %q and Source %q", d, ccOldCatalogID, ccNewCatalogID)
			}
			if last := item.Stale[len(item.Stale)-1]; last.Field != ccCatalogDeltaField {
				t.Errorf("last delta = %+v, want the catalog delta to come after every other delta", last)
			}
		})
	}
}

// TestBuild_CatalogChanged_NonEntrySkillFile_OnlyTheCatalogDelta verifies a non-entry skill file
// skips version staleness but not the catalog check: a skill version bump leaves its delta list
// at the catalog delta alone.
func TestBuild_CatalogChanged_NonEntrySkillFile_OnlyTheCatalogDelta(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{
		recorded:           domain.CatalogIdentity{ID: ccOldCatalogID},
		skillSourceVersion: "2.0",
	})

	item := ccItemAt(t, p, ccSkillCompanionPath)

	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	if len(item.Stale) != 1 || item.Stale[0].Field != ccCatalogDeltaField {
		t.Errorf("Stale = %+v, want exactly the catalog delta", item.Stale)
	}
}

// TestBuild_CatalogChanged_ComposesWithVersionDeltaIntoOneItem verifies a catalog change and a
// version change on the same agent yield one update item carrying both deltas, with the catalog
// delta last and the catalog ids named once in the reason.
func TestBuild_CatalogChanged_ComposesWithVersionDeltaIntoOneItem(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{
		recorded:            domain.CatalogIdentity{ID: ccOldCatalogID},
		workerSourceVersion: "2.0",
	})

	var workerItems int
	for _, it := range p.Items {
		if it.TargetPath == ccWorkerTarget {
			workerItems++
		}
	}
	item := ccItemAt(t, p, ccWorkerTarget)

	if workerItems != 1 {
		t.Errorf("plan has %d items for the worker, want 1", workerItems)
	}
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	var sawVersion bool
	for _, d := range item.Stale {
		if d.Field == "version" && d.Deployed == "1.0" && d.Source == "2.0" {
			sawVersion = true
		}
	}
	if !sawVersion {
		t.Errorf("Stale = %+v, want the version delta 1.0 -> 2.0", item.Stale)
	}
	if last := item.Stale[len(item.Stale)-1]; last.Field != ccCatalogDeltaField {
		t.Errorf("last delta = %+v, want the catalog delta", last)
	}
	if !strings.Contains(item.Reason, "2.0") {
		t.Errorf("Reason = %q, want it to keep the version part", item.Reason)
	}
	for _, id := range []string{ccOldCatalogID, ccNewCatalogID} {
		if n := strings.Count(item.Reason, id); n != 1 {
			t.Errorf("Reason = %q names catalog %q %d times, want once (the catalog delta is not part of the generic version part)",
				item.Reason, id, n)
		}
	}
}

// TestBuild_CatalogChanged_HookAndSkillEntryWithVersionBump_CatalogDeltaLast verifies the catalog
// delta also follows the version delta of a hook and of a skill entry file.
func TestBuild_CatalogChanged_HookAndSkillEntryWithVersionBump_CatalogDeltaLast(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{
		recorded:           domain.CatalogIdentity{ID: ccOldCatalogID},
		hookSourceVersion:  "2.0",
		skillSourceVersion: "2.0",
	})

	for _, target := range []string{ccHookTarget, ccSkillEntryTarget} {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)

			if item.Action != domain.ActionUpdate {
				t.Fatalf("Action = %q, want %q", item.Action, domain.ActionUpdate)
			}
			if len(item.Stale) != 2 || item.Stale[0].Field != "version" || item.Stale[1].Field != ccCatalogDeltaField {
				t.Errorf("Stale = %+v, want the version delta followed by the catalog delta", item.Stale)
			}
			for _, id := range []string{ccOldCatalogID, ccNewCatalogID} {
				if !strings.Contains(item.Reason, id) {
					t.Errorf("Reason = %q, want it to name catalog %q", item.Reason, id)
				}
			}
		})
	}
}

// TestBuild_SameCatalogId_StaysUnchanged verifies an item recorded under the catalog id that
// supplies it now stays unchanged, even when the catalog's name and version differ.
func TestBuild_SameCatalogId_StaysUnchanged(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: ccNewCatalogID, Name: "Older name", Version: "0.1.0"}
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{recorded: recorded})

	for _, target := range ccPlannedTargets() {
		t.Run(target, func(t *testing.T) {
			item := ccItemAt(t, p, target)

			if item.Action != domain.ActionUnchanged {
				t.Errorf("Action = %q (reason %q), want %q", item.Action, item.Reason, domain.ActionUnchanged)
			}
		})
	}
}

// TestBuild_UnknownCatalog_LeavesVersionBasedOutcomeUnchanged verifies that when the recorded
// catalog is unknown, or the supplier is unknown (a catalog that is not a stack, one that
// reports no provenance, one that does not know the item), the plan equals the plan of a
// workspace whose catalogs agree: nothing is stale, no gap is added.
func TestBuild_UnknownCatalog_LeavesVersionBasedOutcomeUnchanged(t *testing.T) {
	known := domain.CatalogIdentity{ID: ccOldCatalogID}
	cases := []struct {
		name  string
		build func(t *testing.T) domain.Plan
	}{
		{"recorded catalog unknown", func(t *testing.T) domain.Plan {
			return ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{})
		}},
		{"catalog is not a stack", func(t *testing.T) domain.Plan {
			o := ccCatalogOptions{recorded: known}
			return ccBuild(t, ccFakeCatalog(o), o)
		}},
		{"catalog reports no provenance for any item", func(t *testing.T) domain.Plan {
			o := ccCatalogOptions{recorded: known}
			return ccBuild(t, &provenanceCatalog{fakeCatalog: ccFakeCatalog(o)}, o)
		}},
		{"catalog does not know the item", func(t *testing.T) domain.Plan {
			o := ccCatalogOptions{recorded: known}
			other := map[domain.CatalogItem]domain.CatalogLayer{
				{Kind: domain.CatalogItemAgent, Key: "unrelated"}: {Identity: domain.CatalogIdentity{ID: ccNewCatalogID}},
			}
			return ccBuild(t, &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: other}, o)
		}},
	}
	baseline := ccStackBuild(t, ccOldCatalogID, ccCatalogOptions{recorded: known})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.build(t)

			for _, target := range ccPlannedTargets() {
				item := ccItemAt(t, p, target)
				want := ccItemAt(t, baseline, target)
				if item.Action != want.Action || item.Reason != want.Reason || len(item.Stale) != 0 || item.Conflict != nil {
					t.Errorf("%s: Action %q Reason %q Stale %+v Conflict %+v, want the unchanged outcome %q",
						target, item.Action, item.Reason, item.Stale, item.Conflict, want.Action)
				}
			}
			if len(p.Gaps) != len(baseline.Gaps) {
				t.Errorf("Gaps = %+v, want the same gaps as the unchanged plan (%+v)", p.Gaps, baseline.Gaps)
			}
		})
	}
}

// TestBuild_UnknownCatalog_KeepsVersionStaleness verifies the version rules still decide when the
// catalog is unknown: a version bump is an update without a catalog delta.
func TestBuild_UnknownCatalog_KeepsVersionStaleness(t *testing.T) {
	o := ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}, workerSourceVersion: "2.0"}

	p := ccBuild(t, ccFakeCatalog(o), o)

	item := ccItemAt(t, p, ccWorkerTarget)
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	if _, has := ccCatalogDelta(item); has {
		t.Errorf("Stale = %+v, want no catalog delta for a catalog without provenance", item.Stale)
	}
}

// TestBuild_CatalogChanged_LocallyModifiedHook_StaysConflict verifies the content-hash check of
// hooks still comes first: a modified hook is a conflict, not a catalog update.
func TestBuild_CatalogChanged_LocallyModifiedHook_StaysConflict(t *testing.T) {
	p := ccStackBuild(t, ccNewCatalogID, ccCatalogOptions{
		recorded:         domain.CatalogIdentity{ID: ccOldCatalogID},
		hookDeployedHash: "sha256:locally-modified",
	})

	item := ccItemAt(t, p, ccHookTarget)

	if item.Action != domain.ActionConflict || item.Conflict == nil {
		t.Errorf("Action %q Conflict %+v, want a conflict for a locally modified hook", item.Action, item.Conflict)
	}
}

// TestBuild_CatalogChanged_MissingFileOrEntry_KeepsCreateAndConflict verifies the catalog check
// does not apply to an item with no deployed file (create) or no manifest entry (conflict).
func TestBuild_CatalogChanged_MissingFileOrEntry_KeepsCreateAndConflict(t *testing.T) {
	o := ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}}
	cat := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccSupplier(ccNewCatalogID)}
	states := ccDeployedStates(o)
	delete(states, ccWorkerTarget) // not deployed
	// The orchestrator file is deployed but no manifest entry names its target.
	entries := ccManifestEntries(o.recorded)
	kept := entries[:0]
	for _, e := range entries {
		if e.TargetPath != ccOrchTarget {
			kept = append(kept, e)
		}
	}

	p := ccBuildWith(t, cat, states, kept)

	if got := ccItemAt(t, p, ccWorkerTarget); got.Action != domain.ActionCreate {
		t.Errorf("undeployed worker: Action = %q, want %q", got.Action, domain.ActionCreate)
	}
	if got := ccItemAt(t, p, ccOrchTarget); got.Action != domain.ActionConflict {
		t.Errorf("orchestrator without manifest entry: Action = %q, want %q", got.Action, domain.ActionConflict)
	}
}
