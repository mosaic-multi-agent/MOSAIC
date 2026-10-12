package engine_test

// Boundary answers for a forked run scope admitted in orchestrated mode. A fork
// closes at its join: the join row is the last row of its phase, and the fork
// branch rows before it never are. The table-position answers of legacy tables
// are pinned in legacy_regression_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// Rows: 0 agent-a (forks to agent-b and agent-c), 1 agent-b, 2 agent-c,
// 3 agent-d (join, last row of the phase).
const forkJoinPhaseBoundaryV216Content = forkColumns +
	"| PLANNING | agent-a | FALSE | agent-b, agent-c | - | - | - | a.md |\n" +
	"| PLANNING | agent-b | FALSE | agent-d | - | - | a.md | b.md |\n" +
	"| PLANNING | agent-c | FALSE | agent-d | - | - | a.md | c.md |\n" +
	"| PLANNING | agent-d | FALSE | COMPLETE | - | agent-b, agent-c | b.md, c.md | d.md |\n"

func TestPhaseEnd_ForkJoinedRunScope_OnlyJoinRowEndsPhase(t *testing.T) {
	aw := bndAdmitRaw(t, forkJoinPhaseBoundaryV216Content, domain.ExecutionModeOrchestrated)
	steps := bndSteps(t, "0:", "1:", "2:", "3:")

	phases := bndPhaseEnds(aw, nil, domain.ExecutionModeOrchestrated, steps)
	iterations := bndIterationEnds(aw, nil, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "phase end", phases, []string{"3:"})
	requireEnds(t, "iteration end", iterations, []string{})
}
