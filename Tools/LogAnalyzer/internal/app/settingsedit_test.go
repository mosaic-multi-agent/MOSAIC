package app_test

// Tests for Service.MarkUserInteractionTool.
//
// Rules under test:
//   - The tool is saved through the settings port for every harness given, in
//     sorted order, and the report is recomputed from the cached session with
//     the tool counted as user-interaction, without reading anything again.
//   - The result names the settings path, the tool and the sorted harnesses and
//     keeps the quality summary of the report that was passed in.
//   - Validation failures (no store, bad harness, bad tool name) and store
//     failures return an error, leave the session settings unchanged and, for
//     validation failures, save nothing.

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const markedTool = "mcp__contact-user__ask"

type settingsAdd struct {
	harness domain.Harness
	name    string
}

// recordingSettingsStore records every AddUserInteractionTool call and fails
// for the harnesses listed in failFor.
type recordingSettingsStore struct {
	filePath string
	adds     []settingsAdd
	failFor  map[domain.Harness]error
	// levels lists the level of every save; globalPath is the global file
	// ("" = fakeGlobalSettingsPath); globalUnavailable makes that level unusable.
	levels            []domain.ConfigLevel
	globalPath        string
	globalUnavailable bool
}

func (s *recordingSettingsStore) Path() string { return s.filePath }

// fakeGlobalSettingsPath is the global settings file of recordingSettingsStore.
const fakeGlobalSettingsPath = "/test/global/analysis.yaml"

func (s *recordingSettingsStore) Paths() domain.ConfigPaths {
	global := domain.ConfigFile{Level: domain.LevelGlobal, Path: fakeGlobalSettingsPath}
	if s.globalPath != "" {
		global.Path = s.globalPath
	}
	if s.globalUnavailable {
		global = domain.ConfigFile{Level: domain.LevelGlobal, Unavailable: "user config folder not found"}
	}
	return domain.ConfigPaths{
		Global:  global,
		Project: domain.ConfigFile{Level: domain.LevelProject, Path: s.filePath},
	}
}

func (s *recordingSettingsStore) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	return domain.DefaultAnalysisSettings(), nil, nil
}

func (s *recordingSettingsStore) AddUserInteractionTool(_ context.Context, level domain.ConfigLevel, h domain.Harness, name string) (domain.ConfigFile, error) {
	if err := s.failFor[h]; err != nil {
		return domain.ConfigFile{}, err
	}
	s.adds = append(s.adds, settingsAdd{h, name})
	s.levels = append(s.levels, level)
	paths := s.Paths()
	file := paths.Project
	if level == domain.LevelGlobal {
		file = paths.Global
	}
	file.Exists = true
	return file, nil
}

// markRun is a digest run whose agent made a call of an unconfigured tool
// inside its one-minute invocation.
func markRun() domain.RunAggregate {
	run := reanalyzeRun()
	inv := &run.Activity.Invocations[0]
	inv.ToolCalls = []domain.ToolCall{reanalyzeTool("m", markedTool, 20)}
	return run
}

type markEnv struct {
	env   *digestEnv
	svc   *app.Service
	store *recordingSettingsStore
	ia    *scriptedInteraction
	res   app.Result
}

// newMarkEnv analyses markRun through a service holding the recording store;
// withStore false leaves Deps.Settings nil.
func newMarkEnv(t *testing.T, withStore bool) markEnv {
	t.Helper()
	return newMarkEnvFor(t, withStore, markRun())
}

// newMarkEnvFor is newMarkEnv for a given digest run.
func newMarkEnvFor(t *testing.T, withStore bool, run domain.RunAggregate) markEnv {
	t.Helper()
	ia := &scriptedInteraction{}
	e := newDigestEnv().withInteraction(ia)
	e.digestFile(dgFilePath, dgDigest(dgT1, run))
	store := &recordingSettingsStore{filePath: "/root/MosaicLogAnalyzer/config/analysis.yaml"}
	deps := app.Deps{
		Source: e.logSource, Reader: e.reader, Pricing: e.pricing, Digests: e.digests,
		Clock: e.clock, Interaction: e.interaction, WorkDir: "/workdir",
	}
	if withStore {
		deps.Settings = store
	}
	svc := app.New(deps)
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgFilePath})
	if err != nil || res.Session == nil {
		t.Fatalf("setup: Analyze err=%v session nil=%v", err, res.Session == nil)
	}
	return markEnv{env: e, svc: svc, store: store, res: res, ia: ia}
}

func (m markEnv) mark(tool string, harnesses ...domain.Harness) (app.SettingsEditResult, error) {
	return m.svc.MarkUserInteractionTool(context.Background(), m.res.Session, m.res.Report, tool, harnesses)
}

