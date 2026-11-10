package domain_test

// Tests for the display symbol of a currency and the formatting of an amount:
// two decimals, '.' separator, the sign before the symbol, and the ASCII form of
// the euro sign.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const euroSign = "€"

// nanosOf converts cents to the internal Money unit.
func nanosOf(cents int64) domain.Money { return domain.Money(cents * 10_000_000) }

func TestCurrencySymbol_PerCurrencyAndGlyphMode(t *testing.T) {
	cases := []struct {
		cur   domain.CurrencyCode
		ascii bool
		want  string
	}{
		{domain.CurrencyUSD, false, "$"},
		{domain.CurrencyUSD, true, "$"},
		{domain.CurrencyEUR, false, euroSign},
		{domain.CurrencyEUR, true, "EUR "},
	}
	for _, c := range cases {
		if got := c.cur.Symbol(c.ascii); got != c.want {
			t.Errorf("%s.Symbol(ascii=%v) = %q, want %q", c.cur, c.ascii, got, c.want)
		}
	}
}

func TestFormatAmount_SymbolSignAndDecimals(t *testing.T) {
	cases := []struct {
		name  string
		money domain.Money
		cur   domain.CurrencyCode
		ascii bool
		want  string
	}{
		{"usd", nanosOf(123), domain.CurrencyUSD, false, "$1.23"},
		{"usd ascii", nanosOf(123), domain.CurrencyUSD, true, "$1.23"},
		{"eur", nanosOf(123), domain.CurrencyEUR, false, euroSign + "1.23"},
		{"eur ascii", nanosOf(123), domain.CurrencyEUR, true, "EUR 1.23"},
		{"usd negative", nanosOf(-123), domain.CurrencyUSD, false, "-$1.23"},
		{"eur negative", nanosOf(-123), domain.CurrencyEUR, false, "-" + euroSign + "1.23"},
		{"eur negative ascii", nanosOf(-123), domain.CurrencyEUR, true, "-EUR 1.23"},
		{"zero", 0, domain.CurrencyUSD, false, "$0.00"},
		{"zero eur", 0, domain.CurrencyEUR, false, euroSign + "0.00"},
		{"pads to two decimals", nanosOf(500), domain.CurrencyEUR, false, euroSign + "5.00"},
		{"large amount has no group separator", nanosOf(123_456_789), domain.CurrencyUSD, false, "$1234567.89"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.FormatAmount(c.money, c.cur, c.ascii); got != c.want {
				t.Errorf("FormatAmount = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFormatAmount_RoundsHalfUpToTwoDecimals(t *testing.T) {
	cases := []struct {
		name  string
		money domain.Money
		want  string
	}{
		{"half rounds up", domain.Money(1_005_000_000), "$1.01"},
		{"below half rounds down", domain.Money(1_004_999_999), "$1.00"},
		{"sub-cent amount shows zero", domain.Money(4_000_000), "$0.00"},
		{"negative rounds by magnitude", domain.Money(-1_005_000_000), "-$1.01"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.FormatAmount(c.money, domain.CurrencyUSD, false); got != c.want {
				t.Errorf("FormatAmount(%d nanos) = %q, want %q", c.money.Nanos(), got, c.want)
			}
		})
	}
}
