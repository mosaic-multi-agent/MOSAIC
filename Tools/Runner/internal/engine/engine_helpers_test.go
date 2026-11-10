package engine_test

// Tests for engine.Next and engine.ResumePoint.
//
// Coverage:
//
//   NEXT keyword routing:
//   - On Success = NEXT → dispatches the row below when it is in the current scope.
//   - NEXT past the last row of the run scope → CompleteDecision (same as COMPLETE).
//   - NEXT into the first row of a block → enters the block with its schedule.
//   - NEXT into a block without a schedule → StopDecision.
//
//   Initial dispatch (no prior invocations):
//   - No log entries and empty CurrentState → dispatches first pre-execution row.
//   - No log entries, workflow starts with EXECUTION rows → dispatches first EXECUTION row of stage 1.
//
//   Linear pre/post-execution advance (non-EXECUTION rows):
//   - SUCCESS + unambiguous On Success hint → dispatches the named agent.
//   - SUCCESS + On Success = "COMPLETE" → returns CompleteDecision.
//   - SUCCESS + absent On Success → returns DeviationDecision.
//   - SUCCESS + free-form On Success (ambiguous) → returns DeviationDecision.
//
//   Staged EXECUTION — approach-driven group ordering (AC7.2):
//   - TDD approach → test group dispatched first, then implementation group.
//   - Implementation-First approach → implementation group first, then test group.
//   - Implementation-Only approach → only implementation group rows dispatched.
//   - Tests-Only approach → only test group rows dispatched.
//
//   Staged EXECUTION — stage-to-stage progression:
//   - After last row in test group (TDD) → dispatches first row of implementation group.
//   - After last row of last group in stage N → dispatches first row of stage N+1 (test group first for TDD).
//   - After last row of last stage → dispatches first post-EXECUTION row.
//   - After last row of last stage, no post-EXECUTION rows → returns CompleteDecision.
//   - Tests-Only approach, last stage: after final test-group row → dispatches first post-EXECUTION row.
//   - Implementation-First approach, last stage: after final test-group row → dispatches first post-EXECUTION row.
//
//   On Success inside a block:
//   - A block row routes within its scope by its On Success (NEXT, COMPLETE or a named row).
//
//   HITL computation (AC7.4):
//   - Row HITL=true, Stage HITL=false → effective HITL=true.
//   - Row HITL=false, Stage HITL=true, inside EXECUTION → effective HITL=true.
//   - Row HITL=false, Stage HITL=true, outside EXECUTION → effective HITL=false.
//   - Row HITL=false, Stage HITL=false → effective HITL=false.
//
//   Agent instance ID assignment (AC7.5):
//   - seq=0 → dispatched AgentInstanceID is "{agent}#1".
//   - seq=5 → dispatched AgentInstanceID is "{agent}#6".
//   - IDs are monotonically increasing: each Next call increments seq.
//
//   Templated artifact path resolution (AC7.6):
//   - {Stage} in input artifacts → replaced with the current iteration ID.
//   - {Stage} in output artifacts → replaced with the current iteration ID.
//   - Stage-* in input artifacts → expanded to one path per iteration of the schedule.
//   - Stage-* in output artifacts → passed through unexpanded.
//   - Unresolvable placeholder → StopDecision with descriptive reason.
//
//   Non-EXECUTION Stage-* resolution using refreshedStages (AC7.9):
//   - plan-review row with Stage-*/Plan.md input + refreshedStages(3 stages) → 3 expanded paths.
//   - Nil refreshedStages on non-EXECUTION row → uses run-start schedules for expansion.
//
//   On Findings hint-based auto-routing for COMPLETED_NEEDS_ACTION, mode-gated:
//   - auto-review + CNA + unambiguous On Findings → DispatchDecision (loop-back to named agent).
//   - auto + CNA + unambiguous On Findings → DeviationDecision (On-Findings route gated to auto-review).
//   - CNA + On Findings column absent → DeviationDecision in all modes.
//   - CNA + On Findings value is "" (dash/empty) → DeviationDecision in all modes.
//   - CNA + free-form On Findings (e.g. "agent (or other based on issue)") → DeviationDecision in all modes.
//   - auto-review + CNA inside EXECUTION + unambiguous On Findings → DispatchDecision (loop-back).
//   - auto + CNA inside EXECUTION + unambiguous On Findings → DeviationDecision.
//   - Non-CNA non-SUCCESS (e.g. NEEDS_CLARIFICATION) → DeviationDecision regardless of On Findings.
//
//   Routing hint interpretation for non-SUCCESS responses:
//   - BLOCKED response + unambiguous On Findings → DeviationDecision (On Findings only for CNA).
//
//   Mode-gated routing (Stage 2):
//   - auto mode SUCCESS → routes to On Success target (same as auto-review).
//   - auto-review mode SUCCESS → routes to On Success target (same as auto).
//   - orchestrated mode, first call → ConsultDecision with ConsultTriggerOrchestratedMode.
//   - orchestrated mode, after SUCCESS → ConsultDecision (engine never routes).
//   - orchestrated mode, after CNA (even with unambiguous OnFindings) → ConsultDecision.
//   - auto + CNA + ambiguous OnFindings → DeviationDecision.
//   - auto-review + CNA + ambiguous OnFindings → DeviationDecision.
//   - auto + non-CNA non-SUCCESS → DeviationDecision.
//   - auto-review + non-CNA non-SUCCESS → DeviationDecision.
//
//   Review artifact injection (Stage 2, auto-review only):
//   - auto-review CNA auto-route-back: LastOutputArtifacts injected into dispatched InputArtifacts.
//   - Injected artifacts appear after table-row entries, not before.
//   - Paths already in table InputArtifacts are not duplicated by injection.
//   - SUCCESS-routed dispatches never carry injected artifacts.
//   - auto mode CNA deviates; no injection occurs on deviation decisions.
//
//   Resume point derivation (AC7.7):
//   - No execution log entries → RowIndex=0, RerunLast=false.
//   - Last log entry agent matches CurrentState → RowIndex = next row, RerunLast=false.
//   - Mismatch between last log entry and CurrentState → RerunLast=true, RowIndex = last log row.
//   - Resume from mid-stage position → GroupIndex and Iteration derived correctly.
//   - Resume with no interruption at end of pre-execution rows → RowIndex = first EXECUTION row.
//
//   Canonical phase and stage recording (AC4.2-AC4.5):
//   - Grouped EXECUTION dispatch step records bare phase + group-qualified stage ("1.Test").
//   - Ungrouped staged dispatch step records bare phase + plain stage number ("1").
//   - Non-staged dispatch step records an empty stage.
//   - Target-format recorded state (bare phase, group-qualified stage) resolves position,
//     including disambiguating a repeated agent by group ("1.Test" vs "1.Implementation"),
//     at both the Next and ResumePoint levels.
//   - A legacy artifact (qualified phase, "Stage-N" stage) still resolves position; those
//     tests are in legacy_regression_test.go.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// ---- Helper functions ----

