package pricing_test

// Shared helpers and fixtures for the provider-level pricing tests.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

// tableAndFindings is the result of one Load.
type tableAndFindings struct {
	table    domain.PricingTable
	findings []domain.Finding
}

// probeInputNanos prices one request of 1M input tokens for (provider, model)
// against the table and returns whether it was priced and its input cost in
// nanodollars (the input rate in USD per million tokens times 1e9). A request
// of 1M input tokens is above any long-context threshold used in these tests.
func probeInputNanos(table domain.PricingTable, provider domain.ProviderID, model domain.ModelID) (priced bool, nanos int64) {
	ic := cost.PriceInvocation(domain.InvocationUsage{
		Model:    model,
		Provider: provider,
		Usage:    domain.TokenUsage{Input: domain.Tokens(1_000_000)},
	}, table)
	return ic.Priced, ic.Money.Input.Amount.Nanos()
}

// assertPricedAt fails unless (provider, model) is priced with the given input cost.
func assertPricedAt(t *testing.T, table domain.PricingTable, provider domain.ProviderID, model domain.ModelID, wantNanos int64) {
	t.Helper()
	priced, nanos := probeInputNanos(table, provider, model)
	if !priced {
		t.Errorf("(%q, %q) is unpriced, want priced at %d nanos per 1M input", provider, model, wantNanos)
		return
	}
	if nanos != wantNanos {
		t.Errorf("(%q, %q) input = %d nanos, want %d", provider, model, nanos, wantNanos)
	}
}

// assertUnpricedKey fails when (provider, model) is priced.
func assertUnpricedKey(t *testing.T, table domain.PricingTable, provider domain.ProviderID, model domain.ModelID) {
	t.Helper()
	if priced, _ := probeInputNanos(table, provider, model); priced {
		t.Errorf("(%q, %q) is priced, want unpriced", provider, model)
	}
}

// loadFrom writes content as the pricing file of a fresh root and loads it.
func loadFrom(t *testing.T, content string) (tableAndFindings, error) {
	t.Helper()
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, content)
	table, findings, err := pricing.NewStore(root).Load(context.Background())
	return tableAndFindings{table: table, findings: findings}, err
}

// withInput returns a minimal valid entry for (provider, model) with the given input rate.
func withInput(t *testing.T, provider domain.ProviderID, model domain.ModelID, input string) domain.ModelPricing {
	t.Helper()
	e := newMinimalEntry(t, model)
	e.Provider = provider
	e.Standard.Input = mustRate(t, input)
	return e
}

// putAndReload puts the entry and loads the file again.
func putAndReload(t *testing.T, store *pricing.Store, entry domain.ModelPricing) tableAndFindings {
	t.Helper()
	if err := store.Put(context.Background(), entry); err != nil {
		t.Fatalf("Put: %v", err)
	}
	table, findings, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load after Put: %v", err)
	}
	return tableAndFindings{table: table, findings: findings}
}

// bothSectionsYAML has top-level models and two providers. Input rates identify
// the entry: plain-model 3.00 top-level and 9.00 under github-copilot;
// claude-sonnet-5-5 5.00 under anthropic; claude-sonnet-5.5 7.00 under github-copilot.
const bothSectionsYAML = `
models:
  plain-model:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
providers:
  anthropic:
    models:
      claude-sonnet-5-5:
        input: "5.00"
        cache_read: "0.50"
        cache_write_5m: "6.25"
        cache_write_1h: "10.00"
        output: "25.00"
  github-copilot:
    models:
      claude-sonnet-5.5:
        input: "7.00"
        cache_read: "0.70"
        cache_write_5m: "8.75"
        cache_write_1h: "14.00"
        output: "35.00"
      plain-model:
        input: "9.00"
        cache_read: "0.90"
        cache_write_5m: "11.25"
        cache_write_1h: "18.00"
        output: "45.00"
`
