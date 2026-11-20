package config_test

// Shared helpers for the save indentation and comment preservation tests.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

// seedTemplateUserConfig writes the commented user-config template fixture into a fresh
// root and returns the root.
func seedTemplateUserConfig(t *testing.T) string {
	t.Helper()
	root := makeRoot(t)
	writeUserConfig(t, root, readFixture(t, userTemplateFixture))
	return root
}

// loadMutateSave loads the stored config, applies mutate and saves it, returning the saved value.
func loadMutateSave(t *testing.T, store config.UserConfigStore, mutate func(*config.UserConfig)) config.UserConfig {
	t.Helper()
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	mutate(&cfg)
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return cfg
}

// setTier records a tier model for a harness, creating the maps when needed.
func setTier(cfg *config.UserConfig, harness string, tier domain.Tier, model string) {
	if cfg.TierModels == nil {
		cfg.TierModels = map[string]map[domain.Tier]string{}
	}
	if cfg.TierModels[harness] == nil {
		cfg.TierModels[harness] = map[domain.Tier]string{}
	}
	cfg.TierModels[harness][tier] = model
}

// assertReloadsTo fails unless Load returns want; nil and empty collections compare equal.
func assertReloadsTo(t *testing.T, store config.UserConfigStore, want config.UserConfig) {
	t.Helper()
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, want.SchemaVersion)
	}
	if len(got.TierModels) != len(want.TierModels) ||
		(len(want.TierModels) > 0 && !reflect.DeepEqual(got.TierModels, want.TierModels)) {
		t.Errorf("TierModels = %v, want %v", got.TierModels, want.TierModels)
	}
	if len(got.CustomModelIDs) != len(want.CustomModelIDs) ||
		(len(want.CustomModelIDs) > 0 && !reflect.DeepEqual(got.CustomModelIDs, want.CustomModelIDs)) {
		t.Errorf("CustomModelIDs = %v, want %v", got.CustomModelIDs, want.CustomModelIDs)
	}
}

// keyBlock returns the lines of the top-level key's value block: every line after the
// "<key>:" line up to the next line that starts in column 0 (key or comment) or the end.
// Blank lines inside the block are dropped.
func keyBlock(t *testing.T, content, key string) []string {
	t.Helper()
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("key %q not found in:\n%s", key, content)
	}
	var block []string
	for _, l := range lines[start+1:] {
		if l == "" {
			continue
		}
		if l[0] != ' ' {
			break
		}
		block = append(block, l)
	}
	return block
}

// commentLines returns every line whose first non-space character is '#', trimmed on the right.
func commentLines(content string) []string {
	var out []string
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			out = append(out, strings.TrimRight(l, " \r"))
		}
	}
	return out
}

// assertSameComments fails unless got carries exactly the comment lines of want, in order.
func assertSameComments(t *testing.T, got, want string) {
	t.Helper()
	g, w := commentLines(got), commentLines(want)
	if reflect.DeepEqual(g, w) {
		return
	}
	for _, line := range w {
		if !containsLine(g, line) {
			t.Errorf("comment line lost by Save: %q", line)
			return
		}
	}
	t.Errorf("comment lines changed or reordered by Save: got %d comment lines, want %d", len(g), len(w))
}

func containsLine(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}
