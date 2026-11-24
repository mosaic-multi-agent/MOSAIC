package main

// exit_wiring.go is the composition root's exit wiring: every end of the process goes through
// the shared fatal reporter, and each frontend returns after its terminating call so an
// injected Exit that returns ends the run.

import (
	"context"
	"io"
	"os"

	"mosaic-common/fatal"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui"
)

// exitSeams are the process facts the wiring reads, injectable so tests can drive every exit
// path without a real terminal, working directory or TUI.
type exitSeams struct {
	// IsTerminal reports whether stdin and stdout are both attached to a terminal.
	IsTerminal func() bool
	// Getwd returns the working directory.
	Getwd func() (string, error)
	// Stdout receives the version line and the result of CLI commands.
	Stdout io.Writer
	// RunTUI runs the interactive frontend.
	RunTUI func(ctx context.Context, sess session.Session, opts tui.Options) error
}

// defaultExitSeams returns the seams bound to the real process.
func defaultExitSeams() exitSeams {
	return exitSeams{
		IsTerminal: stdioIsTerminal,
		Getwd:      os.Getwd,
		Stdout:     os.Stdout,
		RunTUI:     tui.Run,
	}
}

// runTool runs the whole tool for args (os.Args[1:]) and ends it through the fatal reporter.
// base supplies the reporter's seams (Args, GOOS, Stdin, Stderr, Exit, Now, FailureDir);
// runTool sets Tool, Version and Mode on it. After every terminating call runTool returns, so
// an injected Exit that returns ends the run.
func runTool(base fatal.Config, args []string, seams exitSeams) {
	fe := selectFrontend(args, seams.IsTerminal)
	base.Tool = "mosaic-run"
	base.Version = ToolVersion
	base.Mode = fatal.ModeCLI
	if fe == frontendTUI {
		base.Mode = fatal.ModeTUI
	}
	r := fatal.Install(base)
	defer fatal.Recover()

	// The entry-point-only --dev flag is consumed here and stripped before cobra sees it.
	devMode := scanBoolFlag(args, "--dev")
	cobraArgs := stripBoolFlag(args, "--dev")

	switch fe {
	case frontendVersion:
		r.Exit(runVersion(seams.Stdout))
	case frontendVersionUsage:
		r.Exit(runVersionUsage(r.Stderr()))
	case frontendTUI:
		runTUIMode(r, cobraArgs, devMode, seams)
	case frontendDevTest:
		r.ExitCLI(runDevTestMode(cobraArgs, seams.Getwd, seams.Stdout, r.Stderr()))
	default:
		r.ExitCLI(runCLI(args, cobraArgs, seams.Getwd, seams.Stdout, r.Stderr()))
	}
}
