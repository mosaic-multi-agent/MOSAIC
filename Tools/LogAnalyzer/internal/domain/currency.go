package domain

// CurrencyCode is a supported ISO currency code for pricing files and reports.
type CurrencyCode string

const (
	CurrencyUSD     CurrencyCode = "USD"
	CurrencyEUR     CurrencyCode = "EUR"
	DefaultCurrency              = CurrencyUSD
)

// SupportedCurrencies lists the currencies a pricing file may state.
func SupportedCurrencies() []CurrencyCode {
	return []CurrencyCode{CurrencyUSD, CurrencyEUR}
}

// ParseCurrency accepts exactly "USD" or "EUR" (case-sensitive).
func ParseCurrency(s string) (CurrencyCode, bool) {
	for _, c := range SupportedCurrencies() {
		if string(c) == s {
			return c, true
		}
	}
	return "", false
}

// ExcludedPrice records a pricing key left out of the table by a currency mismatch.
type ExcludedPrice struct {
	Key      PricingKey
	Level    ConfigLevel  // always LevelGlobal today
	Path     string       // file the entry came from
	Currency CurrencyCode // the excluded file's currency
}

// UnpricedReason explains why an unpriced key is unpriced.
type UnpricedReason struct {
	Key    PricingKey
	Reason string
}

// FindingPricingCurrencyMismatch is the warning raised when the global and project
// pricing files state different currencies.
const FindingPricingCurrencyMismatch FindingKind = "pricing_currency_mismatch"

// NewPricingTableIn builds a table whose rates are in cur, with keys excluded by a
// currency mismatch. NewPricingTable(entries) means USD, no exclusions.
func NewPricingTableIn(cur CurrencyCode, entries []ModelPricing, excluded []ExcludedPrice) PricingTable {
	t := NewPricingTable(entries)
	t.currency = cur
	if len(excluded) > 0 {
		t.excluded = make(map[PricingKey]ExcludedPrice, len(excluded))
		for _, e := range excluded {
			t.excluded[e.Key] = e
		}
	}
	return t
}

// Currency is the currency of the table's rates; USD unless built with NewPricingTableIn.
func (t PricingTable) Currency() CurrencyCode {
	if t.currency == "" {
		return DefaultCurrency
	}
	return t.currency
}

// Excluded reports whether key was excluded by a currency mismatch.
func (t PricingTable) Excluded(key PricingKey) (ExcludedPrice, bool) {
	e, ok := t.excluded[key]
	return e, ok
}
