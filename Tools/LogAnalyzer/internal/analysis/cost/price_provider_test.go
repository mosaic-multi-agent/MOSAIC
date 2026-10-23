package cost_test

// Tests for provider-keyed request pricing.
//
// Rules under test:
//   - A request with a provider is priced only from the entry of that provider
//     and model.
//   - A request without a provider is priced only from the entry that has no
//     provider (top-level models).
//   - There is no fallback between the two; a request that matches nothing is
//     unpriced.
//   - Tier selection and the 5m fallback for unsplit cache writes work the same
//     under a provider.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

const (
	providerAnthropic = domain.ProviderID("anthropic")
	providerCopilot   = domain.ProviderID("github-copilot")
)

// entryWithInput returns a flat entry for (provider, model) whose input rate is
// the given value; every other rate is the same across entries.
func entryWithInput(t *testing.T, provider domain.ProviderID, model domain.ModelID, input string) domain.ModelPricing {
	t.Helper()
	p := flatPricing(t, model)
	p.Provider = provider
	p.Standard.Input = mustParseRate(t, input)
	return p
}

// oneMillionInput prices one request of 1M input tokens for (provider, model).
func oneMillionInput(provider domain.ProviderID, model domain.ModelID, table domain.PricingTable) cost.InvocationCost {
	return cost.PriceInvocation(domain.InvocationUsage{
		Model:    model,
		Provider: provider,
		Harness:  domain.HarnessClaudeCode,
		Usage:    domain.TokenUsage{Input: domain.Tokens(1_000_000)},
	}, table)
}

func TestPriceInvocation_SameRunTwoProviders_PricedFromTwoDifferentEntries(t *testing.T) {
	table := testTable(
		entryWithInput(t, providerCopilot, "claude-sonnet-5-5", "3.00"),
		entryWithInput(t, providerAnthropic, "claude-sonnet-5-5", "5.00"),
	)

	// Same model ID on both requests: only the provider tells the entries apart.
	viaCopilot := oneMillionInput(providerCopilot, "claude-sonnet-5-5", table)
	viaAnthropic := oneMillionInput(providerAnthropic, "claude-sonnet-5-5", table)

	if !viaCopilot.Priced || !viaAnthropic.Priced {
		t.Fatalf("both requests must be priced, got copilot=%v anthropic=%v", viaCopilot.Priced, viaAnthropic.Priced)
	}
	assertKnownMoney(t, "copilot input", viaCopilot.Money.Input, 3_000_000_000)
	assertKnownMoney(t, "anthropic input", viaAnthropic.Money.Input, 5_000_000_000)
}

func TestPriceInvocation_SameModelUnderTwoProviders_EachUsesItsOwnEntry(t *testing.T) {
	table := testTable(
		entryWithInput(t, providerAnthropic, "shared-model", "5.00"),
		entryWithInput(t, providerCopilot, "shared-model", "3.00"),
	)

	viaAnthropic := oneMillionInput(providerAnthropic, "shared-model", table)
	viaCopilot := oneMillionInput(providerCopilot, "shared-model", table)

	assertKnownMoney(t, "anthropic input", viaAnthropic.Money.Input, 5_000_000_000)
	assertKnownMoney(t, "copilot input", viaCopilot.Money.Input, 3_000_000_000)
}

func TestPriceInvocation_ProviderRecordWithOnlyOtherProviderEntry_Unpriced(t *testing.T) {
	table := testTable(entryWithInput(t, providerAnthropic, "shared-model", "5.00"))

	ic := oneMillionInput(providerCopilot, "shared-model", table)

	if ic.Priced {
		t.Fatal("a github-copilot record must not be priced from an anthropic entry")
	}
	assertUnpriced(t, "Input", ic.Money.Input)
	assertUnpriced(t, "Total", ic.Money.Total)
}

func TestPriceInvocation_ProviderRecordWithOnlyTopLevelEntry_Unpriced(t *testing.T) {
	table := testTable(entryWithInput(t, "", "shared-model", "5.00"))

	ic := oneMillionInput(providerAnthropic, "shared-model", table)

	if ic.Priced {
		t.Fatal("a record with a provider must not fall back to the top-level models entry")
	}
	assertUnpriced(t, "Input", ic.Money.Input)
}

