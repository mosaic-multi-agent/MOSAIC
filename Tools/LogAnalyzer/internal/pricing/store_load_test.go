package pricing_test

// Tests for loading the pricing file: valid entries with and without a
// long-context block, absent and comments-only files, unparseable documents.

import (
	"context"
	"os"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

func TestStore_Load_EntryWithoutLongContext_LoadsFiveStandardRates(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedOneModel)

	table, findings, err := pricing.NewStore(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("well-formed file must produce no findings; got %v", findings)
	}
	got, found := table.Lookup("test-model-a")
	if !found {
		t.Fatal("test-model-a must be found")
	}
	want := newMinimalEntry(t, "test-model-a")
	if got != want {
		t.Errorf("loaded entry = %+v, want %+v", got, want)
	}
	if got.HasLongContextTier() {
		t.Error("entry without long_context must report no long-context tier")
	}
}

func TestStore_Load_EntryWithLongContext_LoadsThresholdAndSecondRateSet(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedTwoModels)

	table, findings, err := pricing.NewStore(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("well-formed file must produce no findings; got %v", findings)
	}
	if table.Len() != 2 {
		t.Fatalf("table must hold 2 entries; got %d", table.Len())
	}
	got, found := table.Lookup("test-model-b")
	if !found {
		t.Fatal("test-model-b must be found")
	}
	want := domain.ModelPricing{
		Model: "test-model-b",
		Standard: domain.RateSet{
			Input: mustRate(t, "1.00"), CacheRead: mustRate(t, "0.10"),
			CacheWrite5m: mustRate(t, "1.25"), CacheWrite1h: mustRate(t, "2.00"),
			Output: mustRate(t, "5.00"),
		},
		HasLongContext: true,
		LongContext: domain.LongContextPricing{
			Threshold: 100_000,
			Rates: domain.RateSet{
				Input: mustRate(t, "2.00"), CacheRead: mustRate(t, "0.20"),
				CacheWrite5m: mustRate(t, "2.50"), CacheWrite1h: mustRate(t, "4.00"),
				Output: mustRate(t, "8.00"),
			},
		},
	}
	if got != want {
		t.Errorf("loaded entry = %+v, want %+v", got, want)
	}
}

func TestStore_Load_AbsentFile_ReturnsEmptyTableAndNoError(t *testing.T) {
	store := pricing.NewStore(t.TempDir())

	table, findings, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load with absent file must return nil error; got: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("absent file must produce no findings; got %d", len(findings))
	}
	if table.Len() != 0 {
		t.Errorf("absent file must yield an empty table; got %d entries", table.Len())
	}
}

func TestStore_Load_AbsentFile_DoesNotCreateTheFile(t *testing.T) {
	store := pricing.NewStore(t.TempDir())

	if _, _, err := store.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Errorf("Load must never create the pricing file; stat err = %v", err)
	}
}

func TestStore_Load_CommentsOnlyFile_ReturnsEmptyTable(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, commentsOnlyYAML)

	table, findings, err := pricing.NewStore(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load of a comments-only file must not return an error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("comments-only file must produce no findings; got %d", len(findings))
	}
	if table.Len() != 0 {
		t.Errorf("comments-only file must yield an empty table; got %d entries", table.Len())
	}
}

func TestStore_Load_MalformedYAML_ReturnsErrorAndEmptyTable(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, malformedYAML)

	table, _, err := pricing.NewStore(root).Load(context.Background())
	if err == nil {
		t.Fatal("Load of a malformed YAML document must return a non-nil error")
	}
	if table.Len() != 0 {
		t.Errorf("malformed YAML must yield an empty table; got %d entries", table.Len())
	}
}

func TestStore_Load_UnconfiguredModel_ReturnsNotConfiguredOutcome(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedOneModel)

	table, _, err := pricing.NewStore(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, found := table.Lookup("unconfigured-model"); found {
		t.Error("lookup for a model absent from the YAML must return found=false")
	}
	if _, found := table.Lookup("test-model-a"); !found {
		t.Error("the configured model must be found")
	}
}

func TestStore_Load_LookupByModelIDOnly_HarnessNotPartOfKey(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedOneModel)

	table, _, err := pricing.NewStore(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, found := table.Lookup("test-model-a"); !found {
		t.Error("bare model ID lookup must find the entry")
	}
	for _, qualified := range []domain.ModelID{"claude-code/test-model-a", "opencode/test-model-a"} {
		if _, found := table.Lookup(qualified); found {
			t.Errorf("harness-qualified key %q must not be found", qualified)
		}
	}
}
