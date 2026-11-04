package cli_test

// Tests for effort in the text report: the first body line under
// "Orchestrator:" and under each "Agent: <id>" is "Effort: <levels>", or
// "Effort: -" when no level was reported.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func effortTextReport(orch, agent domain.EffortLevels) domain.Report {
	totals := reasoningTotals(140, domain.AbsentTokens())
	return domain.Report{
		Currency: domain.Currency,
		AllRuns:  totals,
		Runs: []domain.RunReport{{
			Run:          domain.NamedRun(cliTestRunID),
			Totals:       totals,
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: totals, Effort: orch},
			Agents: []domain.ActorReport{{
				Actor:  domain.AgentInstance("Implementer#3"),
				Totals: totals,
				Effort: agent,
			}},
		}},
	}
}

// lineAfterHeading returns the line directly after the first line whose
// trimmed text starts with heading.
func lineAfterHeading(t *testing.T, out, heading string) string {
	t.Helper()
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), heading) && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	t.Fatalf("no %q heading in output:\n%s", heading, out)
	return ""
}

func TestEncodeText_OrchestratorMediumAgentHigh_EffortIsFirstLineUnderEach(t *testing.T) {
	high := domain.EffortLevels{{Level: "high", Records: 3}}
	out := encodeTextString(t, effortTextReport(domain.EffortLevels{{Level: "medium", Records: 4}}, high))

	if got, want := lineAfterHeading(t, out, "Orchestrator:"), "    Effort: medium"; got != want {
		t.Errorf("line under Orchestrator = %q, want %q", got, want)
	}
	if got, want := lineAfterHeading(t, out, "Agent: Implementer#3"), "    Effort: high"; got != want {
		t.Errorf("line under Agent = %q, want %q", got, want)
	}
}

func TestEncodeText_MixedLevels_ListEveryLevelWithItsRecordCount(t *testing.T) {
	mixed := domain.EffortLevels{{Level: "medium", Records: 12}, {Level: "high", Records: 3}}
	out := encodeTextString(t, effortTextReport(mixed, nil))

	if got, want := lineAfterHeading(t, out, "Orchestrator:"), "    Effort: medium (12), high (3)"; got != want {
		t.Errorf("line under Orchestrator = %q, want %q", got, want)
	}
}

func TestEncodeText_AbsentEffort_PrintsNotProvidedMarker(t *testing.T) {
	out := encodeTextString(t, effortTextReport(nil, nil))

	if got, want := lineAfterHeading(t, out, "Orchestrator:"), "    Effort: -"; got != want {
		t.Errorf("line under Orchestrator = %q, want %q", got, want)
	}
	if got, want := lineAfterHeading(t, out, "Agent: Implementer#3"), "    Effort: -"; got != want {
		t.Errorf("line under Agent = %q, want %q", got, want)
	}
}
