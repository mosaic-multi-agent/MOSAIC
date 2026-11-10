package analysis_test

// Tests that notification events and lifecycle events carrying the new fact
// blocks stay benign: no finding, no change to totals, and run_end/session_end
// still mark the run as complete.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const unknownHarnessForBenign = domain.Harness("some-new-harness")

func notificationEvent() domain.Event {
	return domain.Event{
		Type:    domain.EventNotification,
		Harness: unknownHarnessForBenign,
		Notification: &domain.NotificationFields{
			NotificationType: "permission_prompt",
			RequiresResponse: domain.BoolTrue,
			Resolution:       "approved",
			NotificationID:   "n-1",
		},
	}
}

func orchestratorBaseline() []domain.Event {
	return []domain.Event{usageRecordEvent("", "rec-1", "claude-sonnet", domain.TokenUsage{Input: domain.Tokens(10)})}
}

func agentBaseline(id domain.AgentInstanceID) []domain.Event {
	return []domain.Event{invEndEvent(id, "claude-sonnet", domain.TokenUsage{Input: domain.Tokens(10)})}
}

func assertBenign(t *testing.T, base, with analysis.Input) {
	t.Helper()
	wantAgg, wantFindings := analysis.Aggregate(base)
	gotAgg, gotFindings := analysis.Aggregate(with)

	if len(gotFindings) != len(wantFindings) {
		t.Errorf("findings = %v, want same as baseline %v", gotFindings, wantFindings)
	}
	if hasFindingKind(gotFindings, domain.FindingUnrecognisedHarness) {
		t.Errorf("benign event raised FindingUnrecognisedHarness: %v", gotFindings)
	}
	gotAgg, wantAgg = withoutCarriedFacts(gotAgg), withoutCarriedFacts(wantAgg)
	if !reflect.DeepEqual(gotAgg, wantAgg) {
		t.Errorf("aggregate changed by benign event:\n got  %+v\n want %+v", gotAgg, wantAgg)
	}
}

// withoutCarriedFacts clears the facts that are carried, not shown, so only the
// totals-relevant parts of the aggregate are compared.
func withoutCarriedFacts(a domain.Aggregate) domain.Aggregate {
	runs := make([]domain.RunAggregate, len(a.Runs))
	for i, r := range a.Runs {
		r.Facts, r.Sessions, r.Turns, r.Notifications = domain.RunFacts{}, nil, nil, nil
		runs[i] = r
	}
	a.Runs = runs
	return a
}

func TestAggregate_NotificationOnOrchestratorStream_IsBenign(t *testing.T) {
	base := orchestratorOnlyInput(orchestratorBaseline()...)
	with := orchestratorOnlyInput(append(orchestratorBaseline(), notificationEvent())...)

	assertBenign(t, base, with)
}

func TestAggregate_NotificationOnAgentStream_IsBenign(t *testing.T) {
	id := domain.AgentInstanceID("Agent#1")
	base := oneAgentStream(testRunRef, id, agentBaseline(id)...)
	with := oneAgentStream(testRunRef, id, append(agentBaseline(id), notificationEvent())...)

	assertBenign(t, base, with)
}

func TestAggregate_RunAndSessionStartWithBlocks_AreBenign(t *testing.T) {
	start := []domain.Event{
		{Type: domain.EventRunStart, Harness: unknownHarnessForBenign,
			RunStart: &domain.RunStartFields{AdapterVersion: "0.9.1", Model: "m", Provider: "p"}},
		{Type: domain.EventSessionStart, Harness: unknownHarnessForBenign,
			SessionStart: &domain.SessionStartFields{Resumed: domain.BoolTrue, Model: "m", Provider: "p"}},
	}
	base := orchestratorOnlyInput(orchestratorBaseline()...)
	with := orchestratorOnlyInput(append(start, orchestratorBaseline()...)...)

	assertBenign(t, base, with)
}

func TestAggregate_RunEndAndSessionEndWithBlocks_ClearProvisional(t *testing.T) {
	ends := map[string]domain.Event{
		"run_end":     {Type: domain.EventRunEnd, RunEnd: &domain.RunEndFields{Outcome: "completed"}},
		"session_end": {Type: domain.EventSessionEnd, SessionEnd: &domain.SessionEndFields{Reason: "user_exit"}},
	}
	for name, end := range ends {
		t.Run(name, func(t *testing.T) {
			agg, findings := analysis.Aggregate(orchestratorOnlyInput(append(orchestratorBaseline(), end)...))

			if requireOneRun(t, agg).Provisional {
				t.Error("run with an end event carrying a block must not be Provisional")
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}
