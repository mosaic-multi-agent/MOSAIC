package pricing_test

// Tests for writing entries into a pricing file that has the provider level.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

func TestPut_ProviderEntry_GoesUnderProviderSectionNotTopLevel(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedOneModel)
	store := pricing.NewStore(t.TempDir(), root)

	got := putAndReload(t, store, withInput(t, "github-copilot", "claude-sonnet-5.5", "7.00"))

	assertPricedAt(t, got.table, "github-copilot", "claude-sonnet-5.5", 7_000_000_000)
	assertUnpricedKey(t, got.table, "", "claude-sonnet-5.5")
	assertPricedAt(t, got.table, "", "test-model-a", 3_000_000_000)
	if !strings.Contains(readFile(t, path), "providers:") {
		t.Error("rewritten file should contain a providers section")
	}
}

func TestPut_EntryWithoutProvider_GoesUnderTopLevelModelsAndKeepsProviders(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, bothSectionsYAML)
	store := pricing.NewStore(t.TempDir(), root)

	got := putAndReload(t, store, withInput(t, "", "new-plain", "2.00"))

	assertPricedAt(t, got.table, "", "new-plain", 2_000_000_000)
	assertUnpricedKey(t, got.table, "anthropic", "new-plain")
	assertPricedAt(t, got.table, "anthropic", "claude-sonnet-5-5", 5_000_000_000)
	assertPricedAt(t, got.table, "github-copilot", "plain-model", 9_000_000_000)
	assertPricedAt(t, got.table, "", "plain-model", 3_000_000_000)
}

func TestPut_ProviderEntry_ReplacesOnlyEntryAtThatPath(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, bothSectionsYAML)
	store := pricing.NewStore(t.TempDir(), root)

	got := putAndReload(t, store, withInput(t, "github-copilot", "plain-model", "11.00"))

	assertPricedAt(t, got.table, "github-copilot", "plain-model", 11_000_000_000)
	assertPricedAt(t, got.table, "", "plain-model", 3_000_000_000) // top-level same model untouched
	assertPricedAt(t, got.table, "github-copilot", "claude-sonnet-5.5", 7_000_000_000)
	assertPricedAt(t, got.table, "anthropic", "claude-sonnet-5-5", 5_000_000_000)
	if got.table.Len() != 4 {
		t.Errorf("table has %d entries, want 4", got.table.Len())
	}
}

func TestPut_NewProviderEntry_KeepsOtherProvidersAndTopLevel(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, bothSectionsYAML)
	store := pricing.NewStore(t.TempDir(), root)

	got := putAndReload(t, store, withInput(t, "amazon-bedrock", "claude-sonnet-5-5", "6.00"))

	assertPricedAt(t, got.table, "amazon-bedrock", "claude-sonnet-5-5", 6_000_000_000)
	assertPricedAt(t, got.table, "anthropic", "claude-sonnet-5-5", 5_000_000_000)
	assertPricedAt(t, got.table, "", "plain-model", 3_000_000_000)
	if got.table.Len() != 5 {
		t.Errorf("table has %d entries, want 5", got.table.Len())
	}
	if f := invalidEntryFindings(got.findings); len(f) != 0 {
		t.Errorf("unexpected findings after rewrite: %v", f)
	}
}

func TestPut_RewriteKeepsLeadingCommentBlockWithBothSections(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, "# my own notes\n# keep this line\n\n"+bothSectionsYAML)
	store := pricing.NewStore(t.TempDir(), root)

	if _, err := store.Put(context.Background(), domain.LevelProject, withInput(t, "anthropic", "another-model", "4.00")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	raw := readFile(t, path)
	if !strings.HasPrefix(raw, "# my own notes\n# keep this line\n") {
		t.Errorf("leading comment block was not preserved; file starts with %q", raw[:min(len(raw), 60)])
	}
	if !strings.Contains(raw, "providers:") || !strings.Contains(raw, "\nmodels:") {
		t.Error("rewritten file must keep both the providers and models sections")
	}
}

func TestPut_ProviderEntryWithLongContext_RoundTrips(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())
	entry := newEntryWithLongContext(t, "lc-model")
	entry.Provider = "anthropic"

	got := putAndReload(t, store, entry)

	// 1M input tokens is above the 200k threshold, so the long-context input
	// rate (6.00) applies.
	assertPricedAt(t, got.table, "anthropic", "lc-model", 6_000_000_000)
	// The entry belongs under the provider only: strict lookup without a provider misses it.
	assertUnpricedKey(t, got.table, "", "lc-model")
	raw := readFile(t, store.Path())
	providersAt := strings.Index(raw, "\nproviders:")
	entryAt := strings.Index(raw, "lc-model:")
	if providersAt < 0 || entryAt < providersAt {
		t.Errorf("lc-model must be written under the providers section; providers at %d, entry at %d", providersAt, entryAt)
	}
}

func TestPut_ProviderEntryOnAbsentFile_StartsFromTemplateAndLoads(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())

	got := putAndReload(t, store, withInput(t, "anthropic", "first-model", "5.00"))

	header := leadingCommentBlock(readFile(t, store.Path()))
	if strings.TrimSpace(header) == "" {
		t.Error("a file created by Put must start with the template comment block")
	}
	if !strings.Contains(header, "providers") {
		t.Error("the template header must describe the provider level")
	}
	assertPricedAt(t, got.table, "anthropic", "first-model", 5_000_000_000)
	if f := invalidEntryFindings(got.findings); len(f) != 0 {
		t.Errorf("unexpected findings: %v", f)
	}
}