var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// newTestAgents returns a minimal agent reference map for the given identifiers.
func newTestAgents(ids ...string) map[string]domain.AgentReference {
	m := make(map[string]domain.AgentReference, len(ids))
	for _, id := range ids {
		m[id] = domain.AgentReference{
			Identifier:     id,
			DefinitionPath: "/agents/" + id + ".md",
			InvocationKind: domain.InvocationOrdinary,
		}
	}
	return m
}

// emptyState returns an ArtifactState with no log entries and an empty CurrentState.
func emptyState() domain.ArtifactState {
	return domain.ArtifactState{}
}

// stateAfter returns an ArtifactState reflecting that agentID (in format "name#seq")
// just completed with the given status in the given phase/stage.
func stateAfter(phase, stage, agentID string, status domain.StatusCode, seq int) domain.ArtifactState {
	return domain.ArtifactState{
		GlobalSequence: seq,
		CurrentState: domain.CurrentState{
			Phase:      phase,
			Stage:      stage,
			LastStatus: status,
			LastAgent:  agentID,
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			{
				Seq:    seq,
				Agent:  agentID,
				Phase:  phase,
				Stage:  stage,
				Status: status,
			},
		},
	}
}

// successResponse returns a ProtocolResponse with SUCCESS status.
func successResponse(agentID string) *domain.ProtocolResponse {
	return &domain.ProtocolResponse{
		AgentInstanceID: agentID,
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "completed successfully",
	}
}

