package settings_test

// Tests for Store.AddUserInteractionTool: validation, file creation with the
// commented template, and preservation of existing content.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

func addTool(s *settings.Store, h domain.Harness, name string) error {
	_, err := s.AddUserInteractionTool(context.Background(), domain.LevelProject, h, name)
	return err
}

func TestAdd_EmptyHarness_ErrorWrapsInvalidHarnessAndWritesNothing(t *testing.T) {
	s, _ := newTestStore(t)

	err := addTool(s, "", "ask")

	if !errors.Is(err, domain.ErrInvalidHarness) {
		t.Errorf("err = %v, want ErrInvalidHarness", err)
	}
	if _, statErr := os.Stat(s.Path()); !os.IsNotExist(statErr) {
		t.Error("nothing may be written for an invalid harness")
	}
}

func TestAdd_InvalidName_ErrorWrapsInvalidToolNameAndWritesNothing(t *testing.T) {
	for _, name := range []string{"", "   ", "bad\nname"} {
		s, _ := newTestStore(t)

		err := addTool(s, domain.HarnessClaudeCode, name)

		if !errors.Is(err, domain.ErrInvalidToolName) {
			t.Errorf("name %q: err = %v, want ErrInvalidToolName", name, err)
		}
		if _, statErr := os.Stat(s.Path()); !os.IsNotExist(statErr) {
			t.Errorf("name %q: nothing may be written for an invalid name", name)
		}
	}
}

func TestAdd_InvalidNameOnExistingFile_LeavesFileUnchanged(t *testing.T) {
	s, _ := newTestStore(t)
	const original = "# mine\nwrite_tools:\n  claude-code: [Write]\n"
	writeSettingsFile(t, s, original)

	if err := addTool(s, domain.HarnessClaudeCode, "bad\nname"); err == nil {
		t.Fatal("expected an error for an invalid name")
	}

	if got := readSettingsFile(t, s); got != original {
		t.Errorf("file changed on validation failure:\n%s", got)
	}
}

func TestAdd_AbsentFile_CreatesFileWithParentDirectories(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	if _, err := os.Stat(s.Path()); err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
}

func TestAdd_AbsentFile_WritesCommentedTemplateThenTheEntry(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	content := readSettingsFile(t, s)
	comments := commentLines(content)
	if len(comments) < 3 {
		t.Fatalf("expected a commented template, got %d comment lines:\n%s", len(comments), content)
	}
	for _, key := range []string{"write_tools", "user_interaction_tools", "detected_compaction", "drop_percent"} {
		if !strings.Contains(strings.Join(comments, "\n"), key) {
			t.Errorf("template comments must explain %q", key)
		}
	}
	if !strings.Contains(content, "user_interaction_tools:") {
		t.Errorf("file must hold the user_interaction_tools entry:\n%s", content)
	}
}

func TestAdd_AbsentFile_TemplateMarksNonClaudeDefaultsUnverified(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	lines := commentLines(readSettingsFile(t, s))
	comments := strings.ToLower(strings.Join(lines, "\n"))
	for _, h := range []string{"ghcp-cli", "opencode", "vscode-ghcp"} {
		marked := false
		for _, line := range lines {
			l := strings.ToLower(line)
			if strings.Contains(l, h) && strings.Contains(l, "unverified") {
				marked = true
			}
		}
		if !marked {
			t.Errorf("a template comment line must name %s and mark it unverified", h)
		}
	}
	for _, name := range []string{"ask_user", "apply_patch", "vscode/askquestions"} {
		if !strings.Contains(comments, name) {
			t.Errorf("template comments must list the default %q", name)
		}
	}
}

func TestAdd_AbsentFile_TemplateExplainsGlobAndReplaceSemantics(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	comments := strings.ToLower(strings.Join(commentLines(readSettingsFile(t, s)), "\n"))
	if !strings.Contains(comments, "*") {
		t.Error("template must explain '*' patterns")
	}
	if !strings.Contains(comments, "replace") {
		t.Error("template must explain that a harness list replaces the default list")
	}
}

func TestAdd_AbsentFile_LoadsBackAsDefaultListPlusName(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, findings := mustLoad(t, s)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none (the template must parse cleanly)", findings)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask") {
		t.Error("the added name must be persisted")
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("the harness default list is written alongside the added name")
	}
	if !got.IsUserInteractionTool(domain.HarnessGHCPCLI, "ask_user") {
		t.Error("other harnesses keep their defaults")
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Edit") || got.CompactionDropPercent != 30 {
		t.Error("write tools and drop percent stay at their defaults")
	}
}

