package pricing_test

// Tests for Put: round trip with and without a long-context block, keeping other
// entries (valid or not), replacing an entry entirely, leading comments, and an
// unparseable document.

import (
	"context"
	"os"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

// putAndLoad writes entry through a store and loads the table back.
func putAndLoad(t *testing.T, store *pricing.Store, entry domain.ModelPricing) (domain.PricingTable, []domain.Finding) {
	t.Helper()
	if _, err := store.Put(context.Background(), domain.LevelProject, entry); err != nil {
		t.Fatalf("Put: %v", err)
	}
	table, findings, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load after Put: %v", err)
	}
	return table, findings
}

func TestStore_Put_EntryWithoutLongContext_RoundTripsUnchanged(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())
	entry := newMinimalEntry(t, "round-trip-model")

	table, findings := putAndLoad(t, store, entry)

	got, found := table.Lookup("round-trip-model")
	if !found {
		t.Fatal("entry must be found after Put then Load")
	}
	if got != asProject(entry) {
		t.Errorf("round trip = %+v, want %+v", got, entry)
	}
	if len(findings) != 0 {
		t.Errorf("a written entry must load without findings; got %v", findings)
	}
}

func TestStore_Put_EntryWithLongContext_RoundTripsUnchanged(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())
	entry := newEntryWithLongContext(t, "round-trip-lc")

	table, findings := putAndLoad(t, store, entry)

	got, found := table.Lookup("round-trip-lc")
	if !found {
		t.Fatal("entry must be found after Put then Load")
	}
	if got != asProject(entry) {
		t.Errorf("round trip = %+v, want %+v", got, entry)
	}
	if len(findings) != 0 {
		t.Errorf("a written entry must load without findings; got %v", findings)
	}
}

func TestStore_Put_EntryWithoutLongContext_WritesNoActiveLongContextBlock(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())

	putAndLoad(t, store, newMinimalEntry(t, "plain-model"))

	active := strings.Join(activeLines(readFile(t, store.Path())), "\n")
	if !strings.Contains(active, "plain-model") || !strings.Contains(active, "input:") {
		t.Fatalf("the entry itself must be written; active lines:\n%s", active)
	}
	for _, line := range activeLines(readFile(t, store.Path())) {
		if strings.Contains(line, "long_context") {
			t.Errorf("an entry without long context must not write a long_context block; found %q", line)
		}
	}
}

func TestStore_Put_WritesNewKeysOnly(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())

	putAndLoad(t, store, newEntryWithLongContext(t, "keys-model"))

	active := strings.Join(activeLines(readFile(t, store.Path())), "\n")
	for _, key := range []string{"input:", "cache_read:", "cache_write_5m:", "cache_write_1h:", "output:", "long_context:", "threshold:"} {
		if !strings.Contains(active, key) {
			t.Errorf("written entry must contain key %q", key)
		}
	}
	for _, key := range oldFormatKeys {
		if mentionsKey(active, key) {
			t.Errorf("written entry must not contain old key %q", key)
		}
	}
}

func TestStore_Put_RatesPrecisionPreservedInRoundTrip(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())
	entry := newEntryWithLongContext(t, "precision-model")
	entry.Standard.CacheRead = mustRate(t, "0.375")
	entry.LongContext.Rates.CacheWrite1h = mustRate(t, "12.125")

	table, _ := putAndLoad(t, store, entry)

	got, _ := table.Lookup("precision-model")
	if got.Standard.CacheRead != entry.Standard.CacheRead {
		t.Errorf("Standard.CacheRead = %d nanos, want %d", int64(got.Standard.CacheRead), int64(entry.Standard.CacheRead))
	}
	if got.LongContext.Rates.CacheWrite1h != entry.LongContext.Rates.CacheWrite1h {
		t.Errorf("LongContext.Rates.CacheWrite1h = %d nanos, want %d",
			int64(got.LongContext.Rates.CacheWrite1h), int64(entry.LongContext.Rates.CacheWrite1h))
	}
}

func TestStore_Put_ExistingFileWithEntries_PreservesExistingEntries(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedTwoModels)
	store := pricing.NewStore(t.TempDir(), root)

	table, findings := putAndLoad(t, store, newMinimalEntry(t, "test-model-new"))

	if len(findings) != 0 {
		t.Errorf("no findings expected; got %v", findings)
	}
	if table.Len() != 3 {
		t.Fatalf("table must hold 3 entries; got %d", table.Len())
	}
	b, _ := table.Lookup("test-model-b")
	if !b.HasLongContext || b.LongContext.Threshold != 100_000 {
		t.Errorf("the existing long-context entry must survive intact; got %+v", b)
	}
}

