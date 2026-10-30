package cli_test

// Tests that the command line never writes a configuration file: prices and tool
// marks are changed in the TUI or by editing the files, and the help of the
// `pricing` and `settings` commands says so.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// readOnlyEnv is a service over one run with an unpriced model, wired the way
// the composition root wires the command line: the interaction has no answers.
type readOnlyEnv struct {
	svc      *app.Service
	pricing  *fakePricingStore
	settings *fakeSettingsStore
}

func newReadOnlyEnv(t *testing.T) readOnlyEnv {
	t.Helper()
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"}
	reader := newFakeEventReader()
	reader.addEvents(cliTestOrchFile,
		domain.Event{Type: domain.EventToolCallStart, ToolCallStart: &domain.ToolCallStartFields{
			CallID: "c1", ToolName: "UnmarkedTool"}},
		domain.Event{Type: domain.EventToolCallEnd, ToolCallEnd: &domain.ToolCallEndFields{
			CallID: "c1", ToolName: "UnmarkedTool", Status: domain.ToolCallSuccess}},
		cliTestTurnEvent("unpriced-model", domain.TokenUsage{Input: domain.Tokens(1_000_000)}),
		cliTestRunEndEvent(),
	)
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			return domain.Inventory{Source: s, Runs: []domain.RunEntry{{
				Run: cliTestRunRef, Dir: "/logs/" + cliTestRunID, OrchestratorFile: cliTestOrchFile,
			}}}, nil
		},
	}
	pricing := newFakePricingStore()
	settings := &fakeSettingsStore{filePath: "/root/MosaicLogAnalyzer/config/analysis.yaml"}
	svc := app.New(app.Deps{
		Source:      logSource,
		Reader:      reader,
		Pricing:     pricing,
		Settings:    settings,
		Clock:       newFakeClock(),
		Interaction: cli.NewInteraction(io.Discard, cli.Answers{}),
		WorkDir:     "/workdir",
		Ingester:    &fakeUsageIngester{result: doneResult()},
	})
	return readOnlyEnv{svc: svc, pricing: pricing, settings: settings}
}

func TestExecute_EveryCommand_OverUnpricedModelsWritesNoPricingOrSettingsFile(t *testing.T) {
	commands := [][]string{
		{"report", "--path", "/logs"},
		{"report", "--path", "/logs", "--format", "json"},
		{"total", "--path", "/logs", "--run", cliTestRunID},
		{"pricing", "path"},
		{"settings", "path"},
		{"export", "--path", "/logs", "--out", "/out/all.digest.json"},
		{"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/exports/otel.jsonl"},
	}
	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			env := newReadOnlyEnv(t)

			cli.Execute(cli.Options{Service: env.svc, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}, args)

			if env.pricing.puts != 0 {
				t.Errorf("the command wrote the pricing file %d time(s); no command may write it", env.pricing.puts)
			}
			if env.settings.adds != 0 {
				t.Errorf("the command wrote the settings file %d time(s); no command may write it", env.settings.adds)
			}
		})
	}
}

// helpOf returns everything the command prints for --help on either stream.
func helpOf(t *testing.T, args ...string) string {
	t.Helper()
	env := newReadOnlyEnv(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	cli.Execute(cli.Options{Service: env.svc, Stdout: stdout, Stderr: stderr}, append(args, "--help"))

	return stdout.String() + stderr.String()
}

func TestExecute_PricingHelp_SaysPricesAreChangedInTheTUIOrByEditingAndPointsToPricingPath(t *testing.T) {
	help := helpOf(t, "pricing")

	for _, want := range []string{"TUI", "pricing path"} {
		if !strings.Contains(help, want) {
			t.Errorf("`pricing --help` must mention %q\n%s", want, help)
		}
	}
	if !strings.Contains(strings.ToLower(help), "edit") {
		t.Errorf("`pricing --help` must say the files can be edited\n%s", help)
	}
}

func TestExecute_SettingsHelp_SaysToolMarksAreChangedInTheTUIOrByEditingAndPointsToSettingsPath(t *testing.T) {
	help := helpOf(t, "settings")

	for _, want := range []string{"TUI", "settings path"} {
		if !strings.Contains(help, want) {
			t.Errorf("`settings --help` must mention %q\n%s", want, help)
		}
	}
	if !strings.Contains(strings.ToLower(help), "edit") {
		t.Errorf("`settings --help` must say the files can be edited\n%s", help)
	}
}
