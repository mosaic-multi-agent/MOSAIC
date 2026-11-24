package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"

	"mosaic-common/fatal"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/findingsfile"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/otelusage"
	"mosaic-log-analyzer/internal/pricing"
	"mosaic-log-analyzer/internal/settings"
	"mosaic-log-analyzer/internal/transcript"
	"mosaic-log-analyzer/internal/tui"
)

// exitSeams are the collaborators of run that tests replace. The zero value of a
// field means the real one.
type exitSeams struct {
	// IsTerminal is the per-fd terminal check of the mode decision (default isatty).
	IsTerminal func(fd uintptr) bool
	// Getwd returns the working directory (default os.Getwd).
	Getwd func() (string, error)
	// RunTUI runs the TUI frontend (default tui.Run).
	RunTUI func(opts tui.Options) (int, error)
	// RunCLI runs the CLI frontend for args and returns its exit code (default
	// builds the real adapters and calls cli.Execute).
	RunCLI func(args []string) int
}

// run is the process wiring: it decides the mode, installs the fatal-exit
// reporter on top of base (keeping every seam field base sets), and ends the
// process through the reporter on every path. args is os.Args[1:].
func run(base fatal.Config, args []string, seams exitSeams) {
	if seams.IsTerminal == nil {
		seams.IsTerminal = isatty.IsTerminal
	}
	if seams.Getwd == nil {
		seams.Getwd = os.Getwd
	}
	if seams.RunTUI == nil {
		seams.RunTUI = tui.Run
	}

	version := versionRequested(args)
	cfg := base
	cfg.Tool = "mosaic-log-analyzer"
	cfg.Version = ToolVersion
	cfg.Mode = fatal.ModeCLI
	if !version && wantsTUI(args, seams.IsTerminal) {
		cfg.Mode = fatal.ModeTUI
	}
	if cfg.FailureDir == nil {
		cfg.FailureDir = globalFailureDir
	}
	r := fatal.Install(cfg)
	defer fatal.Recover()

	if version {
		r.Exit(runVersion(os.Stdout))
		return
	}

	workDir, err := seams.Getwd()
	if err != nil {
		r.Fail(fmt.Sprintf("error: getting working directory: %v", err))
		return
	}

	if cfg.Mode == fatal.ModeTUI {
		runTUIMode(r, workDir, args, seams)
		return
	}
	runCLI := seams.RunCLI
	if runCLI == nil {
		runCLI = func(args []string) int { return executeCLI(r, workDir, args) }
	}
	r.ExitCLI(runCLI(args))
}

// globalFailureDir is the folder failure files go to: the global config folder,
// created when absent. It fails when the folder cannot be determined or created.
func globalFailureDir() (string, error) {
	dir := globalConfigDir()
	if dir == "" {
		return "", fmt.Errorf("no user config folder")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// adapters are the concrete infrastructure adapters shared by both frontends.
type adapters struct {
	scanner     *logscan.Scanner
	pricing     *pricing.Store
	settings    *settings.Store
	usageIngest *otelusage.Adapter
}

// newService wires the application service around the adapters for one frontend.
// ingest is set for the CLI, which runs ingest-otel.
func newService(a adapters, workDir string, interact domain.Interaction, ingest bool) *app.Service {
	deps := app.Deps{
		Source:        a.scanner,
		Reader:        logread.NewReader(),
		Pricing:       a.pricing,
		Settings:      a.settings,
		Digests:       digest.NewStore(),
		ToolVersion:   ToolVersion,
		Clock:         &realClock{},
		Interaction:   interact,
		WorkDir:       workDir,
		IngestProbe:   a.usageIngest,
		Alarms:        a.scanner,
		Transcripts:   transcript.NewReader(),
		FindingsFiles: findingsfile.NewWriter(),
	}
	if ingest {
		deps.Ingester = a.usageIngest
	}
	return app.New(deps)
}

// newAdapters constructs the adapters and writes the global config templates.
func newAdapters(workDir string) adapters {
	globalDir := globalConfigDir()
	a := adapters{
		scanner:     logscan.NewScanner(),
		pricing:     pricing.NewStore(globalDir, workDir),
		settings:    settings.NewStore(globalDir, workDir),
		usageIngest: otelusage.New(),
	}
	ensureGlobalFiles(a.pricing, a.settings)
	return a
}

// runTUIMode runs the TUI frontend and ends the process through r.
func runTUIMode(r *fatal.Reporter, workDir string, args []string, seams exitSeams) {
	a := newAdapters(workDir)
	tuiInteract, tuiQuestions := tui.NewInteraction()
	svc := newService(a, workDir, tuiInteract, false)

	code, err := seams.RunTUI(tui.Options{
		Service:     svc,
		InitialPath: scanFlag(args, "--path", ""),
		Version:     ToolVersion,
		Interaction: tuiInteract,
		Questions:   tuiQuestions,
		NewProgram:  r.NewProgram,
	})
	if err != nil {
		r.Fail(fmt.Sprintf("error: the TUI could not run: %v", err))
		return
	}
	r.Exit(code)
}

// executeCLI builds the real adapters and runs the cobra command tree, writing
// stderr through the reporter so a failure is reported with its text.
func executeCLI(r *fatal.Reporter, workDir string, args []string) int {
	a := newAdapters(workDir)
	interact := cli.NewInteraction(os.Stdout, cli.Answers{})
	svc := newService(a, workDir, interact, true)
	return cli.Execute(cli.Options{
		Service: svc,
		Stdout:  os.Stdout,
		Stderr:  r.Stderr(),
		Version: ToolVersion,
		Getenv:  os.Getenv,
	}, args)
}
