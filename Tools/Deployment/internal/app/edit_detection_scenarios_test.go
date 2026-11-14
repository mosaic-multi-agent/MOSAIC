package app_test

// edit_detection_scenarios_test.go specifies, flow by flow and for a Markdown harness and for
// Codex alike, how Update treats a hand edit of a deployed agent whose manifest entry carries a
// Deploy-owned content record: frontmatter edits and project or custom region edits are not
// edits of Deploy-owned content (no reason, no backup), while an edit of the body outside
// regions or of a Deploy-owned region is reported, naming the region, and backed up before the
// update. A manifest written before the record moved to the canonical form (Markdown records
// over the raw bytes) causes no false edit.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

// owEditReason is the fixed reason prefix of an edit of Deploy-owned content.
const owEditReason = "Deploy-owned content was edited locally"

// owScenario is the outcome of one Update over a workspace holding one hand edit.
type owScenario struct {
	item    domain.PlanItem
	sum     domain.RunSummary
	backups [][]byte
	edited  []byte
}

// owRunScenario deploys, applies edit to the workspace and runs Update (conflicts skipped).
func owRunScenario(t *testing.T, h owHarness, edit func(ws string) []byte) owScenario {
	t.Helper()
	ws := owDeploy(t, h)
	edited := edit(ws)
	spy := &owSpy{}
	sum, err := owFlowNamed(t, "Update").run(t, owService(t, spy), h, ws, owFlowOpts{})
	if err != nil {
		t.Fatalf("Update after the hand edit: %v", err)
	}
	return owScenario{
		item:    owPlanItem(t, spy.lastPlan(t), owRunner),
		sum:     sum,
		backups: gamBackups(t, ws, h.rel(owRunner)),
		edited:  edited,
	}
}

// requireNoEdit asserts the scenario produced no edit of Deploy-owned content.
func (s owScenario) requireNoEdit(t *testing.T) {
	t.Helper()
	for _, r := range s.item.Reasons {
		if strings.HasPrefix(r, owEditReason) {
			t.Errorf("reasons = %q; want no %q reason", s.item.Reasons, owEditReason)
		}
	}
	if s.item.Action == domain.ActionConflict {
		t.Errorf("action = %s; a file the manifest record vouches for must not become a conflict", s.item.Action)
	}
	if s.item.BackupBeforeWrite || len(s.backups) != 0 {
		t.Errorf("BackupBeforeWrite = %v, backups taken = %d; want no backup", s.item.BackupBeforeWrite, len(s.backups))
	}
	if len(s.sum.OwnedKeyDifferences) != 0 {
		t.Errorf("OwnedKeyDifferences = %v; want none", s.sum.OwnedKeyDifferences)
	}
}

// requireEdit asserts the scenario reported exactly the given edit reason and backed the file up.
func (s owScenario) requireEdit(t *testing.T, wantReason string) {
	t.Helper()
	found := false
	for _, r := range s.item.Reasons {
		found = found || r == wantReason
	}
	if !found {
		t.Errorf("reasons = %q; want %q", s.item.Reasons, wantReason)
	}
	if s.item.Action != domain.ActionUpdate {
		t.Errorf("action = %s; want %s", s.item.Action, domain.ActionUpdate)
	}
	if !s.item.BackupBeforeWrite {
		t.Error("BackupBeforeWrite = false; want the edited file backed up before the update")
	}
	if len(s.backups) != 1 || string(s.backups[0]) != string(s.edited) {
		t.Errorf("backups = %d; want one backup holding the edited file", len(s.backups))
	}
	if len(s.sum.OwnedKeyDifferences) != 0 {
		t.Errorf("OwnedKeyDifferences = %v; want none", s.sum.OwnedKeyDifferences)
	}
}

func TestEditDetection_FrontmatterKeyEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	fields := []struct{ field, value string }{
		{"model", "edited-model"},
		{"sandbox_mode", "read-only"},
		{"user_key", "keep-me"},
	}
	for _, h := range owHarnesses {
		for _, f := range fields {
			t.Run(h.name+"/"+f.field, func(t *testing.T) {
				res := owRunScenario(t, h, func(ws string) []byte {
					return owEditFrontmatter(t, h, ws, owRunner, f.field, f.value)
				})

				res.requireNoEdit(t)
			})
		}
	}
}

func TestEditDetection_BodyEditOutsideRegions_IsReportedAndBackedUp(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res := owRunScenario(t, h, func(ws string) []byte {
				return owEditBody(t, h, ws, owRunner, owOutsideEdit(t))
			})

			res.requireEdit(t, owEditReason+" (regions: "+domain.OutsideRegionsName+")")
		})
	}
}

func TestEditDetection_DeployOwnedRegionEdit_NamesTheRegionAndIsBackedUp(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res := owRunScenario(t, h, func(ws string) []byte {
				return owEditBody(t, h, ws, owRunner, owIdentityEdit(t))
			})

			res.requireEdit(t, owEditReason+" (regions: Identity)")
		})
	}
}

func TestEditDetection_ProjectRegionEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res := owRunScenario(t, h, func(ws string) []byte {
				return owEditBody(t, h, ws, owRunner, owProjectEdit(t))
			})

			res.requireNoEdit(t)
		})
	}
}

func TestEditDetection_CustomRegionEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			res := owRunScenario(t, h, func(ws string) []byte {
				return owEditBody(t, h, ws, owRunner, owCustomEdit(t))
			})

			res.requireNoEdit(t)
		})
	}
}

// TestEditDetection_IntegrationEraMarkdownManifest_CausesNoFalseEdit seeds the manifest with
// the records of the integration build, which hashed the raw file bytes. For Markdown the
// canonical form is the raw file, so those records stay valid and an untouched file is not
// reported as edited.
func TestEditDetection_IntegrationEraMarkdownManifest_CausesNoFalseEdit(t *testing.T) {
	h := owHarnesses[0]
	ws := owDeploy(t, h)
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		legacy, err := transform.OwnedContentOf(domain.ArtifactAgent, gamRead(t, ws, e.TargetPath))
		if err != nil {
			t.Fatalf("raw-basis record of %s: %v", e.TargetPath, err)
		}
		m.Entries[i].Owned = &legacy
		m.Entries[i].ContentHash = manifest.Hash(gamRead(t, ws, e.TargetPath))
	}
	gamSaveManifest(t, ws, m)

	spy, sum := owUpdate(t, h, ws, false)

	for _, it := range spy.lastPlan(t).Items {
		for _, r := range it.Reasons {
			if strings.HasPrefix(r, owEditReason) {
				t.Errorf("%s: reason %q; an unmodified file must not be reported as edited", it.Ref.Key, r)
			}
		}
		if it.BackupBeforeWrite {
			t.Errorf("%s: BackupBeforeWrite set for an unmodified file", it.Ref.Key)
		}
	}
	if len(sum.OwnedKeyDifferences) != 0 {
		t.Errorf("OwnedKeyDifferences = %v; want none", sum.OwnedKeyDifferences)
	}
}
