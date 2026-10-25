package pricing_test

// Tests for loading a pricing file that has the provider level.

import (
	"strings"
	"testing"
)

const providerEntryBody = `        input: "5.00"
        cache_read: "0.50"
        cache_write_5m: "6.25"
        cache_write_1h: "10.00"
        output: "25.00"
`

func TestLoad_BothSections_EntriesKeyedByProviderAndModel(t *testing.T) {
	got, err := loadFrom(t, bothSectionsYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if f := invalidEntryFindings(got.findings); len(f) != 0 {
		t.Fatalf("unexpected invalid_pricing_entry findings: %v", f)
	}
	assertPricedAt(t, got.table, "", "plain-model", 3_000_000_000)
	assertPricedAt(t, got.table, "anthropic", "claude-sonnet-5-5", 5_000_000_000)
	assertPricedAt(t, got.table, "github-copilot", "claude-sonnet-5.5", 7_000_000_000)
	assertPricedAt(t, got.table, "github-copilot", "plain-model", 9_000_000_000)
}

func TestLoad_BothSections_NoFallbackBetweenSections(t *testing.T) {
	got, err := loadFrom(t, bothSectionsYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertUnpricedKey(t, got.table, "", "claude-sonnet-5-5")               // provider entry never serves a plain record
	assertUnpricedKey(t, got.table, "github-copilot", "claude-sonnet-5-5") // another provider's entry
	assertUnpricedKey(t, got.table, "anthropic", "plain-model")            // top-level entry never serves a provider record
}

func TestLoad_OnlyProviderSection_LoadsEntries(t *testing.T) {
	got, err := loadFrom(t, "providers:\n  anthropic:\n    models:\n      m1:\n"+providerEntryBody)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertPricedAt(t, got.table, "anthropic", "m1", 5_000_000_000)
	if got.table.Len() != 1 {
		t.Errorf("table has %d entries, want 1", got.table.Len())
	}
}

func TestLoad_OldFormatEntryInProviderSection_FindingNamesModelAndProviderAndIsSkipped(t *testing.T) {
	content := bothSectionsYAML + `  amazon-bedrock:
    models:
      old-model:
        input: "3.00"
        cached_input: "0.30"
        cache_write: "3.75"
        output_under_threshold: "15.00"
`
	got, err := loadFrom(t, content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	f, ok := findingFor(got.findings, "old-model")
	if !ok {
		t.Fatalf("expected an invalid_pricing_entry finding naming old-model, got %v", got.findings)
	}
	if !strings.Contains(f.Detail, "amazon-bedrock") {
		t.Errorf("finding detail %q should name the provider", f.Detail)
	}
	assertUnpricedKey(t, got.table, "amazon-bedrock", "old-model")
	assertPricedAt(t, got.table, "anthropic", "claude-sonnet-5-5", 5_000_000_000) // siblings still load
}

func TestLoad_ZeroRateInProviderSection_FindingAndSkipped(t *testing.T) {
	content := "providers:\n  anthropic:\n    models:\n      bad-model:\n" +
		strings.Replace(providerEntryBody, `input: "5.00"`, `input: "0"`, 1) +
		"      good-model:\n" + providerEntryBody
	got, err := loadFrom(t, content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := findingFor(got.findings, "bad-model"); !ok {
		t.Fatalf("expected a finding for bad-model, got %v", got.findings)
	}
	assertUnpricedKey(t, got.table, "anthropic", "bad-model")
	assertPricedAt(t, got.table, "anthropic", "good-model", 5_000_000_000)
}

func TestLoad_UnparseableRateInProviderSection_FindingAndSkipped(t *testing.T) {
	content := "providers:\n  anthropic:\n    models:\n      bad-model:\n" +
		strings.Replace(providerEntryBody, `input: "5.00"`, `input: "abc"`, 1)
	got, err := loadFrom(t, content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := findingFor(got.findings, "bad-model"); !ok {
		t.Fatalf("expected a finding for bad-model, got %v", got.findings)
	}
	assertUnpricedKey(t, got.table, "anthropic", "bad-model")
}

func TestLoad_InvalidTopLevelEntryAlongsideProviderSection_FindingAndProviderStillLoads(t *testing.T) {
	content := "models:\n  old-top:\n    input: \"3.00\"\n    cached_input: \"0.30\"\n" +
		"providers:\n  anthropic:\n    models:\n      good-model:\n" + providerEntryBody
	got, err := loadFrom(t, content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := findingFor(got.findings, "old-top"); !ok {
		t.Fatalf("expected a finding for old-top, got %v", got.findings)
	}
	assertUnpricedKey(t, got.table, "", "old-top")
	assertPricedAt(t, got.table, "anthropic", "good-model", 5_000_000_000)
}

func TestLoad_EmptyProviderIDKey_EntryYieldsFindingAndIsSkipped(t *testing.T) {
	content := "providers:\n  \"\":\n    models:\n      orphan-model:\n" + providerEntryBody
	got, err := loadFrom(t, content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if n := len(invalidEntryFindings(got.findings)); n != 1 {
		t.Fatalf("want exactly one invalid_pricing_entry finding, got %d: %v", n, got.findings)
	}
	assertUnpricedKey(t, got.table, "", "orphan-model") // must not leak into top-level models
}

func TestLoad_ProviderWithoutModelsOrEmptyModels_NoFindingNoEntries(t *testing.T) {
	got, err := loadFrom(t, "providers:\n  anthropic:\n  github-copilot:\n    models:\n  amazon-bedrock:\n    models: {}\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.findings) != 0 {
		t.Errorf("findings = %v, want none", got.findings)
	}
	if got.table.Len() != 0 {
		t.Errorf("table has %d entries, want 0", got.table.Len())
	}
}

func TestLoad_OnlyTopLevelModels_LoadsExactlyAsBefore(t *testing.T) {
	got, err := loadFrom(t, wellFormedTwoModels)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.findings) != 0 {
		t.Errorf("findings = %v, want none", got.findings)
	}
	assertPricedAt(t, got.table, "", "test-model-a", 3_000_000_000)
	assertUnpricedKey(t, got.table, "anthropic", "test-model-a")
	if got.table.Len() != 2 {
		t.Errorf("table has %d entries, want 2", got.table.Len())
	}
}
