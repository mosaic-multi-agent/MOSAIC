package transcript_test

// Tests for reading Claude Code JSONL transcripts through transcript.Reader.
//
// Rules under test:
//   - Assistant ids are message.id when a non-empty string, else the record uuid;
//     records with neither are ignored; ids are distinct, in first-seen order.
//   - The first user record gives the first prompt: a string content as is, a block
//     list as its text blocks joined by a newline. If that record has neither
//     form, there is no prompt and no later record is used.
//   - The orchestrator transcript at the run root yields assistant ids only.
//   - Very long lines do not fail the read; unparseable lines are skipped.
//   - A missing or unreadable raw file yields no entry.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/transcript"
)

const agentHint = "implementation-tdd#28"

func TestTranscripts_ClaudeAgent_ReturnsDistinctAssistantIDsInFirstSeenOrder(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, agentHint)
	want := []string{"msg_A", "uuid-only-1", "uuid-empty-id", "msg_B"}
	if !equalStrings(f.AssistantIDs, want) {
		t.Errorf("AssistantIDs = %v, want %v", f.AssistantIDs, want)
	}
}

func TestTranscripts_ClaudeAgent_FillsKeyFields(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, agentHint)
	if f.Run != domain.NamedRun(runA) || f.Format != domain.TranscriptClaudeCodeJSONL {
		t.Errorf("run/format = %+v / %q", f.Run, f.Format)
	}
	if f.Path != filepath.Join(agentDir(run, agentHint), "04_session.raw") {
		t.Errorf("Path = %q, want the raw transcript file", f.Path)
	}
}

func TestTranscripts_ClaudeAgent_FirstPromptFromStringContent(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, agentHint)
	if !f.HasFirstPrompt || f.FirstPrompt != "agent_instance_id: implementation-tdd#28\nWrite the parser." {
		t.Errorf("prompt = %q (has=%v), want the first user record's string content", f.FirstPrompt, f.HasFirstPrompt)
	}
}

func TestTranscripts_ClaudeAgent_FirstPromptFromTextBlocksJoinedByNewline(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), "reviewer#3", fixtureBytes(t, "claude_agent_blocks.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, "reviewer#3")
	want := "agent_instance_id: reviewer#3\nReview the change."
	if !f.HasFirstPrompt || f.FirstPrompt != want {
		t.Errorf("prompt = %q (has=%v), want %q", f.FirstPrompt, f.HasFirstPrompt, want)
	}
	if !equalStrings(f.AssistantIDs, []string{"msg_X"}) {
		t.Errorf("AssistantIDs = %v, want [msg_X]", f.AssistantIDs)
	}
}

func TestTranscripts_ClaudeAgent_UnusableFirstUserRecordGivesNoPromptAndNoFallback(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent_unusable_first_user.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, agentHint)
	if f.HasFirstPrompt || f.FirstPrompt != "" {
		t.Errorf("prompt = %q (has=%v), want none", f.FirstPrompt, f.HasFirstPrompt)
	}
	if !equalStrings(f.AssistantIDs, []string{"msg_Y"}) {
		t.Errorf("AssistantIDs = %v, want [msg_Y]", f.AssistantIDs)
	}
}

