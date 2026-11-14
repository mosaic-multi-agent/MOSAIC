package app

// transform_ghcp_pair_backup_wording_test.go specifies that the reasons of the removals and
// replacements planned for the GHCP pair state only why the file changes. The backup action is
// added by the review and the dry-run report, so a reason never reads as if a backup already
// existed.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestPlanGhcpPair_RemovalAndReplacementReasons_DoNotSpeakAboutBackups(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpPairHooks(t, c,
				[]string{"logger.sh", "extra.sh"}, []string{"logger.sh"}, []string{"logger.sh", "extra.sh"})
			src.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpRegFile, true)}
			tgt.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpRegFile, true)}
			tpWriteFile(t, ws, tpRegFile, "{\"edited\":true}\n")
			tpPlaceAgent(t, ws, ".github/agents", "a.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, ".github/agents", "b.md", "subagent.md")
			tpRecord(t, ws, c.srcID,
				tpAgentEntry("a", ".github/agents/a.agent.md"),
				tpAgentEntry("b", ".github/agents/b.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if len(p.Removals)+len(p.Replacements) == 0 {
				t.Fatal("setup: the plan has no removal or replacement")
			}
			for _, r := range p.Removals {
				if mentionsBackup(r.Reason) {
					t.Errorf("removal %s: reason %q speaks about a backup", r.Path, r.Reason)
				}
			}
			for _, r := range p.Replacements {
				if mentionsBackup(r.Reason) {
					t.Errorf("replacement %s: reason %q speaks about a backup", r.Path, r.Reason)
				}
			}
		})
	}
}

func mentionsBackup(text string) bool {
	lower := strings.ToLower(text)
	for _, w := range []string{"backup", "backed up", "back up"} {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}
