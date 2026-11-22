package builtindesc_test

// Tests for Parse over descriptor fixtures in testdata/. Each defect fixture
// carries exactly one defect; the valid fixtures show what a good descriptor
// yields and that keys Common does not read are tolerated.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/harness/builtindesc"
)

func parseFixture(t *testing.T, name string) (builtindesc.Descriptor, string, error) {
	t.Helper()
	path := filepath.Join("testdata", name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	d, perr := builtindesc.Parse(src, path)
	return d, path, perr
}

func TestParse_DefectFixturesAreRejectedWithTheOffendingField(t *testing.T) {
	cases := []struct {
		fixture string
		field   string
	}{
		{"blank_id.yaml", "id"},
		{"missing_display_name.yaml", "display_name"},
		{"missing_agents_dir.yaml", "paths.agents.project"},
		{"skills_supported_without_dir.yaml", "paths.skills.project"},
		{"extension_without_dot.yaml", "extensions.agent"},
		{"unknown_format_id.yaml", "agent_format_id"},
		{"unknown_loading_mechanism.yaml", "cli.loading_mechanism"},
		{"blank_label.yaml", "cli.label"},
		{"cli_without_models.yaml", "models"},
		{"empty_model_ids.yaml", "models.ids"},
		{"duplicate_model_ids.yaml", "models.ids"},
		{"blank_format_hint.yaml", "models.format_hint"},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			d, path, err := parseFixture(t, tc.fixture)

			var verr *builtindesc.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Parse error = %v, want a *ValidationError", err)
			}
			if verr.Field != tc.field {
				t.Errorf("ValidationError.Field = %q, want %q", verr.Field, tc.field)
			}
			if verr.Origin != path {
				t.Errorf("ValidationError.Origin = %q, want %q", verr.Origin, path)
			}
			if !reflectZero(d) {
				t.Errorf("Descriptor on failure = %+v, want zero value", d)
			}
		})
	}
}

func TestParse_CLISectionThatIsNotAMappingIsRejectedWithOriginAndCLI(t *testing.T) {
	for _, fixture := range []string{"cli_null.yaml", "cli_scalar.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			_, path, err := parseFixture(t, fixture)

			if err == nil {
				t.Fatal("Parse succeeded, want an error")
			}
			if !strings.HasPrefix(err.Error(), path) {
				t.Errorf("error %q does not start with the origin %q", err, path)
			}
			if !strings.Contains(err.Error(), "cli") {
				t.Errorf("error %q does not mention cli", err)
			}
		})
	}
}

func TestParse_SyntaxErrorIsPrefixedWithOrigin(t *testing.T) {
	_, err := builtindesc.Parse([]byte("id: [unterminated\n"), "origin-x")

	if err == nil {
		t.Fatal("Parse succeeded, want an error")
	}
	if !strings.HasPrefix(err.Error(), "origin-x") {
		t.Errorf("error %q does not start with the origin", err)
	}
}

func TestParse_ValidCLIFixtureYieldsAllFacts(t *testing.T) {
	d, _, err := parseFixture(t, "valid_cli.yaml")

	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if d.ID != "fixture" || d.DisplayName != "Fixture" {
		t.Errorf("ID/DisplayName = %q/%q, want fixture/Fixture", d.ID, d.DisplayName)
	}
	if d.AgentsDir != ".fixture/agents" || d.SkillsDir != ".fixture/skills" || d.HooksDir != ".fixture/hooks" {
		t.Errorf("dirs = %q %q %q", d.AgentsDir, d.SkillsDir, d.HooksDir)
	}
	if d.AgentExtension != ".md" {
		t.Errorf("AgentExtension = %q, want .md", d.AgentExtension)
	}
	if d.AgentFormatID != "markdown" {
		t.Errorf("AgentFormatID = %q, want markdown when agent_format_id is absent", d.AgentFormatID)
	}
	if d.CLI == nil || d.CLI.Label != "Fixture CLI" || d.CLI.LoadingMechanism != builtindesc.LoadingMechanismName {
		t.Errorf("CLI = %+v, want label Fixture CLI and mechanism name", d.CLI)
	}
	if d.Models == nil || strings.Join(d.Models.IDs, ",") != "model-a,model-b" || d.Models.FormatHint != "model-<n>" {
		t.Errorf("Models = %+v, want ids [model-a model-b] and hint model-<n>", d.Models)
	}
}

func TestParse_DescriptorWithoutCLIAndWithModelsIsNotCLIBacked(t *testing.T) {
	d, _, err := parseFixture(t, "valid_not_cli.yaml")

	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if d.CLI != nil {
		t.Errorf("CLI = %+v, want nil", *d.CLI)
	}
	if d.Models == nil {
		t.Error("Models is nil, want the declared model data")
	}
}

func TestParse_DeployOnlyKeysAreIgnoredAndUnsupportedHooksYieldNoDirectory(t *testing.T) {
	d, _, err := parseFixture(t, "valid_deploy_only_keys.yaml")

	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if d.HooksDir != "" {
		t.Errorf("HooksDir = %q, want empty for unsupported hooks", d.HooksDir)
	}
	if d.AgentFormatID != "codex-toml" || d.AgentExtension != ".toml" {
		t.Errorf("format/extension = %q/%q, want codex-toml/.toml", d.AgentFormatID, d.AgentExtension)
	}
}

func reflectZero(d builtindesc.Descriptor) bool {
	return d.ID == "" && d.DisplayName == "" && d.AgentsDir == "" && d.SkillsDir == "" &&
		d.HooksDir == "" && d.AgentExtension == "" && d.AgentFormatID == "" &&
		d.Models == nil && d.CLI == nil
}