func TestTranscripts_ClaudeOrchestrator_ReturnsAssistantIDsOnly(t *testing.T) {
	root := t.TempDir()
	run := withOrchestrator(t, newRun(t, root, runA), fixtureBytes(t, "claude_orchestrator.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 {
		t.Fatalf("facts = %+v, want exactly the orchestrator entry", got)
	}
	f := got[0]
	if f.Owner != domain.StreamOrchestrator || f.InstanceHint != "" || f.Run != domain.NamedRun(runA) {
		t.Errorf("owner/hint/run = %v / %q / %+v", f.Owner, f.InstanceHint, f.Run)
	}
	if f.Path != filepath.Join(run.Dir, "00_orchestrator_session.raw") {
		t.Errorf("Path = %q", f.Path)
	}
	if !equalStrings(f.AssistantIDs, []string{"msg_O1", "msg_O2", "o-uuid-3"}) {
		t.Errorf("AssistantIDs = %v, want [msg_O1 msg_O2 o-uuid-3]", f.AssistantIDs)
	}
	if f.HasFirstPrompt || f.FirstPrompt != "" {
		t.Errorf("orchestrator must carry no prompt, got %q (has=%v)", f.FirstPrompt, f.HasFirstPrompt)
	}
}

func TestTranscripts_ClaudeAgent_VeryLongLineDoesNotFailTheRead(t *testing.T) {
	root := t.TempDir()
	huge := strings.Repeat("x", 3<<20)
	content := `{"type":"user","uuid":"u1","message":{"role":"user","content":"` + huge + `"}}` + "\n" +
		`{"type":"assistant","uuid":"a1","message":{"id":"msg_after_long","role":"assistant","content":[]}}` + "\n"
	run := withAgent(t, newRun(t, root, runA), agentHint, []byte(content), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	f := findAgent(t, got, agentHint)
	if !equalStrings(f.AssistantIDs, []string{"msg_after_long"}) {
		t.Errorf("AssistantIDs = %v, want the record after the long line", f.AssistantIDs)
	}
	if !f.HasFirstPrompt || len(f.FirstPrompt) != len(huge) {
		t.Errorf("first prompt length = %d (has=%v), want %d", len(f.FirstPrompt), f.HasFirstPrompt, len(huge))
	}
}

func TestTranscripts_ClaudeAgent_MissingRawFileYieldsNoEntry(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	if err := os.Remove(filepath.Join(agentDir(run, agentHint), "04_session.raw")); err != nil {
		t.Fatalf("remove raw: %v", err)
	}

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 0 {
		t.Errorf("facts = %+v, want none for a missing raw file", got)
	}
}

func TestTranscripts_ClaudeAgent_UnreadableRawFileYieldsNoEntryAndOthersStillRead(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	run = withAgent(t, run, "reviewer#3", fixtureBytes(t, "claude_agent_blocks.jsonl"), claudeFormat)
	// Replace the first agent's raw file with a directory: it cannot be read as a file.
	raw := filepath.Join(agentDir(run, agentHint), "04_session.raw")
	if err := os.Remove(raw); err != nil {
		t.Fatalf("remove raw: %v", err)
	}
	if err := os.Mkdir(raw, 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 || got[0].InstanceHint != "reviewer#3" {
		t.Errorf("facts = %+v, want only the readable reviewer#3 entry", got)
	}
}

func TestTranscripts_OrdersByRunThenOrchestratorThenAgentHint(t *testing.T) {
	root := t.TempDir()
	a := newRun(t, root, runA)
	a = withAgent(t, a, "reviewer#3", fixtureBytes(t, "claude_agent_blocks.jsonl"), claudeFormat)
	a = withAgent(t, a, agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	a = withOrchestrator(t, a, fixtureBytes(t, "claude_orchestrator.jsonl"), claudeFormat)
	b := withOrchestrator(t, newRun(t, root, runB), fixtureBytes(t, "claude_orchestrator.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, a, b))

	type key struct {
		run   string
		owner domain.StreamKind
		hint  domain.AgentInstanceID
	}
	want := []key{
		{runA, domain.StreamOrchestrator, ""},
		{runA, domain.StreamAgentInstance, "implementation-tdd#28"},
		{runA, domain.StreamAgentInstance, "reviewer#3"},
		{runB, domain.StreamOrchestrator, ""},
	}
	if len(got) != len(want) {
		t.Fatalf("facts = %+v, want %d entries", got, len(want))
	}
	for i, w := range want {
		if got[i].Run.ID != w.run || got[i].Owner != w.owner || got[i].InstanceHint != w.hint {
			t.Errorf("entry %d = %s/%v/%q, want %s/%v/%q", i, got[i].Run.ID, got[i].Owner, got[i].InstanceHint, w.run, w.owner, w.hint)
		}
	}
}
