package domain_test

// Tests for the context bands: BandOf boundaries, the band names as text, and
// the band ranges.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestBandOf_BoundariesMapToTheStatedBands(t *testing.T) {
	cases := []struct {
		tokens int64
		want   domain.ContextBand
	}{
		{0, domain.BandBelowOptimal},
		{99_999, domain.BandBelowOptimal},
		{100_000, domain.BandOptimal},
		{249_999, domain.BandOptimal},
		{250_000, domain.BandAboveOptimal},
		{349_999, domain.BandAboveOptimal},
		{350_000, domain.BandDegraded},
		{5_000_000, domain.BandDegraded},
	}
	for _, tc := range cases {
		if got := domain.BandOf(tc.tokens); got != tc.want {
			t.Errorf("BandOf(%d) = %v, want %v", tc.tokens, got, tc.want)
		}
	}
}

func TestContextBand_NamesAreAvailableAsText(t *testing.T) {
	cases := []struct {
		band       domain.ContextBand
		label, key string
	}{
		{domain.BandBelowOptimal, "below optimal", "below_optimal"},
		{domain.BandOptimal, "optimal", "optimal"},
		{domain.BandAboveOptimal, "above optimal", "above_optimal"},
		{domain.BandDegraded, "degraded", "degraded"},
	}
	for _, tc := range cases {
		if got := tc.band.Label(); got != tc.label {
			t.Errorf("Label(%d) = %q, want %q", tc.band, got, tc.label)
		}
		if got := tc.band.Key(); got != tc.key {
			t.Errorf("Key(%d) = %q, want %q", tc.band, got, tc.key)
		}
	}
}

func TestContextBandRanges_ListTheFourBandsInOrderWithAnOpenEnd(t *testing.T) {
	got := domain.ContextBandRanges()

	want := []domain.ContextBandRange{
		{Band: domain.BandBelowOptimal, From: 0, To: 100_000},
		{Band: domain.BandOptimal, From: 100_000, To: 250_000},
		{Band: domain.BandAboveOptimal, From: 250_000, To: 350_000},
		{Band: domain.BandDegraded, From: 350_000, To: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("ranges = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("range %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRunContext_Find_ReturnsTheInvocationById(t *testing.T) {
	rc := domain.RunContext{Invocations: []domain.ActorContext{
		{Actor: domain.AgentInstance("a#1")},
		{Actor: domain.AgentInstance("b#2"), Peak: domain.ContextFigure{Tokens: domain.Tokens(7)}},
	}}

	got, ok := rc.Find("b#2")

	if !ok {
		t.Fatal("Find(b#2) not found")
	}
	if v, _ := got.Peak.Tokens.Value(); v != 7 {
		t.Errorf("found the wrong invocation: peak = %d", v)
	}
	if _, ok := rc.Find("missing#9"); ok {
		t.Error("Find(missing#9) found an invocation, want none")
	}
}

func TestActorContext_Counts_SplitLoggedFromDetectedMarks(t *testing.T) {
	a := domain.ActorContext{Compactions: []domain.CompactionMark{
		{Kind: domain.CompactionLogged},
		{Kind: domain.CompactionDetected},
		{Kind: domain.CompactionDetected},
	}}

	if a.LoggedCount() != 1 || a.DetectedCount() != 2 {
		t.Errorf("logged = %d, detected = %d; want 1 and 2", a.LoggedCount(), a.DetectedCount())
	}
}
