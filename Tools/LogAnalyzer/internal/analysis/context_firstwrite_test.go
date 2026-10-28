package analysis_test

// Tests for the context at first write (a heuristic) and the context bands of
// the figures.
//
// Rules under test:
//   - The first write is the actor's write-class tool call with the earliest
//     non-zero start; its context is that of the last call (with a present
//     context) that started at or before it.
//   - It is absent without a write-class call, and when no call precedes it.
//   - Which tool names are write-class comes from the analysis settings, per
//     the actor's harness (usage harness, else the run's only harness, else
//     the union of all harness lists).
//   - Peak, final and first-write context each carry their band.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func ctxWithHarness(u domain.InvocationUsage, h domain.Harness) domain.InvocationUsage {
	u.Harness = h
	return u
}

// ctxThreeCallAgent has calls of context 100, 200 and 300 at 10s, 20s and 30s.
func ctxThreeCallAgent(tools ...domain.ToolCall) ctxAgent {
	return ctxAgent{id: ctxAgentID, tools: tools, usage: []domain.InvocationUsage{
		ctxCall("r1", 10, 100, 0, 0, 1),
		ctxCall("r2", 20, 150, 50, 0, 1),
		ctxCall("r3", 30, 100, 100, 100, 1),
	}}
}

func TestFirstWrite_IsTheContextOfTheLastCallBeforeTheWrite(t *testing.T) {
	agent := ctxThreeCallAgent(
		ctxTool("t1", "Read", 5, 6),
		ctxTool("t2", "Write", 25, 26),
		ctxTool("t3", "Edit", 40, 41),
	)

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
	if !got.FirstWriteAt.Equal(ctxAt(25)) {
		t.Errorf("first write at = %v, want %v", got.FirstWriteAt, ctxAt(25))
	}
}

func TestFirstWrite_MarksTheSourceCallInTheRows(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("t2", "Write", 25, 26))

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	marked := map[string]bool{}
	for _, c := range got.Calls {
		marked[c.RecordID] = c.FirstWrite
	}
	if marked["r1"] || !marked["r2"] || marked["r3"] {
		t.Errorf("first-write marks = %v, want only r2", marked)
	}
}

func TestFirstWrite_CallStartingExactlyAtTheWriteStart_Counts(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("t2", "Write", 20, 21))

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWrite_EarliestWriteWinsRegardlessOfListOrder(t *testing.T) {
	agent := ctxThreeCallAgent(
		ctxTool("late", "Edit", 35, 36),
		ctxTool("early", "Write", 15, 16),
	)

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 100)
}

func TestFirstWrite_WriteWithoutStartTime_IsNotTheFirstWrite(t *testing.T) {
	endOnly := domain.ToolCall{CallID: "e", ToolName: "Write", Pairing: domain.ToolCallEndOnly}
	unstarted := domain.ToolCall{CallID: "u", ToolName: "Write", Pairing: domain.ToolCallStartOnly}
	agent := ctxThreeCallAgent(endOnly, unstarted, ctxTool("w", "Edit", 25, 26))

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWrite_NoWriteClassToolCall_IsAbsent(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("t1", "Read", 5, 6), ctxTool("t2", "Bash", 25, 26))

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
	if !got.FirstWriteAt.IsZero() {
		t.Errorf("first write at = %v, want zero", got.FirstWriteAt)
	}
	for _, c := range got.Calls {
		if c.FirstWrite {
			t.Errorf("call %s is marked first write without a write-class call", c.RecordID)
		}
	}
}

func TestFirstWrite_NoCallBeforeTheWrite_IsAbsent(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("t1", "Write", 5, 6))

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
}

