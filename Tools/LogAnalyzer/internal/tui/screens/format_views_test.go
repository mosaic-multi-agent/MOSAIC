package screens_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestFormatRatio(t *testing.T) {
	cases := []struct {
		name string
		in   domain.Ratio
		want string
	}{
		{"fraction rounds to one decimal", domain.Ratio{Numerator: 3000, Denominator: 4500}, "66.7%"},
		{"exact half", domain.Ratio{Numerator: 1, Denominator: 2}, "50.0%"},
		{"zero numerator is a real zero", domain.Ratio{Numerator: 0, Denominator: 10}, "0.0%"},
		{"whole", domain.Ratio{Numerator: 7, Denominator: 7}, "100.0%"},
		{"zero denominator is undefined", domain.Ratio{Numerator: 5, Denominator: 0}, screens.AbsentMarker},
		{"negative denominator is undefined", domain.Ratio{Numerator: 5, Denominator: -1}, screens.AbsentMarker},
	}
	for _, c := range cases {
		if got := screens.FormatRatio(c.in); got != c.want {
			t.Errorf("%s: FormatRatio(%+v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestFormatAssumedTier(t *testing.T) {
	if got := screens.FormatAssumedTier(0, nil); got != "" {
		t.Errorf("FormatAssumedTier(0) = %q, want empty", got)
	}
	got := screens.FormatAssumedTier(12345, nil)
	assertContains(t, got, screens.AssumedTierLabel(nil), "12,345")
}
