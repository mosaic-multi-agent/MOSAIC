package screens_test

// The pricing screen shows where each price comes from: a source column that
// reads global or project for a price loaded from that level, and nothing for a
// model without a price or a price that comes from no file.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// pricedFrom is a priced status whose entry was loaded from level.
func pricedFrom(model domain.ModelID, level domain.ConfigLevel) app.ModelPricingStatus {
	return app.ModelPricingStatus{
		Key: domain.PricingKey{Model: model}, Model: model, Priced: true,
		Pricing: domain.ModelPricing{Model: model, Source: level},
	}
}

func sourceStatuses() []app.ModelPricingStatus {
	return []app.ModelPricingStatus{
		pricedFrom("alpha", domain.LevelGlobal),
		pricedFrom("bravo", domain.LevelProject),
		pricedFrom("charlie", domain.LevelBuiltIn),
		{Key: domain.PricingKey{Model: "delta"}, Model: "delta"},
	}
}

func sourceView(t *testing.T) string {
	t.Helper()
	colourOff(t)
	return screens.NewPricingScreen(sourceStatuses(), "", wideW, tallH, plain()).View(wideW, tallH)
}

func TestPricingScreen_HasASourceColumn(t *testing.T) {
	view := sourceView(t)

	if !strings.Contains(strings.ToLower(view), "source") {
		t.Errorf("the pricing table must have a source column\n%s", view)
	}
}

func TestPricingScreen_GlobalPriceReadsGlobal(t *testing.T) {
	line := lineContaining(t, sourceView(t), "alpha")

	if !strings.Contains(line, "global") || strings.Contains(line, "project") {
		t.Errorf("alpha row = %q, want the source global", line)
	}
}

func TestPricingScreen_ProjectPriceReadsProject(t *testing.T) {
	line := lineContaining(t, sourceView(t), "bravo")

	if !strings.Contains(line, "project") || strings.Contains(line, "global") {
		t.Errorf("bravo row = %q, want the source project", line)
	}
}

func TestPricingScreen_PriceFromNoFileAndUnpricedModelShowNoSource(t *testing.T) {
	view := sourceView(t)

	for _, model := range []string{"charlie", "delta"} {
		line := lineContaining(t, view, model)
		if strings.Contains(line, "global") || strings.Contains(line, "project") {
			t.Errorf("%s row = %q, want no source", model, line)
		}
	}
}
