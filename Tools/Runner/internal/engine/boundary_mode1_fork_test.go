package engine_test

// Fork rows in Mode 1. A fork list (a comma list of targets in On Success)
// leaves the phase, or ends the iteration, only when every listed target does.
// Every fixture here makes the table position and the declared routing
// disagree, so an answer taken from position, from the first target only or
// from any single target is wrong.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// forkTargetInOwnPhaseContent: scout, the last RESEARCH row by position, forks
// to targets, one of which (prep) is an earlier row of RESEARCH. The planner
// jumps over prep, so prep only runs as a fork target. With publisher set the
// table also holds a second REPORT branch.
//
//	0 planner  1 prep  2 scout (fork)  3 writer  [4 publisher]  merge (join, last)
func forkTargetInOwnPhaseContent(targets string, publisher bool) string {
	waits := "prep, writer"
	rows := forkColumns +
		"| PLANNING | planner | FALSE | scout | - | - | - | out-0.md |\n" +
		"| RESEARCH | prep | FALSE | merge | - | - | - | out-1.md |\n" +
		"| RESEARCH | scout | FALSE | " + targets + " | - | - | - | out-2.md |\n" +
		"| REPORT | writer | FALSE | merge | - | - | - | out-3.md |\n"
	if publisher {
		waits = "prep, writer, publisher"
		rows += "| REPORT | publisher | FALSE | merge | - | - | - | out-4.md |\n"
	}
	return rows + "| REPORT | merge | FALSE | COMPLETE | - | " + waits + " | - | out-9.md |\n"
}

// One listed target stays in RESEARCH, so the fork is not the phase end, in
// whichever position the in-phase target is listed. By position scout is the
// last RESEARCH row, so a positional answer calls it the phase end; prep, which
// routes to REPORT, is the phase end although scout follows it in RESEARCH.
func TestPhaseEndMode1_ForkWithOneTargetInOwnPhase_IsNotPhaseEnd(t *testing.T) {
	cases := []struct {
		name      string
		targets   string
		publisher bool
		steps     []string
		want      []string
	}{
		{"in-phase target last of two", "writer, prep", false,
			[]string{"0:", "2:", "1:", "3:", "4:"}, []string{"0:", "1:", "4:"}},
		{"in-phase target first of two", "prep, writer", false,
			[]string{"0:", "2:", "1:", "3:", "4:"}, []string{"0:", "1:", "4:"}},
		{"in-phase target last of three", "writer, publisher, prep", true,
			[]string{"0:", "2:", "1:", "3:", "4:", "5:"}, []string{"0:", "1:", "5:"}},
		{"in-phase target in the middle of three", "writer, prep, publisher", true,
			[]string{"0:", "2:", "1:", "3:", "4:", "5:"}, []string{"0:", "1:", "5:"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := bndAdmitRaw(t, forkTargetInOwnPhaseContent(tc.targets, tc.publisher), domain.ExecutionModeOrchestrated)
			steps := bndSteps(t, tc.steps...)

			got := bndPhaseEnds(aw, nil, domain.ExecutionModeOrchestrated, steps)

			requireEnds(t, "phase end", got, tc.want)
		})
	}
}

// forkEveryTargetLeavesBeforeLaterSamePhaseContent: scout forks to two REPORT
// rows, so every target leaves RESEARCH although cleanup, a later row of
// RESEARCH, follows in the table.
//
//	0 scout (fork)  1 writer-a  2 writer-b  3 publisher (join)  4 cleanup
const forkEveryTargetLeavesBeforeLaterSamePhaseContent = forkColumns +
	"| RESEARCH | scout | FALSE | writer-a, writer-b | - | - | - | out-0.md |\n" +
	"| REPORT | writer-a | FALSE | publisher | - | - | - | out-1.md |\n" +
	"| REPORT | writer-b | FALSE | publisher | - | - | - | out-2.md |\n" +
	"| REPORT | publisher | FALSE | cleanup | - | writer-a, writer-b | - | out-3.md |\n" +
	"| RESEARCH | cleanup | FALSE | COMPLETE | - | - | - | out-4.md |\n"

