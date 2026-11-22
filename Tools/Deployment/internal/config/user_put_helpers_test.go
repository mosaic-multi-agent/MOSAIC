package config_test

// Shared helpers for the PutToolDestination and custom-answer save tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

// mainMapping builds a mapping for generic with one main destination holding names.
func mainMapping(generic string, names ...string) domain.ToolMapping {
	return domain.ToolMapping{
		Generic:      generic,
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: names}},
	}
}

// fieldDest builds a field destination with the given format, separator and names.
func fieldDest(field, format, separator string, names ...string) domain.ToolDestination {
	return domain.ToolDestination{
		Kind: domain.DestField, Field: field, Format: domain.ToolValueFormat(format),
		Separator: separator, Names: names,
	}
}

// userConfigPath returns the path of user-config.yaml under root.
func userConfigPath(root string) string {
	return filepath.Join(root, "MosaicDeploy", "config", "user-config.yaml")
}

// mustPut calls PutToolDestination and fails the test on error.
func mustPut(t *testing.T, store config.UserConfigStore, harness string, m domain.ToolMapping) {
	t.Helper()
	if err := store.PutToolDestination(harness, m); err != nil {
		t.Fatalf("PutToolDestination(%q, %q): %v", harness, m.Generic, err)
	}
}

// mustLoad loads the stored config and fails the test on error.
func mustLoad(t *testing.T, store config.UserConfigStore) config.UserConfig {
	t.Helper()
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

// findMapping returns the mapping for generic in list, or false when absent.
func findMapping(list []domain.ToolMapping, generic string) (domain.ToolMapping, bool) {
	for _, m := range list {
		if m.Generic == generic {
			return m, true
		}
	}
	return domain.ToolMapping{}, false
}

// assertMainNames fails unless the harness's entry for generic is one main destination with names.
func assertMainNames(t *testing.T, cfg config.UserConfig, harness, generic string, names ...string) {
	t.Helper()
	m, ok := findMapping(cfg.ToolDestinations[harness], generic)
	if !ok {
		t.Fatalf("no tool_destinations entry for %s/%s in %+v", harness, generic, cfg.ToolDestinations)
	}
	if len(m.Destinations) != 1 || m.Destinations[0].Kind != domain.DestMain {
		t.Fatalf("%s/%s destinations = %+v, want one main destination", harness, generic, m.Destinations)
	}
	got := strings.Join(m.Destinations[0].Names, "|")
	if got != strings.Join(names, "|") {
		t.Errorf("%s/%s names = %q, want %q", harness, generic, got, strings.Join(names, "|"))
	}
}

// assertFileUnchanged fails unless the user config file still holds want exactly.
func assertFileUnchanged(t *testing.T, root, want string) {
	t.Helper()
	data, err := os.ReadFile(userConfigPath(root))
	if err != nil {
		t.Fatalf("reading user-config.yaml: %v", err)
	}
	if string(data) != want {
		t.Errorf("file changed although the put had to fail:\n%s", data)
	}
}

// assertAllLineEndingsCRLF fails when s holds a line feed that is not preceded by a carriage return.
func assertAllLineEndingsCRLF(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Errorf("a bare LF line ending appeared in a CRLF file:\n%q", s)
	}
}
