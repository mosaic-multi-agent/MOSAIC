package domain_test

// Tests for the token category display vocabulary: reasoning is shown as part
// of output in breakdowns but is never one of the billable categories.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestTokenCategoryLabel_Reasoning(t *testing.T) {
	if got := domain.CategoryReasoning.Label(); got != "reasoning" {
		t.Errorf("Label() = %q, want %q", got, "reasoning")
	}
}

func TestTokenCategoryIsBillable_OnlyTheFourBillableCategories(t *testing.T) {
	for _, c := range domain.BillableCategories() {
		if !c.IsBillable() {
			t.Errorf("%q must be billable", c)
		}
	}
	if domain.CategoryReasoning.IsBillable() {
		t.Error("reasoning must not be billable")
	}
}

func TestBreakdownCategories_BillableOnesThenReasoning(t *testing.T) {
	got := domain.BreakdownCategories()

	want := append(domain.BillableCategories(), domain.CategoryReasoning)
	if len(got) != len(want) {
		t.Fatalf("BreakdownCategories() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("BreakdownCategories() = %v, want %v", got, want)
		}
	}
}

func TestTokenUsageGet_Reasoning_ReturnsReasoningCount(t *testing.T) {
	u := domain.TokenUsage{Output: domain.Tokens(100), Reasoning: domain.Tokens(40)}

	got, ok := u.Get(domain.CategoryReasoning)

	if v, present := got.Value(); !ok || !present || v != 40 {
		t.Errorf("Get(reasoning) = (%d, present=%v), ok=%v; want present 40, ok", v, present, ok)
	}
}

func TestTokenUsageGet_Reasoning_AbsentStaysAbsent(t *testing.T) {
	u := domain.TokenUsage{Output: domain.Tokens(100)}

	got, ok := u.Get(domain.CategoryReasoning)

	if !ok {
		t.Error("reasoning must be a recognised category")
	}
	if got.IsPresent() {
		t.Errorf("Get(reasoning) = %v, want absent", got)
	}
}