func markWaiting(t *testing.T, report domain.Report) (time.Duration, []domain.CountedTool) {
	t.Helper()
	run, ok := report.FindRun(dgRunA)
	if !ok {
		t.Fatalf("run %s not in the report", dgRunA)
	}
	return run.WallTime.Waiting, run.WallTime.CountedTools
}

func TestMarkUserInteractionTool_Success_SavesAndRecomputesWallTime(t *testing.T) {
	m := newMarkEnv(t, true)
	if waiting, _ := markWaiting(t, m.res.Report); waiting != 0 {
		t.Fatalf("setup: waiting before marking = %v, want 0", waiting)
	}

	got, err := m.mark(markedTool, domain.HarnessClaudeCode)

	if err != nil {
		t.Fatalf("MarkUserInteractionTool returned unexpected error: %v", err)
	}
	if want := []settingsAdd{{domain.HarnessClaudeCode, markedTool}}; !reflect.DeepEqual(m.store.adds, want) {
		t.Errorf("saved = %v, want %v", m.store.adds, want)
	}
	waiting, counted := markWaiting(t, got.Report)
	if waiting != time.Second {
		t.Errorf("waiting after marking = %v, want 1s (the tool call)", waiting)
	}
	if len(counted) != 1 || counted[0].Tool != markedTool || counted[0].Harness != domain.HarnessClaudeCode {
		t.Errorf("counted tools = %v, want %s under claude-code", counted, markedTool)
	}
}

func TestMarkUserInteractionTool_Success_ResultNamesPathToolAndHarnesses(t *testing.T) {
	m := newMarkEnv(t, true)

	got, err := m.mark(markedTool, domain.HarnessClaudeCode)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ConfigPath != m.store.filePath || got.Tool != markedTool {
		t.Errorf("ConfigPath/Tool = %q/%q, want %q/%q", got.ConfigPath, got.Tool, m.store.filePath, markedTool)
	}
	if want := []domain.Harness{domain.HarnessClaudeCode}; !reflect.DeepEqual(got.Harnesses, want) {
		t.Errorf("Harnesses = %v, want %v", got.Harnesses, want)
	}
}

func TestMarkUserInteractionTool_Success_KeepsTheInputReportsQuality(t *testing.T) {
	m := newMarkEnv(t, true)
	input := m.res.Report
	input.Quality = domain.NewQualitySummary([]domain.Finding{
		{Kind: domain.FindingInvalidSettingsEntry, Severity: domain.SeverityWarning},
	})

	got, err := m.svc.MarkUserInteractionTool(context.Background(), m.res.Session, input, markedTool,
		[]domain.Harness{domain.HarnessClaudeCode})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got.Report.Quality, input.Quality) {
		t.Errorf("Quality = %+v, want the input report's %+v", got.Report.Quality, input.Quality)
	}
}

func TestMarkUserInteractionTool_Success_UpdatesTheSessionSettingsWithoutRereading(t *testing.T) {
	m := newMarkEnv(t, true)
	readsBefore := len(m.env.digests.readCalls)
	classifiesBefore := len(m.env.logSource.classifyCalls)
	logReadsBefore := totalReads(m.env.reader)

	if _, err := m.mark(markedTool, domain.HarnessClaudeCode); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !m.res.Session.Settings().IsUserInteractionTool(domain.HarnessClaudeCode, markedTool) {
		t.Error("session settings must count the marked tool as user-interaction for claude-code")
	}
	if m.res.Session.Settings().IsUserInteractionTool(domain.HarnessGHCPCLI, markedTool) {
		t.Error("session settings must not count the tool for a harness it was not saved under")
	}
	if len(m.env.digests.readCalls) != readsBefore {
		t.Error("marking a tool must not read digest files again")
	}
	if len(m.env.logSource.classifyCalls) != classifiesBefore || totalReads(m.env.reader) != logReadsBefore {
		t.Error("marking a tool must not classify sources or read log events again")
	}
}

func totalReads(r *fakeEventReader) int {
	n := 0
	for _, c := range r.readCounts {
		n += c
	}
	return n
}

// markMixedRun has a claude-code agent and a ghcp-cli agent, in separate
// non-overlapping invocations, that each made one one-second call of markedTool.
func markMixedRun() domain.RunAggregate {
	run := markRun()
	ghcp := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "tester#1", Type: "tester"}
	usage := []domain.InvocationUsage{{
		Model: dgModel, Harness: domain.HarnessGHCPCLI, RecordID: "g1",
		At:    dgT1.Add(125 * time.Second),
		Usage: domain.TokenUsage{Input: domain.Tokens(1000), Output: domain.Tokens(1)},
	}}
	run.Agents = append(run.Agents, domain.ActorAggregate{Actor: ghcp, Invocations: usage})
	run.Activity.Invocations = append(run.Activity.Invocations, domain.Invocation{
		Actor: ghcp, State: domain.InvocationClosed, StatusCode: "SUCCESS",
		StartedAt: dgT1.Add(120 * time.Second), EndedAt: dgT1.Add(180 * time.Second), Usage: usage,
		ToolCalls: []domain.ToolCall{reanalyzeTool("g", markedTool, 130)},
	})
	return run
}

