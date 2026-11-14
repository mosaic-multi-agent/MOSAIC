package app_test

// edit_detection_no_record_test.go specifies, flow by flow and for a Markdown harness and for
// Codex alike, how Update treats a deployed agent that no usable manifest record vouches for:
//
//   - the user edited the file and there is no manifest, an unusable manifest or no entry at
//     the target path: the item is a conflict, no "edited locally" reason is given, and the
//     frontmatter differences are reported in RunSummary.OwnedKeyDifferences;
//   - the deployed agent cannot be parsed or decoded: the item is a conflict with a parse
//     reason, and neither a record comparison nor owned-key differences are produced;
//   - a manifest entry written before records existed (no owned content) causes no edit check.
//
// It also pins the shape of the owned-key differences and that none are reported while a record
// vouches for the file.

import (
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// nrParseReason is the fixed prefix of the plan reason for a deployed agent that cannot be read.
const nrParseReason = "deployed file could not be read/parsed"

// nrEditedDescription is the hand-edited frontmatter value the scenarios write.
const nrEditedDescription = "hand edited description"

// nrCatalogDescription is the catalog description of the runner agent that an update writes.
const nrCatalogDescription = "Executes tests and reports results for the parity sample catalog"

// nrDiffReason is the fixed reason prefix of an owned-key difference without a record.
const nrDiffReason = "origin could not be confirmed"

// diffKeys are the frontmatter keys that differ for a hand-edited description when no record
// vouches for the file: Codex also reports the agent name, which it never writes back.
func (h owHarness) diffKeys() []string {
	if h.formatID != "" {
		return []string{"description", "name"}
	}
	return []string{"description"}
}

// nrNoRecord prepares the manifest of a deployed workspace so that no usable record vouches for
// the runner agent.
type nrNoRecord struct {
	name  string
	apply func(t *testing.T, h owHarness, ws string)
}

var nrNoRecordCases = []nrNoRecord{
	{"NoManifest", func(t *testing.T, h owHarness, ws string) {
		gamRemoveManifest(t, ws)
	}},
	{"UnusableManifest", func(t *testing.T, h owHarness, ws string) {
		gamWrite(t, ws, manifest.Dir+"/"+manifest.FileName, "{{{ this is : [not a manifest")
	}},
	{"NoEntryAtTargetPath", func(t *testing.T, h owHarness, ws string) {
		gamDropEntry(t, ws, owRunner, h.rel(owRunner))
	}},
}

// gamRemoveManifest deletes the manifest of a workspace.
func gamRemoveManifest(t *testing.T, ws string) {
	t.Helper()
	if err := os.Remove(gamAbs(ws, manifest.Dir+"/"+manifest.FileName)); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}
}

// nrRun deploys, runs setup on the workspace and runs Update with the given conflict decision.
func nrRun(t *testing.T, h owHarness, decision domain.ConflictDecision, setup func(ws string) []byte) (owScenario, string) {
	t.Helper()
	ws := owDeploy(t, h)
	edited := setup(ws)
	spy := &owSpy{}
	sum, err := owFlowNamed(t, "Update").run(t, owService(t, spy), h, ws, owFlowOpts{conflict: decision})
	if err != nil {
		t.Fatalf("Update on %s: %v", h.name, err)
	}
	return owScenario{
		item:    owPlanItem(t, spy.lastPlan(t), owRunner),
		sum:     sum,
		backups: gamBackups(t, ws, h.rel(owRunner)),
		edited:  edited,
	}, ws
}

// requireNoEditReason fails when the item carries a Deploy-owned content edit reason.
func (s owScenario) requireNoEditReason(t *testing.T) {
	t.Helper()
	for _, r := range s.item.Reasons {
		if strings.HasPrefix(r, owEditReason) {
			t.Errorf("reasons = %q; want no %q reason", s.item.Reasons, owEditReason)
		}
	}
}

