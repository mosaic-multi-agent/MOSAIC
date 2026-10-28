package tui

// Tests for the mark-tool-as-user-interaction flow of the Tools view.
//
// Rules under test:
//   - The mark key issues a command that saves the selected tool for its
//     harness through the service.
//   - On success the model's report is replaced, the runs list is refreshed,
//     the Tools view is reopened from the new report and the status line names
//     the tool and the settings path.
//   - On failure the status line shows an error and the report is unchanged.
//   - The mark key is listed in the help overlay.

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	markSettingsPath = "/test/analysis.yaml"
	markToolName     = "mcp__contact-user__ask"
)

var (
	// markKeyHelpEntry matches the mark key standing alone as a help entry key.
	markKeyHelpEntry = regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(screens.KeyMarkUserTool) + `\s`)
	// markKeyOverlayEntry matches an overlay line pairing the key with its purpose.
	markKeyOverlayEntry = regexp.MustCompile(`(?m)(^|\s)` + regexp.QuoteMeta(screens.KeyMarkUserTool) + `\s+.*user-interaction`)
)

type tuiSettingsAdd struct {
	harness domain.Harness
	name    string
}

// tuiFakeSettingsStore records saves and fails them when err is set.
type tuiFakeSettingsStore struct {
	adds []tuiSettingsAdd
	err  error
}

func (f *tuiFakeSettingsStore) Path() string { return markSettingsPath }

func (f *tuiFakeSettingsStore) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	return domain.DefaultAnalysisSettings(), nil, nil
}

func (f *tuiFakeSettingsStore) AddUserInteractionTool(_ context.Context, h domain.Harness, name string) error {
	if f.err != nil {
		return f.err
	}
	f.adds = append(f.adds, tuiSettingsAdd{h, name})
	return nil
}

func markToolCall(id, name string, sec int) domain.ToolCall {
	start := time.Date(2026, 3, 1, 10, 0, sec, 0, time.UTC)
	return domain.ToolCall{CallID: id, ToolName: name, StartedAt: start, EndedAt: start.Add(time.Second), Pairing: domain.ToolCallPaired}
}

// markDigestRun has one claude-code agent that called markToolName twice and
// Bash once, so the first row of the Tools view is markToolName.
func markDigestRun() domain.RunAggregate {
	t0 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	usage := []domain.InvocationUsage{{
		Model: exportModel, Harness: domain.HarnessClaudeCode, RecordID: "r1", At: t0.Add(5 * time.Second),
		Usage: domain.TokenUsage{Input: domain.Tokens(100)},
	}}
	run := exportTestRun(exportRunA)
	run.Agents = []domain.ActorAggregate{{Actor: agent, Invocations: usage}}
	run.Activity = domain.RunActivity{Invocations: []domain.Invocation{{
		Actor: agent, State: domain.InvocationClosed, StatusCode: "SUCCESS",
		StartedAt: t0, EndedAt: t0.Add(time.Minute), Usage: usage,
		ToolCalls: []domain.ToolCall{
			markToolCall("a", markToolName, 10), markToolCall("b", markToolName, 20), markToolCall("c", "Bash", 30),
		},
	}}}
	return run
}

// newMarkModel returns a model on the Tools view of the only run, whose session
// was loaded by the real service from a scripted digest.
func newMarkModel(t *testing.T, store *tuiFakeSettingsStore) Model {
	t.Helper()
	return newMarkModelFor(t, store, markDigestRun())
}

// markTwoHarnessRun adds a ghcp-cli agent that also called markToolName, in its
// own invocation after the claude-code one.
func markTwoHarnessRun() domain.RunAggregate {
	run := markDigestRun()
	t0 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	other := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "tester#1", Type: "tester"}
	usage := []domain.InvocationUsage{{
		Model: exportModel, Harness: domain.HarnessGHCPCLI, RecordID: "g1", At: t0.Add(125 * time.Second),
		Usage: domain.TokenUsage{Input: domain.Tokens(100)},
	}}
	run.Agents = append(run.Agents, domain.ActorAggregate{Actor: other, Invocations: usage})
	run.Activity.Invocations = append(run.Activity.Invocations, domain.Invocation{
		Actor: other, State: domain.InvocationClosed, StatusCode: "SUCCESS",
		StartedAt: t0.Add(2 * time.Minute), EndedAt: t0.Add(3 * time.Minute), Usage: usage,
		ToolCalls: []domain.ToolCall{markToolCall("g", markToolName, 130)},
	})
	return run
}

