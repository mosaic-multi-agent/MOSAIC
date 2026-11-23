package main

// exit_wiring_guard_test.go covers the exit wiring paths beyond the TUI start: the guarded
// program factory handed to the TUI, panics outside the TUI, and startup failures reached
// through the catalogue folder flag.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/tui"
)

// panickingModel panics as soon as the program initialises it.
type panickingModel struct{}

func (panickingModel) Init() tea.Cmd                       { panic("model exploded") }
func (panickingModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return panickingModel{}, nil }
func (panickingModel) View() string                        { return "" }

func TestRunTool_TUIStart_ReceivesAGuardedProgramFactory(t *testing.T) {
	root := newStackMosaicRoot(t)
	var got tui.Options
	capture := func(_ context.Context, _ app.Service, opts tui.Options) error {
		got = opts
		return nil
	}

	runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: capture})

	if got.NewProgram == nil {
		t.Fatal("tui.Options.NewProgram is nil: the TUI would build an unguarded program")
	}
}

func TestRunTool_PanicInsideTUIProgram_IsReportedWithMessageAndPauses(t *testing.T) {
	root := newStackMosaicRoot(t)
	runProgram := func(_ context.Context, _ app.Service, opts tui.Options) error {
		if opts.NewProgram == nil {
			return errors.New("no guarded program factory")
		}
		p := opts.NewProgram(panickingModel{}, tea.WithInput(nil), tea.WithOutput(&strings.Builder{}), tea.WithoutSignals())
		_, err := p.Run()
		return err
	}

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: runProgram})

	content := requireReported(t, run, "panic: model exploded", true)
	if !strings.Contains(string(content), "Kind: panic") {
		t.Errorf("failure file is not a panic report:\n%s", content)
	}
}

func TestRunTool_PanicInCLIMode_IsReportedWithFileAndNeverPauses(t *testing.T) {
	getwd := func() (string, error) { panic("cwd exploded") }
	args := []string{"render", "--source-agent", "default-only-agent", "--workflows", "", "--overwrite",
		"--output", "json", "--harness", "claude-code", "--workspace", t.TempDir()}

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: args, Getwd: getwd})

	content := requireReported(t, run, "panic: cwd exploded", false)
	if !strings.Contains(string(content), "Kind: panic") {
		t.Errorf("failure file is not a panic report:\n%s", content)
	}
}

func TestRunTool_InvalidCatalogFolder_WritesFileNamesItAndNeverPauses(t *testing.T) {
	root := newStackMosaicRoot(t)
	args := append(renderJSONArgs(root, t.TempDir()), "--catalog-folder", filepath.Join(t.TempDir(), "missing-catalog"))

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: args})

	content := requireReported(t, run, "missing-catalog", false)
	if !strings.Contains(string(content), "Kind: failure") {
		t.Errorf("failure file is not a failure report:\n%s", content)
	}
}

// Fatal exits for the tool config, the user config and harness discovery.
//
// Harness discovery has no cheap trigger: with external modules disallowed the registry only
// consults built-in harnesses, which cannot fail on a fixture root, so it is excluded here.

// newRootWithUnreadableConfig returns a fixture root where the named config file is a
// directory, so loading it fails with an error other than "file absent".
func newRootWithUnreadableConfig(t *testing.T, name string) string {
	t.Helper()
	root := newStackMosaicRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "MosaicDeploy", "config", name), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	return root
}

func TestRunTool_TUIOnWindows_ToolConfigLoadFailure_ShowsMessageAndPathAndPauses(t *testing.T) {
	root := newRootWithUnreadableConfig(t, "tool-config.yaml")

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: noopTUI})

	content := requireReported(t, run, "error: load tool config: ", true)
	if !strings.Contains(string(content), "Kind: failure") {
		t.Errorf("failure file is not a failure report:\n%s", content)
	}
}

func TestRunTool_CLIToolConfigLoadFailure_WritesFileNamesItAndNeverPauses(t *testing.T) {
	root := newRootWithUnreadableConfig(t, "tool-config.yaml")

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: renderJSONArgs(root, t.TempDir())})

	requireReported(t, run, "error: load tool config: ", false)
}

func TestRunTool_TUIOnWindows_UserConfigLoadFailure_ShowsMessageAndPathAndPauses(t *testing.T) {
	root := newRootWithUnreadableConfig(t, "user-config.yaml")

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: noopTUI})

	content := requireReported(t, run, "error: load user config: ", true)
	if !strings.Contains(string(content), "Kind: failure") {
		t.Errorf("failure file is not a failure report:\n%s", content)
	}
}

func TestRunTool_CLIUserConfigLoadFailure_WritesFileNamesItAndNeverPauses(t *testing.T) {
	root := newRootWithUnreadableConfig(t, "user-config.yaml")

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: renderJSONArgs(root, t.TempDir())})

	requireReported(t, run, "error: load user config: ", false)
}

// Accepted gap: outcome code 2 (completed with skips) is not reachable through the wiring
// without a deployment fixture; exit code 3 is covered at binary and wiring level.
