package config_test

// Tests that PutToolDestination keeps every comment and the formatting of
// user-config.yaml and changes only the added or replaced entry.

import (
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/config"
)

const commentedConfig = "# top comment\n" +
	"schema_version: \"1\"   # version note\n" +
	"\n" +
	"# the destinations\n" +
	"tool_destinations:\n" +
	"  # claude-code notes\n" +
	"  claude-code:   # trailing on harness\n" +
	"    # first entry notes\n" +
	"    - generic: knowledge_base   # generic note\n" +
	"      destinations:\n" +
	"        - to: main   # dest note\n" +
	"          names: [\"Read\"]\n" +
	"  # codex notes\n" +
	"  codex:\n" +
	"    - generic: terminal\n" +
	"      destinations:\n" +
	"        - to: main   # codex dest note\n" +
	"          names: [\"shell\"]\n" +
	"# end comment\n"

const addedClaudeEntry = "    - generic: pr_comments_read\n" +
	"      destinations:\n" +
	"        - to: main\n" +
	"          names: [\"mcp__github__get_pr\", \"mcp__jira__*\"]\n"

func newRootWithConfig(t *testing.T, content string) (string, config.UserConfigStore) {
	t.Helper()
	root := makeRoot(t)
	writeUserConfig(t, root, []byte(content))
	return root, config.NewUserConfigStore(root)
}

func TestPutToolDestination_AddToExistingHarness_InsertsOnlyTheEntryAtTheEndOfItsList(t *testing.T) {
	root, store := newRootWithConfig(t, commentedConfig)

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "mcp__github__get_pr", "mcp__jira__*"))

	want := strings.Replace(commentedConfig, "  # codex notes\n", addedClaudeEntry+"  # codex notes\n", 1)
	if got := readSavedUserConfig(t, root); got != want {
		t.Errorf("file differs from the original plus the added entry.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPutToolDestination_AddToExistingHarness_LoadsNewEntryAndKeepsExistingOnes(t *testing.T) {
	_, store := newRootWithConfig(t, commentedConfig)

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "mcp__github__get_pr", "mcp__jira__*"))

	cfg := mustLoad(t, store)
	assertMainNames(t, cfg, "claude-code", "pr_comments_read", "mcp__github__get_pr", "mcp__jira__*")
	assertMainNames(t, cfg, "claude-code", "knowledge_base", "Read")
	assertMainNames(t, cfg, "codex", "terminal", "shell")
}

func TestPutToolDestination_AddForNewHarness_KeepsAllCommentsAndLoads(t *testing.T) {
	root, store := newRootWithConfig(t, commentedConfig)

	mustPut(t, store, "opencode", mainMapping("pr_comments_read", "github_get_pr"))

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, commentedConfig)
	if !strings.Contains(got, commentedConfig[:strings.Index(commentedConfig, "# end comment")]) {
		t.Errorf("text before the end comment was altered:\n%s", got)
	}
	assertMainNames(t, mustLoad(t, store), "opencode", "pr_comments_read", "github_get_pr")
	assertMainNames(t, mustLoad(t, store), "codex", "terminal", "shell")
}

func TestPutToolDestination_ReplaceSameGeneric_ReplacesOnlyThatEntry(t *testing.T) {
	root, store := newRootWithConfig(t, commentedConfig)

	mustPut(t, store, "claude-code", mainMapping("knowledge_base", "Grep"))

	cfg := mustLoad(t, store)
	assertMainNames(t, cfg, "claude-code", "knowledge_base", "Grep")
	if n := len(cfg.ToolDestinations["claude-code"]); n != 1 {
		t.Errorf("claude-code has %d entries after a replace, want 1", n)
	}
	got := readSavedUserConfig(t, root)
	for _, kept := range []string{"# top comment", "# version note", "# claude-code notes", "# trailing on harness",
		"# codex notes", "# codex dest note", "# end comment", "    - generic: terminal\n"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q outside the replaced entry was lost:\n%s", kept, got)
		}
	}
	if strings.Contains(got, `["Read"]`) {
		t.Errorf("the replaced entry's old names are still in the file:\n%s", got)
	}
}