func TestPriceInvocation_RecordWithoutProviderOnlyProviderEntry_Unpriced(t *testing.T) {
	table := testTable(entryWithInput(t, providerAnthropic, "shared-model", "5.00"))

	ic := oneMillionInput("", "shared-model", table)

	if ic.Priced {
		t.Fatal("a record without a provider must never be priced from a provider section")
	}
	assertUnpriced(t, "Input", ic.Money.Input)
}

func TestPriceInvocation_RecordWithoutProvider_UsesTopLevelEntryNotProviderEntry(t *testing.T) {
	table := testTable(
		entryWithInput(t, "", "shared-model", "3.00"),
		entryWithInput(t, providerAnthropic, "shared-model", "5.00"),
	)

	ic := oneMillionInput("", "shared-model", table)

	if !ic.Priced {
		t.Fatal("a record without provider must be priced from the top-level entry")
	}
	assertKnownMoney(t, "Input", ic.Money.Input, 3_000_000_000)
}

func TestPriceInvocation_ProviderEntryAddedBeforeTopLevel_StillSeparateEntries(t *testing.T) {
	// Entry order must not decide which entry a request sees.
	table := testTable(
		entryWithInput(t, providerAnthropic, "shared-model", "5.00"),
		entryWithInput(t, "", "shared-model", "3.00"),
	)

	plain := oneMillionInput("", "shared-model", table)
	viaAnthropic := oneMillionInput(providerAnthropic, "shared-model", table)

	assertKnownMoney(t, "no-provider input", plain.Money.Input, 3_000_000_000)
	assertKnownMoney(t, "anthropic input", viaAnthropic.Money.Input, 5_000_000_000)
}

func TestPriceInvocation_UnderProvider_LongContextTierSelectedAtThreshold(t *testing.T) {
	entry := tieredPricing(t, "tier-model", 200_000)
	entry.Provider = providerCopilot
	table := testTable(entry)

	ic := cost.PriceInvocation(domain.InvocationUsage{
		Model:    "tier-model",
		Provider: providerCopilot,
		Usage: domain.TokenUsage{
			Input:  domain.Tokens(200_000),
			Output: domain.Tokens(1_000_000),
		},
	}, table)

	if !ic.Priced {
		t.Fatal("request must be priced from the provider entry")
	}
	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext", ic.Tier)
	}
	assertKnownMoney(t, "Output", ic.Money.Output, 22_500_000_000) // long-context output 22.50
}

func TestPriceInvocation_UnderProvider_BelowThresholdUsesStandardRates(t *testing.T) {
	entry := tieredPricing(t, "tier-model", 200_000)
	entry.Provider = providerCopilot
	table := testTable(entry)

	ic := cost.PriceInvocation(domain.InvocationUsage{
		Model:    "tier-model",
		Provider: providerCopilot,
		Usage: domain.TokenUsage{
			Input:  domain.Tokens(199_999),
			Output: domain.Tokens(1_000_000),
		},
	}, table)

	if !ic.Priced {
		t.Fatal("request must be priced from the provider entry")
	}
	if ic.Tier != cost.TierStandard {
		t.Fatalf("Tier = %v, want TierStandard", ic.Tier)
	}
	assertKnownMoney(t, "Output", ic.Money.Output, 15_000_000_000)
}

func TestPriceInvocation_UnderProvider_UnsplitCacheWritePricedAt5mRate(t *testing.T) {
	entry := flatPricing(t, "cache-model")
	entry.Provider = providerAnthropic
	table := testTable(entry)

	ic := cost.PriceInvocation(domain.InvocationUsage{
		Model:    "cache-model",
		Provider: providerAnthropic,
		Usage:    domain.TokenUsage{CacheCreation: domain.Tokens(1_000_000)},
	}, table)

	if !ic.Priced {
		t.Fatal("request must be priced from the provider entry")
	}
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 3_750_000_000) // 5m rate 3.75
	if ic.AssumedTierCacheWrite != 1_000_000 {
		t.Errorf("AssumedTierCacheWrite = %d, want 1000000", ic.AssumedTierCacheWrite)
	}
}
