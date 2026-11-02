package pricing_test

// Tests for entries that cannot be priced: old-format entries, missing,
// unparseable or negative rates, and incomplete long-context blocks. Each is
// reported as an invalid_pricing_entry finding, the model stays unpriced and
// the rest of the file still loads.

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

// oldFormatKeys are the keys of the retired pricing schema.
var oldFormatKeys = []string{
	"cached_input", "cache_write", "output_under_threshold",
	"output_at_threshold", "context_length_threshold",
}

// mentionsKey reports whether text contains key as a whole identifier, so that
// the old key cache_write does not match the new keys cache_write_5m/_1h.
func mentionsKey(text, key string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(key) + `($|[^A-Za-z0-9_])`).MatchString(text)
}

// loadWith writes a file holding bad-model (body) followed by a valid sibling
// and loads it.
func loadWith(t *testing.T, badEntry string) (domain.PricingTable, []domain.Finding) {
	t.Helper()
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, "models:\n"+badEntry+validSiblingYAML)
	table, findings, err := pricing.NewStore(t.TempDir(), root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load must not fail for a per-entry problem: %v", err)
	}
	return table, findings
}

// assertSkippedWithFinding checks the isolation contract for bad-model and
// returns the finding Detail.
func assertSkippedWithFinding(t *testing.T, table domain.PricingTable, findings []domain.Finding) string {
	t.Helper()
	if _, found := table.Lookup("bad-model"); found {
		t.Error("an invalid entry must not appear in the table")
	}
	if _, found := table.Lookup("valid-model"); !found {
		t.Error("a valid sibling entry must still load")
	}
	invalid := invalidEntryFindings(findings)
	if len(invalid) != 1 {
		t.Fatalf("want exactly 1 invalid_pricing_entry finding, got %d: %v", len(invalid), findings)
	}
	if !strings.Contains(invalid[0].Detail, "bad-model") {
		t.Errorf("finding Detail must name the model; got %q", invalid[0].Detail)
	}
	return invalid[0].Detail
}

func TestStore_Load_OldFormatEntry_IsInvalidWithRecognisableDetail(t *testing.T) {
	table, findings := loadWith(t, `  bad-model:
    input: "3.00"
    cached_input: "0.30"
    cache_write: "3.75"
    output_under_threshold: "15.00"
    output_at_threshold: "22.50"
    context_length_threshold: 200000
`)

	detail := assertSkippedWithFinding(t, table, findings)
	if !strings.Contains(detail, "old pricing format") {
		t.Errorf("Detail must contain %q; got %q", "old pricing format", detail)
	}
	named := false
	for _, k := range oldFormatKeys {
		if mentionsKey(detail, k) {
			named = true
		}
	}
	if !named {
		t.Errorf("Detail must name at least one offending old key; got %q", detail)
	}
}

func TestStore_Load_EachOldKeyAloneMakesTheEntryInvalid(t *testing.T) {
	// A complete new-format entry plus a single old key is still invalid.
	for _, key := range oldFormatKeys {
		t.Run(key, func(t *testing.T) {
			value := `"1.00"`
			if key == "context_length_threshold" {
				value = "200000"
			}
			table, findings := loadWith(t, `  bad-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
    `+key+": "+value+"\n")

			detail := assertSkippedWithFinding(t, table, findings)
			if !strings.Contains(detail, "old pricing format") {
				t.Errorf("Detail must contain %q; got %q", "old pricing format", detail)
			}
			if !mentionsKey(detail, key) {
				t.Errorf("Detail must name the offending key %q; got %q", key, detail)
			}
		})
	}
}

func TestStore_Load_InvalidEntries_NameTheOffendingField(t *testing.T) {
	cases := []struct {
		name  string
		entry string
		field string
	}{
		{"missing standard rate", `  bad-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    output: "15.00"
`, "cache_write_1h"},
		{"non-numeric rate", `  bad-model:
    input: "abc"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
`, "input"},
		{"negative rate", `  bad-model:
    input: "3.00"
    cache_read: "-0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
`, "cache_read"},
		{"long_context without threshold", `  bad-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
    long_context:
      input: "6.00"
      cache_read: "0.60"
      cache_write_5m: "7.50"
      cache_write_1h: "12.00"
      output: "22.50"
`, "long_context.threshold"},
		{"long_context zero threshold", `  bad-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
    long_context:
      threshold: 0
      input: "6.00"
      cache_read: "0.60"
      cache_write_5m: "7.50"
      cache_write_1h: "12.00"
      output: "22.50"
`, "long_context.threshold"},
		{"long_context incomplete rate set", `  bad-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
    long_context:
      threshold: 200000
      input: "6.00"
      cache_read: "0.60"
      cache_write_5m: "7.50"
      cache_write_1h: "12.00"
`, "long_context.output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table, findings := loadWith(t, tc.entry)

			detail := assertSkippedWithFinding(t, table, findings)
			if !strings.Contains(detail, tc.field) {
				t.Errorf("Detail must name %q; got %q", tc.field, detail)
			}
			if strings.Contains(detail, "old pricing format") {
				t.Errorf("a new-format problem must not be reported as old format; got %q", detail)
			}
		})
	}
}

func TestStore_Load_OldFormatEntry_DoesNotPreventOtherEntriesLoading(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, `models:
  old-model:
    input: "3.00"
    cached_input: "0.30"
    cache_write: "3.75"
    output_under_threshold: "15.00"
  test-model-a:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
`)

	table, findings, err := pricing.NewStore(t.TempDir(), root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, found := table.Lookup("old-model"); found {
		t.Error("old-format model must stay unpriced")
	}
	if got, found := table.Lookup("test-model-a"); !found || got != asProject(newMinimalEntry(t, "test-model-a")) {
		t.Errorf("new-format entry must load intact; found=%v got=%+v", found, got)
	}
	if _, ok := findingFor(findings, "old-model"); !ok {
		t.Errorf("want an invalid_pricing_entry finding for old-model; got %v", findings)
	}
}
