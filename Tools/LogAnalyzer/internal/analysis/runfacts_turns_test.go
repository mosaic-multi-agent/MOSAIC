package analysis_test

// Turns and notifications are kept as lists per run, attributed to the actor of
// the stream, including user turns and subagent turns that contribute no usage.
// Keeping them never changes any actor's usage entries.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestRunTurns_OrchestratorUserAndAssistant_KeptInFeedOrder(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfTurn(domain.TurnUser, rmAt(1), domain.TokenUsage{}),
		rfTurn(domain.TurnAssistant, rmAt(2), domain.TokenUsage{Input: domain.Tokens(7)}),
		rfTurn(domain.TurnUser, rmAt(3), domain.TokenUsage{}),
	))

	want := []domain.Turn{
		{Actor: domain.Orchestrator(), Role: domain.TurnUser, At: rmAt(1)},
		{Actor: domain.Orchestrator(), Role: domain.TurnAssistant, At: rmAt(2)},
		{Actor: domain.Orchestrator(), Role: domain.TurnUser, At: rmAt(3)},
	}
	if !reflect.DeepEqual(run.Turns, want) {
		t.Errorf("Turns = %+v, want %+v", run.Turns, want)
	}
}

func TestRunTurns_UserTurnsAddNoUsageEntries(t *testing.T) {
	run := rfRun(t, rfOrch(rfTurn(domain.TurnUser, rmAt(1), domain.TokenUsage{})))

	if len(run.Turns) != 1 {
		t.Fatalf("got %d turns, want the user turn kept", len(run.Turns))
	}
	if len(run.Orchestrator.Invocations) != 0 {
		t.Errorf("user turn added usage entries: %+v", run.Orchestrator.Invocations)
	}
}

func TestRunTurns_SubagentTurns_KeptForTheStreamInstanceWithoutUsage(t *testing.T) {
	start := rmInvStart("w#1", "worker", rmAt(1))
	run := rfRun(t, rfAgent("w#1",
		start,
		rfTurn(domain.TurnUser, rmAt(2), domain.TokenUsage{}),
		rfTurn(domain.TurnAssistant, rmAt(3), domain.TokenUsage{Input: domain.Tokens(500)}),
		invEndEvent("w#1", "model-a", domain.TokenUsage{Input: domain.Tokens(40)}),
	))

	want := []domain.Turn{
		{Actor: domain.AgentInstance("w#1"), Role: domain.TurnUser, At: rmAt(2)},
		{Actor: domain.AgentInstance("w#1"), Role: domain.TurnAssistant, At: rmAt(3)},
	}
	if !reflect.DeepEqual(run.Turns, want) {
		t.Errorf("Turns = %+v, want %+v (actor Type stays empty)", run.Turns, want)
	}
	agent := requireOneAgent(t, run)
	assertTokenPresent(t, "agent input total", agent.Totals().Input, 40)
	if len(agent.Invocations) != 1 {
		t.Errorf("agent usage entries = %d, want 1", len(agent.Invocations))
	}
}

func TestRunTurns_SubagentTurnAlone_CreatesNoAgentAggregate(t *testing.T) {
	run := rfRun(t, rfAgent("w#1", rfTurn(domain.TurnAssistant, rmAt(1), domain.TokenUsage{Input: domain.Tokens(9)})))

	if len(run.Turns) != 1 {
		t.Fatalf("got %d turns, want 1", len(run.Turns))
	}
	if len(run.Agents) != 0 {
		t.Errorf("turn created agent aggregates: %+v", run.Agents)
	}
}

