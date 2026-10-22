package domain_test

// Tests for pricing-entry validation semantics.
//
// Design invariants under test:
//   - An entry without a long-context block has no long-context tier.
//   - An entry with a long-context block exposes a long-context tier.
//   - Standard rates are all required (> 0); a long-context block additionally
//     needs a positive threshold and its own five positive rates.
//   - Validation errors name the model and the offending wire field.
//   - Lookup on a model absent from the table returns found=false (the explicit
//     not-configured outcome), never a zero-valued entry.
//   - PricingTable.With returns a copy; the original is not mutated.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// validRates returns a full rate set at the given multiple of the base rates.
func validRates(mult int64) domain.RateSet {
	return domain.RateSet{
		Input:        domain.Rate(3_000_000_000 * mult),
		CacheRead:    domain.Rate(300_000_000 * mult),
		CacheWrite5m: domain.Rate(3_750_000_000 * mult),
		CacheWrite1h: domain.Rate(6_000_000_000 * mult),
		Output:       domain.Rate(15_000_000_000 * mult),
	}
}

// validEntry returns an entry with all five standard rates and no long-context tier.
func validEntry(model domain.ModelID) domain.ModelPricing {
	return domain.ModelPricing{Model: model, Standard: validRates(1)}
}

// validEntryWithThreshold returns a valid entry with a long-context tier.
func validEntryWithThreshold(model domain.ModelID) domain.ModelPricing {
	e := validEntry(model)
	e.HasLongContext = true
	e.LongContext = domain.LongContextPricing{Threshold: 200_000, Rates: validRates(2)}
	return e
}

func TestModelPricing_HasLongContextTier_False_WhenNoLongContext(t *testing.T) {
	if validEntry("model-a").HasLongContextTier() {
		t.Error("HasLongContextTier must be false when HasLongContext=false")
	}
}

func TestModelPricing_HasLongContextTier_True_WhenLongContextPresent(t *testing.T) {
	if !validEntryWithThreshold("model-a").HasLongContextTier() {
		t.Error("HasLongContextTier must be true when HasLongContext=true")
	}
}

func TestModelPricing_Validate_ValidWithoutLongContext(t *testing.T) {
	if err := validEntry("model-a").Validate(); err != nil {
		t.Errorf("valid entry without long context must pass Validate: %v", err)
	}
}

func TestModelPricing_Validate_ValidWithLongContext(t *testing.T) {
	if err := validEntryWithThreshold("model-a").Validate(); err != nil {
		t.Errorf("valid entry with long context must pass Validate: %v", err)
	}
}

func TestModelPricing_Validate_EmptyModelID_IsInvalid(t *testing.T) {
	if err := validEntry("").Validate(); err == nil {
		t.Error("entry with empty ModelID must fail Validate")
	}
}