func TestPutToolDestination_ReplaceSecondOfTwoEntries_LeavesFirstEntryAndItsCommentsByteIdentical(t *testing.T) {
	first := "    # first notes\n    - generic: a\n      destinations:\n        - to: main   # keep me\n          names: [\"x\"]\n"
	second := "    - generic: b\n      destinations:\n        - to: main\n          names: [\"y\"]\n"
	content := "tool_destinations:\n  claude-code:\n" + first + second + "# tail\n"
	root, store := newRootWithConfig(t, content)

	mustPut(t, store, "claude-code", mainMapping("b", "z"))

	want := "tool_destinations:\n  claude-code:\n" + first +
		"    - generic: b\n      destinations:\n        - to: main\n          names: [\"z\"]\n" + "# tail\n"
	if got := readSavedUserConfig(t, root); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPutToolDestination_EmptyBlockWithTrailingComment_BecomesBlockAndKeepsComment(t *testing.T) {
	root, store := newRootWithConfig(t, "# head\nschema_version: \"1\"\ntool_destinations: {}   # none yet\ntier_models:\n  claude-code:\n    HIGH: m1   # tier note\n")

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "a"))

	got := readSavedUserConfig(t, root)
	for _, kept := range []string{"# head\n", "# none yet", "tier_models:\n  claude-code:\n    HIGH: m1   # tier note\n"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q was lost:\n%s", kept, got)
		}
	}
	if strings.Contains(got, "{}") {
		t.Errorf("the empty flow value was kept next to the new entry:\n%s", got)
	}
	assertMainNames(t, mustLoad(t, store), "claude-code", "pr_comments_read", "a")
}

func TestPutToolDestination_NullBlock_BecomesBlock(t *testing.T) {
	root, store := newRootWithConfig(t, "schema_version: \"1\"\ntool_destinations:   # nothing\n")

	mustPut(t, store, "claude-code", mainMapping("g", "a"))

	if got := readSavedUserConfig(t, root); !strings.Contains(got, "# nothing") {
		t.Errorf("trailing comment on the key line was lost:\n%s", got)
	}
	assertMainNames(t, mustLoad(t, store), "claude-code", "g", "a")
}

func TestPutToolDestination_MissingKey_AppendsNewBlockAfterAllExistingText(t *testing.T) {
	original := "# head comment\nschema_version: \"1\"\n\n# tiers\ntier_models:\n  claude-code:\n    HIGH: m1   # note\n# tail comment\n"
	root, store := newRootWithConfig(t, original)

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "mcp__github__get_pr", "mcp__jira__*"))

	want := original + "tool_destinations:\n  claude-code:\n" + addedClaudeEntry
	if got := readSavedUserConfig(t, root); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPutToolDestination_MissingFile_CreatesFileWithTheBlockOnly(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "mcp__github__get_pr", "mcp__jira__*"))

	want := "tool_destinations:\n  claude-code:\n" + addedClaudeEntry
	if got := readSavedUserConfig(t, root); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	assertMainNames(t, mustLoad(t, store), "claude-code", "pr_comments_read", "mcp__github__get_pr", "mcp__jira__*")
}

