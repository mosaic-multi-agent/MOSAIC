package screens_test

// Field-bounded matching: a figure counts as shown only when it stands as a whole
// cell value, so "12" is not found inside "$12.34" or "111,111".

import (
	"regexp"
	"testing"
)

// hasField reports whether the row shows want as a whole figure: not glued to
// digits, thousands separators, decimal points or a currency sign.
func hasField(row, want string) bool {
	re := regexp.MustCompile(`(^|[^0-9,.$])` + regexp.QuoteMeta(want) + `($|[^0-9,.])`)
	return re.MatchString(stripStyling(row))
}

func assertFields(t *testing.T, row string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !hasField(row, w) {
			t.Errorf("row does not show %q as a figure of its own: %q", w, row)
		}
	}
}

func assertNoFields(t *testing.T, row string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if hasField(row, u) {
			t.Errorf("row must not show %q as a figure: %q", u, row)
		}
	}
}
