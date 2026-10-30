package cli_test

// Tests for the `settings path` subcommand.

import (
	"bytes"
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// fakeSettingsStore implements domain.SettingsStore and records calls that
// would create or modify the settings file.
type fakeSettingsStore struct {
	filePath string
	loads    int
	adds     int
}

func (s *fakeSettingsStore) Path() string { return s.filePath }

func (s *fakeSettingsStore) Paths() domain.ConfigPaths {
	return domain.ConfigPaths{Project: domain.ConfigFile{Level: domain.LevelProject, Path: s.filePath}}
}

func (s *fakeSettingsStore) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	s.loads++
	return domain.DefaultAnalysisSettings(), nil, nil
}

func (s *fakeSettingsStore) AddUserInteractionTool(_ context.Context, level domain.ConfigLevel, _ domain.Harness, _ string) (domain.ConfigFile, error) {
	s.adds++
	return domain.ConfigFile{Level: level, Path: s.filePath, Exists: true}, nil
}

func TestExecute_SettingsPathSubcommand_PrintsSettingsFilePath(t *testing.T) {
	const settingsPath = "/root/MosaicLogAnalyzer/config/analysis.yaml"
	settings := &fakeSettingsStore{filePath: settingsPath}
	svc := app.New(app.Deps{
		Source:      &fakeLogSource{},
		Reader:      newFakeEventReader(),
		Pricing:     &fakePricingStore{filePath: "/root/MosaicLogAnalyzer/config/pricing.yaml", table: domain.EmptyPricingTable()},
		Settings:    settings,
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	})
	stdout := &bytes.Buffer{}

	exitCode := cli.Execute(cli.Options{Service: svc, Stdout: stdout, Stderr: &bytes.Buffer{}},
		[]string{"settings", "path"})

	if exitCode != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)", exitCode, cli.ExitSuccess)
	}
	if got, want := stdout.String(), settingsPath+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if settings.adds != 0 {
		t.Error("`settings path` must never create or modify the settings file")
	}
}

func TestExecute_SettingsPathSubcommand_DoesNotPrintPricingPath(t *testing.T) {
	const pricingPath = "/root/pricing.yaml"
	svc := app.New(app.Deps{
		Source:      &fakeLogSource{},
		Reader:      newFakeEventReader(),
		Pricing:     &fakePricingStore{filePath: pricingPath, table: domain.EmptyPricingTable()},
		Settings:    &fakeSettingsStore{filePath: "/root/analysis.yaml"},
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	})
	stdout := &bytes.Buffer{}

	cli.Execute(cli.Options{Service: svc, Stdout: stdout, Stderr: &bytes.Buffer{}}, []string{"settings", "path"})

	if got := stdout.String(); got != "/root/analysis.yaml\n" {
		t.Errorf("stdout = %q, want only the settings path", got)
	}
}
