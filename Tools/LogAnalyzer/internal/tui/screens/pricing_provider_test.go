package screens_test

// The pricing screen lists one row per (provider, model), reports the
// highlighted (provider, model) for editing, and the unpriced footers and
// markers name the provider of each gap.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func sharedModelStatuses() []app.ModelPricingStatus {
	return []app.ModelPricingStatus{
		{Key: domain.PricingKey{Provider: "p1", Model: "shared-model"}, Model: "shared-model", Priced: true},
		{Key: domain.PricingKey{Provider: "p2", Model: "shared-model"}, Model: "shared-model", Priced: false},
	}
}

func lineContaining(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("no line contains %q in:\n%s", needle, view)
	return ""
}

func TestPricingScreen_RowsNameTheProviderWithTheirOwnState(t *testing.T) {
	s := screens.NewPricingScreen(sharedModelStatuses(), "", wideW, tallH, plain())

	view := s.View(wideW, tallH)

	if l := lineContaining(t, view, "shared-model [p1]"); !strings.Contains(l, "(priced)") {
		t.Errorf("p1 row = %q, want it marked priced", l)
	}
	if l := lineContaining(t, view, "shared-model [p2]"); !strings.Contains(l, "unpriced") {
		t.Errorf("p2 row = %q, want it marked unpriced", l)
	}
}

func TestPricingScreen_SelectedKey_IsTheHighlightedProviderRow(t *testing.T) {
	s := screens.NewPricingScreen(sharedModelStatuses(), "", wideW, tallH, plain())

	press(s, "down", "enter")

	if !s.Done() {
		t.Fatal("Enter on a row should finish the screen")
	}
	want := domain.PricingKey{Provider: "p2", Model: "shared-model"}
	if got := s.SelectedKey(); got != want {
		t.Errorf("SelectedKey = %+v, want %+v", got, want)
	}
}

func TestPricingScreen_SelectedKey_FirstRowIsFirstProvider(t *testing.T) {
	s := screens.NewPricingScreen(sharedModelStatuses(), "", wideW, tallH, plain())

	press(s, "enter")

	want := domain.PricingKey{Provider: "p1", Model: "shared-model"}
	if got := s.SelectedKey(); got != want {
		t.Errorf("SelectedKey = %+v, want %+v", got, want)
	}
}

func TestRunDetail_UnpricedFooter_NamesTheProvider(t *testing.T) {
	run := overviewRun()
	run.UnpricedModels = []domain.PricingKey{{Provider: "p2", Model: "shared-model"}}
	s := screens.NewRunDetailScreen(run, wideW, tallH, plain())

	view := s.View(wideW, tallH)

	l := lineContaining(t, view, "Unpriced models")
	if !strings.Contains(l, "shared-model [p2]") {
		t.Errorf("unpriced footer = %q, want it to name shared-model [p2]", l)
	}
}

func TestOverview_UnpricedMarker_NamesTheProvider(t *testing.T) {
	run := overviewRun()
	run.UnpricedModels = []domain.PricingKey{{Provider: "p2", Model: "shared-model"}}

	view := overviewView(run, tallH)

	l := lineContaining(t, view, screens.UnpricedMarker+":")
	if !strings.Contains(l, "shared-model [p2]") {
		t.Errorf("unpriced marker line = %q, want it to name shared-model [p2]", l)
	}
}
