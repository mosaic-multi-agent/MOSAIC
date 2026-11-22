package analysis_test

// Tests for the instance prompt mismatch check on in-memory transcript facts.
//
// Rules under test:
//   - An agent whose own first prompt names (structured form only) a different
//     instance id than its folder gives one finding for the folder id, for
//     claude-code and opencode transcripts.
//   - Its own id, no id, a bare Name#N, other formats, orchestrator transcripts and
//     facts with no first prompt give none.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func ipFacts(format domain.TranscriptFormat, prompt string) domain.TranscriptFacts {
	return domain.TranscriptFacts{
		Run: testRunRef, Owner: domain.StreamAgentInstance, InstanceHint: tgFolder,
		Path: tgRawPath, Format: format, FirstPrompt: prompt, HasFirstPrompt: true,
	}
}

func TestInstancePromptMismatch_FirstPromptNamesAnotherId_OneFindingForTheFolderId(t *testing.T) {
	formats := map[string]domain.TranscriptFormat{
		"claude-code": domain.TranscriptClaudeCodeJSONL,
		"opencode":    domain.TranscriptOpenCodeMessages,
	}
	for name, format := range formats {
		t.Run(name, func(t *testing.T) {
			facts := ipFacts(format, "agent_instance_id: coder#8\nDo the work.")

			got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts})

			if len(got) != 1 {
				t.Fatalf("findings = %+v, want exactly one", got)
			}
			f := got[0]
			if f.Kind != domain.FindingInstancePromptMismatch || f.Severity != domain.SeverityWarning {
				t.Errorf("finding = %+v, want an instance_prompt_mismatch warning", f)
			}
			if f.Run != testRunRef || f.Path != tgRawPath || f.Actor != domain.AgentInstance(tgFolder) {
				t.Errorf("run/path/actor = %+v / %q / %+v, want %+v / %q / the folder id %q",
					f.Run, f.Path, f.Actor, testRunRef, tgRawPath, tgFolder)
			}
			if !strings.Contains(f.Detail, string(tgOtherAgent)) {
				t.Errorf("detail %q does not name the prompt's id %q", f.Detail, tgOtherAgent)
			}
		})
	}
}

func TestInstancePromptMismatch_FirstPromptNamesTheFolderId_NoFinding(t *testing.T) {
	facts := ipFacts(domain.TranscriptClaudeCodeJSONL, "agent_instance_id: "+string(tgFolder)+"\nDo the work.")

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestInstancePromptMismatch_FirstPromptWithoutAnId_IsSkipped(t *testing.T) {
	facts := ipFacts(domain.TranscriptOpenCodeMessages, "Do the work, no id here.")

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestInstancePromptMismatch_BareNameHashNumber_IsIgnored(t *testing.T) {
	facts := ipFacts(domain.TranscriptClaudeCodeJSONL, "Please hand over to coder#8 when done.")

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none: only the agent_instance_id form counts", got)
	}
}

func TestInstancePromptMismatch_FirstOccurrenceDecides(t *testing.T) {
	facts := ipFacts(domain.TranscriptClaudeCodeJSONL,
		"agent_instance_id: "+string(tgFolder)+"\nlater: agent_instance_id: coder#8")

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none: the first structured id is the folder's own", got)
	}
}

func TestInstancePromptMismatch_FactsWithoutAFirstPrompt_AreSkipped(t *testing.T) {
	facts := ipFacts(domain.TranscriptClaudeCodeJSONL, "agent_instance_id: coder#8")
	facts.HasFirstPrompt = false

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestInstancePromptMismatch_OtherFormats_AreNotCompared(t *testing.T) {
	facts := ipFacts("some-other-format", "agent_instance_id: coder#8")

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestInstancePromptMismatch_OrchestratorTranscript_IsNotCompared(t *testing.T) {
	facts := ipFacts(domain.TranscriptClaudeCodeJSONL, "agent_instance_id: coder#8")
	facts.Owner = domain.StreamOrchestrator
	facts.InstanceHint = ""

	if got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{facts}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestInstancePromptMismatch_SeveralAgents_OneFindingPerMismatchingInvocation(t *testing.T) {
	a := ipFacts(domain.TranscriptClaudeCodeJSONL, "agent_instance_id: coder#8")
	b := ipFacts(domain.TranscriptOpenCodeMessages, "agent_instance_id: coder#8")
	b.InstanceHint = "reviewer#2"
	c := ipFacts(domain.TranscriptClaudeCodeJSONL, "agent_instance_id: tester#4")
	c.InstanceHint = "tester#4"

	got := analysis.InstancePromptMismatchFindings([]domain.TranscriptFacts{a, b, c})

	if len(got) != 2 {
		t.Fatalf("findings = %+v, want two", got)
	}
	if got[0].Actor != domain.AgentInstance(tgFolder) || got[1].Actor != domain.AgentInstance("reviewer#2") {
		t.Errorf("actors = %+v / %+v, want the folder ids in input order", got[0].Actor, got[1].Actor)
	}
}
