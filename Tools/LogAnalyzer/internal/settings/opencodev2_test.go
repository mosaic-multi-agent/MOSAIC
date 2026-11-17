package settings_test

// Tests for opencode-v2 in the settings layer.
//
// Rules under test:
//   - A default_cache_write_tier entry for opencode-v2 is accepted and applies
//     to that harness only, without a finding.
//   - A write_tools / user_interaction_tools list for opencode-v2 replaces its
//     defaults.
//   - The settings template names opencode-v2 in the harness key paragraph,
//     in both built-in default lists and in the verification status, and stays
//     comment-only.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestLoadCacheTier_OpenCodeV2Entry_IsAcceptedAndAppliesToThatHarnessOnly(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+":\n  opencode-v2: 1h\n")

	got, findings := loadLayered(t, l)

	assertTier(t, got, domain.HarnessOpenCodeV2, domain.CacheWriteTier1h)
	assertTier(t, got, domain.HarnessOpenCode, domain.CacheWriteTier5m)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none for a recognised harness", findings)
	}
}

func TestLoadCacheTier_OpenCodeV2WithoutSetting_KeepsFiveMinutes(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+":\n  claude-code: 1h\n")

	got, _ := loadLayered(t, l)

	assertTier(t, got, domain.HarnessOpenCodeV2, domain.CacheWriteTier5m)
}

func TestLoad_OpenCodeV2WriteToolsList_ReplacesItsDefaults(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, "write_tools:\n  opencode-v2: [write]\n")

	got, findings := loadLayered(t, l)

	if !got.IsWriteTool(domain.HarnessOpenCodeV2, "write") {
		t.Error("the listed opencode-v2 write tool must be a write tool")
	}
	if got.IsWriteTool(domain.HarnessOpenCodeV2, "patch") {
		t.Error("a harness list REPLACES the opencode-v2 defaults, so patch must no longer be a write tool")
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestEnsureGlobalFile_Template_HarnessKeyParagraphNamesOpenCodeV2(t *testing.T) {
	_, content := ensuredTemplate(t)

	want := "# A harness key is the harness name as logged (claude-code, ghcp-cli,\n" +
		"# opencode, opencode-v2, vscode-ghcp). A harness list written here REPLACES\n" +
		"# that harness's default list; an empty list ([]) means no tools. A harness\n" +
		"# you do not list keeps its default list.\n"
	if !hasLine(content, want) {
		t.Errorf("template must contain the harness key paragraph naming opencode-v2:\n%s", want)
	}
}

func TestEnsureGlobalFile_Template_ListsOpenCodeV2DefaultsAfterTheV1Lines(t *testing.T) {
	_, content := ensuredTemplate(t)

	for _, want := range []string{
		"#     opencode:     apply_patch\n#     opencode-v2:  write, edit, patch\n",
		"#     opencode:     question\n#     opencode-v2:  question\n",
	} {
		if !hasLine(content, want) {
			t.Errorf("template must contain the consecutive lines:\n%s", want)
		}
	}
}

func TestEnsureGlobalFile_Template_MarksOpenCodeV2DefaultsUnverified(t *testing.T) {
	_, content := ensuredTemplate(t)

	want := "#   opencode: UNVERIFIED defaults; from captured system prompts, not real MOSAIC logs.\n" +
		"#   opencode-v2: UNVERIFIED defaults; from the OpenCode V2 tool list, not real MOSAIC logs.\n"
	if !hasLine(content, want) {
		t.Errorf("template must state the opencode-v2 verification status right after opencode:\n%s", want)
	}
}

func TestEnsureGlobalFile_TemplateWithOpenCodeV2_StaysCommentOnly(t *testing.T) {
	_, content := ensuredTemplate(t)

	if !hasLine(content, "opencode-v2") {
		t.Fatal("precondition: template must mention opencode-v2")
	}
	if active := nonCommentLines(content); len(active) != 0 {
		t.Errorf("template must hold comments only, active lines: %v", active)
	}
}
