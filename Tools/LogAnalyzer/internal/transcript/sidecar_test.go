package transcript_test

// Tests for sidecar handling and source kinds of transcript.Reader.
//
// Rules under test:
//   - An unknown native_format, a missing sidecar and an unparseable sidecar yield
//     no entry and no failure; other transcripts of the run are still read.
//   - Digest sources, unusable sources and the unattributable bucket yield nothing.
//   - A single-run source is read like a logs root.
//   - A cancelled context returns what was gathered so far, never failing.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/transcript"
)

func TestTranscripts_UnknownNativeFormatYieldsNoEntry(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), "ghcp-cli-session-jsonl")

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 0 {
		t.Errorf("facts = %+v, want none for an unknown format", got)
	}
}

func TestTranscripts_MissingSidecarYieldsNoEntry(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	if err := os.Remove(filepath.Join(agentDir(run, agentHint), "04_session.meta.json")); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 0 {
		t.Errorf("facts = %+v, want none without a sidecar", got)
	}
}

func TestTranscripts_UnparseableSidecarYieldsNoEntryOthersStillRead(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	run = withAgent(t, run, "reviewer#3", fixtureBytes(t, "claude_agent_blocks.jsonl"), claudeFormat)
	writeFile(t, filepath.Join(agentDir(run, agentHint), "04_session.meta.json"), []byte("{not json"))

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 || got[0].InstanceHint != "reviewer#3" {
		t.Errorf("facts = %+v, want only reviewer#3", got)
	}
}

func TestTranscripts_OrchestratorUnknownFormatYieldsNoEntryButAgentsStillRead(t *testing.T) {
	root := t.TempDir()
	run := withOrchestrator(t, newRun(t, root, runA), fixtureBytes(t, "claude_orchestrator.jsonl"), "something-else")
	run = withAgent(t, run, agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 || got[0].Owner != domain.StreamAgentInstance {
		t.Errorf("facts = %+v, want only the agent entry", got)
	}
}

func TestTranscripts_OrchestratorOpenCodeFormatIsRead(t *testing.T) {
	root := t.TempDir()
	run := withOrchestrator(t, newRun(t, root, runA), fixtureBytes(t, "opencode_user_first.json"), openCodeFormat)

	got := transcript.NewReader().Transcripts(context.Background(), logsRoot(root, run))

	if len(got) != 1 || got[0].Owner != domain.StreamOrchestrator || got[0].Format != domain.TranscriptOpenCodeMessages {
		t.Fatalf("facts = %+v, want one orchestrator opencode entry", got)
	}
	if got[0].HasFirstPrompt {
		t.Errorf("orchestrator must carry no prompt, got %q", got[0].FirstPrompt)
	}
}

func TestTranscripts_SingleRunSourceIsRead(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	inv := domain.Inventory{
		Source: domain.Source{Kind: domain.SourceSingleRun, Path: run.Dir},
		Runs:   []domain.RunEntry{run},
	}

	got := transcript.NewReader().Transcripts(context.Background(), inv)

	if len(got) != 1 {
		t.Errorf("facts = %+v, want the agent entry of the single run", got)
	}
}

func TestTranscripts_NonLogSourceKindsYieldNothing(t *testing.T) {
	kinds := map[string]domain.SourceKind{
		"digest file": domain.SourceDigestFile,
		"digest dir":  domain.SourceDigestDir,
		"unusable":    domain.SourceUnusable,
		"not found":   domain.SourceNotFound,
	}
	for name, kind := range kinds {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
			inv := logsRoot(root, run)
			inv.Source.Kind = kind

			got := transcript.NewReader().Transcripts(context.Background(), inv)

			if got != nil {
				t.Errorf("facts = %+v, want nil for source kind %s", got, name)
			}
		})
	}
}

func TestTranscripts_UnattributableBucketIsNeverRead(t *testing.T) {
	root := t.TempDir()
	bucket := withOrchestrator(t, newRun(t, root, "unknown-run"), fixtureBytes(t, "claude_orchestrator.jsonl"), claudeFormat)
	bucket.Run = domain.RunRef{Kind: domain.RunUnattributable}
	bucket = withAgent(t, bucket, agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	inv := logsRoot(root)
	inv.Unattributable = &bucket

	got := transcript.NewReader().Transcripts(context.Background(), inv)

	if len(got) != 0 {
		t.Errorf("facts = %+v, want none from the unattributable bucket", got)
	}
}

func TestTranscripts_CancelledContextReturnsWithoutFailing(t *testing.T) {
	root := t.TempDir()
	run := withAgent(t, newRun(t, root, runA), agentHint, fixtureBytes(t, "claude_agent.jsonl"), claudeFormat)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := transcript.NewReader().Transcripts(ctx, logsRoot(root, run))

	if len(got) > 1 {
		t.Errorf("facts = %+v, want at most the entries gathered before cancellation", got)
	}
}
