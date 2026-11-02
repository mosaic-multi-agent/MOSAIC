package analysis_test

// Tests for effort per invocation in the priced report: the orchestrator and
// each agent (ActorReport.Effort) and each dispatch (Dispatch.Effort) carry the
// levels of their effective, superseded usage records with record counts.
//
// Rules under test:
//   - The orchestrator and a subagent are counted separately.
//   - Several levels are all kept, each with its record count.
//   - A superseded record counts once, at its last observed value.
//   - No effort (OpenCode, pre-1.3.0 logs, legacy-only actors) gives empty levels.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const effortRunID = "20261022T120000Z-eff1"

func effortFixtureRun(t *testing.T) domain.RunReport {
	t.Helper()
	run := domain.NamedRun(effortRunID)
	in := analysis.Input{Streams: []analysis.Stream{
		fixtureStream(t, "effort_orchestrator.jsonl", orchStream(run)),
		fixtureStream(t, "effort_agent.jsonl", agentStream(run, "worker#1")),
	}}
	return effortReport(t, in, effortRunID)
}

func requireAgentReport(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.ActorReport {
	t.Helper()
	for _, a := range run.Agents {
		if a.Actor.Instance == id {
			return a
		}
	}
	t.Fatalf("run has no agent report for %s", id)
	return domain.ActorReport{}
}

func TestEffort_OrchestratorMediumSubagentHigh_EachGetsItsOwnLevel(t *testing.T) {
	run := effortFixtureRun(t)

	orch := run.Orchestrator.Effort
	if len(orch) != 1 || orch[0].Level != "medium" {
		t.Errorf("orchestrator effort = %v, want medium only", orch)
	}
	dispatch := requireDispatch(t, requireGroup(t, run, "worker"), "worker#1")
	if len(dispatch.Effort) == 0 || dispatch.Effort[0].Level != "high" {
		t.Errorf("worker#1 dispatch effort = %v, want high first", dispatch.Effort)
	}
}

func TestEffort_SupersededRecords_CountOnceAtTheirLastValue(t *testing.T) {
	run := effortFixtureRun(t)

	// e3 was observed as high, then as medium: three records, all medium.
	if got := run.Orchestrator.Effort; len(got) != 1 || got[0].Records != 3 {
		t.Errorf("orchestrator effort = %v, want [{medium 3}] (e3 superseded counts once, as medium)", got)
	}
	// a1 was observed twice at high: two high records and one medium.
	if got := run.Orchestrator.Effort.Label(); got != "medium" {
		t.Errorf("orchestrator label = %q, want medium", got)
	}
	dispatch := requireDispatch(t, requireGroup(t, run, "worker"), "worker#1")
	if got := dispatch.Effort.Label(); got != "high (2), medium (1)" {
		t.Errorf("worker#1 dispatch label = %q, want %q", got, "high (2), medium (1)")
	}
}

func TestEffort_AgentActorReport_MatchesItsDispatch(t *testing.T) {
	run := effortFixtureRun(t)

	agent := requireAgentReport(t, run, "worker#1")
	dispatch := requireDispatch(t, requireGroup(t, run, "worker"), "worker#1")
	if got := agent.Effort.Label(); got != "high (2), medium (1)" {
		t.Errorf("agent actor effort label = %q, want %q", got, "high (2), medium (1)")
	}
	if agent.Effort.Label() != dispatch.Effort.Label() {
		t.Errorf("actor effort %q differs from dispatch effort %q", agent.Effort.Label(), dispatch.Effort.Label())
	}
}

func TestEffort_MixedLevelsOnOrchestrator_AllLevelsWithCounts(t *testing.T) {
	in := orchestratorOnlyInput(
		v130Record("", "r1", "anthropic", "high", withReasoning(10, domain.AbsentTokens())),
		v130Record("", "r2", "anthropic", "medium", withReasoning(10, domain.AbsentTokens())),
		v130Record("", "r3", "anthropic", "medium", withReasoning(10, domain.AbsentTokens())),
	)

	run := effortReport(t, in, testRunID)

	if got := run.Orchestrator.Effort.Label(); got != "medium (2), high (1)" {
		t.Errorf("orchestrator effort label = %q, want %q", got, "medium (2), high (1)")
	}
}

func TestEffort_OpenCodeFixture_HasNoEffort(t *testing.T) {
	run := domain.NamedRun(v130OpenCodeRun)
	in := analysis.Input{Streams: []analysis.Stream{fixtureStream(t, "v130_opencode.jsonl", orchStream(run))}}

	report := effortReport(t, in, v130OpenCodeRun)

	if !report.Orchestrator.Effort.IsEmpty() {
		t.Errorf("OpenCode orchestrator effort = %v, want empty", report.Orchestrator.Effort)
	}
}

func TestEffort_LegacyOnlyAgent_HasNoEffort(t *testing.T) {
	in := oneAgentStream(testRunRef, "legacy#1",
		rmInvStart("legacy#1", "legacy", rmAt(1)),
		invEndEvent("legacy#1", v130Model, withReasoning(10, domain.AbsentTokens())),
	)

	run := effortReport(t, in, testRunID)

	agent := requireAgentReport(t, run, "legacy#1")
	if !agent.Effort.IsEmpty() {
		t.Errorf("legacy-only agent effort = %v, want empty", agent.Effort)
	}
	dispatch := requireDispatch(t, requireGroup(t, run, "legacy"), "legacy#1")
	if !dispatch.Effort.IsEmpty() {
		t.Errorf("legacy-only dispatch effort = %v, want empty", dispatch.Effort)
	}
}

func TestEffort_PreEffortUsageRecords_HaveNoEffort(t *testing.T) {
	in := orchestratorOnlyInput(
		v130Record("", "r1", "", "", withReasoning(10, domain.AbsentTokens())),
	)

	run := effortReport(t, in, testRunID)

	if !run.Orchestrator.Effort.IsEmpty() {
		t.Errorf("orchestrator effort = %v, want empty when no record carries effort", run.Orchestrator.Effort)
	}
}

func effortReport(t *testing.T, in analysis.Input, runID string) domain.RunReport {
	t.Helper()
	report := analysis.Analyze(in, domain.EmptyPricingTable(), domain.DefaultAnalysisSettings(), testSource(), nil)
	return requireRunReport(t, report, runID)
}
