package analysis_test

// Tests for the tool figures of one actor.
//
// Rules under test:
//   - An actor's tool figures follow the Tools view's rules and order: tools
//     ascending by name with the unnamed tool last, calls, errors, unpaired
//     calls and durations from paired calls only, and the context growth figures
//     (a tool seen only through growth gets a row without calls).
//   - A start-only or end-only call counts as a call and as unpaired, and shows no
//     duration.
//   - The orchestrator's figures equal the Tools view's orchestrator rows, a type
//     of one invocation equals its type rows, and the actor rows of a type or of
//     the run add up to the Tools view's rows.
//   - An actor without tool calls and growth has no rows.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// afStartOnly is a call with a start and no end.
func afStartOnly(callID, name string, sec int) domain.ToolCall {
	return domain.ToolCall{CallID: callID, ToolName: name, StartedAt: ctxAt(sec), Pairing: domain.ToolCallStartOnly}
}

// afEndOnly is a call with an end and no start, so no name.
func afEndOnly(callID string, sec int, status domain.ToolCallStatus) domain.ToolCall {
	return domain.ToolCall{CallID: callID, EndedAt: ctxAt(sec), Status: status, Pairing: domain.ToolCallEndOnly}
}

// afToolsRun: an orchestrator, two coders, a tester with one start-only call and a
// reviewer without tools.
//
//	orchestrator  Read 2s ok, Bash 1s error, Bash start-only; Read growth 400
//	coder#1       Read 3s, end-only call, Bash start-only;   Glob growth 201 without a Glob call
//	coder#2       Read 1s error, Grep 2s;                    Read growth 400
//	tester#1      Write start-only
//	reviewer#1    nothing
func afToolsRun(t *testing.T) domain.RunReport {
	t.Helper()
	none := domain.NewRequestedTools(nil)
	return afPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, tgOne("Read")),
			tgCall("o2", 20, 1500, 50, none),
		},
		orchTools: []domain.ToolCall{
			ctxTool("ot1", "Read", 11, 13),
			afStatusTool("ot2", "Bash", 20, domain.ToolCallError),
			afStartOnly("ot3", "Bash", 30),
		},
		agents: []ctxAgent{
			afAgent("coder#1", "coder", []domain.InvocationUsage{
				tgCall("a1", 10, 2000, 100, tgOne("Glob")),
				tgCall("a2", 20, 2301, 10, none),
			}, ctxTool("at1", "Read", 11, 14), afEndOnly("at2", 40, ""), afStartOnly("at3", "Bash", 50)),
			afAgent("coder#2", "coder", []domain.InvocationUsage{
				tgCall("b1", 10, 3000, 100, tgOne("Read")),
				tgCall("b2", 20, 3500, 10, none),
			}, afStatusTool("bt1", "Read", 11, domain.ToolCallError), ctxTool("bt2", "Grep", 20, 22)),
			afAgent("tester#1", "tester", nil, afStartOnly("tt1", "Write", 10)),
			afAgent("reviewer#1", "reviewer", nil),
		},
	})
}

func afTools(t *testing.T, run domain.RunReport, id domain.AgentInstanceID, typ string) []domain.ToolFigures {
	t.Helper()
	return afSession(t, run, afRef(id, typ)).Tools
}

func TestActorTools_RowsFollowTheToolsViewOrderWithGrowthOnlyRowsAndTheUnnamedToolLast(t *testing.T) {
	run := afToolsRun(t)

	got := toolNames(afTools(t, run, "coder#1", "coder"))

	if want := []string{"Bash", "Glob", "Read", ""}; !rmEqualStrings(got, want) {
		t.Errorf("coder#1 tools = %q, want %q", got, want)
	}
}

func TestActorTools_CountsErrorsUnpairedAndDurationsOfTheActorsOwnCalls(t *testing.T) {
	run := afToolsRun(t)

	read := requireTool(t, afTools(t, run, "coder#2", "coder"), "Read")
	grep := requireTool(t, afTools(t, run, "coder#2", "coder"), "Grep")

	if read.Calls != 1 || read.Errors != 1 || read.Unpaired != 0 {
		t.Errorf("coder#2 Read = %d calls, %d errors, %d unpaired, want 1, 1, 0", read.Calls, read.Errors, read.Unpaired)
	}
	assertDurations(t, "coder#2 Read", read.Durations, 1, time.Second, time.Second, time.Second, time.Second)
	assertDurations(t, "coder#2 Grep", grep.Durations, 1, 2*time.Second, 2*time.Second, 2*time.Second, 2*time.Second)
}

func TestActorTools_StartOnlyCallCountsAsUnpairedAndHasNoDuration(t *testing.T) {
	run := afToolsRun(t)

	bash := requireTool(t, afTools(t, run, "coder#1", "coder"), "Bash")

	if bash.Calls != 1 || bash.Unpaired != 1 || bash.Errors != 0 {
		t.Errorf("Bash = %d calls, %d unpaired, %d errors, want 1, 1, 0", bash.Calls, bash.Unpaired, bash.Errors)
	}
	assertDurations(t, "Bash", bash.Durations, 0, 0, 0, 0, 0)
}

