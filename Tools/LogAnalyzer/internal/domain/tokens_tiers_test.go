package domain_test

// Tests for the cache-write tier split carried on TokenUsage.
//
// Rules under test:
//   - HasCacheWriteSplit is true only when both tier fields are present.
//   - Adding or summing usages adds the tier fields with the same
//     absent-preserving rule as the four categories; a tier is never invented
//     for a summand that carried no split.
//   - UnsplitCacheCreation is the part of the cache-write total not covered by
//     the split: the whole total without a split, zero with a full split, absent
//     when there is no cache-write total, and never negative.
//   - The four billable categories (IsEmpty, TotalContext, Get,
//     BillableCategories) are unaffected by the tier fields.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func splitUsage(total, fiveMin, oneHour int64) domain.TokenUsage {
	return domain.TokenUsage{
		CacheCreation:   domain.Tokens(total),
		CacheCreation5m: domain.Tokens(fiveMin),
		CacheCreation1h: domain.Tokens(oneHour),
	}
}

func totalOnlyUsage(total int64) domain.TokenUsage {
	return domain.TokenUsage{CacheCreation: domain.Tokens(total)}
}

func wantTokens(t *testing.T, name string, got domain.TokenCount, want int64) {
	t.Helper()
	v, ok := got.Value()
	if !ok || v != want {
		t.Errorf("%s = (%d, present=%v), want present %d", name, v, ok, want)
	}
}

func wantAbsent(t *testing.T, name string, got domain.TokenCount) {
	t.Helper()
	if v, ok := got.Value(); ok {
		t.Errorf("%s = %d, want absent", name, v)
	}
}

func TestTokenUsage_HasCacheWriteSplit(t *testing.T) {
	cases := []struct {
		name string
		u    domain.TokenUsage
		want bool
	}{
		{"both tiers present", splitUsage(150, 100, 50), true},
		{"both tiers present and zero", splitUsage(0, 0, 0), true},
		{"neither tier present", totalOnlyUsage(150), false},
		{"only 5m present", domain.TokenUsage{CacheCreation: domain.Tokens(5), CacheCreation5m: domain.Tokens(5)}, false},
		{"only 1h present", domain.TokenUsage{CacheCreation: domain.Tokens(5), CacheCreation1h: domain.Tokens(5)}, false},
		{"empty usage", domain.TokenUsage{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.u.HasCacheWriteSplit(); got != c.want {
				t.Errorf("HasCacheWriteSplit() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTokenUsage_UnsplitCacheCreation(t *testing.T) {
	t.Run("no split returns the whole total", func(t *testing.T) {
		wantTokens(t, "unsplit", totalOnlyUsage(150).UnsplitCacheCreation(), 150)
	})
	t.Run("full split leaves present zero", func(t *testing.T) {
		wantTokens(t, "unsplit", splitUsage(150, 100, 50).UnsplitCacheCreation(), 0)
	})
	t.Run("absent total is absent", func(t *testing.T) {
		wantAbsent(t, "unsplit", domain.TokenUsage{Input: domain.Tokens(5)}.UnsplitCacheCreation())
	})
	t.Run("tiers larger than total clamp to zero", func(t *testing.T) {
		wantTokens(t, "unsplit", splitUsage(120, 100, 50).UnsplitCacheCreation(), 0)
	})
}

func TestSumUsage_TierFields_AddedPerFieldWhenBothSummandsSplit(t *testing.T) {
	sum := domain.SumUsage(splitUsage(150, 100, 50), splitUsage(30, 10, 20))

	wantTokens(t, "CacheCreation", sum.CacheCreation, 180)
	wantTokens(t, "CacheCreation5m", sum.CacheCreation5m, 110)
	wantTokens(t, "CacheCreation1h", sum.CacheCreation1h, 70)
	if !sum.HasCacheWriteSplit() {
		t.Error("HasCacheWriteSplit() = false, want true")
	}
	wantTokens(t, "unsplit", sum.UnsplitCacheCreation(), 0)
}

func TestSumUsage_TierFields_TotalOnlySummandIsNotGivenATier(t *testing.T) {
	sum := domain.SumUsage(splitUsage(150, 100, 50), totalOnlyUsage(40))

	wantTokens(t, "CacheCreation", sum.CacheCreation, 190)
	wantTokens(t, "CacheCreation5m", sum.CacheCreation5m, 100)
	wantTokens(t, "CacheCreation1h", sum.CacheCreation1h, 50)
	wantTokens(t, "unsplit", sum.UnsplitCacheCreation(), 40)
}

func TestSumUsage_TierFields_AllTotalOnlyStaysAbsent(t *testing.T) {
	sum := domain.SumUsage(totalOnlyUsage(40), totalOnlyUsage(60))

	wantTokens(t, "CacheCreation", sum.CacheCreation, 100)
	wantAbsent(t, "CacheCreation5m", sum.CacheCreation5m)
	wantAbsent(t, "CacheCreation1h", sum.CacheCreation1h)
	if sum.HasCacheWriteSplit() {
		t.Error("HasCacheWriteSplit() = true, want false")
	}
	wantTokens(t, "unsplit", sum.UnsplitCacheCreation(), 100)
}

func TestTokenUsage_Add_TierFields_SplitWithEmptyKeepsSplit(t *testing.T) {
	got := splitUsage(150, 100, 50).Add(domain.TokenUsage{})

	wantTokens(t, "CacheCreation5m", got.CacheCreation5m, 100)
	wantTokens(t, "CacheCreation1h", got.CacheCreation1h, 50)
}

func TestTokenUsage_FourCategoryBehaviourIgnoresTierFields(t *testing.T) {
	tiersOnly := domain.TokenUsage{CacheCreation5m: domain.Tokens(5), CacheCreation1h: domain.Tokens(5)}
	if !tiersOnly.IsEmpty() {
		t.Error("IsEmpty() = false for a usage with only tier fields, want true")
	}

	u := splitUsage(150, 100, 50)
	u.Input = domain.Tokens(10)
	wantTokens(t, "TotalContext", u.TotalContext(), 160)

	cats := domain.BillableCategories()
	want := []domain.TokenCategory{
		domain.CategoryInput, domain.CategoryCacheRead, domain.CategoryCacheCreation, domain.CategoryOutput,
	}
	if len(cats) != len(want) {
		t.Fatalf("BillableCategories() = %v, want %v", cats, want)
	}
	for i := range want {
		if cats[i] != want[i] {
			t.Errorf("BillableCategories()[%d] = %q, want %q", i, cats[i], want[i])
		}
	}
	got, ok := u.Get(domain.CategoryCacheCreation)
	if !ok {
		t.Fatal("Get(CategoryCacheCreation) not recognised")
	}
	wantTokens(t, "Get(cache_creation)", got, 150)
}