// markUnnamedToolRun has a single call whose tool name is empty.
func markUnnamedToolRun() domain.RunAggregate {
	run := markDigestRun()
	run.Activity.Invocations[0].ToolCalls = []domain.ToolCall{markToolCall("u", "", 10)}
	return run
}

func newMarkModelFor(t *testing.T, store *tuiFakeSettingsStore, run domain.RunAggregate) Model {
	t.Helper()
	digests := &tuiFakeDigestStore{digest: domain.Digest{
		Header:    domain.DigestHeader{Version: "1.0.0"},
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}},
	}}
	svc := app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     &tuiFakePricingStore{path: "/test/pricing.yaml"},
		Digests:     digests,
		Settings:    store,
		Clock:       &tuiFakeClock{},
		Interaction: &tuiFakeInteraction{},
		ToolVersion: "test",
	})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: exportSource})
	if err != nil || res.Outcome != app.OutcomeReport || res.Session == nil {
		t.Fatalf("setup: Analyze outcome=%v err=%v", res.Outcome, err)
	}
	m := newNavModel()
	m.ctx = context.Background()
	m.width = 200
	m.opts.Service = svc
	updated, _ := m.Update(analysisDoneMsg{result: res})
	m = send(asModel(updated), "enter", screens.KeyViewTools)
	assertOn(t, m, screens.ScreenTools)
	return m
}

// markAndFinish presses the mark key and feeds the command's message back.
func markAndFinish(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := pressKey(m, screens.KeyMarkUserTool)
	return runCmd(t, asModel(updated), cmd)
}

func TestMarkTool_MarkKeySavesTheSelectedToolForItsHarness(t *testing.T) {
	store := &tuiFakeSettingsStore{}
	m := newMarkModel(t, store)

	markAndFinish(t, m)

	if want := []tuiSettingsAdd{{domain.HarnessClaudeCode, markToolName}}; !reflect.DeepEqual(store.adds, want) {
		t.Errorf("saved = %v, want %v", store.adds, want)
	}
}

func TestMarkTool_RowWithSeveralHarnesses_SavesTheToolUnderEachOfThem(t *testing.T) {
	store := &tuiFakeSettingsStore{}
	m := newMarkModelFor(t, store, markTwoHarnessRun())

	markAndFinish(t, m)

	want := []tuiSettingsAdd{{domain.HarnessClaudeCode, markToolName}, {domain.HarnessGHCPCLI, markToolName}}
	if !reflect.DeepEqual(store.adds, want) {
		t.Errorf("saved = %v, want %v", store.adds, want)
	}
}

func TestMarkTool_RefusedRow_SavesNothing(t *testing.T) {
	store := &tuiFakeSettingsStore{}
	m := newMarkModelFor(t, store, markUnnamedToolRun())

	_, cmd := pressKey(m, screens.KeyMarkUserTool)
	if cmd != nil {
		cmd()
	}

	if len(store.adds) != 0 {
		t.Errorf("saved = %v, want nothing for a row without a tool name", store.adds)
	}
}

func TestMarkTool_Success_ReplacesTheReportAndCountsTheToolInTheWallTime(t *testing.T) {
	m := newMarkModel(t, &tuiFakeSettingsStore{})
	before, _ := m.report.FindRun(exportRunA)
	if len(before.WallTime.CountedTools) != 0 {
		t.Fatalf("setup: counted tools before marking = %v, want none", before.WallTime.CountedTools)
	}

	m = markAndFinish(t, m)

	after, _ := m.report.FindRun(exportRunA)
	if len(after.WallTime.CountedTools) != 1 || after.WallTime.CountedTools[0].Tool != markToolName {
		t.Errorf("counted tools after marking = %v, want %s", after.WallTime.CountedTools, markToolName)
	}
}

