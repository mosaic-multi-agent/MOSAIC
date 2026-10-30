package domain_test

// Tests for the cache-write tier list and the per-harness default tier.
//
// Rules under test:
//   - The allowed tiers are the cache-write tiers of the log format, from one
//     list: 5m and 1h, in that order.
//   - Parsing accepts exactly those tier names.
//   - A harness without a configured tier uses the 5m default; a configured
//     harness uses its tier and leaves the other harnesses on the default.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestCacheWriteTiers_ListsTheLogFormatTiersInOrder(t *testing.T) {
	want := []domain.CacheWriteTier{domain.CacheWriteTier5m, domain.CacheWriteTier1h}

	got := domain.CacheWriteTiers()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("CacheWriteTiers() = %v, want %v", got, want)
	}
}

func TestParseCacheWriteTier_EveryListedTierParsesToItself(t *testing.T) {
	tiers := domain.CacheWriteTiers()
	if len(tiers) == 0 {
		t.Fatal("CacheWriteTiers() is empty")
	}
	for _, tier := range tiers {
		got, ok := domain.ParseCacheWriteTier(string(tier))

		if !ok || got != tier {
			t.Errorf("ParseCacheWriteTier(%q) = %q, %v; want %q, true", tier, got, ok, tier)
		}
	}
}

func TestParseCacheWriteTier_ValuesOutsideTheListAreRejected(t *testing.T) {
	for _, s := range []string{"", "2h", "1H", "5M", " 1h", "1h ", "60m", "true", "0"} {
		if got, ok := domain.ParseCacheWriteTier(s); ok {
			t.Errorf("ParseCacheWriteTier(%q) = %q, true; want rejected", s, got)
		}
	}
}

func TestDefaultCacheWriteTier_IsFiveMinutes(t *testing.T) {
	if domain.DefaultCacheWriteTier != domain.CacheWriteTier5m {
		t.Errorf("DefaultCacheWriteTier = %q, want 5m", domain.DefaultCacheWriteTier)
	}
}

func TestCacheWriteTierFor_ConfiguredHarnessUsesItsTier(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	s.CacheWriteTiers = map[domain.Harness]domain.CacheWriteTier{
		domain.HarnessClaudeCode: domain.CacheWriteTier1h,
	}

	if got := s.CacheWriteTierFor(domain.HarnessClaudeCode); got != domain.CacheWriteTier1h {
		t.Errorf("CacheWriteTierFor(claude-code) = %q, want 1h", got)
	}
}

func TestCacheWriteTierFor_OtherHarnessesKeepTheDefault(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	s.CacheWriteTiers = map[domain.Harness]domain.CacheWriteTier{
		domain.HarnessClaudeCode: domain.CacheWriteTier1h,
	}

	for _, h := range []domain.Harness{domain.HarnessOpenCode, domain.HarnessVSCodeGHCP, domain.HarnessGHCPCLI, ""} {
		if got := s.CacheWriteTierFor(h); got != domain.CacheWriteTier5m {
			t.Errorf("CacheWriteTierFor(%q) = %q, want the 5m default", h, got)
		}
	}
}

func TestCacheWriteTierFor_BuiltInSettingsGiveFiveMinutesForEveryHarness(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	for _, h := range []domain.Harness{
		domain.HarnessClaudeCode, domain.HarnessOpenCode, domain.HarnessVSCodeGHCP, domain.HarnessGHCPCLI,
	} {
		if got := s.CacheWriteTierFor(h); got != domain.CacheWriteTier5m {
			t.Errorf("CacheWriteTierFor(%q) = %q, want 5m", h, got)
		}
	}
}
