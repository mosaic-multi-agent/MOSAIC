package session_test

// PHASE_END in Mode 1 for fork rows and consulted routes. The table position
// and the declared routing disagree in every fixture, so a trigger that looks
// at the position, at one target of a fork list or at the consulted next step
// fires after the wrong step.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// mode1Dispatch is one step the scripted consultant dispatches: the agent and
// its zero-based table row.
type mode1Dispatch struct {
	agent string
	row   int
}

// runMode1Phase plays fixture with the consultant dispatching steps in order and
// then stopping. Every step and every phase-committer invocation succeeds. It
// returns the invoked agents.
func runMode1Phase(t *testing.T, fixture, workflowID string, steps []mode1Dispatch) []string {
	t.Helper()
	consultant := &scriptedRoutingConsultant{}
	names := []string{routePhaseAgent}
	for _, s := range steps {
		consultant.queueDispatch(s.agent, "task", s.row)
		names = append(names, s.agent)
	}
	consultant.queueStop("done")
	ses, f, orchPath := routeSession(t, fixture, "", routePhaseAgent, consultant, names...)
	for _, s := range steps {
		queueCond(f, s.agent, domain.StatusSUCCESS)
	}
	for range steps {
		queueCond(f, routePhaseAgent, domain.StatusSUCCESS)
	}

	got, err := ses.Start(context.Background(), routeConfig(orchPath, workflowID, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	return invokedAgents(f)
}

// Every fork target lies in REPORT, so the fork row ends RESEARCH although
// cleanup, a later RESEARCH row, follows in the table.
func TestSession_Start_PhaseEndMode1_ForkWithEveryTargetOutsidePhase_FiresAfterFork(t *testing.T) {
	steps := []mode1Dispatch{{"scout", 0}, {"writer-a", 1}, {"writer-b", 2}, {"publisher", 3}, {"cleanup", 4}}

	got := runMode1Phase(t, "phase-end-fork-all-leave-orch.md", "fork-all-leave", steps)

	requireAgentSequence(t, got, []string{
		"scout", routePhaseAgent, "writer-a", "writer-b", "publisher", routePhaseAgent, "cleanup", routePhaseAgent,
	})
}

// One listed target (prep) is a RESEARCH row: the fork row, the last RESEARCH
// row by position, does not fire the trigger. prep routes to REPORT and does.
func TestSession_Start_PhaseEndMode1_ForkWithOneTargetInOwnPhase_DoesNotFireAfterFork(t *testing.T) {
	steps := []mode1Dispatch{{"planner", 0}, {"scout", 2}, {"prep", 1}, {"writer", 3}, {"merge", 4}}

	got := runMode1Phase(t, "phase-end-fork-in-phase-orch.md", "fork-in-phase", steps)

	requireAgentSequence(t, got, []string{
		"planner", routePhaseAgent, "scout", "prep", routePhaseAgent, "writer", "merge", routePhaseAgent,
	})
}

// The scout declares a row of another phase next, and the consultant runs the
// analyst, a later RESEARCH row, anyway. The trigger after the scout follows the
// declared route.
func TestSession_Start_PhaseEndMode1_ConsultedRouteDiffersFromDeclared_FiresAfterDeclaredLeave(t *testing.T) {
	steps := []mode1Dispatch{{"scout", 0}, {"analyst", 1}, {"reporter", 2}}

	got := runMode1Phase(t, "phase-end-consulted-orch.md", "consulted", steps)

	requireAgentSequence(t, got, []string{
		"scout", routePhaseAgent, "analyst", routePhaseAgent, "reporter", routePhaseAgent,
	})
}
