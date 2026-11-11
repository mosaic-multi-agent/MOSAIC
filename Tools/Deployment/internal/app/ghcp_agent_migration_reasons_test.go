package app_test

// ghcp_agent_migration_reasons_test.go verifies that the GHCP agent-name migration adds its
// reason to an agent item without replacing the reasons the planner gave it, and that no reason,
// removal or replacement text of the migration speaks about backups: the review states the backup
// action itself.

import (
	"strings"
	"testing"
)

// gmrBackupWords are the words a plan text uses when it describes a backup.
var gmrBackupWords = []string{"backup", "backed up", "back up"}

func gmrMentionsBackup(text string) bool {
	lower := strings.ToLower(text)
	for _, w := range gmrBackupWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// TestGhcpFlows_MigrationOfAnAgentWithExistingFile_KeepsThePlannersReasons covers an X.agent.md
// already on disk beside the recorded X.md, in states that give the planner its own reason (not
// recorded; recorded but not readable as an agent).
func TestGhcpFlows_MigrationOfAnAgentWithExistingFile_KeepsThePlannersReasons(t *testing.T) {
	variants := []gamExistingVariant{
		{name: "unrecorded", body: "stale user copy\n"},
		{name: "recorded", recorded: true, recordedBody: "recorded copy\n", body: "recorded copy\n"},
	}
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			for _, v := range variants {
				t.Run(h+"/"+flow.name+"/"+v.name, func(t *testing.T) {
					// Arrange
					key := flow.renamed[0]
					ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
					newRel := gamRel(key, ".agent.md")
					gamWrite(t, ws, newRel, v.body)
					if v.recorded {
						gamRecordAgentAt(t, ws, key, newRel, v.recordedBody)
					}
					c := gamNewReviewCapture(ws)

					// Act
					err := flow.run(t, ws, h, c, true)

					// Assert
					if err != nil {
						t.Fatalf("%s for %s: %v", flow.name, h, err)
					}
					it, ok := gplAgentItem(gplReviewedPlan(t, c), key)
					if !ok {
						t.Fatalf("plan has no item for %s", newRel)
					}
					if len(it.Reasons) < 2 {
						t.Fatalf("Reasons = %q, want the planner's reason and the migration reason", it.Reasons)
					}
					legacyRel := gamRel(key, ".md")
					if !strings.Contains(it.Reasons[len(it.Reasons)-1], legacyRel) {
						t.Errorf("Reasons = %q, want the migration reason (naming %s) added last", it.Reasons, legacyRel)
					}
					if strings.Contains(it.Reasons[0], legacyRel) {
						t.Errorf("Reasons = %q, the planner's own reason was replaced by the migration reason", it.Reasons)
					}
				})
			}
		}
	}
}

// TestGhcpFlows_MigrationTexts_DoNotSpeakAboutBackups verifies no item reason, removal reason or
// replacement reason of a migrating plan mentions a backup; the review adds the backup action.
func TestGhcpFlows_MigrationTexts_DoNotSpeakAboutBackups(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				key := flow.renamed[0]
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				gamWrite(t, ws, gamRel(key, ".agent.md"), "stale user copy\n")
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				migrated, ok := gplAgentItem(p, key)
				if !ok {
					t.Fatalf("plan has no item for %s", gamRel(key, ".agent.md"))
				}
				legacyRel := gamRel(key, ".md")
				if len(migrated.Reasons) == 0 || !strings.Contains(migrated.Reasons[len(migrated.Reasons)-1], legacyRel) {
					t.Fatalf("Reasons = %q, want the migration reason (naming %s) recorded before checking its wording", migrated.Reasons, legacyRel)
				}
				for _, it := range p.Items {
					for _, r := range it.Reasons {
						if gmrMentionsBackup(r) {
							t.Errorf("%s: reason %q speaks about a backup", it.TargetPath, r)
						}
					}
				}
				for _, r := range p.Removals {
					if gmrMentionsBackup(r.Reason) {
						t.Errorf("removal %s: reason %q speaks about a backup", r.Path, r.Reason)
					}
				}
				for _, r := range p.Replacements {
					if gmrMentionsBackup(r.Reason) {
						t.Errorf("replacement %s: reason %q speaks about a backup", r.Path, r.Reason)
					}
				}
			})
		}
	}
}
