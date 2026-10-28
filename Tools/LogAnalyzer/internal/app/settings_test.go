package app_test

// Tests for the analysis settings surface of the app layer.
//
// Coverage:
//   - Service.SettingsPath exposes the injected store's path
//   - Analyze loads the settings, keeps them in the session, and falls back to
//     built-in defaults when no settings store is configured
//   - Settings findings join the report's quality summary
//   - An unloadable settings document makes Analyze fail like a pricing error

import (
	"context"
	"errors"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// fakeSettingsStore is an in-memory implementation of domain.SettingsStore.
type fakeSettingsStore struct {
	filePath string
	settings domain.AnalysisSettings
	findings []domain.Finding
	loadErr  error
}

func (s *fakeSettingsStore) Path() string { return s.filePath }

func (s *fakeSettingsStore) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	return s.settings, s.findings, s.loadErr
}

func (s *fakeSettingsStore) AddUserInteractionTool(_ context.Context, _ domain.Harness, _ string) error {
	return nil
}

// newSettingsTestService builds a service over a one-run source that has one
// priced agent invocation; settings may be nil.
func newSettingsTestService(settings domain.SettingsStore) *app.Service {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
	const agent = domain.AgentInstanceID("TestAgent#1")

	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			inv := oneRunTwoAgentInventory(s)
			inv.Runs[0].Agents = inv.Runs[0].Agents[:1]
			return inv, nil
		},
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile, appTestRunEndEvent())
	reader.addEvents(appTestAgent1File,
		appTestInvEndEvent(agent, "model-a", domain.TokenUsage{Input: domain.Tokens(100)}))

	pricing := newFakePricingStore()
	pricing.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing("model-a")})

	return app.New(app.Deps{
		Source:      logSource,
		Reader:      reader,
		Pricing:     pricing,
		Settings:    settings,
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	})
}

func analyzeSettingsTest(t *testing.T, settings domain.SettingsStore) (app.Result, error) {
	t.Helper()
	return newSettingsTestService(settings).Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
}

func TestSettingsPath_ReturnsInjectedStorePath(t *testing.T) {
	svc := newSettingsTestService(&fakeSettingsStore{filePath: "/root/MosaicLogAnalyzer/config/analysis.yaml"})

	if got := svc.SettingsPath(); got != "/root/MosaicLogAnalyzer/config/analysis.yaml" {
		t.Errorf("SettingsPath() = %q, want the store's path", got)
	}
}

func TestSettingsPath_NilStore_IsEmpty(t *testing.T) {
	svc := newSettingsTestService(nil)

	if got := svc.SettingsPath(); got != "" {
		t.Errorf("SettingsPath() = %q, want empty without a settings store", got)
	}
}

func TestAnalyze_NilSettingsStore_AppliesBuiltInDefaultsWithoutError(t *testing.T) {
	result, err := analyzeSettingsTest(t, nil)

	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", result.Outcome)
	}
	got := result.Session.Settings()
	if got.CompactionDropPercent != 30 {
		t.Errorf("session CompactionDropPercent = %d, want default 30", got.CompactionDropPercent)
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Write") ||
		!got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("session settings must hold the built-in defaults when no store is configured")
	}
	if hasFindingKindApp(result.Report.Quality.Findings, domain.FindingInvalidSettingsEntry) {
		t.Error("no settings findings expected without a store")
	}
}

func TestAnalyze_SessionKeepsTheLoadedSettings(t *testing.T) {
	custom := domain.AnalysisSettings{
		WriteTools:            map[domain.Harness][]string{domain.HarnessClaudeCode: {"OnlyThis"}},
		UserInteractionTools:  map[domain.Harness][]string{domain.HarnessClaudeCode: {"mcp__contact-user__*"}},
		CompactionDropPercent: 45,
	}

	result, err := analyzeSettingsTest(t, &fakeSettingsStore{filePath: "/s.yaml", settings: custom})

	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	got := result.Session.Settings()
	if got.CompactionDropPercent != 45 {
		t.Errorf("session CompactionDropPercent = %d, want 45", got.CompactionDropPercent)
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "OnlyThis") || got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("session must keep the loaded write-tool list, not the defaults")
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__contact-user__ask") {
		t.Error("session must keep the loaded glob pattern")
	}
}

func TestAnalyze_SettingsFindingsJoinQualitySummary(t *testing.T) {
	finding := domain.Finding{
		Kind:     domain.FindingInvalidSettingsEntry,
		Severity: domain.SeverityWarning,
		Path:     "/s.yaml",
		Detail:   "invalid drop_percent",
	}
	store := &fakeSettingsStore{
		filePath: "/s.yaml",
		settings: domain.DefaultAnalysisSettings(),
		findings: []domain.Finding{finding},
	}

	result, err := analyzeSettingsTest(t, store)

	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if !hasFindingKindApp(result.Report.Quality.Findings, domain.FindingInvalidSettingsEntry) {
		t.Errorf("settings finding missing from Report.Quality.Findings: %v", result.Report.Quality.Findings)
	}
	if n := result.Report.Quality.Counts[domain.FindingInvalidSettingsEntry]; n != 1 {
		t.Errorf("Quality.Counts[invalid_settings_entry] = %d, want 1", n)
	}
	if result.Report.Quality.Incomplete() {
		t.Error("a settings finding is not a data-loss kind; the report must not be marked incomplete")
	}
}

func TestAnalyze_SettingsLoadError_IsReturned(t *testing.T) {
	loadErr := errors.New("settings document is unparseable")
	store := &fakeSettingsStore{filePath: "/s.yaml", loadErr: loadErr}

	_, err := analyzeSettingsTest(t, store)

	if !errors.Is(err, loadErr) {
		t.Errorf("Analyze err = %v, want the settings load error", err)
	}
}

func totalForRunSettingsTest(t *testing.T, settings domain.SettingsStore) (app.RunTotal, app.Outcome, error) {
	t.Helper()
	return newSettingsTestService(settings).TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)
}

func TestTotalForRun_SettingsLoadError_IsReturned(t *testing.T) {
	loadErr := errors.New("settings document is unparseable")
	store := &fakeSettingsStore{filePath: "/s.yaml", loadErr: loadErr}

	_, _, err := totalForRunSettingsTest(t, store)

	if !errors.Is(err, loadErr) {
		t.Errorf("TotalForRun err = %v, want the settings load error", err)
	}
}

func TestTotalForRun_NilSettingsStore_WorksWithoutError(t *testing.T) {
	_, outcome, err := totalForRunSettingsTest(t, nil)

	if err != nil || outcome != app.OutcomeReport {
		t.Errorf("TotalForRun = (%v, %v), want report outcome without error", outcome, err)
	}
}

func TestTotalForRun_SettingsWithFindings_StillReturnsTheTotal(t *testing.T) {
	store := &fakeSettingsStore{
		filePath: "/s.yaml",
		settings: domain.DefaultAnalysisSettings(),
		findings: []domain.Finding{{
			Kind:     domain.FindingInvalidSettingsEntry,
			Severity: domain.SeverityWarning,
			Path:     "/s.yaml",
			Detail:   "invalid drop_percent",
		}},
	}

	_, outcome, err := totalForRunSettingsTest(t, store)

	if err != nil || outcome != app.OutcomeReport {
		t.Errorf("TotalForRun = (%v, %v), want report outcome without error", outcome, err)
	}
}
