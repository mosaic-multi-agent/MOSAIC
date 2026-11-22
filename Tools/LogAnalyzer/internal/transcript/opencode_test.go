package transcript_test

// Tests for reading OpenCode session-message documents through transcript.Reader.
//
// Rules under test:
//   - The first prompt is the text parts of the first message in data whose
//     info.role is user, joined by a newline; assistant messages before it are
//     skipped; later user messages are not used.
//   - Large unrelated values (info.summary.diffs) do not disturb the result.
//   - A document with no user message, or one that is malformed, yields an entry
//     with no prompt (and no assistant ids).

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/transcript"
)

func openCodeFacts(t *testing.T, content []byte, hint string) domain.TranscriptFacts {
	t.Helper()
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), hint, content, openCodeFormat)
	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))
	return findAgent(t, got, hint)
}

func TestTranscripts_OpenCode_FirstUserMessageTextPartsJoinedByNewline(t *testing.T) {
	f := openCodeFacts(t, fixtureBytes(t, "opencode_user_first.json"), agentHint)

	want := "agent_instance_id: implementation-tdd#28\nWrite the parser."
	if !f.HasFirstPrompt || f.FirstPrompt != want {
		t.Errorf("prompt = %q (has=%v), want %q", f.FirstPrompt, f.HasFirstPrompt, want)
	}
	if f.Format != domain.TranscriptOpenCodeMessages {
		t.Errorf("Format = %q", f.Format)
	}
	if f.AssistantIDs != nil {
		t.Errorf("AssistantIDs = %v, want nil for opencode", f.AssistantIDs)
	}
}

func TestTranscripts_OpenCode_SkipsLeadingAssistantMessage(t *testing.T) {
	f := openCodeFacts(t, fixtureBytes(t, "opencode_assistant_first.json"), "reviewer#3")

	want := "agent_instance_id: reviewer#3\nReview the change."
	if !f.HasFirstPrompt || f.FirstPrompt != want {
		t.Errorf("prompt = %q (has=%v), want %q", f.FirstPrompt, f.HasFirstPrompt, want)
	}
}

func TestTranscripts_OpenCode_LargeUnrelatedFieldInFirstMessageIsSkipped(t *testing.T) {
	bigDiff := strings.Repeat("d", 5<<20)
	doc := `{"data":[{"info":{"id":"m1","role":"user","summary":{"diffs":[{"after":"` + bigDiff + `"}]}},` +
		`"parts":[{"type":"text","text":"the prompt"}]}]}`

	f := openCodeFacts(t, []byte(doc), agentHint)

	if !f.HasFirstPrompt || f.FirstPrompt != "the prompt" {
		t.Errorf("prompt = %q (has=%v), want the text after the large field", f.FirstPrompt, f.HasFirstPrompt)
	}
}

func TestTranscripts_OpenCode_PartsBeforeInfoStillFindPrompt(t *testing.T) {
	doc := `{"data":[{"parts":[{"type":"text","text":"parts first"}],"info":{"id":"m1","role":"user"}}]}`

	f := openCodeFacts(t, []byte(doc), agentHint)

	if !f.HasFirstPrompt || f.FirstPrompt != "parts first" {
		t.Errorf("prompt = %q (has=%v), want %q", f.FirstPrompt, f.HasFirstPrompt, "parts first")
	}
}

func TestTranscripts_OpenCode_NoUserMessageYieldsEntryWithoutPrompt(t *testing.T) {
	f := openCodeFacts(t, fixtureBytes(t, "opencode_no_user.json"), agentHint)

	if f.HasFirstPrompt || f.FirstPrompt != "" {
		t.Errorf("prompt = %q (has=%v), want none", f.FirstPrompt, f.HasFirstPrompt)
	}
	if f.Format != domain.TranscriptOpenCodeMessages || f.Run != domain.NamedRun(runA) {
		t.Errorf("entry must still be returned with format and run, got %+v", f)
	}
}

func TestTranscripts_OpenCode_MalformedDocumentYieldsEntryWithoutPrompt(t *testing.T) {
	f := openCodeFacts(t, fixtureBytes(t, "opencode_malformed.json"), agentHint)

	if f.HasFirstPrompt || f.FirstPrompt != "" || f.AssistantIDs != nil {
		t.Errorf("facts = %+v, want no prompt and no ids", f)
	}
}
