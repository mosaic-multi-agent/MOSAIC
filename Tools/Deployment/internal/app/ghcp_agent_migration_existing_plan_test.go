package app_test

// ghcp_agent_migration_existing_plan_test.go verifies the plan the GHCP agent-name migration
// leaves in the four agent-writing flows on both targets: an X.agent.md that exists keeps the
// state it would have without the migration (unchanged, outdated, edited or conflicting), the
// migration adds its removal and its reason without replacing anything, and a recorded X.md whose
// file is missing triggers nothing.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestGhcpFlows_ExistingAgentFile_KeepsItsOwnStateInThePlan covers every flow and state of an
// X.agent.md beside a recorded, present X.md.
func TestGhcpFlows_ExistingAgentFile_KeepsItsOwnStateInThePlan(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			for _, st := range gmeStates() {
				t.Run(h+"/"+flow.name+"/"+st.name, func(t *testing.T) {
					// Arrange
					subject := flow.renamed[0]
					ws := gmeWorkspace(t, h)
					st.prepare(t, ws, subject)
					c := gamNewReviewCapture(ws)

					// Act
					err := flow.run(t, ws, h, c, true)

					// Assert
					if err != nil {
						t.Fatalf("%s for %s: %v", flow.name, h, err)
					}
					p := gplReviewedPlan(t, c)
					gmeAssertPlanItem(t, p, subject, st)
					gmeAssertOthersUnchanged(t, p, flow.renamed[1:])
					gmeAssertReplacementsMeanWrites(t, p)
				})
			}
		}
	}
}

// TestGhcpFlows_ExistingAgentFile_IsNotShownAsNewOrRenamed verifies no agent item of a plan over
// an up-to-date X.agent.md is a create or a rename, and that nothing is listed as a replacement.
func TestGhcpFlows_ExistingAgentFile_IsNotShownAsNewOrRenamed(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws := gmeWorkspace(t, h)
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				gplAssertNoNewAgent(t, p)
				if len(p.Replacements) != 0 {
					t.Errorf("replacements = %+v, want none for agents that are up to date", p.Replacements)
				}
				for _, it := range p.Items {
					if it.RenamedFrom != "" || strings.Contains(reasonText(it), "renamed from") {
						t.Errorf("%s: shown as a rename (%q) although its X.agent.md exists", it.TargetPath, reasonText(it))
					}
				}
			})
		}
	}
}

// TestGhcpFlows_RecordedLegacyFileMissing_TriggersNoMigration verifies a manifest entry for an
// X.md whose file is gone adds no removal, replacement or migration reason, and that a real run
// neither rewrites nor backs up the existing X.agent.md files.
func TestGhcpFlows_RecordedLegacyFileMissing_TriggersNoMigration(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws := parityDeploy(t, h)
				before := map[string]string{}
				for _, a := range parityAgents {
					gamRecordAgentAt(t, ws, a.key, gamRel(a.key, ".md"), "a file that is gone\n")
					before[a.key] = string(gamRead(t, ws, gamRel(a.key, ".agent.md")))
				}
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				if len(p.Removals) != 0 || len(p.Replacements) != 0 {
					t.Errorf("removals = %+v, replacements = %+v, want none: no legacy file exists", p.Removals, p.Replacements)
				}
				for _, key := range flow.renamed {
					it, ok := gplAgentItem(p, key)
					if !ok {
						t.Errorf("%s: plan has no item", key)
						continue
					}
					if it.Action != domain.ActionUnchanged || strings.Contains(reasonText(it), gmeMigrationReasonPrefix) || it.RenamedFrom != "" {
						t.Errorf("%s: action %q, reasons %q, renamed from %q, want an unchanged agent with no migration", key, it.Action, it.Reasons, it.RenamedFrom)
					}
				}
				gmeAssertNothingWritten(t, ws, flow, h, before)
			})
		}
	}
}

// gmeAssertNothingWritten runs the flow for real and checks no agent file changed and no backup
// of an agent or of a legacy file was made.
func gmeAssertNothingWritten(t *testing.T, ws string, flow gplFlow, h string, before map[string]string) {
	t.Helper()
	if err := flow.run(t, ws, h, gamNewReviewCapture(ws), false); err != nil {
		t.Fatalf("%s real run for %s: %v", flow.name, h, err)
	}
	for key, want := range before {
		if got := string(gamRead(t, ws, gamRel(key, ".agent.md"))); got != want {
			t.Errorf("%s: the run rewrote an up-to-date agent:\n%s", key, parityFirstDiff(want, got))
		}
		for _, ext := range []string{".md", ".agent.md"} {
			if n := len(gamBackups(t, ws, gamRel(key, ext))); n != 0 {
				t.Errorf("%s: %d backups of %s made although nothing was written", key, n, gamRel(key, ext))
			}
		}
	}
}
