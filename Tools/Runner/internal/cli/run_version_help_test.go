package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
)

func TestRun_TopLevelHelp_ListsVersionFlag(t *testing.T) {
	var out, errOut bytes.Buffer

	cli.Run(context.Background(), []string{"--help"}, nil, nil, &scriptedSession{}, &out, &errOut)

	help := out.String() + errOut.String()
	if !strings.Contains(help, "--version") {
		t.Fatalf("top-level help does not list --version:\n%s", help)
	}
}

func TestAllValueBearingFlagNames_ExcludesVersion(t *testing.T) {
	for _, name := range cli.AllValueBearingFlagNames() {
		if strings.TrimLeft(name, "-") == "version" {
			t.Fatalf("--version must be boolean, found value-bearing flag %q", name)
		}
	}
}