func TestEditDetection_NoUsableRecordAndUserEdit_IsAConflict(t *testing.T) {
	for _, h := range owHarnesses {
		for _, c := range nrNoRecordCases {
			t.Run(h.name+"/"+c.name, func(t *testing.T) {
				res, _ := nrRun(t, h, domain.DecisionOverwrite, func(ws string) []byte {
					edited := owEditFrontmatter(t, h, ws, owRunner, "description", nrEditedDescription)
					c.apply(t, h, ws)
					return edited
				})

				if res.item.Action != domain.ActionConflict {
					t.Errorf("action = %s; want %s for a user-edited file no record vouches for", res.item.Action, domain.ActionConflict)
				}
				if res.item.Conflict == nil {
					t.Error("item.Conflict = nil; want the local modification of the conflict question")
				}
				res.requireNoEditReason(t)
			})
		}
	}
}

func TestEditDetection_NoUsableRecordAndUserEdit_ReportsFrontmatterDifferences(t *testing.T) {
	for _, h := range owHarnesses {
		for _, c := range nrNoRecordCases {
			t.Run(h.name+"/"+c.name, func(t *testing.T) {
				res, _ := nrRun(t, h, domain.DecisionOverwrite, func(ws string) []byte {
					edited := owEditFrontmatter(t, h, ws, owRunner, "description", nrEditedDescription)
					c.apply(t, h, ws)
					return edited
				})

				diffs := res.sum.OwnedKeyDifferences
				entry, ok := findOwnedKeyEntry(diffs, "description")
				if !ok {
					t.Fatalf("no OwnedKeyDifference for \"description\"; got %v", diffs)
				}
				if !entry.DeployedPresent || entry.Deployed != nrEditedDescription {
					t.Errorf("deployed side = (%v, %q); want the hand-edited value %q", entry.DeployedPresent, entry.Deployed, nrEditedDescription)
				}
				if !entry.IncomingPresent || entry.Incoming != nrCatalogDescription {
					t.Errorf("incoming side = (%v, %q); want the catalog value %q", entry.IncomingPresent, entry.Incoming, nrCatalogDescription)
				}
				nrRequireKeySet(t, diffs, h.diffKeys())
				for _, d := range diffs {
					if !strings.HasPrefix(d.Reason, nrDiffReason) || !d.DeployedPresent {
						t.Errorf("difference %+v; want a deployed value and a reason starting %q", d, nrDiffReason)
					}
				}
				if h.formatID != "" {
					nrRequireCodexNameDifference(t, diffs)
				}
			})
		}
	}
}

// nrRequireKeySet fails unless the distinct keys of diffs are exactly want.
func nrRequireKeySet(t *testing.T, diffs []domain.OwnedKeyDifference, want []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, d := range diffs {
		seen[d.Key] = true
	}
	if len(seen) != len(want) {
		t.Errorf("differing keys = %v; want exactly %v", seen, want)
		return
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("differing keys = %v; want exactly %v", seen, want)
		}
	}
}

// nrRequireCodexNameDifference fails unless a Codex "name" difference for the runner reports the
// deployed name with no incoming value.
func nrRequireCodexNameDifference(t *testing.T, diffs []domain.OwnedKeyDifference) {
	t.Helper()
	for _, d := range diffs {
		if d.Key == "name" && d.Deployed == owRunner {
			if d.IncomingPresent || d.Incoming != "" {
				t.Errorf("name difference %+v; want no incoming value", d)
			}
			return
		}
	}
	t.Errorf("no \"name\" difference for %s in %v", owRunner, diffs)
}

func TestEditDetection_NoUsableRecordAndUserEdit_SkipLeavesFileUntouched(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, ws := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte {
				edited := owEditFrontmatter(t, h, ws, owRunner, "description", nrEditedDescription)
				gamDropEntry(t, ws, owRunner, h.rel(owRunner))
				return edited
			})

			if string(gamRead(t, ws, h.rel(owRunner))) != string(res.edited) {
				t.Error("the skipped conflict changed the user's file")
			}
			if len(res.backups) != 0 {
				t.Errorf("backups taken = %d; want none for a skipped conflict", len(res.backups))
			}
			res.requireNoEditReason(t)
		})
	}
}

// nrCorrupt replaces the deployed runner with a file its format cannot read as an agent.
func nrCorrupt(t *testing.T, h owHarness, ws string) []byte {
	t.Helper()
	bad := "---\nmosaic_id: [unclosed\ndescription: : :\n---\nbody\n"
	if h.formatID != "" {
		bad = "# mosaic_id: " + owRunner + "\nname = \"a\"\nname = \"b\"\n"
	}
	gamWrite(t, ws, h.rel(owRunner), bad)
	return gamRead(t, ws, h.rel(owRunner))
}

