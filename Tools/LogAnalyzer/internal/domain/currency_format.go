package domain

// Symbol is the text shown before an amount in c: USD "$" (both modes); EUR the euro
// sign U+20AC, or "EUR " (with a trailing space) when ascii.
func (c CurrencyCode) Symbol(ascii bool) string {
	switch c {
	case CurrencyEUR:
		if ascii {
			return "EUR "
		}
		return "€"
	case CurrencyUSD, "":
		return "$"
	default:
		return string(c) + " "
	}
}

// FormatAmount renders m rounded half-up to 2 decimals with '.' as separator, the sign
// before the symbol: "$1.23", "-$1.23", "<U+20AC>1.23", "-EUR 1.23".
func FormatAmount(m Money, c CurrencyCode, ascii bool) string {
	sign := ""
	if m < 0 {
		// StringFixed rounds a negative amount into a malformed figure; format its magnitude.
		sign = "-"
		m = -m
	}
	return sign + c.Symbol(ascii) + m.StringFixed(2)
}
