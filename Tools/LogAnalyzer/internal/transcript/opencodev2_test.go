package transcript_test

// Tests for reading OpenCode V2 session exports (`session export` shape
// {info, messages}) through transcript.Reader.
//
// Rules under test:
//   - A transcript whose sidecar names opencode-v2-session-export-json is read,
//     with the V2 format and no assistant ids.
//   - The first prompt of an agent transcript is the text of the first message
//     of type "user"; other message types are skipped, later user messages are
//     not used.
//   - One leading "You are a subagent spawned by another session.\n" is removed;
//     nothing else is rewritten.
//   - No user message, a first user message without a string text, and a
//     malformed document give an entry with no prompt, never an error.
//   - Key order does not matter, large unrelated values are skipped and reading
//     stops after the first user message.
//   - Orchestrator transcripts carry no prompt; runs without transcript files
//     (Runner mode) give no entry.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/transcript"
)

const (
	openCodeV2Format = "opencode-v2-session-export-json"
	subagentPrefix   = "You are a subagent spawned by another session.\n"
)

func openCodeV2Facts(t *testing.T, content []byte) domain.TranscriptFacts {
	t.Helper()
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, content, openCodeV2Format)
	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))
	return findAgent(t, got, agentHint)
}

func assertV2Prompt(t *testing.T, f domain.TranscriptFacts, want string) {
	t.Helper()
	if !f.HasFirstPrompt || f.FirstPrompt != want {
		t.Errorf("prompt = %q (has=%v), want %q", f.FirstPrompt, f.HasFirstPrompt, want)
	}
}

func assertNoV2Prompt(t *testing.T, f domain.TranscriptFacts) {
	t.Helper()
	if f.HasFirstPrompt || f.FirstPrompt != "" {
		t.Errorf("prompt = %q (has=%v), want none", f.FirstPrompt, f.HasFirstPrompt)
	}
	if f.Format != domain.TranscriptOpenCodeV2Export || f.Run != domain.NamedRun(runA) {
		t.Errorf("an entry with the V2 format and run must still be returned, got %+v", f)
	}
}

func TestTranscripts_OpenCodeV2_ChildExportYieldsPromptWithoutTheSubagentPrefix(t *testing.T) {
	f := openCodeV2Facts(t, fixtureBytes(t, "opencodev2_child_export.json"))

	assertV2Prompt(t, f, "Your name is BETA. Step 1: run the shell command ping -n 20 127.0.0.1. "+
		"Step 2: run echo beta. Step 3: read README.md.")
	if f.Format != domain.TranscriptOpenCodeV2Export {
		t.Errorf("Format = %q, want %q", f.Format, domain.TranscriptOpenCodeV2Export)
	}
	if f.AssistantIDs != nil {
		t.Errorf("AssistantIDs = %v, want nil for opencode-v2", f.AssistantIDs)
	}
}

func TestTranscripts_OpenCodeV2_ParentExportSkipsLeadingAgentSwitchedMessage(t *testing.T) {
	f := openCodeV2Facts(t, fixtureBytes(t, "opencodev2_parent_export.json"))

	if !f.HasFirstPrompt || !strings.HasPrefix(f.FirstPrompt, "Dispatch two spike-child subagents in parallel.") {
		t.Errorf("prompt = %q (has=%v), want the text of the first user message", f.FirstPrompt, f.HasFirstPrompt)
	}
	if strings.HasPrefix(f.FirstPrompt, subagentPrefix) {
		t.Error("a prompt without the subagent prefix must be returned unchanged")
	}
}

func TestTranscripts_OpenCodeV2_OnlyTheLeadingPrefixIsRemovedAndOnlyOnce(t *testing.T) {
	cases := map[string]struct{ text, want string }{
		"prefix twice":           {subagentPrefix + subagentPrefix + "Work.", subagentPrefix + "Work."},
		"prefix in the middle":   {"Intro. " + subagentPrefix + "Work.", "Intro. " + subagentPrefix + "Work."},
		"prefix without newline": {"You are a subagent spawned by another session. Work.", "You are a subagent spawned by another session. Work."},
		"prefix only":            {subagentPrefix, ""},
		"no prefix":              {"agent_instance_id: coder#8\nWork.", "agent_instance_id: coder#8\nWork."},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := `{"info":{"id":"s"},"messages":[{"type":"user","text":` + jsonString(c.text) + `}]}`

			f := openCodeV2Facts(t, []byte(doc))

			assertV2Prompt(t, f, c.want)
		})
	}
}

func TestTranscripts_OpenCodeV2_PromptWithInstanceIdAfterPrefixIsKeptWhole(t *testing.T) {
	doc := `{"messages":[{"type":"user","text":` + jsonString(subagentPrefix+"agent_instance_id: coder#8\nWork.") + `}]}`

	f := openCodeV2Facts(t, []byte(doc))

	assertV2Prompt(t, f, "agent_instance_id: coder#8\nWork.")
}

