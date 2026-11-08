package cli_test

// The top-level help lists the --version flag. The flag itself is handled by the
// composition root; the help is what tells users it exists.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
)

func TestHelp_TopLevelListsVersionFlag(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	cli.Execute(cli.Options{Stdout: stdout, Stderr: stderr}, []string{"--help"})

	if help := stdout.String() + stderr.String(); !strings.Contains(help, "--version") {
		t.Errorf("top-level help does not list --version:\n%s", help)
	}
}
