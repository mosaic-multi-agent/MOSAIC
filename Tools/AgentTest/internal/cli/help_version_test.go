package cli_test

// Tests for the --version entry on the usage surface.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-agent-test/internal/cli"
)

func TestFlags_VersionIsDocumentedAndConsumesNoValue(t *testing.T) {
	var found *cli.FlagSpec
	for _, f := range cli.Flags() {
		if f.Name == "--version" {
			f := f
			found = &f
		}
	}

	if found == nil {
		t.Fatal(`cli.Flags() has no entry named "--version"`)
	}
	if found.ConsumesValue {
		t.Error(`"--version" has ConsumesValue = true, want false`)
	}
	if found.Placeholder != "" {
		t.Errorf(`"--version" has Placeholder %q, want none`, found.Placeholder)
	}
	if found.Summary == "" {
		t.Error(`"--version" has an empty Summary`)
	}
}

func TestValueConsumingFlags_ExcludesVersion(t *testing.T) {
	for _, name := range cli.ValueConsumingFlags() {
		if name == "--version" {
			t.Error(`cli.ValueConsumingFlags() includes "--version", want it excluded`)
		}
	}
}

func TestUsage_ListsVersionFlag(t *testing.T) {
	var buf bytes.Buffer

	if err := cli.Usage(&buf, cli.Options{}); err != nil {
		t.Fatalf("cli.Usage: %v", err)
	}

	if !strings.Contains(buf.String(), "--version") {
		t.Errorf("cli.Usage output does not mention --version\noutput:\n%s", buf.String())
	}
}

func TestExecute_UsageInvocations_ListVersionAndStillOmitInterceptor(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, nil} {
		var stdout, stderr bytes.Buffer

		cli.Execute(context.Background(), args, cli.Options{Stdout: &stdout, Stderr: &stderr})

		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "--version") {
			t.Errorf("Execute(%q, ...) output does not mention --version\n%s", args, combined)
		}
		if strings.Contains(strings.ToLower(combined), "intercept") {
			t.Errorf("Execute(%q, ...) output mentions the interceptor route\n%s", args, combined)
		}
	}
}