func TestEditDetection_UnparseableDeployedAgent_IsAConflictWithParseReason(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, _ := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte { return nrCorrupt(t, h, ws) })

			if res.item.Action != domain.ActionConflict {
				t.Errorf("action = %s; want %s", res.item.Action, domain.ActionConflict)
			}
			found := false
			for _, r := range res.item.Reasons {
				found = found || strings.HasPrefix(r, nrParseReason)
			}
			if !found {
				t.Errorf("reasons = %q; want one starting %q", res.item.Reasons, nrParseReason)
			}
			res.requireNoEditReason(t)
		})
	}
}

// TestEditDetection_UnparseableCodexAgent_IsAConflictOnEveryProbePath covers the probe paths on
// which no manifest entry claims the target path, in addition to the id-resolved path above.
func TestEditDetection_UnparseableCodexAgent_IsAConflictOnEveryProbePath(t *testing.T) {
	h := owHarnesses[1]
	if h.formatID == "" {
		t.Fatal("owHarnesses[1] is not the Codex harness")
	}
	for _, c := range nrNoRecordCases {
		t.Run(c.name, func(t *testing.T) {
			res, _ := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte {
				c.apply(t, h, ws)
				return nrCorrupt(t, h, ws)
			})

			if res.item.Action != domain.ActionConflict {
				t.Errorf("action = %s; want %s", res.item.Action, domain.ActionConflict)
			}
			found := false
			for _, r := range res.item.Reasons {
				found = found || strings.HasPrefix(r, nrParseReason)
			}
			if !found {
				t.Errorf("reasons = %q; want one starting %q", res.item.Reasons, nrParseReason)
			}
			if len(res.sum.OwnedKeyDifferences) != 0 {
				t.Errorf("OwnedKeyDifferences = %v; want none for a file that could not be decoded", res.sum.OwnedKeyDifferences)
			}
		})
	}
}

func TestEditDetection_UnparseableDeployedAgent_ReportsNoOwnedKeyDifferences(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, _ := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte { return nrCorrupt(t, h, ws) })

			if len(res.sum.OwnedKeyDifferences) != 0 {
				t.Errorf("OwnedKeyDifferences = %v; want none for a file that could not be decoded", res.sum.OwnedKeyDifferences)
			}
		})
	}
}

func TestEditDetection_UnparseableDeployedAgent_SkipLeavesFileUntouched(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, ws := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte { return nrCorrupt(t, h, ws) })

			if string(gamRead(t, ws, h.rel(owRunner))) != string(res.edited) {
				t.Error("the skipped conflict changed the unparseable file")
			}
			if len(res.backups) != 0 {
				t.Errorf("backups taken = %d; want none", len(res.backups))
			}
		})
	}
}

// nrDropRecords turns every agent entry into one written before records existed.
func nrDropRecords(t *testing.T, ws string) {
	t.Helper()
	m := gamManifest(t, ws)
	for i := range m.Entries {
		m.Entries[i].Owned = nil
	}
	gamSaveManifest(t, ws, m)
}

func TestEditDetection_EntryWithoutRecord_BodyEditCausesNoEditCheck(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, _ := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte {
				edited := owEditBody(t, h, ws, owRunner, owOutsideEdit(t))
				nrDropRecords(t, ws)
				return edited
			})

			res.requireNoEdit(t)
		})
	}
}

func TestEditDetection_EntryWithoutRecord_FrontmatterEditCausesNoEditCheck(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res, _ := nrRun(t, h, domain.DecisionSkip, func(ws string) []byte {
				edited := owEditFrontmatter(t, h, ws, owRunner, "description", nrEditedDescription)
				nrDropRecords(t, ws)
				return edited
			})

			res.requireNoEdit(t)
		})
	}
}

func TestEditDetection_RecordPresent_FrontmatterEditReportsNoOwnedKeyDifferences(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res := owRunScenario(t, h, func(ws string) []byte {
				return owEditFrontmatter(t, h, ws, owRunner, "description", nrEditedDescription)
			})

			if len(res.sum.OwnedKeyDifferences) != 0 {
				t.Errorf("OwnedKeyDifferences = %v; want none while a record vouches for the file", res.sum.OwnedKeyDifferences)
			}
		})
	}
}
