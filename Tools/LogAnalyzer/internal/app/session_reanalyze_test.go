package app_test

// Tests that a session recomputes the context figures from its cached aggregate
// when the analysis settings change, without reading anything again.
//
// Rules under test:
//   - Analyze computes context figures for digest-loaded runs from their
//     per-call records, using the loaded settings.
//   - Session.Reanalyze replaces the session's settings and returns a report
//     whose first-write context follows the new settings.
//   - Session.Reprice afterwards prices with the replaced settings.

import (
	"context"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const reanalyzeAgent = domain.AgentInstanceID("coder#1")

func reanalyzeCall(id string, sec int, input int64) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: dgModel, Harness: domain.HarnessClaudeCode, RecordID: id,
		At:    dgT1.Add(time.Duration(sec) * time.Second),
		Usage: domain.TokenUsage{Input: domain.Tokens(input), Output: domain.Tokens(1)},
	}
}

func reanalyzeTool(id, name string, startSec int) domain.ToolCall {
	start := dgT1.Add(time.Duration(startSec) * time.Second)
	return domain.ToolCall{
		CallID: id, ToolName: name, StartedAt: start, EndedAt: start.Add(time.Second), Pairing: domain.ToolCallPaired,
	}
}

// reanalyzeRun is a digest run whose agent made calls of 100k and 200k tokens
// with a Bash call between them and a Write call after the second.
func reanalyzeRun() domain.RunAggregate {
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: reanalyzeAgent, Type: "coder"}
	usage := []domain.InvocationUsage{reanalyzeCall("r1", 10, 100_000), reanalyzeCall("r2", 30, 200_000)}
	run := dgRun(dgRunA, 1000)
	run.Agents = []domain.ActorAggregate{{Actor: agent, Invocations: usage}}
	run.Activity = domain.RunActivity{Invocations: []domain.Invocation{{
		Actor: agent, State: domain.InvocationClosed, StatusCode: "SUCCESS",
		StartedAt: dgT1, EndedAt: dgT1.Add(time.Minute), Usage: usage,
		ToolCalls: []domain.ToolCall{reanalyzeTool("b", "Bash", 20), reanalyzeTool("w", "Write", 40)},
	}}}
	return run
}

func reanalyzeFirstWrite(t *testing.T, report domain.Report) int64 {
	t.Helper()
	run, ok := report.FindRun(dgRunA)
	if !ok {
		t.Fatalf("run %s not in the report", dgRunA)
	}
	got, ok := run.Context.Find(reanalyzeAgent)
	if !ok {
		t.Fatalf("no context figures for %s", reanalyzeAgent)
	}
	v, present := got.FirstWrite.Tokens.Value()
	if !present {
		t.Fatalf("first write context is absent")
	}
	return v
}

func bashAsWriteTool() domain.AnalysisSettings {
	s := domain.DefaultAnalysisSettings()
	s.WriteTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"Bash"}}
	return s
}

func TestAnalyze_DigestRun_ContextFiguresUseTheDefaultSettings(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, reanalyzeRun()))

	res := analyzeDGReport(t, e, dgFilePath)

	if got := reanalyzeFirstWrite(t, res.Report); got != 200_000 {
		t.Errorf("first write context = %d, want 200000 (the Write call follows the second call)", got)
	}
}

func TestAnalyze_DigestRun_ContextFiguresFollowTheLoadedNonDefaultSettings(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, reanalyzeRun()))
	svc := app.New(app.Deps{
		Source:      e.logSource,
		Reader:      e.reader,
		Pricing:     e.pricing,
		Digests:     e.digests,
		Settings:    &fakeSettingsStore{filePath: "/s.yaml", settings: bashAsWriteTool()},
		Clock:       e.clock,
		Interaction: e.interaction,
		WorkDir:     "/workdir",
	})

	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgFilePath})

	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}
	if got := reanalyzeFirstWrite(t, res.Report); got != 100_000 {
		t.Errorf("first write context with Bash as loaded write tool = %d, want 100000", got)
	}
}

func TestSessionReanalyze_NewSettings_RecomputeFirstWriteFromTheCachedAggregate(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, reanalyzeRun()))
	res := analyzeDGReport(t, e, dgFilePath)
	readsBefore := len(e.digests.readCalls)

	got := res.Session.Reanalyze(bashAsWriteTool())

	if v := reanalyzeFirstWrite(t, got); v != 100_000 {
		t.Errorf("first write context with Bash as write tool = %d, want 100000", v)
	}
	if len(e.digests.readCalls) != readsBefore {
		t.Error("Reanalyze must not read digest files again")
	}
}

func TestSessionReanalyze_ReplacesTheSessionSettings(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, reanalyzeRun()))
	res := analyzeDGReport(t, e, dgFilePath)

	res.Session.Reanalyze(bashAsWriteTool())

	got := res.Session.Settings()
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Bash") || got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("Settings() must return the settings given to Reanalyze")
	}
}

func TestSessionReprice_AfterReanalyze_UsesTheReplacedSettings(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, reanalyzeRun()))
	res := analyzeDGReport(t, e, dgFilePath)
	res.Session.Reanalyze(bashAsWriteTool())

	got := res.Session.Reprice(flatPricing(dgModel))

	if v := reanalyzeFirstWrite(t, got); v != 100_000 {
		t.Errorf("first write context after Reprice = %d, want 100000 (replaced settings)", v)
	}
}
