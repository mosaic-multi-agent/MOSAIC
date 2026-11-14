package main

import (
	"fmt"
	"io"

	"mosaic-agent-test/internal/cli"
)

// FrontendVersion is the frontend selected when the invocation asks for the
// version line instead of any other frontend.
const FrontendVersion Frontend = "version"

// versionFlag is the flag that requests the version line.
const versionFlag = "--version"

// versionRequested reports whether args (os.Args[1:], unmodified) ask for the
// version line instead of any other frontend. A "--version" token anywhere
// wins, unless it is the value of a value-consuming flag. The equals form
// ("--version=x") is not a version request.
func versionRequested(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == versionFlag {
			return true
		}
		if valueConsumingFlags[arg] && i+1 < len(args) {
			i++
		}
	}
	return false
}

// runVersion writes the version line to stdout and returns the process exit
// code.
func runVersion(stdout io.Writer) int {
	if _, err := fmt.Fprintf(stdout, "mosaic-agent-test %s\n", ToolVersion); err != nil {
		return cli.ExitFailure
	}
	return cli.ExitSuccess
}
