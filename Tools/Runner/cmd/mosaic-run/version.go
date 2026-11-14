package main

import (
	"fmt"
	"io"

	"mosaic-run/internal/cli"
)

// frontend names the entry path main() takes for an invocation.
type frontend string

const (
	// frontendVersion prints the version line and exits (lone --version).
	frontendVersion frontend = "version"
	// frontendVersionUsage rejects --version combined with any other argument
	// with a usage error (exit 2); nothing else runs.
	frontendVersionUsage frontend = "version-usage"
	// frontendTUI launches the interactive TUI.
	frontendTUI frontend = "tui"
	// frontendDevTest routes to the --dev test subcommand.
	frontendDevTest frontend = "dev-test"
	// frontendCLI runs the non-interactive CLI.
	frontendCLI frontend = "cli"
)

// selectFrontend decides the entry path for args (os.Args[1:], unmodified).
// isTerminal reports whether stdin and stdout are both terminals; it is not
// called when args contain --version. The --version decision is taken first,
// before the --tui, positional and terminal rules, which stay unchanged.
func selectFrontend(args []string, isTerminal func() bool) frontend {
	if versionRequested(args) {
		return frontendVersion
	}
	if mentionsVersion(args) {
		return frontendVersionUsage
	}
	cobraArgs := stripBoolFlag(args, "--dev")
	if wantsTUIWith(cobraArgs, isTerminal) {
		return frontendTUI
	}
	if scanBoolFlag(args, "--dev") && firstPositionalArg(cobraArgs) == "test" {
		return frontendDevTest
	}
	return frontendCLI
}

// versionRequested reports whether args (os.Args[1:], unmodified) ask for
// the version line instead of any other frontend.
func versionRequested(args []string) bool {
	return len(args) == 1 && args[0] == "--version"
}

// mentionsVersion reports whether any token is the --version flag in its bare
// or "=value" form.
func mentionsVersion(args []string) bool {
	return hasFlag(args, "--version")
}

// runVersion writes the version line to stdout and returns the process
// exit code.
func runVersion(stdout io.Writer) int {
	if _, err := fmt.Fprintf(stdout, "mosaic-run %s\n", ToolVersion); err != nil {
		return cli.ExitFailure
	}
	return 0
}

// runVersionUsage reports that --version cannot be combined with other
// arguments and returns the usage exit code.
func runVersionUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "error: --version cannot be combined with other arguments or flags")
	return cli.ExitUsage
}