func TestPhaseEndMode1_ForkWithEveryTargetOutsidePhase_IsPhaseEndDespiteLaterRowOfSamePhase(t *testing.T) {
	aw := bndAdmitRaw(t, forkEveryTargetLeavesBeforeLaterSamePhaseContent, domain.ExecutionModeOrchestrated)
	steps := bndSteps(t, "0:", "1:", "2:", "3:", "4:")

	got := bndPhaseEnds(aw, nil, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "phase end", got, []string{"0:", "3:", "4:"})
}

// Targets of a row inside a block must lie in the block's scope, so a fork in a
// block never leaves its iteration or its phase through the list. A fork row
// that is the last row of the scope by position, and lists earlier rows of the
// scope, ends neither: its targets run next. By position lead is the last row of
// the last iteration, so a positional phase-end answer is true.
//
//	0 planner  1 prep  2 aux  3 lead (fork to prep and aux, last row)  4 reporter
func TestBoundariesMode1_ForkWithTargetsInsideBlock_EndsNeitherIterationNorPhase(t *testing.T) {
	for _, targets := range []string{"prep, aux", "aux, prep"} {
		t.Run(targets, func(t *testing.T) {
			content := scopeColumns +
				"| PLANNING | planner | FALSE | NEXT | - | - | Plan.md |\n" +
				"| BUILD.[Stage] | prep | FALSE | NEXT | - | - | out-1-{Stage}.md |\n" +
				"| BUILD.[Stage] | aux | FALSE | NEXT | - | - | out-2-{Stage}.md |\n" +
				"| BUILD.[Stage] | lead | FALSE | " + targets + " | - | - | out-3-{Stage}.md |\n" +
				"| REPORT | reporter | FALSE | COMPLETE | - | - | out-4.md |\n" +
				"\n**Blocks:**\n\n| Block | Source | Section |\n|-------|--------|---------|\n| Stage | Plan.md | Stages |\n"
			aw := bndAdmitRaw(t, content, domain.ExecutionModeOrchestrated)
			schedules := scheduleOf("Stage", idsOf("", "1")...)
			steps := bndSteps(t, "0:", "1:1", "2:1", "3:1", "4:")

			iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)
			phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

			requireEnds(t, "iteration end", iterations, []string{})
			requireEnds(t, "phase end", phases, []string{"0:", "4:"})
		})
	}
}

// A fork row answers false from both helpers when Mode is not Orchestrated
// (Modes 2 and 3 and the zero value): the engine has no routed position for it.
// scout forks to targets that all leave RESEARCH, so the Mode 1 answer is true
// and a mode-blind implementation gives true; in the other fixture scout is the
// last RESEARCH row by position, so a positional implementation gives true.
func TestBoundaries_ForkRowWithModeOtherThanOrchestrated_IsNeitherBoundary(t *testing.T) {
	cases := []struct {
		name    string
		content string
		step    string
	}{
		{"every target leaves the phase", forkEveryTargetLeavesBeforeLaterSamePhaseContent, "0:"},
		{"last row of the phase by position", forkTargetInOwnPhaseContent("writer, prep", false), "2:"},
	}
	modes := append([]domain.ExecutionMode{domain.ExecutionModeUnset}, autoModes...)
	for _, tc := range cases {
		for _, mode := range modes {
			t.Run(tc.name+"/"+modeName(mode), func(t *testing.T) {
				aw := bndAdmitRaw(t, tc.content, domain.ExecutionModeOrchestrated)
				steps := bndSteps(t, tc.step)

				phases := bndPhaseEnds(aw, nil, mode, steps)
				iterations := bndIterationEnds(aw, nil, mode, steps)

				requireEnds(t, "phase end", phases, []string{})
				requireEnds(t, "iteration end", iterations, []string{})
			})
		}
	}
}
