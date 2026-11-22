package app_test

// In-memory transcript port fake and a scenario builder for the transcript
// integrity tests.

import (
	"context"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// MockTranscriptSource implements domain.TranscriptSource. Like the real
// adapter it answers only for the runs of the inventory it is given and only
// for log sources, and it records every call.
type MockTranscriptSource struct {
	facts []domain.TranscriptFacts
	calls []domain.Inventory
}

func (m *MockTranscriptSource) Transcripts(_ context.Context, inv domain.Inventory) []domain.TranscriptFacts {
	m.calls = append(m.calls, inv)
	if inv.Source.Kind != domain.SourceLogsRoot && inv.Source.Kind != domain.SourceSingleRun {
		return nil
	}
	inRun := make(map[domain.RunRef]bool, len(inv.Runs))
	for _, r := range inv.Runs {
		inRun[r.Run] = true
	}
	var out []domain.TranscriptFacts
	for _, f := range m.facts {
		if inRun[f.Run] {
			out = append(out, f)
		}
	}
	return out
}

var _ domain.TranscriptSource = (*MockTranscriptSource)(nil)

const (
	tcAgent = domain.AgentInstanceID("planner#9")
	tcOther = domain.AgentInstanceID("coder#8")
)

// tcService builds the env's service with the transcript port wired.
func tcService(e *digestEnv, port domain.TranscriptSource) *app.Service {
	return app.New(app.Deps{
		Source:      e.logSource,
		Reader:      e.reader,
		Pricing:     e.pricing,
		Digests:     e.digests,
		Clock:       e.clock,
		Interaction: e.interaction,
		WorkDir:     "/workdir",
		ToolVersion: e.toolVersion,
		Transcripts: port,
	})
}

func tcUsageRecord(recordID string) domain.Event {
	return domain.Event{
		Type: domain.EventUsageRecord,
		UsageRecord: &domain.UsageRecordFields{
			RecordID: recordID, Model: dgModel, Usage: igUsage(), HasUsage: true,
		},
	}
}

// tcAddAgent adds an agent folder of the first logs run with one invocation
// start and the given usage records (and a closing end).
func tcAddAgent(e *digestEnv, folder domain.AgentInstanceID, recordIDs ...string) {
	events := []domain.Event{igStart(folder, "planner")}
	for _, id := range recordIDs {
		events = append(events, tcUsageRecord(id))
	}
	events = append(events, appTestInvEndEvent(folder, dgModel, igUsage()))
	addLogAgent(e, 0, folder, events...)
}

func tcAgentFacts(run string, folder domain.AgentInstanceID, ids []string, prompt string) domain.TranscriptFacts {
	return domain.TranscriptFacts{
		Run: domain.NamedRun(run), Owner: domain.StreamAgentInstance, InstanceHint: folder,
		Path:   dgLogsPath + "/" + run + "/" + string(folder) + "/04_session.raw",
		Format: domain.TranscriptClaudeCodeJSONL, AssistantIDs: ids,
		FirstPrompt: prompt, HasFirstPrompt: prompt != "",
	}
}

func tcOrchFacts(run string, ids ...string) domain.TranscriptFacts {
	return domain.TranscriptFacts{
		Run: domain.NamedRun(run), Owner: domain.StreamOrchestrator,
		Path:   dgLogsPath + "/" + run + "/00_orchestrator_session.raw",
		Format: domain.TranscriptClaudeCodeJSONL, AssistantIDs: ids,
	}
}

// tcAnalyze analyses the env's logs path with the port wired.
func tcAnalyze(e *digestEnv, port domain.TranscriptSource) (app.Result, error) {
	return tcService(e, port).Analyze(context.Background(), app.Request{ExplicitPath: dgLogsPath})
}