func TestPutToolDestination_CRLFFile_KeepsCommentsAndLineEndings(t *testing.T) {
	crlf := strings.ReplaceAll(commentedConfig, "\n", "\r\n")
	root, store := newRootWithConfig(t, crlf)

	mustPut(t, store, "claude-code", mainMapping("pr_comments_read", "mcp__github__get_pr", "mcp__jira__*"))

	got := readSavedUserConfig(t, root)
	assertAllLineEndingsCRLF(t, got)
	want := strings.Replace(crlf, "  # codex notes\r\n", strings.ReplaceAll(addedClaudeEntry, "\n", "\r\n")+"  # codex notes\r\n", 1)
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestPutToolDestination_CRLFFileWithoutKey_AppendedBlockUsesCRLF(t *testing.T) {
	root, store := newRootWithConfig(t, "# c\r\nschema_version: \"1\"\r\n")

	mustPut(t, store, "claude-code", mainMapping("g", "a"))

	got := readSavedUserConfig(t, root)
	assertAllLineEndingsCRLF(t, got)
	if !strings.HasPrefix(got, "# c\r\nschema_version: \"1\"\r\n") {
		t.Errorf("original text altered:\n%q", got)
	}
}

func TestPutToolDestination_FlowStyleTargetHarness_IsRenderedAsBlockAndOtherHarnessesKeepComments(t *testing.T) {
	content := "tool_destinations:\n" +
		"  # codex notes\n" +
		"  codex:\n" +
		"    - generic: terminal   # keep\n" +
		"      destinations:\n" +
		"        - to: main\n" +
		"          names: [\"shell\"]\n" +
		"  claude-code: [{generic: old, destinations: [{to: main, names: [\"Read\"]}]}]\n"
	root, store := newRootWithConfig(t, content)

	mustPut(t, store, "claude-code", mainMapping("g", "a"))

	got := readSavedUserConfig(t, root)
	if !strings.Contains(got, "  # codex notes\n  codex:\n    - generic: terminal   # keep\n") {
		t.Errorf("another harness's entry was altered:\n%s", got)
	}
	cfg := mustLoad(t, store)
	assertMainNames(t, cfg, "claude-code", "old", "Read")
	assertMainNames(t, cfg, "claude-code", "g", "a")
	assertMainNames(t, cfg, "codex", "terminal", "shell")
}

func TestPutToolDestination_ExistingIndentStep_IsReusedForTheNewEntry(t *testing.T) {
	content := "tool_destinations:\n" +
		"    claude-code:\n" +
		"        - generic: a\n" +
		"          destinations:\n" +
		"              - to: main\n" +
		"                names: [\"x\"]\n"
	root, store := newRootWithConfig(t, content)

	mustPut(t, store, "claude-code", mainMapping("b", "y"))

	got := readSavedUserConfig(t, root)
	if !strings.HasPrefix(got, content) {
		t.Errorf("existing entry altered:\n%s", got)
	}
	if !strings.Contains(got, "\n        - generic: b\n") {
		t.Errorf("new entry does not use the 4-space step of the existing block:\n%s", got)
	}
	assertMainNames(t, mustLoad(t, store), "claude-code", "b", "y")
}

func TestPutToolDestination_FieldDestination_WritesKeysInOrderAndLoadsBack(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	m := mainMapping("g", "a")
	m.Destinations = append(m.Destinations, fieldDest("tools_extra", "scalar", ";", "b", "c"))

	mustPut(t, store, "claude-code", m)

	got := readSavedUserConfig(t, root)
	want := "          names: [\"a\"]\n" +
		"        - to: field\n" +
		"          field: tools_extra\n" +
		"          format: scalar\n" +
		"          separator: \";\"\n" +
		"          names: [\"b\", \"c\"]\n"
	if !strings.Contains(got, want) {
		t.Errorf("field destination not rendered as the documented entry format:\n%s", got)
	}
	loaded, ok := findMapping(mustLoad(t, store).ToolDestinations["claude-code"], "g")
	if !ok || len(loaded.Destinations) != 2 || loaded.Destinations[1].Separator != ";" {
		t.Errorf("saved mapping did not load back as put: %+v", loaded)
	}
}

func TestPutToolDestination_InvalidMapping_ReturnsErrorAndLeavesFileUnchanged(t *testing.T) {
	root, store := newRootWithConfig(t, commentedConfig)
	bad := mainMapping("g", "a")
	bad.Destinations[0].Field = "not_allowed_on_main"

	err := store.PutToolDestination("claude-code", bad)

	if err == nil {
		t.Fatal("PutToolDestination accepted a main destination that declares a field")
	}
	assertFileUnchanged(t, root, commentedConfig)
	// Control: the same fixture accepts a valid mapping, so the failure above is the validation.
	mustPut(t, store, "claude-code", mainMapping("g", "a"))
	assertMainNames(t, mustLoad(t, store), "claude-code", "g", "a")
}

func TestPutToolDestination_EmptyGeneric_ReturnsErrorAndWritesNothing(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)

	err := store.PutToolDestination("claude-code", mainMapping("", "a"))

	if err == nil {
		t.Fatal("PutToolDestination accepted an empty generic")
	}
	if _, statErr := os.Stat(userConfigPath(root)); statErr == nil {
		t.Error("a file was created for an invalid mapping")
	}
	// Control: the same call with a non-empty generic creates the file.
	mustPut(t, store, "claude-code", mainMapping("g", "a"))
	assertMainNames(t, mustLoad(t, store), "claude-code", "g", "a")
}