func TestAdd_Persists_AcrossStoreInstances(t *testing.T) {
	s, root := newTestStore(t)
	if err := addTool(s, domain.HarnessOpenCode, "my_question"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := mustLoad(t, settings.NewStore(t.TempDir(), root))

	if !got.IsUserInteractionTool(domain.HarnessOpenCode, "my_question") ||
		!got.IsUserInteractionTool(domain.HarnessOpenCode, "question") {
		t.Error("a fresh store on the same root must see the saved list")
	}
}

func TestAdd_ExistingList_AppendsName(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: [First, Second]\n")

	if err := addTool(s, domain.HarnessClaudeCode, "Third"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := mustLoad(t, s)
	for _, name := range []string{"First", "Second", "Third"} {
		if !got.IsUserInteractionTool(domain.HarnessClaudeCode, name) {
			t.Errorf("%s missing after append", name)
		}
	}
	if got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("an existing list must not be merged with the default list")
	}
}

func TestAdd_NameAlreadyPresent_IsNoOpSuccess(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: [First]\n")

	if err := addTool(s, domain.HarnessClaudeCode, "First"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	// A real append path must run: add a different name afterwards.
	if err := addTool(s, domain.HarnessClaudeCode, "Second"); err != nil {
		t.Fatalf("AddUserInteractionTool(Second): %v", err)
	}
	content := readSettingsFile(t, s)
	if n := strings.Count(content, "First"); n != 1 {
		t.Errorf("name present %d times, want 1:\n%s", n, content)
	}
	got, _ := mustLoad(t, s)
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "Second") {
		t.Error("a new name added after the duplicate must be persisted")
	}
}

func TestAdd_ExplicitlyEmptyList_BecomesJustTheName(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: []\n")

	if err := addTool(s, domain.HarnessClaudeCode, "OnlyMine"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := mustLoad(t, s)
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "OnlyMine") {
		t.Error("the added name must be persisted")
	}
	if got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("an empty list is a list: the default list must not be added back")
	}
}

func TestAdd_GlobName_PersistsAndMatchesOnReload(t *testing.T) {
	s, _ := newTestStore(t)

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__*"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, findings := mustLoad(t, s)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__anything") {
		t.Error("an added glob name must match on reload")
	}
}

func TestAdd_HarnessWithoutList_WritesItsDefaultsPlusNameAndKeepsOthers(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: [Only]\n")

	if err := addTool(s, domain.HarnessGHCPCLI, "my_ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := mustLoad(t, s)
	if !got.IsUserInteractionTool(domain.HarnessGHCPCLI, "my_ask") ||
		!got.IsUserInteractionTool(domain.HarnessGHCPCLI, "ask_user") {
		t.Error("the new harness list is its default list plus the name")
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "Only") ||
		got.IsUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion") {
		t.Error("the existing claude-code list must be preserved as is")
	}
}

func TestAdd_ExistingFileWithoutUserInteractionKey_AddsItAndKeepsOtherEntries(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "write_tools:\n  claude-code: [Write]\ndetected_compaction:\n  drop_percent: 45\n")

	if err := addTool(s, domain.HarnessClaudeCode, "mcp__x__ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, findings := mustLoad(t, s)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask") {
		t.Error("the added name must be persisted")
	}
	if got.IsWriteTool(domain.HarnessClaudeCode, "Edit") || !got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("the existing write_tools entry must be preserved")
	}
	if got.CompactionDropPercent != 45 {
		t.Errorf("CompactionDropPercent = %d, want preserved 45", got.CompactionDropPercent)
	}
}

func TestAdd_PreservesLeadingCommentBlockAndUnknownKeys(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s,
		"# my own notes about this file\n# second note line\nfuture_key:\n  nested: 1\nuser_interaction_tools:\n  claude-code: [First]\n")

	if err := addTool(s, domain.HarnessClaudeCode, "Second"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	content := readSettingsFile(t, s)
	for _, want := range []string{"# my own notes about this file", "# second note line", "future_key", "nested"} {
		if !strings.Contains(content, want) {
			t.Errorf("file lost %q:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "Second") {
		t.Errorf("added name missing:\n%s", content)
	}
}

func TestAdd_PreservesGlobEntries(t *testing.T) {
	s, _ := newTestStore(t)
	writeSettingsFile(t, s, "user_interaction_tools:\n  claude-code: [\"mcp__contact-user__*\"]\n")

	if err := addTool(s, domain.HarnessClaudeCode, "Extra"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	got, _ := mustLoad(t, s)
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__contact-user__anything") {
		t.Error("the existing glob pattern must survive the rewrite")
	}
}

func TestAdd_UnparseableExistingFile_ErrorAndFileUnchanged(t *testing.T) {
	s, _ := newTestStore(t)
	const broken = "user_interaction_tools: [unclosed\n  claude-code: {\n"
	writeSettingsFile(t, s, broken)

	err := addTool(s, domain.HarnessClaudeCode, "ask")

	if err == nil {
		t.Fatal("expected an error for an unparseable file")
	}
	if got := readSettingsFile(t, s); got != broken {
		t.Errorf("unparseable file must stay unchanged, got:\n%s", got)
	}
}

func TestAdd_DoesNotTouchPricingFile(t *testing.T) {
	s, root := newTestStore(t)
	pricing := root + "/MosaicLogAnalyzer/config/pricing.yaml"
	const pricingContent = "models:\n  m1: {}\n"
	if err := os.MkdirAll(root+"/MosaicLogAnalyzer/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pricing, []byte(pricingContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := addTool(s, domain.HarnessClaudeCode, "ask"); err != nil {
		t.Fatalf("AddUserInteractionTool: %v", err)
	}

	data, err := os.ReadFile(pricing)
	if err != nil || string(data) != pricingContent {
		t.Errorf("pricing file changed: err=%v content=%q", err, data)
	}
	if _, err := os.Stat(s.Path()); err != nil {
		t.Errorf("settings file must be created next to the pricing file: %v", err)
	}
}