func TestModelPricing_Validate_EachStandardRate_MissingIsInvalid(t *testing.T) {
	cases := []struct {
		field  string
		adjust func(*domain.ModelPricing)
	}{
		{"input", func(p *domain.ModelPricing) { p.Standard.Input = 0 }},
		{"cache_read", func(p *domain.ModelPricing) { p.Standard.CacheRead = 0 }},
		{"cache_write_5m", func(p *domain.ModelPricing) { p.Standard.CacheWrite5m = 0 }},
		{"cache_write_1h", func(p *domain.ModelPricing) { p.Standard.CacheWrite1h = 0 }},
		{"output", func(p *domain.ModelPricing) { p.Standard.Output = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			p := validEntry("model-a")
			tc.adjust(&p)
			err := p.Validate()
			if err == nil {
				t.Fatalf("entry with zero %s must fail Validate", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error %q must name the field %q", err, tc.field)
			}
			if !strings.Contains(err.Error(), "model-a") {
				t.Errorf("error %q must name the model", err)
			}
		})
	}
}

func TestModelPricing_Validate_LongContext_EachRateMissingIsInvalid(t *testing.T) {
	cases := []struct {
		field  string
		adjust func(*domain.ModelPricing)
	}{
		{"long_context.input", func(p *domain.ModelPricing) { p.LongContext.Rates.Input = 0 }},
		{"long_context.cache_read", func(p *domain.ModelPricing) { p.LongContext.Rates.CacheRead = 0 }},
		{"long_context.cache_write_5m", func(p *domain.ModelPricing) { p.LongContext.Rates.CacheWrite5m = 0 }},
		{"long_context.cache_write_1h", func(p *domain.ModelPricing) { p.LongContext.Rates.CacheWrite1h = 0 }},
		{"long_context.output", func(p *domain.ModelPricing) { p.LongContext.Rates.Output = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			p := validEntryWithThreshold("model-a")
			tc.adjust(&p)
			err := p.Validate()
			if err == nil {
				t.Fatalf("entry with zero %s must fail Validate", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error %q must name the field %q", err, tc.field)
			}
		})
	}
}

func TestModelPricing_Validate_LongContext_NonPositiveThresholdIsInvalid(t *testing.T) {
	for _, threshold := range []int64{0, -1} {
		p := validEntryWithThreshold("model-a")
		p.LongContext.Threshold = threshold
		err := p.Validate()
		if err == nil {
			t.Fatalf("threshold %d must fail Validate", threshold)
		}
		if !strings.Contains(err.Error(), "long_context.threshold") {
			t.Errorf("error %q must name long_context.threshold", err)
		}
	}
}

func TestModelPricing_Validate_IgnoresLongContextRates_WhenFlagFalse(t *testing.T) {
	p := validEntry("model-a") // HasLongContext false, long-context data zero
	p.LongContext = domain.LongContextPricing{}
	if err := p.Validate(); err != nil {
		t.Errorf("an unused long-context block must not affect validation: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PricingTable — Lookup not-configured outcome
// ---------------------------------------------------------------------------

func TestPricingTable_Lookup_Empty_IsNotConfigured(t *testing.T) {
	table := domain.EmptyPricingTable()
	_, found := table.Lookup("any-model")
	if found {
		t.Error("Lookup on empty table must return found=false (explicit not-configured outcome)")
	}
}

func TestPricingTable_Lookup_AbsentModel_IsNotConfigured(t *testing.T) {
	// Table has one model; lookup for a different one must return not-configured.
	table := domain.NewPricingTable([]domain.ModelPricing{validEntry("model-a")})
	_, found := table.Lookup("model-b")
	if found {
		t.Error("Lookup for absent model must return found=false, not a zero-valued entry")
	}
}

func TestPricingTable_Lookup_PresentModel_IsFound(t *testing.T) {
	entry := validEntry("model-a")
	table := domain.NewPricingTable([]domain.ModelPricing{entry})
	got, found := table.Lookup("model-a")
	if !found {
		t.Fatal("Lookup must find a model that was added to the table")
	}
	if got.Model != "model-a" {
		t.Errorf("Lookup returned wrong model: %q", got.Model)
	}
}

func TestPricingTable_Lookup_ReturnsCorrectEntry(t *testing.T) {
	entryA := validEntry("model-a")
	entryB := validEntryWithThreshold("model-b")
	table := domain.NewPricingTable([]domain.ModelPricing{entryA, entryB})

	gotA, foundA := table.Lookup("model-a")
	if !foundA {
		t.Fatal("Lookup(model-a) not found")
	}
	if gotA.HasLongContext {
		t.Error("model-a must not have a long-context tier")
	}

	gotB, foundB := table.Lookup("model-b")
	if !foundB {
		t.Fatal("Lookup(model-b) not found")
	}
	if !gotB.HasLongContext {
		t.Error("model-b must have a long-context tier")
	}
	if gotB.LongContext.Threshold != 200_000 {
		t.Errorf("model-b long-context threshold = %d, want 200000", gotB.LongContext.Threshold)
	}
}

// ---------------------------------------------------------------------------
// PricingTable — structural properties
// ---------------------------------------------------------------------------

func TestPricingTable_Len_Empty(t *testing.T) {
	if got := domain.EmptyPricingTable().Len(); got != 0 {
		t.Errorf("EmptyPricingTable.Len() = %d, want 0", got)
	}
}

func TestPricingTable_Len_AfterInsert(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{
		validEntry("m1"),
		validEntry("m2"),
	})
	if got := table.Len(); got != 2 {
		t.Errorf("table with 2 entries Len() = %d, want 2", got)
	}
}

func TestPricingTable_Models_Sorted(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{
		validEntry("zzz-model"),
		validEntry("aaa-model"),
		validEntry("mmm-model"),
	})
	models := table.Models()
	if len(models) != 3 {
		t.Fatalf("Models() len = %d, want 3", len(models))
	}
	if models[0] != "aaa-model" || models[1] != "mmm-model" || models[2] != "zzz-model" {
		t.Errorf("Models() not sorted ascending: %v", models)
	}
}

func TestPricingTable_Models_Empty(t *testing.T) {
	models := domain.EmptyPricingTable().Models()
	if len(models) != 0 {
		t.Errorf("EmptyPricingTable.Models() len = %d, want 0", len(models))
	}
}

// ---------------------------------------------------------------------------
// PricingTable.With — immutability contract
// ---------------------------------------------------------------------------

func TestPricingTable_With_AddsEntry(t *testing.T) {
	entry := validEntry("model-a")
	table := domain.EmptyPricingTable().With(entry)
	_, found := table.Lookup("model-a")
	if !found {
		t.Error("With() must make the entry findable in the returned table")
	}
}

func TestPricingTable_With_DoesNotMutateOriginal(t *testing.T) {
	original := domain.EmptyPricingTable()
	_ = original.With(validEntry("model-a"))
	// The original must still be empty.
	if _, found := original.Lookup("model-a"); found {
		t.Error("With() must return a copy; the original table must not be mutated")
	}
	if original.Len() != 0 {
		t.Errorf("original.Len() = %d after With(), want 0", original.Len())
	}
}

func TestPricingTable_With_ReplacesExistingEntry(t *testing.T) {
	original := validEntry("model-a")
	updated := validEntryWithThreshold("model-a")

	table := domain.NewPricingTable([]domain.ModelPricing{original}).With(updated)
	got, found := table.Lookup("model-a")
	if !found {
		t.Fatal("model-a must still be findable after replacement")
	}
	if !got.HasLongContext {
		t.Error("With() must replace the entry; updated entry must have HasLongContext=true")
	}
	if table.Len() != 1 {
		t.Errorf("Len() after replacement = %d, want 1 (no duplicate)", table.Len())
	}
}

func TestPricingTable_With_PreservesOtherEntries(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{
		validEntry("model-a"),
		validEntry("model-b"),
	}).With(validEntryWithThreshold("model-c"))

	for _, id := range []domain.ModelID{"model-a", "model-b", "model-c"} {
		if _, found := table.Lookup(id); !found {
			t.Errorf("With() must preserve existing entry %q", id)
		}
	}
	if table.Len() != 3 {
		t.Errorf("Len() = %d, want 3 after adding one new entry", table.Len())
	}
}
