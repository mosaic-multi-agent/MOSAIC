package pricing_test

// Shared helpers and YAML fixtures for the pricing store tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// mustRate parses a USD-per-million decimal string into a Rate.
func mustRate(t testing.TB, s string) domain.Rate {
	t.Helper()
	r, err := domain.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// standardRates returns the standard rate set used across the store tests.
func standardRates(t testing.TB) domain.RateSet {
	t.Helper()
	return domain.RateSet{
		Input:        mustRate(t, "3.00"),
		CacheRead:    mustRate(t, "0.30"),
		CacheWrite5m: mustRate(t, "3.75"),
		CacheWrite1h: mustRate(t, "6.00"),
		Output:       mustRate(t, "15.00"),
	}
}

// longContextRates returns the long-context rate set used across the store tests.
func longContextRates(t testing.TB) domain.RateSet {
	t.Helper()
	return domain.RateSet{
		Input:        mustRate(t, "6.00"),
		CacheRead:    mustRate(t, "0.60"),
		CacheWrite5m: mustRate(t, "7.50"),
		CacheWrite1h: mustRate(t, "12.00"),
		Output:       mustRate(t, "22.50"),
	}
}

// newMinimalEntry returns a valid entry with five standard rates and no long-context tier.
func newMinimalEntry(t testing.TB, model domain.ModelID) domain.ModelPricing {
	t.Helper()
	return domain.ModelPricing{Model: model, Standard: standardRates(t)}
}

// newEntryWithLongContext returns a valid entry that also carries a long-context tier.
func newEntryWithLongContext(t testing.TB, model domain.ModelID) domain.ModelPricing {
	t.Helper()
	e := newMinimalEntry(t, model)
	e.HasLongContext = true
	e.LongContext = domain.LongContextPricing{Threshold: 200_000, Rates: longContextRates(t)}
	return e
}

// makeRootWithConfigDir creates a temp root that already contains the pricing
// config directory and returns it with the path of the pricing file.
func makeRootWithConfigDir(t *testing.T) (root string, pricingPath string) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, "MosaicLogAnalyzer", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	return root, filepath.Join(dir, "pricing.yaml")
}

// writePricingYAML writes content to the given path.
func writePricingYAML(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write pricing file: %v", err)
	}
}

// readFile returns the file content as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// invalidEntryFindings keeps only the invalid_pricing_entry findings.
func invalidEntryFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingInvalidPricingEntry {
			out = append(out, f)
		}
	}
	return out
}

// findingFor returns the invalid_pricing_entry finding whose Detail names model.
func findingFor(findings []domain.Finding, model string) (domain.Finding, bool) {
	for _, f := range invalidEntryFindings(findings) {
		if strings.Contains(f.Detail, model) {
			return f, true
		}
	}
	return domain.Finding{}, false
}

// activeLines returns the non-blank, non-comment lines of a YAML document.
func activeLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// leadingCommentBlock returns the leading run of blank and comment lines.
func leadingCommentBlock(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// YAML fixtures (new schema)
// ---------------------------------------------------------------------------

// wellFormedOneModel has a single entry without a long-context block.
const wellFormedOneModel = `
models:
  test-model-a:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
`

// wellFormedTwoModels has one entry without and one with a long-context block.
const wellFormedTwoModels = `
models:
  test-model-a:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
  test-model-b:
    input: "1.00"
    cache_read: "0.10"
    cache_write_5m: "1.25"
    cache_write_1h: "2.00"
    output: "5.00"
    long_context:
      threshold: 100000
      input: "2.00"
      cache_read: "0.20"
      cache_write_5m: "2.50"
      cache_write_1h: "4.00"
      output: "8.00"
`

// commentsOnlyYAML is a documented-but-empty file: no models key.
const commentsOnlyYAML = `
# MosaicLogAnalyzer pricing configuration
#
# models:
#   example-model:
#     input: "3.00"

# No active model prices are defined in this file.
`

// malformedYAML cannot be parsed as a YAML document.
const malformedYAML = `models: {bad yaml: [`

// validSiblingYAML is a valid entry appended to fixtures that test isolation.
const validSiblingYAML = `  valid-model:
    input: "1.00"
    cache_read: "0.10"
    cache_write_5m: "1.25"
    cache_write_1h: "2.00"
    output: "5.00"
`

// asProject returns e marked as loaded from the project level, as the store
// reports every entry it reads from the project file.
func asProject(e domain.ModelPricing) domain.ModelPricing {
	e.Source = domain.LevelProject
	return e
}
