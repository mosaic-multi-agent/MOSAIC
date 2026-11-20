package config_test

// Tests for comment preservation of UserConfigStore.Save when a key is rewritten or removed.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/config"
)

func TestUserConfigStore_Save_RemovingEmptyKeyFromTemplate_KeepsEveryComment(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)

	// First save into the template: tier_models gets values, custom_model_ids stays empty
	// and is removed from the file.
	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-a") })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, string(readFixture(t, userTemplateFixture)))
	if !strings.Contains(got, "#tool_destinations: {}") {
		t.Errorf("the trailing commented tool_destinations section was lost")
	}
	if strings.Contains(got, "custom_model_ids: {}") {
		t.Errorf("the empty custom_model_ids key should have been removed")
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RemovingKeyBeforeCommentedSection_KeepsComments(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)
	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-a") })

	// tier_models precedes the commented custom_model_ids section; empty it again.
	want := loadMutateSave(t, store, func(c *config.UserConfig) { c.TierModels = nil })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, string(readFixture(t, userTemplateFixture)))
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue // template comments may mention claude-code; they must be kept
		}
		if strings.Contains(line, "claude-code") || strings.Contains(line, "opus-a") {
			t.Errorf("tier_models values should be gone after the key was emptied, found %q in:\n%s", line, got)
		}
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RemovingLastKeyInFile_KeepsComments(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("# header\n"+
		"schema_version: \"1\"\n"+
		"# about tiers\n"+
		"tier_models:\n"+
		"  claude-code:\n"+
		"    HIGH: opus-a\n"+
		"# about custom ids\n"+
		"custom_model_ids:\n"+
		"  claude-code:\n"+
		"    - m1\n"+
		"# trailing note\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { c.CustomModelIDs = nil })

	got := readSavedUserConfig(t, root)
	for _, c := range []string{"# header", "# about tiers", "# about custom ids", "# trailing note"} {
		if !strings.Contains(got, c) {
			t.Errorf("comment %q lost when the last key was removed:\n%s", c, got)
		}
	}
	if strings.Contains(got, "custom_model_ids") || strings.Contains(got, "- m1") {
		t.Errorf("the emptied custom_model_ids key and its value should be gone:\n%s", got)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_UserAddedCommentsSurviveRepeatedSaves(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("# my personal note\n"+
		"schema_version: \"1\" # pinned by me\n"+
		"# why I picked opus\n"+
		"tier_models:\n"+
		"  claude-code:\n"+
		"    HIGH: opus-a\n"+
		"# my custom ids\n"+
		"custom_model_ids: {}\n"+
		"# end of my file\n"))

	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "LOW", "haiku-a") })
	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-b") })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, "# my personal note\n# why I picked opus\n# my custom ids\n# end of my file\n")
	if n := strings.Count(got, "# pinned by me"); n != 1 {
		t.Errorf("trailing comment %q appears %d times, want exactly once:\n%s", "# pinned by me", n, got)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RewritingBlock_KeepsCommentsOutsideTheBlock(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("# before schema\n"+
		"schema_version: \"1\"\n"+
		"# before tiers\n"+
		"tier_models:\n"+
		"  claude-code:\n"+
		"    HIGH: opus-a\n"+
		"  opencode:\n"+
		"    HIGH: gpt-a\n"+
		"# after tiers\n"+
		"custom_model_ids: {}\n"+
		"# end of file\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "opencode", "LOW", "gpt-mini") })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, "# before schema\n# before tiers\n# after tiers\n# end of file\n")
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RemovingKeyWithTrailingComment_KeepsCommentsOutsideTheBlock(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("schema_version: \"1\"\n"+
		"tier_models: {}\n"+
		"custom_model_ids:\n"+
		"  claude-code:\n"+
		"    - m1\n"+
		"# end of file\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { c.CustomModelIDs = nil })

	got := readSavedUserConfig(t, root)
	if !strings.Contains(got, "# end of file") {
		t.Errorf("comment %q lost when the key before it was removed:\n%s", "# end of file", got)
	}
	if strings.Contains(got, "- m1") {
		t.Errorf("the removed key's values should be gone:\n%s", got)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RemovingLastKeyWithoutTrailingNewline_KeepsComments(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("# header\n"+
		"schema_version: \"1\"\n"+
		"tier_models: {}\n"+
		"# about custom ids\n"+
		"custom_model_ids:\n"+
		"  claude-code:\n"+
		"    - m1"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { c.CustomModelIDs = nil })

	got := readSavedUserConfig(t, root)
	for _, c := range []string{"# header", "# about custom ids"} {
		if !strings.Contains(got, c) {
			t.Errorf("comment %q lost when the last key (no trailing newline) was removed:\n%s", c, got)
		}
	}
	if strings.Contains(got, "custom_model_ids") || strings.Contains(got, "- m1") {
		t.Errorf("the emptied custom_model_ids key and its value should be gone:\n%s", got)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_CommentsSurviveWhenRewriteAndRemovalCombine(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)

	loadMutateSave(t, store, func(c *config.UserConfig) {
		setTier(c, "claude-code", "HIGH", "opus-a")
		c.CustomModelIDs = map[string][]string{"claude-code": {"m1"}}
	})
	want := loadMutateSave(t, store, func(c *config.UserConfig) {
		setTier(c, "claude-code", "LOW", "haiku-a")
		c.CustomModelIDs = nil
	})

	assertSameComments(t, readSavedUserConfig(t, root), string(readFixture(t, userTemplateFixture)))
	assertReloadsTo(t, store, want)
}
