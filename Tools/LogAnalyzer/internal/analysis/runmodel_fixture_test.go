package analysis_test

// Run-model test over fixture logs read through the real reader: one
// orchestrator stream and three subagent streams (complete, open, and ended
// without a status code), covering every state the model must keep distinct.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

const fixtureRunID = "20261021T120000Z-rm01"

func fixtureStream(t *testing.T, file string, ref domain.StreamRef) analysis.Stream {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", file)
	events, findings, err := logread.NewReader().ReadAll(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadAll(%s): %v", file, err)
	}
	if len(findings) != 0 {
		t.Fatalf("fixture %s produced findings: %v", file, findings)
	}
	ref.Path = path
	return analysis.Stream{Ref: ref, Events: events}
}

func fixtureActivity(t *testing.T) domain.RunActivity {
	t.Helper()
	run := domain.NamedRun(fixtureRunID)
	in := analysis.Input{Streams: []analysis.Stream{
		fixtureStream(t, "runmodel_orchestrator.jsonl", orchStream(run)),
		fixtureStream(t, "runmodel_agent_done.jsonl", agentStream(run, "coder#1")),
		fixtureStream(t, "runmodel_agent_open.jsonl", agentStream(run, "tester#2")),
		fixtureStream(t, "runmodel_agent_nostatus.jsonl", agentStream(run, "reviewer#3")),
	}}
	agg, _ := analysis.Aggregate(in)
	return requireOneRun(t, agg).Activity
}

func TestRunModel_FixtureLogs_OrchestratorToolCalls(t *testing.T) {
	act := fixtureActivity(t)

	if got := rmCallIDs(act.OrchestratorToolCalls); !rmEqualStrings(got, []string{"o-read", "o-bash", "o-dangling"}) {
		t.Fatalf("orchestrator calls = %v, want [o-read o-bash o-dangling]", got)
	}
	read := rmRequireCall(t, act.OrchestratorToolCalls, "o-read")
	if d, ok := read.Duration(); read.Status != domain.ToolCallSuccess || !ok || d != 1500*time.Millisecond {
		t.Errorf("o-read = %+v (duration %v, %v), want success lasting 1.5s", read, d, ok)
	}
	if bash := rmRequireCall(t, act.OrchestratorToolCalls, "o-bash"); bash.Status != domain.ToolCallError || bash.Pairing != domain.ToolCallPaired {
		t.Errorf("o-bash = %+v, want paired error", bash)
	}
	if dangling := rmRequireCall(t, act.OrchestratorToolCalls, "o-dangling"); dangling.Pairing != domain.ToolCallStartOnly || dangling.ToolName != "Grep" {
		t.Errorf("o-dangling = %+v, want start-only Grep", dangling)
	}
}

func TestRunModel_FixtureLogs_CompleteInvocationWithParallelAndOrphanCalls(t *testing.T) {
	inv := rmInvocation(t, fixtureActivity(t), "coder#1")

	if inv.Actor.Type != "coder" || inv.State != domain.InvocationClosed || inv.StatusCode != "SUCCESS" || inv.Model != "model-a" {
		t.Errorf("invocation = %+v, want closed coder SUCCESS model-a", inv)
	}
	if d, ok := inv.Duration(); !ok || d != 90*time.Second {
		t.Errorf("Duration() = (%v, %v), want (1m30s, true) from the logged timestamps", d, ok)
	}
	if got := rmCallIDs(inv.ToolCalls); !rmEqualStrings(got, []string{"p1", "p2", "p-orphan"}) {
		t.Fatalf("calls = %v, want [p1 p2 p-orphan]", got)
	}
	p1 := rmRequireCall(t, inv.ToolCalls, "p1")
	if d, ok := p1.Duration(); p1.Status != domain.ToolCallError || !ok || d != time.Second {
		t.Errorf("p1 = %+v (duration %v, %v), want error lasting 1s", p1, d, ok)
	}
	p2 := rmRequireCall(t, inv.ToolCalls, "p2")
	if d, ok := p2.Duration(); p2.Status != domain.ToolCallSuccess || !ok || d != 1900*time.Millisecond {
		t.Errorf("p2 = %+v (duration %v, %v), want success lasting 1.9s", p2, d, ok)
	}
	if orphan := rmRequireCall(t, inv.ToolCalls, "p-orphan"); orphan.Pairing != domain.ToolCallEndOnly {
		t.Errorf("p-orphan = %+v, want end-only", orphan)
	}
	if len(inv.Usage) != 1 {
		t.Errorf("got %d usage entries, want 1", len(inv.Usage))
	}
}

func TestRunModel_FixtureLogs_OpenAndStatuslessInvocationsStayDistinct(t *testing.T) {
	act := fixtureActivity(t)

	open := rmInvocation(t, act, "tester#2")
	if open.State != domain.InvocationOpen || open.StatusCode != "" || !open.EndedAt.IsZero() || open.Actor.Type != "tester" {
		t.Errorf("tester#2 = %+v, want open tester with no status and no end time", open)
	}
	if len(open.ToolCalls) != 1 || open.ToolCalls[0].Pairing != domain.ToolCallStartOnly {
		t.Errorf("tester#2 calls = %+v, want one start-only call", open.ToolCalls)
	}
	nostatus := rmInvocation(t, act, "reviewer#3")
	if nostatus.State != domain.InvocationClosed || nostatus.StatusCode != "" || nostatus.EndEvents != 1 {
		t.Errorf("reviewer#3 = %+v, want closed with no status code", nostatus)
	}
}

func TestRunModel_FixtureLogs_CompactionsCarryActorAndAreTimeOrdered(t *testing.T) {
	act := fixtureActivity(t)

	if len(act.Compactions) != 2 {
		t.Fatalf("got %d compactions, want 2", len(act.Compactions))
	}
	agent, orch := act.Compactions[0], act.Compactions[1]
	if agent.Actor.Instance != "coder#1" || agent.Trigger != "manual" {
		t.Errorf("first compaction = %+v, want coder#1 manual", agent)
	}
	assertTokenAbsent(t, "coder#1 TokensBefore", agent.TokensBefore)
	if orch.Actor.Kind != domain.ActorOrchestrator || orch.Trigger != "auto" {
		t.Errorf("second compaction = %+v, want orchestrator auto", orch)
	}
	assertTokenPresent(t, "orchestrator TokensBefore", orch.TokensBefore, 180000)
	assertTokenPresent(t, "orchestrator TokensAfter", orch.TokensAfter, 40000)
}
