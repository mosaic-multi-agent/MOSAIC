package domain_test

// Tests for the currency vocabulary and the currency a pricing table carries.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestParseCurrency_AcceptsExactlyUSDAndEUR(t *testing.T) {
	cases := []struct {
		in     string
		want   domain.CurrencyCode
		wantOK bool
	}{
		{"USD", domain.CurrencyUSD, true},
		{"EUR", domain.CurrencyEUR, true},
		{"usd", "", false},
		{"Eur", "", false},
		{"GBP", "", false},
		{"", "", false},
		{" USD", "", false},
	}
	for _, c := range cases {
		got, ok := domain.ParseCurrency(c.in)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("ParseCurrency(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestSupportedCurrencies_ListsUSDAndEUR(t *testing.T) {
	got := domain.SupportedCurrencies()
	want := []domain.CurrencyCode{domain.CurrencyUSD, domain.CurrencyEUR}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SupportedCurrencies() = %v, want %v", got, want)
	}
}

func TestDefaultCurrency_IsUSD(t *testing.T) {
	if domain.DefaultCurrency != domain.CurrencyUSD {
		t.Errorf("DefaultCurrency = %q, want USD", domain.DefaultCurrency)
	}
}

func TestPricingTable_PlainConstructorsMeanUSDWithNoExclusions(t *testing.T) {
	key := domain.PricingKey{Model: "m"}
	for name, table := range map[string]domain.PricingTable{
		"NewPricingTable":     domain.NewPricingTable(nil),
		"EmptyPricingTable":   domain.EmptyPricingTable(),
		"NewPricingTable+one": domain.NewPricingTable([]domain.ModelPricing{{Model: "m"}}),
	} {
		if table.Currency() != domain.CurrencyUSD {
			t.Errorf("%s: Currency() = %q, want USD", name, table.Currency())
		}
		if _, ok := table.Excluded(key); ok {
			t.Errorf("%s: Excluded(%v) = true, want false", name, key)
		}
	}
}

func TestNewPricingTableIn_CarriesCurrencyAndExclusions(t *testing.T) {
	key := domain.PricingKey{Provider: "p", Model: "gone"}
	ex := domain.ExcludedPrice{Key: key, Level: domain.LevelGlobal, Path: "/g/pricing.yaml", Currency: domain.CurrencyUSD}

	table := domain.NewPricingTableIn(domain.CurrencyEUR, []domain.ModelPricing{{Model: "kept"}}, []domain.ExcludedPrice{ex})

	if table.Currency() != domain.CurrencyEUR {
		t.Errorf("Currency() = %q, want EUR", table.Currency())
	}
	got, ok := table.Excluded(key)
	if !ok || got != ex {
		t.Errorf("Excluded(%v) = (%+v, %v), want (%+v, true)", key, got, ok, ex)
	}
	if _, ok := table.Excluded(domain.PricingKey{Model: "kept"}); ok {
		t.Error("a key that was not excluded must not report as excluded")
	}
	if _, found := table.Lookup("kept"); !found {
		t.Error("entries passed to NewPricingTableIn must be found")
	}
}

func TestPricingTableWith_KeepsCurrencyAndClearsExclusionOfAddedKey(t *testing.T) {
	key := domain.PricingKey{Model: "gone"}
	other := domain.PricingKey{Model: "other"}
	table := domain.NewPricingTableIn(domain.CurrencyEUR, nil, []domain.ExcludedPrice{
		{Key: key, Level: domain.LevelGlobal, Currency: domain.CurrencyUSD},
		{Key: other, Level: domain.LevelGlobal, Currency: domain.CurrencyUSD},
	})

	updated := table.With(domain.ModelPricing{Model: "gone"})

	if updated.Currency() != domain.CurrencyEUR {
		t.Errorf("With changed the currency to %q, want EUR", updated.Currency())
	}
	if _, ok := updated.Excluded(key); ok {
		t.Error("an added entry must no longer be excluded")
	}
	if _, ok := updated.Excluded(other); !ok {
		t.Error("With must keep the other exclusions")
	}
	if _, ok := table.Excluded(key); !ok {
		t.Error("With must not mutate the original table")
	}
}