func TestFirstWrite_SkipsLatestCallsWithoutContext(t *testing.T) {
	outputOnly := domain.InvocationUsage{
		Model: ctxModel, Harness: domain.HarnessClaudeCode, RecordID: "r-out", At: ctxAt(22),
		Usage: domain.TokenUsage{Output: domain.Tokens(5)},
	}
	agent := ctxThreeCallAgent(ctxTool("t2", "Write", 25, 26))
	agent.usage = append(agent.usage, outputOnly)

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWrite_FollowsTheSettingsForTheHarness(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("b", "Bash", 15, 16), ctxTool("w", "Write", 25, 26))
	agg := ctxAggregate(ctxRunSpec{agents: []ctxAgent{agent}})
	custom := domain.DefaultAnalysisSettings()
	custom.WriteTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"Bash"}}

	withDefault := ctxRequireAgent(t, ctxPrice(t, agg, domain.DefaultAnalysisSettings()), ctxAgentID)
	withCustom := ctxRequireAgent(t, ctxPrice(t, agg, custom), ctxAgentID)

	ctxAssertTokens(t, "first write, default settings", withDefault.FirstWrite.Tokens, 200)
	ctxAssertTokens(t, "first write, Bash as write tool", withCustom.FirstWrite.Tokens, 100)
}

func TestFirstWrite_ReplacedListNoLongerMatchesTheDefaultNames(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("w", "Write", 25, 26))
	custom := domain.DefaultAnalysisSettings()
	custom.WriteTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {}}

	got := ctxRequireAgent(t, ctxPrice(t, ctxAggregate(ctxRunSpec{agents: []ctxAgent{agent}}), custom), ctxAgentID)

	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
}

func TestFirstWrite_SettingsPatternMatches(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("w", "mcp__files__save", 25, 26))
	custom := domain.DefaultAnalysisSettings()
	custom.WriteTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"mcp__files__*"}}

	got := ctxRequireAgent(t, ctxPrice(t, ctxAggregate(ctxRunSpec{agents: []ctxAgent{agent}}), custom), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWrite_MixedHarnesses_EachActorUsesItsOwnHarnessList(t *testing.T) {
	opencode := ctxAgent{id: "oc#1", tools: []domain.ToolCall{
		ctxTool("a", "apply_patch", 25, 26), // write-class for opencode
	}, usage: []domain.InvocationUsage{
		ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), domain.HarnessOpenCode),
		ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), domain.HarnessOpenCode),
	}}
	claudeWithForeignName := ctxAgent{id: "cc#2", tools: []domain.ToolCall{
		ctxTool("b", "apply_patch", 25, 26), // not write-class for claude-code
	}, usage: []domain.InvocationUsage{ctxCall("r1", 10, 100, 0, 0, 1)}}
	opencodeWithForeignName := ctxAgent{id: "oc#3", tools: []domain.ToolCall{
		ctxTool("c", "Write", 25, 26), // not write-class for opencode
	}, usage: []domain.InvocationUsage{
		ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), domain.HarnessOpenCode),
	}}
	run := ctxPriceDefault(t, ctxRunSpec{
		agents:    []ctxAgent{opencode, claudeWithForeignName, opencodeWithForeignName},
		harnesses: []domain.Harness{domain.HarnessClaudeCode, domain.HarnessOpenCode},
	})

	ctxAssertTokens(t, "opencode apply_patch", ctxRequireAgent(t, run, "oc#1").FirstWrite.Tokens, 200)
	ctxAssertAbsent(t, "claude-code apply_patch", ctxRequireAgent(t, run, "cc#2").FirstWrite.Tokens)
	ctxAssertAbsent(t, "opencode Write", ctxRequireAgent(t, run, "oc#3").FirstWrite.Tokens)
}

func TestFirstWrite_ActorWithoutUsageHarness_UsesTheRunsOnlyHarness(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "create_file", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), ""),
			ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), ""),
		}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}, harnesses: []domain.Harness{domain.HarnessVSCodeGHCP}})

	ctxAssertTokens(t, "first write", ctxRequireAgent(t, run, ctxAgentID).FirstWrite.Tokens, 200)
}

func TestFirstWrite_UnknownHarness_MatchesTheUnionOfAllLists(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "create_file", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), ""),
			ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), ""),
		}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent},
		harnesses: []domain.Harness{domain.HarnessClaudeCode, domain.HarnessVSCodeGHCP}})

	ctxAssertTokens(t, "first write", ctxRequireAgent(t, run, ctxAgentID).FirstWrite.Tokens, 200)
}