func TestMarkUserInteractionTool_MixedRun_MarkedUnderOneHarnessCountsOnlyThatHarnesssCalls(t *testing.T) {
	m := newMarkEnvFor(t, true, markMixedRun())

	got, err := m.mark(markedTool, domain.HarnessClaudeCode)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waiting, counted := markWaiting(t, got.Report)
	if waiting != time.Second {
		t.Errorf("waiting after marking = %v, want 1s (only the claude-code call)", waiting)
	}
	if len(counted) != 1 || counted[0].Harness != domain.HarnessClaudeCode {
		t.Errorf("counted tools = %v, want %s under claude-code only", counted, markedTool)
	}
}

func TestMarkUserInteractionTool_SeveralHarnesses_SavedInSortedOrderAndReportedSorted(t *testing.T) {
	m := newMarkEnv(t, true)

	got, err := m.mark(markedTool, domain.HarnessGHCPCLI, domain.HarnessClaudeCode)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []settingsAdd{{domain.HarnessClaudeCode, markedTool}, {domain.HarnessGHCPCLI, markedTool}}
	if !reflect.DeepEqual(m.store.adds, want) {
		t.Errorf("saved = %v, want %v", m.store.adds, want)
	}
	if wantH := []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}; !reflect.DeepEqual(got.Harnesses, wantH) {
		t.Errorf("Harnesses = %v, want %v", got.Harnesses, wantH)
	}
}

func TestMarkUserInteractionTool_NoSettingsStore_ReturnsErrSettingsUnavailable(t *testing.T) {
	m := newMarkEnv(t, false)

	_, err := m.mark(markedTool, domain.HarnessClaudeCode)

	if !errors.Is(err, app.ErrSettingsUnavailable) {
		t.Errorf("error = %v, want ErrSettingsUnavailable", err)
	}
}

func TestMarkUserInteractionTool_InvalidInput_ReturnsTheErrorAndSavesNothing(t *testing.T) {
	cases := []struct {
		name      string
		tool      string
		harnesses []domain.Harness
		want      error
	}{
		{"no harnesses", markedTool, nil, domain.ErrInvalidHarness},
		{"empty harness among valid ones", markedTool, []domain.Harness{domain.HarnessClaudeCode, ""}, domain.ErrInvalidHarness},
		{"blank tool name", "   ", []domain.Harness{domain.HarnessClaudeCode}, domain.ErrInvalidToolName},
		{"tool name with newline", "a\nb", []domain.Harness{domain.HarnessClaudeCode}, domain.ErrInvalidToolName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newMarkEnv(t, true)

			_, err := m.mark(c.tool, c.harnesses...)

			if !errors.Is(err, c.want) {
				t.Errorf("error = %v, want one wrapping %v", err, c.want)
			}
			if len(m.store.adds) != 0 {
				t.Errorf("saved = %v, want nothing saved before validation passes", m.store.adds)
			}
			if m.res.Session.Settings().IsUserInteractionTool(domain.HarnessClaudeCode, markedTool) {
				t.Error("session settings must stay unchanged")
			}
		})
	}
}

func TestMarkUserInteractionTool_StoreFailure_ReturnsItAndLeavesTheSessionUnchanged(t *testing.T) {
	m := newMarkEnv(t, true)
	boom := errors.New("disk full")
	m.store.failFor = map[domain.Harness]error{domain.HarnessClaudeCode: boom}

	_, err := m.mark(markedTool, domain.HarnessClaudeCode)

	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want the store's error", err)
	}
	if m.res.Session.Settings().IsUserInteractionTool(domain.HarnessClaudeCode, markedTool) {
		t.Error("session settings must stay unchanged after a failed save")
	}
}

func TestMarkUserInteractionTool_FailureOnLaterHarness_KeepsEarlierSaveAndRecomputesNothing(t *testing.T) {
	m := newMarkEnv(t, true)
	boom := errors.New("read-only file")
	m.store.failFor = map[domain.Harness]error{domain.HarnessGHCPCLI: boom}

	_, err := m.mark(markedTool, domain.HarnessClaudeCode, domain.HarnessGHCPCLI)

	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want the store's error", err)
	}
	if want := []settingsAdd{{domain.HarnessClaudeCode, markedTool}}; !reflect.DeepEqual(m.store.adds, want) {
		t.Errorf("saved = %v, want the earlier harness only: %v", m.store.adds, want)
	}
	if m.res.Session.Settings().IsUserInteractionTool(domain.HarnessClaudeCode, markedTool) {
		t.Error("session settings must stay unchanged when any save failed")
	}
}
