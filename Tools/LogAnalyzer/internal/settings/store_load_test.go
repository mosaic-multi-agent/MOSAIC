package settings_test

// Tests for Store.Load with valid, absent and comments-only files.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

func TestNewStore_PathIsResolvedFromMosaicRoot(t *testing.T) {
	root := t.TempDir()

	got := settings.NewStore(root).Path()

	want := filepath.Join(root, "MosaicLogAnalyzer", "config", "analysis.yaml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoad_AbsentFile_YieldsDefaultsWithoutFindingsOrError(t *testing.T) {
	s, _ := newTestStore(t)

	got, findings := mustLoad(t, s)

	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoad_AbsentFile_DoesNotCreateIt(t *testing.T) {
	s, _ := newTestStore(t)

	mustLoad(t, s)

	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Errorf("Load must never write; stat err = %v", err)
	}
}

func TestLoad_CommentsOnlyFile_YieldsDefaults(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "# just a comment\n# another one\n")

	got, findings := mustLoad(t, s)

	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoad_EmptyFile_YieldsDefaults(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "")

	got, findings := mustLoad(t, s)

	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoad_HarnessListReplacesThatHarnessDefaultOnly(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "write_tools:\n  claude-code: [Write]\n")

	got, findings := mustLoad(t, s)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("Write must stay write-class for claude-code")
	}
	if got.IsWriteTool(domain.HarnessClaudeCode, "Edit") {
		t.Error("the file's list REPLACES the default: Edit must no longer be write-class for claude-code")
	}
	if !got.IsWriteTool(domain.HarnessGHCPCLI, "create") {
		t.Error("harnesses absent from the file keep their default list")
	}
	if got.CompactionDropPercent != 30 {
		t.Errorf("CompactionDropPercent = %d, want default 30", got.CompactionDropPercent)
	}
}

func TestLoad_EmptyListMeansNone(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: []\n")

	got, findings := mustLoad(t, s)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("an empty list replaces the default with no tools")
	}
	if !got.IsUserInteractionTool(domain.HarnessGHCPCLI, "ask_user") {
		t.Error("other harnesses keep their defaults")
	}
}

func TestLoad_GlobPatternsAreKept(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"user_interaction_tools:\n  claude-code: [AskUserQuestion, \"mcp__contact-user__*\"]\n")

	got, findings := mustLoad(t, s)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__contact-user__ask_user_questions") {
		t.Error("the glob pattern must match a name it covers")
	}
	if got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__other__ask") {
		t.Error("the glob pattern must not match a name it does not cover")
	}
}

func TestLoad_HarnessKeyNotInDefaultsIsAccepted(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "write_tools:\n  future-harness: [Put]\n")

	got, findings := mustLoad(t, s)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if !got.IsWriteTool("future-harness", "Put") {
		t.Error("any non-empty harness key is accepted verbatim")
	}
}

func TestLoad_DropPercentOverridesDefault(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "detected_compaction:\n  drop_percent: 45\n")

	got, findings := mustLoad(t, s)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if got.CompactionDropPercent != 45 {
		t.Errorf("CompactionDropPercent = %d, want 45", got.CompactionDropPercent)
	}
}

func TestLoad_DropPercentBoundariesAreValid(t *testing.T) {
	for _, v := range []string{"1", "99"} {
		s, _ := newTestStore(t)
		writeSettingsFile(t, s, "detected_compaction:\n  drop_percent: "+v+"\n")

		got, findings := mustLoad(t, s)

		if len(findings) != 0 {
			t.Errorf("drop_percent %s: findings = %v, want none", v, findings)
		}
		if v == "1" && got.CompactionDropPercent != 1 || v == "99" && got.CompactionDropPercent != 99 {
			t.Errorf("drop_percent %s: CompactionDropPercent = %d", v, got.CompactionDropPercent)
		}
	}
}

func TestLoad_UnparseableDocument_IsAnError(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "write_tools: [unclosed\n  claude-code: {\n")

	_, _, err := s.Load(context.Background())

	if err == nil {
		t.Error("Load of an unparseable document must return an error")
	}
}

func TestLoad_UnreadableFile_IsAnError(t *testing.T) {
	s, _ := newTestStore(t)
	// A directory at the file's path cannot be read as a file.
	if err := os.MkdirAll(s.Path(), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, _, err := s.Load(context.Background())

	if err == nil {
		t.Error("Load of an unreadable file must return an error")
	}
}
