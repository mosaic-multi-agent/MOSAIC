package pricing_test

// Helpers for the currency-key tests of the pricing store.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// withCurrency prefixes a pricing document with a currency key line.
func withCurrency(code, doc string) string {
	return "currency: " + code + "\n" + doc
}

// currencyFindings keeps the findings that are about a pricing currency: the
// mismatch warning, or an invalid_pricing_entry whose detail mentions "currency".
func currencyFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingPricingCurrencyMismatch ||
			(f.Kind == domain.FindingInvalidPricingEntry && strings.Contains(strings.ToLower(f.Detail), "currency")) {
			out = append(out, f)
		}
	}
	return out
}

// mismatchFindings keeps only the pricing_currency_mismatch findings.
func mismatchFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingPricingCurrencyMismatch {
			out = append(out, f)
		}
	}
	return out
}

// namesFile reports whether the finding names the file, in Path or in Detail.
func namesFile(f domain.Finding, path string) bool {
	return f.Path == path || strings.Contains(f.Detail, path)
}

// requireCurrency fails unless the table carries the wanted currency.
func requireCurrency(t *testing.T, table domain.PricingTable, want domain.CurrencyCode) {
	t.Helper()
	if got := table.Currency(); got != want {
		t.Errorf("table currency = %q, want %q", got, want)
	}
}

// requireMissing fails when the table prices (provider, model).
func requireMissing(t *testing.T, table domain.PricingTable, provider, model string) {
	t.Helper()
	if _, ok := table.LookupFor(domain.ProviderID(provider), domain.ModelID(model)); ok {
		t.Errorf("table must not price %q/%q", provider, model)
	}
}
