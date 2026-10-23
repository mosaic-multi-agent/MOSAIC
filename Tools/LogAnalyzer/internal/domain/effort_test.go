package domain_test

// Tests for CountEffort and EffortLevels: requests are counted once under their
// verbatim level, ordered by record count then level, and labelled for display.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func effortUsages(levels ...string) []domain.InvocationUsage {
	out := make([]domain.InvocationUsage, len(levels))
	for i, l := range levels {
		out[i] = domain.InvocationUsage{Model: "m", Effort: l}
	}
	return out
}

func TestCountEffort_NoUsages_IsEmptyWithEmptyLabel(t *testing.T) {
	got := domain.CountEffort()

	if !got.IsEmpty() || got.Label() != "" {
		t.Errorf("CountEffort() = %v (empty %v, label %q), want empty with empty label", got, got.IsEmpty(), got.Label())
	}
}

func TestCountEffort_RequestsWithoutEffort_AreNotCounted(t *testing.T) {
	got := domain.CountEffort(effortUsages("", "", "")...)

	if !got.IsEmpty() {
		t.Errorf("CountEffort over requests without effort = %v, want empty", got)
	}
}

func TestCountEffort_SingleLevel_LabelIsTheBareLevel(t *testing.T) {
	got := domain.CountEffort(effortUsages("high", "high", "high")...)

	if len(got) != 1 || got[0].Level != "high" || got[0].Records != 3 {
		t.Fatalf("CountEffort = %v, want [{high 3}]", got)
	}
	if got.IsEmpty() {
		t.Error("IsEmpty() = true for counted levels")
	}
	if got.Label() != "high" {
		t.Errorf("Label() = %q, want %q", got.Label(), "high")
	}
}

func TestCountEffort_MixedLevels_OrderedByRecordsDescending(t *testing.T) {
	got := domain.CountEffort(effortUsages("high", "medium", "medium", "high", "medium")...)

	if want := "medium (3), high (2)"; got.Label() != want {
		t.Errorf("Label() = %q, want %q", got.Label(), want)
	}
}

func TestCountEffort_EqualCounts_OrderedByLevelAscending(t *testing.T) {
	got := domain.CountEffort(effortUsages("medium", "low", "high")...)

	if want := "high (1), low (1), medium (1)"; got.Label() != want {
		t.Errorf("Label() = %q, want %q", got.Label(), want)
	}
}

func TestCountEffort_UnknownLevel_IsKeptVerbatim(t *testing.T) {
	got := domain.CountEffort(effortUsages("XHigh-Custom", "")...)

	if got.Label() != "XHigh-Custom" {
		t.Errorf("Label() = %q, want the level verbatim", got.Label())
	}
}

func TestCountEffort_MixOfLevelAndMissing_CountsOnlyTheLevelled(t *testing.T) {
	got := domain.CountEffort(effortUsages("high", "", "low", "")...)

	if want := "high (1), low (1)"; got.Label() != want {
		t.Errorf("Label() = %q, want %q", got.Label(), want)
	}
}