func TestRunTurns_AgentStreamWithoutInstanceHint_TurnNotKeptAndNoFinding(t *testing.T) {
	agg, findings := rfAggregate(rfOrch(rfTurn(domain.TurnUser, rmAt(1), domain.TokenUsage{})),
		rfAgentNoHint(rfTurn(domain.TurnAssistant, rmAt(2), domain.TokenUsage{})))

	run := requireOneRun(t, agg)
	if len(run.Turns) != 1 || run.Turns[0].Actor.Kind != domain.ActorOrchestrator {
		t.Errorf("Turns = %+v, want only the orchestrator turn", run.Turns)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestRunTurns_AcrossStreams_AreInFeedOrder(t *testing.T) {
	a := analysis.NewAggregator()
	a.Add(orchStream(testRunRef), rfTurn(domain.TurnUser, rmAt(1), domain.TokenUsage{}))
	a.Add(agentStream(testRunRef, "w#1"), rfTurn(domain.TurnAssistant, rmAt(2), domain.TokenUsage{}))
	a.Add(orchStream(testRunRef), rfTurn(domain.TurnAssistant, rmAt(3), domain.TokenUsage{}))
	agg, _ := a.Result()

	run := requireOneRun(t, agg)
	got := make([]string, len(run.Turns))
	for i, tr := range run.Turns {
		got[i] = tr.Actor.Label()
	}
	if !rmEqualStrings(got, []string{"orchestrator", "w#1", "orchestrator"}) {
		t.Errorf("turn actors = %v, want feed order", got)
	}
}

func TestRunTurns_RoleVerbatimAndTimestampAbsentStaysZero(t *testing.T) {
	odd := rfTurn("system", rmAt(1), domain.TokenUsage{})
	noRole := rfTurn("", rmAt(2), domain.TokenUsage{})
	noTime := rfTurn(domain.TurnUser, time.Time{}, domain.TokenUsage{})

	run := rfRun(t, rfOrch(odd, noRole, noTime))

	if len(run.Turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(run.Turns))
	}
	if run.Turns[0].Role != "system" || run.Turns[1].Role != "" {
		t.Errorf("roles = %q/%q, want verbatim system and empty", run.Turns[0].Role, run.Turns[1].Role)
	}
	if !run.Turns[2].At.IsZero() {
		t.Errorf("At = %v, want zero for an unparseable timestamp", run.Turns[2].At)
	}
}

func TestRunTurns_AreKeptInUnattributableBucketAndNotInNamedRun(t *testing.T) {
	agg, _ := rfAggregate(
		rfOrch(rfTurn(domain.TurnUser, rmAt(1), domain.TokenUsage{})),
		rfUnattributable(rfTurn(domain.TurnAssistant, rmAt(2), domain.TokenUsage{})),
	)

	if len(agg.Runs[0].Turns) != 1 || agg.Runs[0].Turns[0].Role != domain.TurnUser {
		t.Errorf("named run turns = %+v", agg.Runs[0].Turns)
	}
	if agg.Unattributable == nil || len(agg.Unattributable.Turns) != 1 || agg.Unattributable.Turns[0].Role != domain.TurnAssistant {
		t.Errorf("bucket turns = %+v", agg.Unattributable)
	}
}

func TestRunNotifications_KeptForEveryStreamWithAllFieldsAndNoUsage(t *testing.T) {
	run := rfRun(t,
		rfOrch(rfNotification(rmAt(1), domain.NotificationFields{
			NotificationType: "permission_prompt", RequiresResponse: domain.BoolTrue,
			Resolution: "approved", NotificationID: "n-1",
		})),
		rfAgent("w#1", rfNotification(rmAt(2), domain.NotificationFields{
			NotificationType: "idle", RequiresResponse: domain.BoolFalse,
		})),
	)

	want := []domain.Notification{
		{Actor: domain.Orchestrator(), At: rmAt(1), Type: "permission_prompt",
			RequiresResponse: domain.BoolTrue, Resolution: "approved", ID: "n-1"},
		{Actor: domain.AgentInstance("w#1"), At: rmAt(2), Type: "idle", RequiresResponse: domain.BoolFalse},
	}
	if !reflect.DeepEqual(run.Notifications, want) {
		t.Errorf("Notifications = %+v, want %+v", run.Notifications, want)
	}
	if len(run.Orchestrator.Invocations) != 0 || len(run.Agents) != 0 {
		t.Errorf("notifications added usage: %+v / %+v", run.Orchestrator.Invocations, run.Agents)
	}
}

func TestRunNotifications_AbsentFieldsStayAbsent(t *testing.T) {
	run := rfRun(t, rfOrch(rfNotification(rmAt(1), domain.NotificationFields{})))

	if len(run.Notifications) != 1 {
		t.Fatalf("got %d notifications, want 1", len(run.Notifications))
	}
	n := run.Notifications[0]
	if n.RequiresResponse != domain.BoolAbsent || n.Type != "" || n.Resolution != "" || n.ID != "" {
		t.Errorf("absent fields were invented: %+v", n)
	}
}

func TestRunNotifications_AgentStreamWithoutInstanceHint_NotKeptAndNoFinding(t *testing.T) {
	agg, findings := rfAggregate(rfAgentNoHint(rfNotification(rmAt(1), domain.NotificationFields{NotificationType: "x"})),
		rfOrch(rfNotification(rmAt(2), domain.NotificationFields{NotificationType: "y"})))

	run := requireOneRun(t, agg)
	if len(run.Notifications) != 1 || run.Notifications[0].Type != "y" {
		t.Errorf("Notifications = %+v, want only the orchestrator one", run.Notifications)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestRunNotifications_AreInFeedOrderAndKeptInUnattributableBucket(t *testing.T) {
	agg, _ := rfAggregate(
		rfOrch(
			rfNotification(rmAt(9), domain.NotificationFields{NotificationType: "late-time-first-fed"}),
			rfNotification(rmAt(1), domain.NotificationFields{NotificationType: "early-time-second-fed"}),
		),
		rfUnattributable(rfNotification(rmAt(3), domain.NotificationFields{NotificationType: "stray"})),
	)

	named := agg.Runs[0].Notifications
	if len(named) != 2 || named[0].Type != "late-time-first-fed" || named[1].Type != "early-time-second-fed" {
		t.Errorf("named notifications = %+v, want feed order", named)
	}
	if agg.Unattributable == nil || len(agg.Unattributable.Notifications) != 1 || agg.Unattributable.Notifications[0].Type != "stray" {
		t.Errorf("bucket notifications = %+v", agg.Unattributable)
	}
}