func TestStore_Put_KeepsOtherOldFormatEntriesAndTheirFindings(t *testing.T) {
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
	store := pricing.NewStore(t.TempDir(), root)

	table, findings := putAndLoad(t, store, newMinimalEntry(t, "added-model"))

	if _, found := table.Lookup("added-model"); !found {
		t.Error("the new entry must be present")
	}
	if got, _ := table.Lookup("test-model-a"); got != asProject(newMinimalEntry(t, "test-model-a")) {
		t.Errorf("the valid existing entry must be unchanged; got %+v", got)
	}
	f, ok := findingFor(findings, "old-model")
	if !ok {
		t.Fatalf("the other old-format entry must be kept and still reported; findings: %v", findings)
	}
	if !strings.Contains(f.Detail, "old pricing format") {
		t.Errorf("finding must still identify the old format; got %q", f.Detail)
	}
	if len(invalidEntryFindings(findings)) != 1 {
		t.Errorf("want exactly the one pre-existing finding; got %v", findings)
	}
}

func TestStore_Put_ReplacesAnInvalidEntryOfTheSameModelEntirely(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, `models:
  old-model:
    input: "3.00"
    cached_input: "0.30"
    cache_write: "3.75"
    output_under_threshold: "15.00"
`)
	store := pricing.NewStore(t.TempDir(), root)
	entry := newMinimalEntry(t, "old-model")

	table, findings := putAndLoad(t, store, entry)

	if got, found := table.Lookup("old-model"); !found || got != asProject(entry) {
		t.Errorf("replaced entry must load as written; found=%v got=%+v", found, got)
	}
	if len(findings) != 0 {
		t.Errorf("no old keys may survive the replacement; findings: %v", findings)
	}
	for _, key := range oldFormatKeys {
		for _, line := range activeLines(readFile(t, path)) {
			if mentionsKey(line, key) {
				t.Errorf("old key %q survived in %q", key, line)
			}
		}
	}
}

func TestStore_Put_ReplacingLongContextEntryWithPlainEntry_DropsTheBlock(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedTwoModels)
	store := pricing.NewStore(t.TempDir(), root)
	plain := newMinimalEntry(t, "test-model-b")

	table, _ := putAndLoad(t, store, plain)

	got, _ := table.Lookup("test-model-b")
	if got != asProject(plain) {
		t.Errorf("replacement = %+v, want %+v (no long-context tier)", got, plain)
	}
}

func TestStore_Put_AbsentFile_CreatesFileAndParentDirectories(t *testing.T) {
	store := pricing.NewStore(t.TempDir(), t.TempDir())

	table, _ := putAndLoad(t, store, newMinimalEntry(t, "create-test-model"))

	if _, err := os.Stat(store.Path()); err != nil {
		t.Errorf("Put must create the pricing file: %v", err)
	}
	if _, found := table.Lookup("create-test-model"); !found {
		t.Error("entry written by Put must be loadable after file creation")
	}
}

func TestStore_Put_ExistingFileWithComments_RawFileRetainsLeadingComments(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, "# user note: these are our production prices\n"+wellFormedOneModel)
	store := pricing.NewStore(t.TempDir(), root)

	putAndLoad(t, store, newMinimalEntry(t, "new-model"))

	const want = "# user note: these are our production prices"
	if !strings.Contains(readFile(t, path), want) {
		t.Errorf("Put must preserve the leading comment %q", want)
	}
}

func TestStore_Put_UnparseableDocument_ReturnsErrorAndLeavesFileUnchanged(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, malformedYAML)
	store := pricing.NewStore(t.TempDir(), root)

	_, err := store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "any-model"))

	if err == nil {
		t.Fatal("Put into an unparseable document must return an error")
	}
	if got := readFile(t, path); got != malformedYAML {
		t.Errorf("the file must be left unchanged; got %q", got)
	}
}

func TestStore_Path_EndsWithConfigRelPathUnderRoot(t *testing.T) {
	root := t.TempDir()
	store := pricing.NewStore(t.TempDir(), root)

	got := strings.ReplaceAll(store.Path(), "\\", "/")
	if !strings.HasSuffix(got, pricing.ConfigRelPath) {
		t.Errorf("Path() = %q must end with %q", got, pricing.ConfigRelPath)
	}
	if !strings.HasPrefix(store.Path(), root) {
		t.Errorf("Path() = %q must be under the root %q", store.Path(), root)
	}
}
