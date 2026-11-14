package descriptor_test

// Tests for the optional experimental declaration of a harness descriptor.
//
// Coverage:
//   - A descriptor without the key loads and is not experimental.
//   - A descriptor with a non-blank reason loads and exposes the trimmed reason.
//   - Every blank or malformed form is rejected with a ValidationError naming the field.
//   - An unknown key inside the mapping is rejected with an error naming the key.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/harness/descriptor"
)

func TestLoad_NoExperimentalKey_LoadsAsNotExperimental(t *testing.T) {
	d := loadFixture(t, "valid-minimal.yaml")

	if d.ExperimentalReason != "" {
		t.Errorf("ExperimentalReason = %q, want empty", d.ExperimentalReason)
	}
}

func TestLoad_ExperimentalWithReason_ExposesTrimmedReason(t *testing.T) {
	d := loadFixture(t, "valid-experimental.yaml")

	const want = "Agents deploy; orchestration is not supported."
	if d.ExperimentalReason != want {
		t.Errorf("ExperimentalReason = %q, want %q", d.ExperimentalReason, want)
	}
}

func TestParse_ExperimentalBlockScalarReason_KeepsInnerNewlines(t *testing.T) {
	src := []byte("schema_version: \"1\"\nid: \"x\"\ndisplay_name: \"X\"\nexperimental:\n  reason: |\n    line one\n    line two\n")

	d, err := descriptor.Parse(src, "inline.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !strings.Contains(d.ExperimentalReason, "line one") || !strings.Contains(d.ExperimentalReason, "\n") ||
		!strings.Contains(d.ExperimentalReason, "line two") || d.ExperimentalReason != strings.TrimSpace(d.ExperimentalReason) {
		t.Errorf("ExperimentalReason = %q, want trimmed text with inner newline kept", d.ExperimentalReason)
	}
}

func TestLoad_InvalidExperimentalDeclaration_RejectedNamingTheField(t *testing.T) {
	cases := []struct {
		fixture string
		field   string
	}{
		{"experimental-bare.yaml", "experimental"},
		{"experimental-null.yaml", "experimental"},
		{"experimental-scalar.yaml", "experimental"},
		{"experimental-empty-mapping.yaml", "experimental.reason"},
		{"experimental-empty-reason.yaml", "experimental.reason"},
		{"experimental-whitespace-reason.yaml", "experimental.reason"},
		{"experimental-non-string-reason.yaml", "experimental.reason"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			path := filepath.Join(testdataDir, "invalid", tc.fixture)

			d, err := descriptor.Load(path)

			if err == nil {
				t.Fatalf("Load succeeded (experimental=%q), want a validation error", d.ExperimentalReason)
			}
			var ve descriptor.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error %v is not a ValidationError", err)
			}
			if !strings.HasPrefix(ve.Field, "experimental") {
				t.Errorf("Field = %q, want a field naming experimental", ve.Field)
			}
			if tc.field == "experimental.reason" && ve.Field != tc.field {
				t.Errorf("Field = %q, want %q", ve.Field, tc.field)
			}
			if ve.File == "" {
				t.Error("File is empty, want the descriptor origin")
			}
		})
	}
}

func TestLoad_ExperimentalUnknownKey_RejectedNamingTheKey(t *testing.T) {
	// Control: the same mapping with only the known key loads, so the failure below is
	// attributable to the unknown nested key and not to the experimental mapping itself.
	loadFixture(t, "valid-experimental.yaml")
	path := filepath.Join(testdataDir, "invalid", "experimental-unknown-key.yaml")

	_, err := descriptor.Load(path)

	if err == nil {
		t.Fatal("Load succeeded, want an error for the unknown key")
	}
	// The quoted key appears only in the reported error line, not in the YAML source snippet.
	if !strings.Contains(err.Error(), `"since"`) {
		t.Errorf("error %q does not name the unknown key \"since\"", err)
	}
}
