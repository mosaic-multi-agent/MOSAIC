package app

// transform_ghcp_pair_reason_test.go specifies that every removal and replacement planned for
// the GHCP pair explains itself: the reason is present and fits on one line. The wording is
// not part of the contract and is not asserted.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestPlanGhcpPair_EveryRemovalAndReplacementCarriesASingleLineReason(t *testing.T) {
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

			// Legacy agent removal and hook-file removal; in-place agent, hook-file and
			// registration replacements.
			if len(p.Removals) != 2 || len(p.Replacements) != 3 {
				t.Fatalf("setup: Removals %v / Replacements %v, want 2 and 3", tpRemovals(p), tpReplacements(p))
			}
			for _, r := range p.Removals {
				assertSingleLineReason(t, "removal "+r.Path, r.Reason)
			}
			for _, r := range p.Replacements {
				assertSingleLineReason(t, "replacement "+r.Path, r.Reason)
			}
		})
	}
}

func assertSingleLineReason(t *testing.T, what, reason string) {
	t.Helper()
	if strings.TrimSpace(reason) == "" {
		t.Errorf("%s has an empty reason", what)
	}
	if strings.ContainsAny(reason, "\r\n") {
		t.Errorf("%s reason spans several lines: %q", what, reason)
	}
}