func TestMarkTool_Success_ReopensTheToolsViewWithTheStatusMessage(t *testing.T) {
	m := markAndFinish(t, newMarkModel(t, &tuiFakeSettingsStore{}))

	assertOn(t, m, screens.ScreenTools)
	want := "Marked " + markToolName + " as user-interaction in " + markSettingsPath
	if view := m.View(); !strings.Contains(view, want) {
		t.Errorf("view must show %q\n%s", want, view)
	}
}

func TestMarkTool_SaveFailure_ShowsAnErrorAndLeavesTheReportUnchanged(t *testing.T) {
	m := newMarkModel(t, &tuiFakeSettingsStore{err: errors.New("disk full")})
	before := m.report

	m = markAndFinish(t, m)

	if !reflect.DeepEqual(m.report, before) {
		t.Error("the report must be unchanged after a failed save")
	}
	assertOn(t, m, screens.ScreenTools)
	if view := m.View(); !strings.Contains(view, "Could not save settings:") || !strings.Contains(view, "disk full") {
		t.Errorf("view must show the save error\n%s", view)
	}
}

func TestMarkTool_DoneMessage_ReplacesTheReportRefreshesTheRunsListAndReopensTheView(t *testing.T) {
	before := navViewsReport(statusToolsRun(runA, 300, "LegacyTool"))
	after := navViewsReport(statusToolsRun(runA, 700, "ToolAfter"))
	m := send(onRunDetail(t, before), screens.KeyViewTools)
	assertOn(t, m, screens.ScreenTools)

	updated, _ := m.Update(settingsEditDoneMsg{result: app.SettingsEditResult{
		Report: after, ConfigPath: markSettingsPath, Tool: "ToolAfter", Harnesses: []domain.Harness{domain.HarnessClaudeCode},
	}})
	m = asModel(updated)

	assertOn(t, m, screens.ScreenTools)
	if view := m.View(); !strings.Contains(view, "ToolAfter") || strings.Contains(view, "LegacyTool") {
		t.Errorf("the reopened Tools view must show the recomputed report's tools\n%s", view)
	}
	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRuns)
	if view := m.View(); !strings.Contains(view, "$7.00") || strings.Contains(view, "$3.00") {
		t.Errorf("the runs list must show the recomputed $7.00 and not the old $3.00\n%s", view)
	}
}

func TestMarkTool_DoneMessageWithError_KeepsTheReportAndShowsTheError(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 300, "ToolA"))
	m := send(onRunDetail(t, report), screens.KeyViewTools)

	updated, _ := m.Update(settingsEditDoneMsg{err: errors.New("permission denied")})
	m = asModel(updated)

	assertOn(t, m, screens.ScreenTools)
	if !reflect.DeepEqual(m.report, report) {
		t.Error("the report must be unchanged after an error")
	}
	view := m.View()
	if !strings.Contains(view, "Could not save settings:") || !strings.Contains(view, "permission denied") {
		t.Errorf("view must show the error\n%s", view)
	}
	if !strings.Contains(view, "ToolA") {
		t.Errorf("the Tools view must still show the unchanged report\n%s", view)
	}
}

func TestMarkTool_KeyIsListedInTheHelpOverlay(t *testing.T) {
	m := send(newMarkModel(t, &tuiFakeSettingsStore{}), screens.KeyHelp)

	view := m.View()

	if !markKeyOverlayEntry.MatchString(view) {
		t.Errorf("the help overlay must list the key %q with its purpose\n%s", screens.KeyMarkUserTool, view)
	}
}

func TestMarkTool_KeyIsListedInTheFrameHelpLine(t *testing.T) {
	m := newMarkModel(t, &tuiFakeSettingsStore{})

	view := m.View()

	if !markKeyHelpEntry.MatchString(view) {
		t.Errorf("the help line of the Tools view must list the key %q\n%s", screens.KeyMarkUserTool, view)
	}
}
