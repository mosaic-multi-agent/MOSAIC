package domain_test

// Tests for the Reasoning count on TokenUsage: it follows the same
// absent-versus-zero arithmetic as every other count and never counts as a
// billable category.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func reasoningUsage(c domain.TokenCount) domain.TokenUsage {
	return domain.TokenUsage{Output: domain.Tokens(100), Reasoning: c}
}

func TestTokenUsageAdd_Reasoning_PresentPlusPresent_Sums(t *testing.T) {
	got := reasoningUsage(domain.Tokens(10)).Add(reasoningUsage(domain.Tokens(15)))

	if v, ok := got.Reasoning.Value(); !ok || v != 25 {
		t.Errorf("Reasoning = (%d, present=%v), want present 25", v, ok)
	}
}

func TestTokenUsageAdd_Reasoning_PresentPlusAbsent_KeepsPresent(t *testing.T) {
	got := reasoningUsage(domain.Tokens(10)).Add(reasoningUsage(domain.AbsentTokens()))

	if v, ok := got.Reasoning.Value(); !ok || v != 10 {
		t.Errorf("Reasoning = (%d, present=%v), want present 10", v, ok)
	}
}

func TestTokenUsageAdd_Reasoning_AbsentPlusAbsent_StaysAbsent(t *testing.T) {
	got := reasoningUsage(domain.AbsentTokens()).Add(reasoningUsage(domain.AbsentTokens()))

	if got.Reasoning.IsPresent() {
		t.Errorf("Reasoning must stay absent, got %v", got.Reasoning)
	}
}

func TestTokenUsageAdd_Reasoning_ZeroPlusZero_StaysPresentZero(t *testing.T) {
	got := reasoningUsage(domain.Tokens(0)).Add(reasoningUsage(domain.Tokens(0)))

	if v, ok := got.Reasoning.Value(); !ok || v != 0 {
		t.Errorf("Reasoning = (%d, present=%v), want present zero", v, ok)
	}
}

func TestSumUsage_Reasoning_FoldsAbsentPreserving(t *testing.T) {
	got := domain.SumUsage(
		reasoningUsage(domain.AbsentTokens()),
		reasoningUsage(domain.Tokens(4)),
		reasoningUsage(domain.Tokens(0)),
		reasoningUsage(domain.Tokens(6)),
	)

	if v, ok := got.Reasoning.Value(); !ok || v != 10 {
		t.Errorf("Reasoning = (%d, present=%v), want present 10", v, ok)
	}
	if v, _ := got.Output.Value(); v != 400 {
		t.Errorf("Output = %d, want 400 (reasoning is not added into output by Add)", v)
	}
}

func TestTokenUsageIsEmpty_ReasoningAlone_StillEmpty(t *testing.T) {
	u := domain.TokenUsage{Reasoning: domain.Tokens(5)}

	if !u.IsEmpty() {
		t.Error("IsEmpty must look only at the four billable categories")
	}
}

func TestTokenUsageTotalContext_IgnoresReasoning(t *testing.T) {
	u := domain.TokenUsage{
		Input:         domain.Tokens(10),
		CacheRead:     domain.Tokens(20),
		CacheCreation: domain.Tokens(30),
		Output:        domain.Tokens(40),
		Reasoning:     domain.Tokens(1000),
	}

	got := u.TotalContext()

	if v, ok := got.Value(); !ok || v != 60 {
		t.Errorf("TotalContext = (%d, present=%v), want present 60 (reasoning excluded)", v, ok)
	}
}

func TestBillableCategories_ExcludeReasoning(t *testing.T) {
	u := domain.TokenUsage{Reasoning: domain.Tokens(7)}

	for _, c := range domain.BillableCategories() {
		if v, _ := u.Get(c); v.IsPresent() {
			t.Errorf("category %q reads the reasoning count; reasoning must not be billable", c)
		}
	}
	if len(domain.BillableCategories()) != 4 {
		t.Errorf("BillableCategories() has %d entries, want 4", len(domain.BillableCategories()))
	}
}
