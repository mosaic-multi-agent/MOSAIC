package settings_test

// Tests for Store.Load with invalid entries: each yields one finding, its
// default applies, and the rest of the file still loads.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// assertOneSettingsFinding checks there is exactly one invalid-settings finding
// carrying the settings file path as warning severity, and no other findings.
func assertOneSettingsFinding(t *testing.T, findings []domain.Finding, wantPath string) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want exactly 1", findings)
	}
	f := findings[0]
	if f.Kind != domain.FindingInvalidSettingsEntry {
		t.Errorf("finding kind = %q, want %q", f.Kind, domain.FindingInvalidSettingsEntry)
	}
	if f.Path != wantPath {
		t.Errorf("finding Path = %q, want the settings path %q", f.Path, wantPath)
	}
	if f.Severity != domain.SeverityWarning {
		t.Errorf("finding severity = %v, want warning", f.Severity)
	}
}

func TestLoad_HarnessListNotASequenceOfStrings_FindingAndDefaultForThatHarness(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"write_tools:\n  claude-code: 5\n  ghcp-cli: [create_x]\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Edit") {
		t.Error("the invalid claude-code list falls back to its default")
	}
	if !got.IsWriteTool(domain.HarnessGHCPCLI, "create_x") || got.IsWriteTool(domain.HarnessGHCPCLI, "create") {
		t.Error("the valid ghcp-cli list still loads and replaces its default")
	}
}

func TestLoad_ListWithNonStringElement_WholeListFallsBackWithFinding(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "write_tools:\n  claude-code: [OnlyThis, 5]\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if got.IsWriteTool(domain.HarnessClaudeCode, "OnlyThis") {
		t.Error("the whole list falls back: the valid sibling element must not be kept")
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Edit") {
		t.Error("the harness default applies after the fallback")
	}
}

func TestLoad_ListWithInvalidName_WholeListFallsBackWithFinding(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"user_interaction_tools:\n  claude-code: [Good, \"   \"]\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if got.IsUserInteractionTool(domain.HarnessClaudeCode, "Good") {
		t.Error("the whole list falls back: the valid sibling name must not be kept")
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("the harness default applies after the fallback")
	}
}

func TestLoad_EmptyHarnessKey_FindingAndOtherEntriesLoad(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"write_tools:\n  \"\": [Ghost]\n  claude-code: [Write]\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if got.IsWriteTool("", "Ghost") {
		t.Error("an entry under an empty harness key must not be applied")
	}
	if got.IsWriteTool(domain.HarnessClaudeCode, "Edit") {
		t.Error("the valid claude-code entry in the same file must still load")
	}
}

func TestLoad_InvalidDropPercent_FindingAndDefault(t *testing.T) {
	for _, v := range []string{"0", "100", "-5", "abc", "12.5", "\"30\""} {
		s, _ := newTestStore(t)
		writeSettingsFile(t, s,
			"write_tools:\n  claude-code: [Write]\ndetected_compaction:\n  drop_percent: "+v+"\n")

		got, findings := mustLoad(t, s)

		if len(findings) != 1 || findings[0].Kind != domain.FindingInvalidSettingsEntry {
			t.Errorf("drop_percent %s: findings = %v, want one invalid-settings finding", v, findings)
			continue
		}
		if got.CompactionDropPercent != 30 {
			t.Errorf("drop_percent %s: CompactionDropPercent = %d, want default 30", v, got.CompactionDropPercent)
		}
		if got.IsWriteTool(domain.HarnessClaudeCode, "Edit") {
			t.Errorf("drop_percent %s: the rest of the file must still load", v)
		}
	}
}

func TestLoad_UnknownTopLevelKey_OneFindingNamingKeyAndRestLoads(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"mystery_section:\n  a: 1\ndetected_compaction:\n  drop_percent: 40\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if !strings.Contains(findings[0].Detail, "mystery_section") {
		t.Errorf("finding detail %q must name the unknown key", findings[0].Detail)
	}
	if got.CompactionDropPercent != 40 {
		t.Errorf("CompactionDropPercent = %d, want 40 (rest of file loads)", got.CompactionDropPercent)
	}
}

func TestLoad_UnknownKeyUnderDetectedCompaction_OneFindingNamingKey(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"detected_compaction:\n  drop_percent: 40\n  min_tokens: 1000\n")

	got, findings := mustLoad(t, s)

	assertOneSettingsFinding(t, findings, s.Path())
	if !strings.Contains(findings[0].Detail, "min_tokens") {
		t.Errorf("finding detail %q must name the unknown key", findings[0].Detail)
	}
	if got.CompactionDropPercent != 40 {
		t.Errorf("CompactionDropPercent = %d, want 40", got.CompactionDropPercent)
	}
}

func TestLoad_SeveralInvalidEntries_OneFindingEach(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"write_tools:\n  claude-code: 5\nuser_interaction_tools:\n  opencode: 7\ndetected_compaction:\n  drop_percent: 0\n")

	got, findings := mustLoad(t, s)

	if n := countKind(findings, domain.FindingInvalidSettingsEntry); n != 3 || len(findings) != 3 {
		t.Errorf("findings = %v, want 3 invalid-settings findings", findings)
	}
	assertBuiltInDefaults(t, got)
}
