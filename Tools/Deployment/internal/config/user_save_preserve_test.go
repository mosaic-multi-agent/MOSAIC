package config_test

// Tests for what UserConfigStore.Save leaves alone: keys whose value did not change stay
// byte-identical, non-empty tool_destinations survive, CRLF files keep their comments, and
// removing a key in the middle of a file keeps the surrounding comments and blank-line layout.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/config"
)

func TestUserConfigStore_Save_UnchangedKeys_StayByteIdentical(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	schemaLine := "schema_version: \"1\"   # pinned by me\n"
	idsBlock := "custom_model_ids:\n    claude-code:\n        - m1   # odd indent kept\n"
	writeUserConfig(t, root, []byte("# top\n"+schemaLine+"\n# ids\n"+idsBlock+"# tail\n"))

	loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-a") })

	got := readSavedUserConfig(t, root)
	for _, want := range []string{schemaLine, idsBlock} {
		if !strings.Contains(got, want) {
			t.Errorf("an unchanged key's text was altered; missing verbatim %q in:\n%s", want, got)
		}
	}
}

func TestUserConfigStore_Save_NonEmptyToolDestinations_SurviveASaveOfAnotherKey(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	block := "tool_destinations:\n" +
		"  claude-code:\n" +
		"    - generic: knowledge_base\n" +
		"      destinations:\n" +
		"        - to: main\n" +
		"          names: [\"Read\"]\n"
	writeUserConfig(t, root, []byte("# note\nschema_version: \"1\"\n"+block+"# end\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "HIGH", "opus-a") })

	got := readSavedUserConfig(t, root)
	if !strings.Contains(got, block) {
		t.Errorf("the tool_destinations block changed although its value did not:\n%s", got)
	}
	assertSameComments(t, got, "# note\n# end\n")
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(reloaded.ToolDestinations["claude-code"]) != 1 {
		t.Errorf("tool_destinations lost on reload: %+v", reloaded.ToolDestinations)
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_CRLFFile_KeepsCommentsAndReloadsToSavedValues(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	crlf := strings.ReplaceAll("# header\n"+
		"schema_version: \"1\"\n"+
		"# about tiers\n"+
		"tier_models:\n"+
		"  claude-code:\n"+
		"    HIGH: opus-a\n"+
		"# about ids\n"+
		"custom_model_ids: {}\n"+
		"# footer\n", "\n", "\r\n")
	writeUserConfig(t, root, []byte(crlf))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { setTier(c, "claude-code", "LOW", "haiku-a") })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, "# header\n# about tiers\n# about ids\n# footer\n")
	for _, line := range keyBlock(t, strings.ReplaceAll(got, "\r\n", "\n"), "tier_models") {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent%2 != 0 || strings.HasPrefix(line, "\t") {
			t.Errorf("rewritten tier_models line is not two-space indented: %q", line)
		}
	}
	assertReloadsTo(t, store, want)
}

func TestUserConfigStore_Save_RemovingMiddleKey_KeepsNeighbourCommentsAndBlankLayout(t *testing.T) {
	root := makeRoot(t)
	store := config.NewUserConfigStore(root)
	writeUserConfig(t, root, []byte("schema_version: \"1\"\n"+
		"\n"+
		"# about ids\n"+
		"custom_model_ids:\n"+
		"  claude-code:\n"+
		"    - m1\n"+
		"\n"+
		"# about tiers\n"+
		"tier_models:\n"+
		"  claude-code:\n"+
		"    HIGH: opus-a\n"))

	want := loadMutateSave(t, store, func(c *config.UserConfig) { c.CustomModelIDs = nil })

	got := readSavedUserConfig(t, root)
	assertSameComments(t, got, "# about ids\n# about tiers\n")
	if strings.Contains(got, "custom_model_ids") || strings.Contains(got, "- m1") {
		t.Errorf("the emptied middle key and its value should be gone:\n%s", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("removal left a run of blank lines:\n%q", got)
	}
	if !strings.Contains(got, "tier_models:\n  claude-code:\n    HIGH: opus-a\n") {
		t.Errorf("the neighbouring tier_models block was disturbed:\n%s", got)
	}
	assertReloadsTo(t, store, want)
}
