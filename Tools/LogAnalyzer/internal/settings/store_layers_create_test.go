package settings_test

// Tests for EnsureGlobalFile: the global analysis.yaml is written from the
// comment-only template when absent and never overwritten. The template explains
// VS Code MCP tool naming and lists the tools seen in real VS Code logs.

import (
	"testing"

	"mosaic-log-analyzer/internal/settings"
)

// ensuredTemplate runs EnsureGlobalFile on a fresh layered store and returns the file content.
func ensuredTemplate(t *testing.T) (layered, string) {
	t.Helper()
	l := newLayered(t)
	created, err := l.store.EnsureGlobalFile()
	if err != nil || !created {
		t.Fatalf("EnsureGlobalFile = (%v, %v), want (true, nil)", created, err)
	}
	return l, readAt(t, l.globalFile)
}

func TestEnsureGlobalFile_Absent_WritesCommentOnlyTemplateThatLoadsAsDefaults(t *testing.T) {
	l, content := ensuredTemplate(t)

	got, findings := loadLayered(t, l)

	if active := nonCommentLines(content); len(active) != 0 {
		t.Errorf("template must hold comments only, active lines: %v", active)
	}
	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestEnsureGlobalFile_Template_ExplainsVSCodeMcpToolNaming(t *testing.T) {
	_, content := ensuredTemplate(t)

	for _, want := range []string{
		"MCP tool names in VS Code",
		"mcp_userinterfac2_userFeedback",
		`"mcp_userinterfac*_userFeedback"`,
		"SHORTENS",
		"replaces the defaults",
	} {
		if !hasLine(content, want) {
			t.Errorf("template must contain %q", want)
		}
	}
}

func TestEnsureGlobalFile_Template_MarksVSCodeToolsSeenInRealLogs(t *testing.T) {
	_, content := ensuredTemplate(t)

	if !hasLine(content, "seen in real logs") {
		t.Error(`template must say which vscode-ghcp tools were "seen in real logs"`)
	}
	for _, tool := range []string{"create_file", "replace_string_in_file", "multi_replace_string_in_file", "vscode_askQuestions"} {
		if !hasLine(content, tool) {
			t.Errorf("template must name %q", tool)
		}
	}
}

func TestEnsureGlobalFile_Template_HasNoActiveUserInteractionEntry(t *testing.T) {
	_, content := ensuredTemplate(t)

	for _, line := range nonCommentLines(content) {
		t.Errorf("template must not contain an active line, got %q", line)
	}
}

func TestEnsureGlobalFile_Present_IsNeverOverwritten(t *testing.T) {
	l := newLayered(t)
	const custom = "detected_compaction:\n  drop_percent: 40\n"
	writeAt(t, l.globalFile, custom)

	created, err := l.store.EnsureGlobalFile()

	if err != nil || created {
		t.Fatalf("EnsureGlobalFile = (%v, %v), want (false, nil)", created, err)
	}
	if got := readAt(t, l.globalFile); got != custom {
		t.Errorf("existing global file was changed:\n%s", got)
	}
}

func TestEnsureGlobalFile_CalledTwice_SecondCallCreatesNothing(t *testing.T) {
	l := newLayered(t)

	first, _ := l.store.EnsureGlobalFile()
	second, err := l.store.EnsureGlobalFile()

	if !first || second || err != nil {
		t.Errorf("calls = (%v, %v, %v), want first true, second false, no error", first, second, err)
	}
}

func TestEnsureGlobalFile_CreatesNoProjectFile(t *testing.T) {
	l := newLayered(t)

	if _, err := l.store.EnsureGlobalFile(); err != nil {
		t.Fatalf("EnsureGlobalFile: %v", err)
	}
	loadLayered(t, l)

	if fileExists(l.projFile) {
		t.Error("the project file is created only by a save")
	}
}

func TestEnsureGlobalFile_NoGlobalFolder_WritesNothingAndReportsNoError(t *testing.T) {
	s := settings.NewStore("", t.TempDir())

	created, err := s.EnsureGlobalFile()

	if created || err != nil {
		t.Errorf("EnsureGlobalFile = (%v, %v), want (false, nil)", created, err)
	}
}
