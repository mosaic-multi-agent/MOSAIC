package harness_test

// Tests for the skills and hooks/plugins directories the CLI harness catalog
// exposes, sourced from the builtin descriptors.

import (
	"testing"

	"mosaic-common/harness"
)

func TestCLIHarnesses_ExposeSkillsAndHooksDirectories(t *testing.T) {
	cases := []struct {
		id        string
		skillsDir string
		hooksDir  string
	}{
		{id: harness.HarnessIDClaudeCode, skillsDir: ".claude/skills", hooksDir: ".claude/hooks"},
		{id: harness.HarnessIDOpenCode, skillsDir: ".opencode/skills", hooksDir: ".opencode/plugins"},
		{id: harness.HarnessIDGHCPCLI, skillsDir: ".github/skills", hooksDir: ".github/hooks"},
		{id: harness.HarnessIDCodex, skillsDir: ".agents/skills", hooksDir: ""},
		{id: harness.HarnessIDOpenCodeV2, skillsDir: ".opencode/skills", hooksDir: ".opencode/plugins"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			entry, ok := harness.LookupCLIHarness(tc.id)
			if !ok {
				t.Fatalf("LookupCLIHarness(%q): not found", tc.id)
			}

			if entry.SkillsDir != tc.skillsDir {
				t.Errorf("SkillsDir = %q, want %q", entry.SkillsDir, tc.skillsDir)
			}
			if entry.HooksDir != tc.hooksDir {
				t.Errorf("HooksDir = %q, want %q", entry.HooksDir, tc.hooksDir)
			}
		})
	}
}

func TestCLIHarnesses_DirectoriesSurviveCopy(t *testing.T) {
	first := harness.CLIHarnesses()
	if len(first) == 0 {
		t.Fatal("CLIHarnesses() returned no entries")
	}
	want := first[0].SkillsDir
	if want == "" {
		t.Fatal("first CLI harness has no SkillsDir")
	}
	first[0].SkillsDir = "mutated"

	second := harness.CLIHarnesses()

	if second[0].SkillsDir != want {
		t.Errorf("mutating a returned slice leaked into the next call: SkillsDir = %q, want %q",
			second[0].SkillsDir, want)
	}
}
