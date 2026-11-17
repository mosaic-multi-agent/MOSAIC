package descriptor_test

// Tests for the optional cli section of a harness descriptor.
//
// Coverage:
//   - A descriptor with the section loads and exposes both values.
//   - A descriptor without the section loads with no CLI facts.
//   - Both accepted loading mechanisms load.
//   - A null or non-mapping section, a blank or missing label, a missing or unknown
//     loading mechanism, and an unknown key inside the section are rejected with an
//     error that carries the descriptor origin.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/harness/descriptor"
)

const cliFactsOrigin = "inline-cli-facts.yaml"

// cliFactsSource builds a minimal descriptor source followed by the given extra YAML.
func cliFactsSource(extra string) []byte {
	return []byte("schema_version: \"1\"\nid: \"x\"\ndisplay_name: \"X\"\n" + extra)
}

func TestParse_CLISection_LoadsAndExposesBothValues(t *testing.T) {
	src := cliFactsSource("cli:\n  label: \"Claude Code CLI\"\n  loading_mechanism: \"name\"\n")

	d, err := descriptor.Parse(src, cliFactsOrigin)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if d.CLI == nil {
		t.Fatal("CLI = nil, want the parsed cli section")
	}
	if d.CLI.Label != "Claude Code CLI" {
		t.Errorf("CLI.Label = %q, want %q", d.CLI.Label, "Claude Code CLI")
	}
	if d.CLI.LoadingMechanism != "name" {
		t.Errorf("CLI.LoadingMechanism = %q, want %q", d.CLI.LoadingMechanism, "name")
	}
}

func TestParse_CLISectionRunnerLoaded_Loads(t *testing.T) {
	src := cliFactsSource("cli:\n  label: \"Codex CLI\"\n  loading_mechanism: \"runner-loaded\"\n")

	d, err := descriptor.Parse(src, cliFactsOrigin)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if d.CLI == nil || d.CLI.LoadingMechanism != "runner-loaded" {
		t.Errorf("CLI = %+v, want loading mechanism runner-loaded", d.CLI)
	}
}

func TestParse_NoCLISection_LoadsWithNilCLI(t *testing.T) {
	d, err := descriptor.Parse(cliFactsSource(""), cliFactsOrigin)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if d.CLI != nil {
		t.Errorf("CLI = %+v, want nil", d.CLI)
	}
}

func TestParse_InvalidCLIValue_RejectedNamingTheField(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		field string
	}{
		{"blank label", "cli:\n  label: \"  \"\n  loading_mechanism: \"name\"\n", "cli.label"},
		{"missing label", "cli:\n  loading_mechanism: \"name\"\n", "cli.label"},
		{"unknown loading mechanism", "cli:\n  label: \"X CLI\"\n  loading_mechanism: \"dynamic\"\n", "cli.loading_mechanism"},
		{"missing loading mechanism", "cli:\n  label: \"X CLI\"\n", "cli.loading_mechanism"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := descriptor.Parse(cliFactsSource(tc.extra), cliFactsOrigin)

			if err == nil {
				t.Fatalf("Parse succeeded (CLI=%+v), want a validation error", d.CLI)
			}
			var ve descriptor.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error %v is not a ValidationError", err)
			}
			if ve.Field != tc.field {
				t.Errorf("Field = %q, want %q", ve.Field, tc.field)
			}
			if ve.File != cliFactsOrigin {
				t.Errorf("File = %q, want the descriptor origin %q", ve.File, cliFactsOrigin)
			}
		})
	}
}

func TestParse_UnloadableCLISection_RejectedWithOriginAndSectionName(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		names string // text the error must additionally contain; empty when none
	}{
		{"unknown key inside section", "cli:\n  label: \"X CLI\"\n  loading_mechanism: \"name\"\n  spawn: true\n", "spawn"},
		{"bare null section", "cli:\n", ""},
		{"scalar section", "cli: \"x\"\n", ""},
		{"sequence section", "cli:\n  - label\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := descriptor.Parse(cliFactsSource(tc.extra), cliFactsOrigin)

			if err == nil {
				t.Fatalf("Parse succeeded (CLI=%+v), want an error", d.CLI)
			}
			if !strings.HasPrefix(err.Error(), cliFactsOrigin) {
				t.Errorf("error %q does not start with the origin %q", err, cliFactsOrigin)
			}
			if !strings.Contains(err.Error(), "cli") {
				t.Errorf("error %q does not mention cli", err)
			}
			// A loader that does not know the section rejects the whole key as an
			// unknown field; that generic outcome must not satisfy this test.
			if strings.Contains(err.Error(), `unknown field "cli"`) {
				t.Errorf("error %q rejects the cli key itself instead of its content", err)
			}
			if tc.names != "" && !strings.Contains(err.Error(), tc.names) {
				t.Errorf("error %q does not name %q", err, tc.names)
			}
		})
	}
}

// Regression coverage: unknown top-level keys were already rejected before the cli section existed.
func TestParse_UnknownKeyOutsideCLISection_StillRejected(t *testing.T) {
	src := cliFactsSource("cli:\n  label: \"X CLI\"\n  loading_mechanism: \"name\"\nbogus_key: 1\n")

	_, err := descriptor.Parse(src, cliFactsOrigin)

	if err == nil {
		t.Fatal("Parse succeeded, want the unknown top-level key rejected")
	}
	if !strings.Contains(err.Error(), "bogus_key") {
		t.Errorf("error %q does not name the unknown key", err)
	}
}
