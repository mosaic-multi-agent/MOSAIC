package analysis_test

// Tests for opencode-v2 in the analysis layer, on in-memory facts and on the
// opencodev2.jsonl fixture read through the real reader.
//
// Rules under test:
//   - opencode-v2 events raise no unrecognised-harness finding and their
//     usage_record tokens are counted.
//   - A subagent dispatch of a V2 log names the dispatched instance and feeds
//     the dispatch-without-invocation check; a re-call by session id and
//     other tools name no instance.
//   - V2 agent transcripts are compared by the instance prompt mismatch check;
//     V2 transcripts are never compared by the capture gap check.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const v2FixtureRun = "20261110T110000Z-v2aa"

func v2FixtureOrchestrator(t *testing.T) analysis.Stream {
	t.Helper()
	return fixtureStream(t, "opencodev2.jsonl", orchStream(domain.NamedRun(v2FixtureRun)))
}

func TestAggregate_OpenCodeV2Harness_RaisesNoUnrecognisedHarnessFinding(t *testing.T) {
	_, findings := analysis.Aggregate(orchestratorOnlyInput(usageEventWithHarness(domain.HarnessOpenCodeV2)))

	if hasFindingKind(findings, domain.FindingUnrecognisedHarness) {
		t.Errorf("opencode-v2 must not raise FindingUnrecognisedHarness; got: %v", findings)
	}
}

func TestAggregate_OpenCodeV2Fixture_CountsUsageRecordTokensWithoutFindings(t *testing.T) {
	agg, findings := analysis.Aggregate(analysis.Input{Streams: []analysis.Stream{v2FixtureOrchestrator(t)}})

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none for an opencode-v2 run", findings)
	}
	units := requireOneRun(t, agg).Orchestrator.Invocations
	if len(units) != 1 {
		t.Fatalf("got %d usage units, want 1", len(units))
	}
	requireUsageUnit(t, units[0], "openai", "")
	assertTokenPresent(t, "Input", units[0].Usage.Input, 300)
	assertTokenPresent(t, "Output", units[0].Usage.Output, 70)
	assertTokenPresent(t, "Reasoning", units[0].Usage.Reasoning, 20)
}

func TestDispatchWithoutInvocation_OpenCodeV2SubagentDispatch_OneFindingForTheDispatchedId(t *testing.T) {
	orch := v2FixtureOrchestrator(t)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch}, domain.Aggregate{})

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one: only the subagent dispatch names an instance", got)
	}
	if got[0].Kind != domain.FindingDispatchWithoutInvocation || !strings.Contains(got[0].Detail, "coder#7") {
		t.Errorf("finding = %+v, want a dispatch_without_invocation naming coder#7", got[0])
	}
}

func TestDispatchWithoutInvocation_OpenCodeV2SubagentDispatchWithFolder_NoFinding(t *testing.T) {
	orch := v2FixtureOrchestrator(t)
	folder := osStream("coder#7")
	folder.Ref.Run = domain.NamedRun(v2FixtureRun)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch, folder}, domain.Aggregate{})

	if len(got) != 0 {
		t.Errorf("findings = %+v, want none when the dispatched instance has a folder", got)
	}
}

func v2Facts(prompt string) domain.TranscriptFacts {
	return ipFacts(domain.TranscriptOpenCodeV2Export, prompt)
}

func TestInstancePromptMismatch_OpenCodeV2_FirstPromptNamesAnotherId_OneFindingForTheFolderId(t *testing.T) {
	got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{v2Facts("agent_instance_id: coder#8\nDo the work.")})

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingInstancePromptMismatch || f.Actor != domain.AgentInstance(tgFolder) || f.Path != tgRawPath {
		t.Errorf("finding = %+v, want an instance_prompt_mismatch for the folder id at the transcript path", f)
	}
	if !strings.Contains(f.Detail, string(tgOtherAgent)) {
		t.Errorf("detail %q does not name the prompt's id %q", f.Detail, tgOtherAgent)
	}
}

func TestInstancePromptMismatch_OpenCodeV2_OwnIdOrNoIdOrNoPrompt_NoFinding(t *testing.T) {
	noPrompt := v2Facts("agent_instance_id: coder#8")
	noPrompt.HasFirstPrompt = false
	orchestrator := v2Facts("agent_instance_id: coder#8")
	orchestrator.Owner, orchestrator.InstanceHint = domain.StreamOrchestrator, ""
	cases := map[string]domain.TranscriptFacts{
		"own id":       v2Facts("agent_instance_id: " + string(tgFolder) + "\nWork."),
		"no id":        v2Facts("Do the work, no id here."),
		"no prompt":    noPrompt,
		"orchestrator": orchestrator,
	}
	for name, facts := range cases {
		t.Run(name, func(t *testing.T) {
			if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
				t.Errorf("findings = %+v, want none", got)
			}
		})
	}
}

func TestCaptureGap_OpenCodeV2Transcripts_AreNotCompared(t *testing.T) {
	s := tgAgent(tgFolder, "x1", "x2")
	agent := tgAgentFacts(tgFolder)
	agent.Format = domain.TranscriptOpenCodeV2Export
	orch := tgOrchFacts()
	orch.Format = domain.TranscriptOpenCodeV2Export

	if got := tgGaps([]analysis.Stream{s, tgOrchestrator("o1")}, agent, orch); len(got) != 0 {
		t.Errorf("findings = %+v, want none: capture gaps apply to claude-code transcripts only", got)
	}
}