func TestActorTools_EndOnlyCallCountsUnderTheUnnamedToolWithoutDuration(t *testing.T) {
	run := afToolsRun(t)

	unnamed := requireTool(t, afTools(t, run, "coder#1", "coder"), "")

	if unnamed.Calls != 1 || unnamed.Unpaired != 1 {
		t.Errorf("unnamed tool = %d calls, %d unpaired, want 1, 1", unnamed.Calls, unnamed.Unpaired)
	}
	assertDurations(t, "unnamed tool", unnamed.Durations, 0, 0, 0, 0, 0)
}

func TestActorTools_PairedCallKeepsItsDurationNextToUnpairedCallsOfOtherTools(t *testing.T) {
	run := afToolsRun(t)

	read := requireTool(t, afTools(t, run, "coder#1", "coder"), "Read")

	assertDurations(t, "coder#1 Read", read.Durations, 1, 3*time.Second, 3*time.Second, 3*time.Second, 3*time.Second)
}

func TestActorTools_GrowthOfTheActorsSingularCallsIsKeptWithoutAToolCall(t *testing.T) {
	run := afToolsRun(t)

	glob := requireTool(t, afTools(t, run, "coder#1", "coder"), "Glob")

	if glob.Calls != 0 || glob.Errors != 0 || glob.Unpaired != 0 {
		t.Errorf("Glob = %d calls, %d errors, %d unpaired, want a growth-only row of zeros", glob.Calls, glob.Errors, glob.Unpaired)
	}
	tgAssertGrowthFigures(t, "coder#1 Glob", glob.Growth, 1, 201, 201, 201)
}

func TestActorTools_ToolWithCallsButNoGrowthSamplesIsNotProvided(t *testing.T) {
	run := afToolsRun(t)

	bash := requireTool(t, afTools(t, run, "coder#1", "coder"), "Bash")

	if bash.Growth.IsProvided() || bash.Growth.Samples != 0 {
		t.Errorf("Bash growth = %+v, want no samples", bash.Growth)
	}
	ctxAssertAbsent(t, "Bash average growth", bash.Growth.Average)
}

func TestActorTools_OrchestratorEqualsTheToolsViewOrchestratorRows(t *testing.T) {
	run := afToolsRun(t)

	got := afSession(t, run, domain.Orchestrator()).Tools

	if !reflect.DeepEqual(got, run.Tools.Orchestrator) {
		t.Errorf("orchestrator tools = %+v\nTools view = %+v", got, run.Tools.Orchestrator)
	}
	if bash := requireTool(t, got, "Bash"); bash.Calls != 2 || bash.Errors != 1 || bash.Unpaired != 1 {
		t.Errorf("orchestrator Bash = %d calls, %d errors, %d unpaired, want 2, 1, 1", bash.Calls, bash.Errors, bash.Unpaired)
	}
}

func TestActorTools_TypeOfOneInvocationEqualsItsTypeRowsInTheToolsView(t *testing.T) {
	run := afToolsRun(t)

	got := afTools(t, run, "tester#1", "tester")

	if want := tgAgentTypeTools(t, run, "tester"); !reflect.DeepEqual(got, want) {
		t.Errorf("tester#1 tools = %+v\nTools view = %+v", got, want)
	}
}

// afTotals are the figures that add up across actors.
type afTotals struct {
	calls, errors, unpaired, durations int
	total                              time.Duration
	samples, negative, notMeasurable   int
}

func afSum(groups ...[]domain.ToolFigures) map[string]afTotals {
	out := map[string]afTotals{}
	for _, tools := range groups {
		for _, f := range tools {
			s := out[f.Tool]
			s.calls += f.Calls
			s.errors += f.Errors
			s.unpaired += f.Unpaired
			s.durations += f.Durations.Count
			s.total += f.Durations.Total
			s.samples += f.Growth.Samples
			s.negative += f.Growth.Negative
			s.notMeasurable += f.Growth.NotMeasurable
			out[f.Tool] = s
		}
	}
	return out
}

func TestActorTools_ActorRowsOfATypeAddUpToTheTypeRowsOfTheToolsView(t *testing.T) {
	run := afToolsRun(t)

	got := afSum(afTools(t, run, "coder#1", "coder"), afTools(t, run, "coder#2", "coder"))
	want := afSum(tgAgentTypeTools(t, run, "coder"))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("coder actor rows add up to %+v, the Tools view's coder rows to %+v", got, want)
	}
	if got["Read"].calls != 2 || got["Read"].errors != 1 || got["Read"].durations != 2 || got["Read"].samples != 1 {
		t.Errorf("coder Read = %+v, want 2 calls, 1 error, 2 durations, 1 growth sample", got["Read"])
	}
}

func TestActorTools_ActorRowsOfTheWholeRunAddUpToTheRunRowsOfTheToolsView(t *testing.T) {
	run := afToolsRun(t)
	var all [][]domain.ToolFigures
	for _, s := range run.ActorSessions {
		all = append(all, s.Tools)
	}

	got, want := afSum(all...), afSum(run.Tools.Run)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("actor rows add up to %+v, the Tools view's run rows to %+v", got, want)
	}
}

func TestActorTools_ActorWithoutToolCallsAndGrowthHasNoRows(t *testing.T) {
	run := afToolsRun(t)

	if got := afTools(t, run, "reviewer#1", "reviewer"); len(got) != 0 {
		t.Errorf("reviewer#1 tools = %+v, want none", got)
	}
}