func TestPutToolDestination_UnparseableFile_ReturnsErrorAndLeavesFileUnchanged(t *testing.T) {
	broken := "tool_destinations: [unterminated\n  claude-code: : :\n"
	root, store := newRootWithConfig(t, broken)

	err := store.PutToolDestination("claude-code", mainMapping("g", "a"))

	if err == nil {
		t.Fatal("PutToolDestination succeeded on an unparseable file")
	}
	assertFileUnchanged(t, root, broken)
	// Control: the same valid mapping succeeds on a parseable file, so the error came from parsing.
	_, okStore := newRootWithConfig(t, "tool_destinations: {}\n")
	mustPut(t, okStore, "claude-code", mainMapping("g", "a"))
	assertMainNames(t, mustLoad(t, okStore), "claude-code", "g", "a")
}

const twoEntries = "tool_destinations:\n  claude-code:\n" +
	"    - generic: a\n      destinations:\n        - to: main\n          names: [\"x\"]\n" +
	"    - generic: b\n      destinations:\n        - to: main\n          names: [\"y\"]\n" +
	"# tail\n"

const twoEntriesFirstReplaced = "tool_destinations:\n  claude-code:\n" +
	"    - generic: a\n      destinations:\n        - to: main\n          names: [\"z\"]\n" +
	"    - generic: b\n      destinations:\n        - to: main\n          names: [\"y\"]\n" +
	"# tail\n"

func TestPutToolDestination_ReplaceFirstOfTwoEntries_ChangesOnlyThatEntryByteExact(t *testing.T) {
	root, store := newRootWithConfig(t, twoEntries)

	mustPut(t, store, "claude-code", mainMapping("a", "z"))

	if got := readSavedUserConfig(t, root); got != twoEntriesFirstReplaced {
		t.Errorf("got:\n%s\nwant:\n%s", got, twoEntriesFirstReplaced)
	}
}

func TestPutToolDestination_ReplaceInCRLFFile_ChangesOnlyThatEntryAndKeepsLineEndings(t *testing.T) {
	crlf := strings.ReplaceAll(twoEntries, "\n", "\r\n")
	root, store := newRootWithConfig(t, crlf)

	mustPut(t, store, "claude-code", mainMapping("a", "z"))

	want := strings.ReplaceAll(twoEntriesFirstReplaced, "\n", "\r\n")
	if got := readSavedUserConfig(t, root); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestPutToolDestination_HarnessKeyWithoutEntries_GetsTheEntryAndKeepsComments(t *testing.T) {
	root, store := newRootWithConfig(t, "# head\ntool_destinations:\n  claude-code:   # empty for now\n  codex:\n    - generic: terminal\n      destinations:\n        - to: main\n          names: [\"shell\"]\n")

	mustPut(t, store, "claude-code", mainMapping("g", "a"))

	got := readSavedUserConfig(t, root)
	for _, kept := range []string{"# head\n", "# empty for now", "    - generic: terminal\n"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q was lost:\n%s", kept, got)
		}
	}
	cfg := mustLoad(t, store)
	assertMainNames(t, cfg, "claude-code", "g", "a")
	assertMainNames(t, cfg, "codex", "terminal", "shell")
}