func TestFirstWrite_ActorHarnessIsTheFirstNonEmptyUsageHarness(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "apply_patch", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxWithHarness(ctxCall("r0", 5, 50, 0, 0, 1), ""),
			ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), domain.HarnessOpenCode),
			ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), domain.HarnessClaudeCode),
		}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent},
		harnesses: []domain.Harness{domain.HarnessClaudeCode, domain.HarnessOpenCode}})

	ctxAssertTokens(t, "first write", ctxRequireAgent(t, run, ctxAgentID).FirstWrite.Tokens, 200)
}

// A union-of-all-lists implementation would find "Write" (claude-code only) and
// return a figure; the run's only harness is vscode-ghcp, so it must be absent.
func TestFirstWrite_RunsOnlyHarnessExcludesNamesOfOtherHarnesses(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), ""),
			ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), ""),
		}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}, harnesses: []domain.Harness{domain.HarnessVSCodeGHCP}})

	ctxAssertAbsent(t, "first write", ctxRequireAgent(t, run, ctxAgentID).FirstWrite.Tokens)
}

// The first non-empty usage harness is opencode, for which "Write" is not
// write-class. Taking the first record's (empty) harness and falling back to the
// union, or taking the last harness (claude-code), would find a figure.
func TestFirstWrite_FirstNonEmptyUsageHarnessExcludesNamesOfOtherHarnesses(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxWithHarness(ctxCall("r0", 5, 50, 0, 0, 1), ""),
			ctxWithHarness(ctxCall("r1", 10, 100, 0, 0, 1), domain.HarnessOpenCode),
			ctxWithHarness(ctxCall("r2", 20, 200, 0, 0, 1), domain.HarnessClaudeCode),
		}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent},
		harnesses: []domain.Harness{domain.HarnessClaudeCode, domain.HarnessOpenCode}})

	ctxAssertAbsent(t, "first write", ctxRequireAgent(t, run, ctxAgentID).FirstWrite.Tokens)
}

func TestBands_PeakFinalAndFirstWriteEachCarryTheirBand(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxCall("r1", 10, 90_000, 0, 0, 1),  // below optimal
			ctxCall("r2", 20, 120_000, 0, 0, 1), // optimal: the first-write call
			ctxCall("r3", 30, 360_000, 0, 0, 1), // degraded: the peak
			ctxCall("r4", 40, 260_000, 0, 0, 1), // above optimal: the final
		}}

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	if got.First.Band != domain.BandBelowOptimal {
		t.Errorf("first band = %v, want below optimal", got.First.Band)
	}
	if got.FirstWrite.Band != domain.BandOptimal {
		t.Errorf("first write band = %v, want optimal", got.FirstWrite.Band)
	}
	if got.Peak.Band != domain.BandDegraded {
		t.Errorf("peak band = %v, want degraded", got.Peak.Band)
	}
	if got.Final.Band != domain.BandAboveOptimal {
		t.Errorf("final band = %v, want above optimal", got.Final.Band)
	}
}

func TestBands_FigureAtABoundaryBelongsToTheUpperBand(t *testing.T) {
	cases := []struct {
		tokens int64
		want   domain.ContextBand
	}{
		{99_999, domain.BandBelowOptimal},
		{100_000, domain.BandOptimal},
		{249_999, domain.BandOptimal},
		{250_000, domain.BandAboveOptimal},
		{349_999, domain.BandAboveOptimal},
		{350_000, domain.BandDegraded},
	}
	for _, tc := range cases {
		agent := ctxAgent{id: ctxAgentID, usage: []domain.InvocationUsage{ctxCall("r1", 10, tc.tokens, 0, 0, 1)}}

		got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

		if got.Peak.Band != tc.want {
			t.Errorf("peak band at %d tokens = %v, want %v", tc.tokens, got.Peak.Band, tc.want)
		}
	}
}
