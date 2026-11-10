package pricing_test

// Helpers and fixtures for the two-level (global + project) pricing store tests.
// Every test works in temporary folders; none touches the real user config folder.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

// layered bundles a store with the two folders it was built from.
type layered struct {
	store      *pricing.Store
	globalDir  string
	root       string
	globalFile string
	projFile   string
}

// newLayered returns a store whose global folder and project root are fresh temp
// folders. The global folder is nested so the store must create it itself.
func newLayered(t *testing.T) layered {
	t.Helper()
	globalDir := filepath.Join(t.TempDir(), "MOSAIC", "LogAnalyzer")
	root := t.TempDir()
	return layered{
		store:      pricing.NewStore(globalDir, root),
		globalDir:  globalDir,
		root:       root,
		globalFile: filepath.Join(globalDir, "pricing.yaml"),
		projFile:   filepath.Join(root, "MosaicLogAnalyzer", "config", "pricing.yaml"),
	}
}

// writeFileAt writes content to path, creating parent folders.
func writeFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// exists reports whether path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// contains reports whether s holds sub.
func contains(s, sub string) bool { return strings.Contains(s, sub) }

// otherRates holds the four non-input rates shared by the generated entries.
const otherRates = "cache_read: \"0.10\"\ncache_write_5m: \"1.25\"\ncache_write_1h: \"2.00\"\noutput: \"5.00\"\n"

// indent prefixes every line of block with n spaces.
func indent(block string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n") + "\n"
}

// modelEntry renders one model entry (without section header) at the given indent
// whose input rate is input.
func modelEntry(model, input string, n int) string {
	return indent(model+":\n"+indent("input: \""+input+"\"\n"+otherRates, 2), n)
}

// modelYAML renders a top-level models section holding the given entries.
func modelYAML(model, input string) string {
	return "models:\n" + modelEntry(model, input, 2)
}

// providerModelYAML renders one provider-scoped model entry.
func providerModelYAML(provider, model, input string) string {
	return "providers:\n  " + provider + ":\n    models:\n" + modelEntry(model, input, 6)
}

// mustLookup returns the entry for (provider, model) or fails the test.
func mustLookup(t *testing.T, table domain.PricingTable, provider, model string) domain.ModelPricing {
	t.Helper()
	e, ok := table.LookupFor(domain.ProviderID(provider), domain.ModelID(model))
	if !ok {
		t.Fatalf("table has no entry for %q/%q", provider, model)
	}
	return e
}
