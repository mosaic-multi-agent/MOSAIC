package config_test

// Tests for the indentation guarantee of UserConfigStore.Save: a rewritten key's value is
// written with two spaces per nesting level, however many times the key was rewritten.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/config"
)

func TestUserConfigStore_Save_RewrittenKeyKeepsTwoSpaceIndent_FromTemplate(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)

	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-a") })
	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "LOW", "haiku-a") })
	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "opencode", "HIGH", "gpt-a") })

	got := keyBlock(t, readSavedUserConfig(t, root), "tier_models")
	wantLines := []string{
		"  claude-code:",
		"    HIGH: opus-a",
		"    LOW: haiku-a",
		"  opencode:",
		"    HIGH: gpt-a",
	}
	if !reflect.DeepEqual(got, wantLines) {
		t.Errorf("tier_models block after three saves =\n%q\nwant\n%q", got, wantLines)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RewrittenKeyKeepsTwoSpaceIndent_WithoutComments(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("schema_version: \"1\"\ntier_models:\n  claude-code:\n    HIGH: opus-a\n"))

	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "LOW", "haiku-a") })
	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "MEDIUM", "sonnet-a") })
	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-b") })

	got := keyBlock(t, readSavedUserConfig(t, root), "tier_models")
	wantLines := []string{
		"  claude-code:",
		"    HIGH: opus-b",
		"    LOW: haiku-a",
		"    MEDIUM: sonnet-a",
	}
	if !reflect.DeepEqual(got, wantLines) {
		t.Errorf("tier_models block after three saves =\n%q\nwant\n%q", got, wantLines)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RewrittenKeyFromFourSpaceIndent_WritesTwoSpaceIndent(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("schema_version: \"1\"\ntier_models:\n    claude-code:\n        HIGH: opus-a\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "LOW", "haiku-a") })

	got := keyBlock(t, readSavedUserConfig(t, root), "tier_models")
	wantLines := []string{
		"  claude-code:",
		"    HIGH: opus-a",
		"    LOW: haiku-a",
	}
	if !reflect.DeepEqual(got, wantLines) {
		t.Errorf("tier_models block after rewrite from four-space indent =\n%q\nwant\n%q", got, wantLines)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RewrittenListValuesKeepTwoSpaceIndent(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)

	loadMutateSave(t, store, func(c *config.UserConfig) {
		c.CustomModelIDs = map[string][]string{"claude-code": {"m1"}}
	})
	loadMutateSave(t, store, func(c *config.UserConfig) {
		c.CustomModelIDs["claude-code"] = append(c.CustomModelIDs["claude-code"], "m2")
	})
	want := loadMutateSave(t, store, func(c *config.UserConfig) {
		c.CustomModelIDs["opencode"] = []string{"m3"}
	})

	got := keyBlock(t, readSavedUserConfig(t, root), "custom_model_ids")
	wantLines := []string{
		"  claude-code:",
		"    - m1",
		"    - m2",
		"  opencode:",
		"    - m3",
	}
	if !reflect.DeepEqual(got, wantLines) {
		t.Errorf("custom_model_ids block after three saves =\n%q\nwant\n%q", got, wantLines)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_ManyRewritesDoNotGrowIndent(t *testing.T) {
	root := seedTemplateUserConfig(t)
	store := config.NewUserConfigStore(root)
	var want config.UserConfig

	for i := 0; i < 6; i++ {
		model := "model-" + string(rune('a'+i))
		want = loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", model) })
	}

	got := keyBlock(t, readSavedUserConfig(t, root), "tier_models")
	wantLines := []string{"  claude-code:", "    HIGH: model-f"}
	if !reflect.DeepEqual(got, wantLines) {
		t.Errorf("tier_models block after six rewrites =\n%q\nwant\n%q", got, wantLines)
	}
	assertReloadsTo(t, store, want)
}