// cnaResponse returns a ProtocolResponse with COMPLETED_NEEDS_ACTION status.
func cnaResponse(agentID string) *domain.ProtocolResponse {
	return &domain.ProtocolResponse{
		AgentInstanceID: agentID,
		StatusCode:      domain.StatusCOMPLETED_NEEDS_ACTION,
		StatusMessage:   "completed with findings",
	}
}

// nonSuccessResponse returns a ProtocolResponse with the given non-SUCCESS status.
func nonSuccessResponse(agentID string, status domain.StatusCode) *domain.ProtocolResponse {
	return &domain.ProtocolResponse{
		AgentInstanceID: agentID,
		StatusCode:      status,
		StatusMessage:   "encountered an issue",
	}
}

// requireDispatch asserts that the decision is a DispatchDecision with exactly one step.
func requireDispatch(t *testing.T, dec domain.EngineDecision) domain.DispatchStep {
	t.Helper()
	if dec.Dispatch == nil {
		var which string
		switch {
		case dec.Complete != nil:
			which = "CompleteDecision"
		case dec.Deviation != nil:
			which = "DeviationDecision"
		case dec.Stop != nil:
			which = fmt.Sprintf("StopDecision(%q)", dec.Stop.Reason)
		default:
			which = "nil decision (all fields nil)"
		}
		t.Fatalf("want DispatchDecision, got %s", which)
	}
	if len(dec.Dispatch.Steps) != 1 {
		t.Fatalf("want exactly 1 dispatch step, got %d", len(dec.Dispatch.Steps))
	}
	return dec.Dispatch.Steps[0]
}

// requireComplete asserts that the decision is a CompleteDecision.
func requireComplete(t *testing.T, dec domain.EngineDecision) domain.CompleteDecision {
	t.Helper()
	if dec.Complete == nil {
		t.Fatalf("want CompleteDecision, got something else (Dispatch=%v Deviation=%v Stop=%v)",
			dec.Dispatch != nil, dec.Deviation != nil, dec.Stop != nil)
	}
	return *dec.Complete
}

// requireDeviation asserts that the decision is a DeviationDecision.
func requireDeviation(t *testing.T, dec domain.EngineDecision) domain.DeviationDecision {
	t.Helper()
	if dec.Deviation == nil {
		t.Fatalf("want DeviationDecision, got something else (Dispatch=%v Complete=%v Stop=%v)",
			dec.Dispatch != nil, dec.Complete != nil, dec.Stop != nil)
	}
	return *dec.Deviation
}

// requireStop asserts that the decision is a StopDecision.
func requireStop(t *testing.T, dec domain.EngineDecision) domain.StopDecision {
	t.Helper()
	if dec.Stop == nil {
		t.Fatalf("want StopDecision, got something else (Dispatch=%v Complete=%v Deviation=%v)",
			dec.Dispatch != nil, dec.Complete != nil, dec.Deviation != nil)
	}
	return *dec.Stop
}

// agentName strips the "#seq" suffix from an agent instance ID.
func agentName(instanceID string) string {
	if i := strings.LastIndex(instanceID, "#"); i >= 0 {
		return instanceID[:i]
	}
	return instanceID
}

// requireConsult asserts that the decision is a ConsultDecision.
func requireConsult(t *testing.T, dec domain.EngineDecision) domain.ConsultDecision {
	t.Helper()
	if dec.Consult == nil {
		var which string
		switch {
		case dec.Dispatch != nil:
			which = "DispatchDecision"
		case dec.Complete != nil:
			which = "CompleteDecision"
		case dec.Deviation != nil:
			which = fmt.Sprintf("DeviationDecision(kind=%q)", dec.Deviation.Info.Kind)
		case dec.Stop != nil:
			which = fmt.Sprintf("StopDecision(%q)", dec.Stop.Reason)
		default:
			which = "nil decision (all fields nil)"
		}
		t.Fatalf("want ConsultDecision, got %s", which)
	}
	return *dec.Consult
}
