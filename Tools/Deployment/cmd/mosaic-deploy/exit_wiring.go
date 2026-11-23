package main

// exit_wiring.go is the composition root's exit wiring: every end of the process goes through
// the shared fatal reporter, and the dependency construction that used to live in main runs
// here so each failing step can report and return.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"mosaic-common/fatal"
	defaultconfig "mosaic-deploy/config"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/logging"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/todo"
	"mosaic-deploy/internal/tui"
)

// exitSeams are the process facts the wiring reads, injectable so tests can drive every exit
// path without a real terminal, working directory or TUI.
type exitSeams struct {
	// IsTerminal reports whether stdin and stdout are both attached to a terminal.
	IsTerminal func() bool
	// Getwd returns the working directory.
	Getwd func() (string, error)
	// Stdout receives the result of CLI commands.
	Stdout io.Writer
	// RunTUI runs the interactive frontend.
	RunTUI func(ctx context.Context, svc app.Service, opts tui.Options) error
}

// defaultExitSeams returns the seams bound to the real process.
func defaultExitSeams() exitSeams {
	return exitSeams{
		IsTerminal: func() bool {
			return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
		},
		Getwd:  os.Getwd,
		Stdout: os.Stdout,
		RunTUI: tui.Run,
	}
}

// runTool runs the whole tool for args and ends it through the fatal reporter. base supplies
// the reporter's seams (Args, GOOS, Stdin, Stderr, Exit, Now, FailureDir); runTool sets Tool,
// Version and Mode on it. After every terminating call runTool returns, so an injected Exit
// that returns ends the run.
func runTool(base fatal.Config, args []string, seams exitSeams) {
	base.Tool = "mosaic-deploy"
	base.Version = ToolVersion
	base.Mode = fatal.ModeCLI
	tuiStart := !versionRequested(args) && len(args) == 0 && seams.IsTerminal()
	if tuiStart {
		base.Mode = fatal.ModeTUI
	}
	r := fatal.Install(base)
	defer fatal.Recover()

	if versionRequested(args) {
		r.Exit(runVersion(seams.Stdout))
		return
	}
	if len(args) == 0 && !tuiStart {
		fmt.Fprintln(r.Stderr(), "usage: mosaic-deploy <deploy|update> [flags]")
		r.Exit(cli.ExitUsage)
		return
	}
	runFrontend(r, args, seams, tuiStart)
}

// runFrontend builds the dependencies and dispatches to the TUI or the CLI. It returns after
// the first terminating call on r.
func runFrontend(r *fatal.Reporter, args []string, seams exitSeams, tuiStart bool) {
	mosaicRoot, catalogFolderFlag, logDir, allowExternal := scanGlobalFlags(args)
	stderr := r.Stderr()

	// The MOSAIC root comes from the working directory when the flag is not provided.
	if mosaicRoot == "" {
		wd, err := seams.Getwd()
		if err != nil {
			r.Fail(fmt.Sprintf("error: cannot determine working directory: %v", err))
			return
		}
		resolved, err := catalog.ResolveRoot(wd)
		if err != nil {
			r.Fail(fmt.Sprintf("error: mosaic-deploy must be run from the MOSAIC repository root: %v", err))
			return
		}
		mosaicRoot = resolved
	}

	// An explicit --catalog-folder is validated fail-fast before any further construction.
	if _, err := resolveCatalogFolder(mosaicRoot, catalogFolderFlag); err != nil {
		r.Fail(fmt.Sprintf("error: %v", err))
		return
	}

	// Missing config files are created from the templates; a failure only warns.
	if err := config.SeedConfigFiles(mosaicRoot, defaultconfig.Templates); err != nil {
		fmt.Fprintf(stderr, "warning: could not create default config files: %v\n", err)
	}

	toolConfigStore := config.NewToolConfigStore(mosaicRoot)
	toolCfg, err := toolConfigStore.Load()
	if err != nil {
		r.Fail(fmt.Sprintf("error: load tool config: %v", err))
		return
	}
	userConfigStore := config.NewUserConfigStore(mosaicRoot)
	userCfg, err := userConfigStore.Load()
	if err != nil {
		r.Fail(fmt.Sprintf("error: load user config: %v", err))
		return
	}

	logger := logging.New(mosaicRoot, toolCfg, logging.WithLogDir(logDir))

	cat, stackDesc, stackOrigin, err := loadRunStack(mosaicRoot, args)
	if err != nil {
		r.Fail(fmt.Sprintf("error: load catalog: %v", err))
		return
	}

	reg, err := discoverHarnesses(
		mosaicRoot, allowExternal || toolCfg.AllowExternalModules,
		toolConfigStore, userConfigStore, toolCfg, userCfg,
		cat.HarnessInjections(),
	)
	if err != nil {
		r.Fail(fmt.Sprintf("error: discover harnesses: %v", err))
		return
	}

	// The todo collector is shared between the Interaction port and the executor so both
	// sinks write to the same checklist.
	todoCollector := todo.NewCollector()
	manifestStore := newManifestStore(reg)
	deps := app.Deps{
		Catalog:        cat,
		Registry:       reg,
		Planner:        plan.New(),
		Executor:       deploy.NewExecutor(manifestStore, logger, todoCollector),
		Manifest:       manifestStore,
		ToolConfig:     toolConfigStore,
		UserConfig:     userConfigStore,
		Logger:         logger,
		Todo:           todoCollector,
		MosaicRoot:     mosaicRoot,
		GOOS:           runtime.GOOS,
		ProtocolLoader: catalog.FileProtocolLoader{},
		BundleLoader:   catalog.FileBundleLoader{},
		ToolVersion:    ToolVersion,

		StackDescription: stackDesc,
		StackFromInput:   stackOrigin == stackconfig.OriginStackInput,
		StackOrigin:      stackOrigin,
	}

	if tuiStart {
		runTUIFrontend(r, args, seams, deps, mosaicRoot)
		return
	}

	deps.Interaction = cli.NewInteraction(buildPreAnswers(args), todoCollector, stderr)
	r.ExitCLI(cli.Run(context.Background(), args, app.New(deps), seams.Stdout, stderr))
}

// runTUIFrontend runs the TUI and ends the process according to how it finished: success and
// an externally interrupted TUI exit quietly, any other error is an unexpected failure.
func runTUIFrontend(r *fatal.Reporter, args []string, seams exitSeams, deps app.Deps, mosaicRoot string) {
	ref := tui.NewInteraction()
	deps.Interaction = ref
	reloadFunc := buildReloadFunc(mosaicRoot, deps)
	launchStack, launchGiven := tuiLaunchStack(args)
	err := seams.RunTUI(context.Background(), app.New(deps), tui.Options{
		Interaction:      ref,
		LaunchStack:      launchStack,
		LaunchStackGiven: launchGiven,
		ReloadCatalog:    reloadFunc,
		ToolVersion:      ToolVersion,
		NewProgram:       r.NewProgram,
	})
	switch {
	case err == nil:
		r.Exit(cli.ExitSuccess)
	case errors.Is(err, tea.ErrInterrupted):
		fmt.Fprintf(r.Stderr(), "error: %v\n", err)
		r.Exit(cli.ExitFailure)
	default:
		r.Fail(fmt.Sprintf("error: %v", err))
	}
}
