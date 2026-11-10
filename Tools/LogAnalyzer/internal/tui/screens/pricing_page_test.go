package screens_test

// The pricing page lists one row per (provider, model) in a table, shows the
// config file and the status message as text in their roles, and scrolls the list
// inside the room the page leaves. Markers stay text, so everything reads with
// colour off.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

const pricingConfig = "/cfg/MosaicLogAnalyzer/config/pricing.yaml"

func newPricing(width, height int) *screens.PricingScreen {
	return screens.NewPricingScreen(pricingModels(), pricingConfig, width, height, paletteStyles())
}

func TestPricingPage_RowsKeepTheirMarkersAsTextWithColourOff(t *testing.T) {
	colourOff(t)

	view := stripStyling(newPricing(100, 24).View(100, 24))

	assertContains(t, lineContaining(t, view, "alpha [p1]"), "(priced)")
	unpriced := lineContaining(t, view, "gamma [p2]")
	assertContains(t, unpriced, screens.AbsentMarker, "unpriced")
	assertContains(t, view, "Config file: "+pricingConfig)
}

func TestPricingPage_StatusColumnLinesUpWhateverTheModelNameLength(t *testing.T) {
	colourOff(t)

	view := stripStyling(newPricing(100, 24).View(100, 24))

	short := displayColumn(lineContaining(t, view, "alpha [p1]"), "(priced)")
	long := displayColumn(lineContaining(t, view, "a-much-longer-model-name [p1]"), "(priced)")
	if short < 0 || short != long {
		t.Errorf("(priced) must start in one column on every row, got %d and %d\n%s", short, long, view)
	}
}

func TestPricingPage_CursorMarkerFollowsTheSelectedRow(t *testing.T) {
	colourOff(t)
	s := newPricing(100, 24)

	assertCursorOn(t, s.View(100, 24), "alpha [p1]")
	press(s, "down")
	assertCursorOn(t, s.View(100, 24), "a-much-longer-model-name [p1]")
	press(s, "j")
	assertCursorOn(t, s.View(100, 24), "gamma [p2]")
	press(s, "up")
	assertCursorOn(t, s.View(100, 24), "a-much-longer-model-name [p1]")
}

// assertCursorOn fails unless the cursor marker is on the row naming want and on
// no other row.
func assertCursorOn(t *testing.T, view, want string) {
	t.Helper()
	plainView := stripStyling(view)
	marked := 0
	for _, l := range strings.Split(plainView, "\n") {
		if strings.HasPrefix(l, ">") {
			marked++
			if !strings.Contains(l, want) {
				t.Errorf("cursor is on %q, want %q", l, want)
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d rows carry the cursor marker, want exactly 1\n%s", marked, plainView)
	}
}

func TestPricingPage_LongListScrollsAndTheCursorRowStaysVisible(t *testing.T) {
	colourOff(t)
	s := screens.NewPricingScreen(numberedModels(30), pricingConfig, 80, 10, paletteStyles())

	first := s.View(80, 10)
	assertWithinBounds(t, first, 80, 10)
	assertContains(t, stripStyling(first), "model-01 [p]")
	press(s, repeat("down", 25)...)
	view := s.View(80, 10)

	assertWithinBounds(t, view, 80, 10)
	assertCursorOn(t, view, "model-26 [p]")
	assertNotContains(t, stripStyling(view), "model-01 [p]")
	press(s, "enter")
	if got := s.SelectedModel(); got != "model-26" {
		t.Errorf("SelectedModel = %q, want model-26", got)
	}
}

func TestPricingPage_StatusMessageIsDrawnInTheGoodRoleWithItsTextIntact(t *testing.T) {
	colourOn(t)
	s := newPricing(100, 30)
	s.SetStatus("Price for alpha written to " + pricingConfig)

	view := s.View(100, 30)

	assertDrawnInRole(t, rawLineWith(t, view, "Price for alpha written"), "Price for alpha written to "+pricingConfig, good)
}

func TestPricingPage_StatusMessageReadsAsTextWithColourOff(t *testing.T) {
	colourOff(t)
	s := newPricing(100, 30)
	s.SetStatus("Price for alpha written")

	assertContains(t, stripStyling(s.View(100, 30)), "Price for alpha written")
	assertNotContains(t, s.View(100, 30), "\x1b[")
}

func TestPricingPage_UnpricedRowIsDrawnInTheWarningRoleAndPricedRowIsNot(t *testing.T) {
	colourOn(t)
	view := newPricing(100, 30).View(100, 30)

	assertContains(t, rawLineWith(t, view, "gamma [p2]"), rolePrefix(warning))
	assertNotContains(t, rawLineWith(t, view, "a-much-longer-model-name [p1]"), rolePrefix(warning))
}

func TestPricingPage_EveryLineStaysWithinTheWidthEvenWithLongPathsAndMessages(t *testing.T) {
	colourOff(t)
	s := screens.NewPricingScreen(pricingModels(), "/"+strings.Repeat("deep/", 40)+"pricing.yaml", 80, 24, paletteStyles())
	s.SetStatus(strings.Repeat("written ", 40))

	assertPageWithinEverySize(t, s)
}
