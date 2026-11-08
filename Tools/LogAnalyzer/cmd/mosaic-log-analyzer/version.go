package main

import (
	"fmt"
	"io"

	"mosaic-log-analyzer/internal/cli"
)

// versionRequested reports whether args (os.Args[1:], unmodified) ask for the
// version line instead of any other frontend. Only a lone --version counts;
// any combination follows the normal command handling.
func versionRequested(args []string) bool {
	return len(args) == 1 && args[0] == "--version"
}

// runVersion writes the version line to stdout and returns the process exit code.
func runVersion(stdout io.Writer) int {
	if _, err := fmt.Fprintf(stdout, "mosaic-log-analyzer %s\n", ToolVersion); err != nil {
		return cli.ExitFailure
	}
	return 0
}
