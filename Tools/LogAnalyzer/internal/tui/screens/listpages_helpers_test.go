package screens_test

// Helpers shared by the tests of the source, pricing, no-data and export pages:
// the page layout they all share, the help overlay and the terminal sizes the
// views must stay inside.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// pageSizes are terminal sizes from roomy to degenerate; no page may draw past
// any of them.
var pageSizes = [][2]int{
	{120, 30}, {80, 24}, {80, 8}, {60, 12}, {40, 10}, {30, 6}, {20, 5}, {12, 4}, {80, 3}, {80, 2}, {80, 1},
}

// assertPageWithinEverySize fails when the screen draws past any of pageSizes.
func assertPageWithinEverySize(t *testing.T, s screens.Screen) {
	t.Helper()
	for _, size := range pageSizes {
		assertWithinBounds(t, s.View(size[0], size[1]), size[0], size[1])
	}
}

// assertPageLayout fails unless the view reads, top to bottom: a header line with
// the title, a rule, the first body line, and a help line at the very bottom.
func assertPageLayout(t *testing.T, view, title, firstBody string, helpWants ...string) []string {
	t.Helper()
	lines := viewLines(view)
	if len(lines) < screens.PageChromeLines+1 {
		t.Fatalf("page has %d lines, want at least %d\n%s", len(lines), screens.PageChromeLines+1, view)
	}
	assertContains(t, lines[0], title)
	assertSeparator(t, lines[1])
	assertContains(t, lines[2], firstBody)
	assertContains(t, tailOf(lines), helpWants...)
	return lines
}

// helpOpenOf reports whether the screen shows a help overlay; a screen without
// the overlay reports false.
func helpOpenOf(s screens.Screen) bool {
	h, ok := s.(interface{ HelpOpen() bool })
	return ok && h.HelpOpen()
}

// assertOverlayListsKey fails unless some entry of the overlay text starts with key.
func assertOverlayListsKey(t *testing.T, overlay, key string) {
	t.Helper()
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(key) + `(\s|/|$)`).MatchString(overlay) {
		t.Errorf("help overlay must list key %q\n%s", key, overlay)
	}
}

// pageOverlayBody is the body of the page while its overlay is open: the lines
// between the rule under the header and the help line.
func pageOverlayBody(s screens.Screen, width, height int) string {
	return strings.Join(overlayLines(openOverlay(s, width, height)), "\n")
}

// finished reports whether the screen asked to move on or to leave.
func finished(s screens.Screen) bool {
	f, ok := s.(interface {
		Done() bool
		Back() bool
	})
	return ok && (f.Done() || f.Back())
}

// pricingModels are three models: two priced with names of different length, and
// one unpriced from another provider.
func pricingModels() []app.ModelPricingStatus {
	return []app.ModelPricingStatus{
		{Key: domain.PricingKey{Provider: "p1", Model: "alpha"}, Model: "alpha", Priced: true},
		{Key: domain.PricingKey{Provider: "p1", Model: "a-much-longer-model-name"}, Model: "a-much-longer-model-name", Priced: true},
		{Key: domain.PricingKey{Provider: "p2", Model: "gamma"}, Model: "gamma", Priced: false},
	}
}

// numberedModels is n unpriced models model-01 ... under provider p.
func numberedModels(n int) []app.ModelPricingStatus {
	out := make([]app.ModelPricingStatus, n)
	for i := range out {
		name := domain.ModelID("model-" + twoDigits(i+1))
		out[i] = app.ModelPricingStatus{Key: domain.PricingKey{Provider: "p", Model: name}, Model: name}
	}
	return out
}
