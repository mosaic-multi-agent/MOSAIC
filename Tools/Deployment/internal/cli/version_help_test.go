package cli_test

// version_help_test.go verifies the top-level help lists the --version flag.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

func TestTopLevelHelp_ListsVersionFlag(t *testing.T) {
	var out, errOut bytes.Buffer

	cli.Run(context.Background(), []string{"--help"}, &spyService{}, &out, &errOut)

	help := out.String() + errOut.String()
	if !strings.Contains(help, "--version") {
		t.Errorf("top-level help does not list --version:\n%s", help)
	}
	if !strings.Contains(help, "Print the tool version and exit") {
		t.Errorf("top-level help does not carry the --version description:\n%s", help)
	}
}