func TestTranscripts_OpenCodeV2_NonUserMessagesWithTextAreSkipped(t *testing.T) {
	doc := `{"info":{"id":"s"},"messages":[` +
		`{"type":"synthetic","text":"not it"},{"text":"also not it","type":"assistant"},` +
		`{"type":"idle"},{"type":"user","text":"the prompt"},{"type":"user","text":"later"}]}`

	f := openCodeV2Facts(t, []byte(doc))

	assertV2Prompt(t, f, "the prompt")
}

func TestTranscripts_OpenCodeV2_MessagesBeforeInfoAndTextBeforeTypeStillFindPrompt(t *testing.T) {
	doc := `{"messages":[{"text":"text first","id":"m1","type":"user"}],"info":{"id":"s"}}`

	f := openCodeV2Facts(t, []byte(doc))

	assertV2Prompt(t, f, "text first")
}

func TestTranscripts_OpenCodeV2_LargeUnrelatedFieldsAreSkipped(t *testing.T) {
	big := strings.Repeat("x", 5<<20)
	doc := `{"info":{"id":"s","blob":"` + big + `"},"messages":[` +
		`{"type":"assistant","content":[{"type":"text","text":"` + big + `"}]},` +
		`{"type":"user","text":"after the large values"}]}`

	f := openCodeV2Facts(t, []byte(doc))

	assertV2Prompt(t, f, "after the large values")
}

func TestTranscripts_OpenCodeV2_ReadingStopsAfterTheFirstUserMessage(t *testing.T) {
	doc := `{"messages":[{"type":"user","text":"early prompt"},{"type":"assistant", "content": [ BROKEN`

	f := openCodeV2Facts(t, []byte(doc))

	assertV2Prompt(t, f, "early prompt")
}

func TestTranscripts_OpenCodeV2_NoUserMessageYieldsEntryWithoutPrompt(t *testing.T) {
	doc := `{"info":{"id":"s"},"messages":[{"type":"agent-switched","agent":"a"},{"type":"assistant","text":"hi"}]}`

	assertNoV2Prompt(t, openCodeV2Facts(t, []byte(doc)))
}

func TestTranscripts_OpenCodeV2_FirstUserMessageWithoutStringTextYieldsNoPromptAndSkipsLaterOnes(t *testing.T) {
	cases := map[string]string{
		"text missing":   `{"messages":[{"type":"user","files":[]},{"type":"user","text":"later"}]}`,
		"text is number": `{"messages":[{"type":"user","text":42},{"type":"user","text":"later"}]}`,
		"text is object": `{"messages":[{"type":"user","text":{"a":"b"}},{"type":"user","text":"later"}]}`,
		"text is null":   `{"messages":[{"type":"user","text":null},{"type":"user","text":"later"}]}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			assertNoV2Prompt(t, openCodeV2Facts(t, []byte(doc)))
		})
	}
}

func TestTranscripts_OpenCodeV2_MalformedDocumentsYieldEntryWithoutPrompt(t *testing.T) {
	cases := map[string]string{
		"not json":          `this is not json`,
		"empty file":        ``,
		"truncated":         `{"info":{"id":"s"},"messages":[{"type":"us`,
		"array at the root": `[{"type":"user","text":"p"}]`,
		"messages not list": `{"messages":{"type":"user","text":"p"}}`,
		"no messages key":   `{"info":{"id":"s"}}`,
		"v1 data shape":     `{"data":[{"info":{"role":"user"},"parts":[{"type":"text","text":"p"}]}]}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			f := openCodeV2Facts(t, []byte(doc))

			assertNoV2Prompt(t, f)
			if f.AssistantIDs != nil {
				t.Errorf("AssistantIDs = %v, want nil", f.AssistantIDs)
			}
		})
	}
}

func TestTranscripts_OpenCodeV2_OrchestratorTranscriptCarriesNoPrompt(t *testing.T) {
	root := t.TempDir()
	run := withOrchestrator(t, newRun(t, root, runA), fixtureBytes(t, "opencodev2_parent_export.json"), openCodeV2Format)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 || got[0].Owner != domain.StreamOrchestrator || got[0].Format != domain.TranscriptOpenCodeV2Export {
		t.Fatalf("facts = %+v, want one orchestrator opencode-v2 entry", got)
	}
	if got[0].HasFirstPrompt || got[0].AssistantIDs != nil {
		t.Errorf("orchestrator entry = %+v, want no prompt and no assistant ids", got[0])
	}
}

func TestTranscripts_OpenCodeV2_RunWithoutTranscriptFilesYieldsNoEntry(t *testing.T) {
	root := t.TempDir()
	run := newRun(t, root, runA)
	writeFile(t, agentDir(run, agentHint)+"/03_events.jsonl", []byte("{}\n"))
	run.Agents = append(run.Agents, domain.AgentEntry{Dir: agentDir(run, agentHint), InstanceHint: domain.AgentInstanceID(agentHint)})

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 0 {
		t.Errorf("facts = %+v, want none for a run without transcripts", got)
	}
}

func TestTranscripts_OpenCodeV2_V1FormatDocumentUnderV2SidecarIsNotReadAsV1(t *testing.T) {
	f := openCodeV2Facts(t, fixtureBytes(t, "opencode_user_first.json"))

	assertNoV2Prompt(t, f)
}

// jsonString quotes s as a JSON string.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
