package tui

// The pricing screen names the effective currency of the report, and the message
// shown after a price is saved states the currency of the file it was written to.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	projectPricingFile = "/test/pricing.yaml"
	savedWidth         = 140
)

// onPricingScreen opens the pricing screen of a report in cur over a service whose
// pricing files state the given currencies.
func onPricingScreen(t *testing.T, cur domain.CurrencyCode, store *tuiFakePricingStore) Model {
	t.Helper()
	store.path = projectPricingFile
	m := newNavModel()
	m.opts.Service = app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     store,
		Clock:       &tuiFakeClock{},
		Interaction: &tuiFakeInteraction{},
	})
	m.session = new(app.Session)
	m.result = &app.Result{Outcome: app.OutcomeReport, Session: new(app.Session)}
	report := makeReport("20260101T120000Z-abcd")
	report.Currency = cur
	report.UnpricedModels = []domain.PricingKey{{Model: "claude-test-model"}}
	updated, _ := m.Update(analysisDoneMsg{result: *m.result})
	m = asModel(updated)
	m.report = report
	m.runsScreen.SetReport(report)
	m = asModel(func() Model { u, _ := pressKey(m, "p"); return asModel(u) }())
	assertOnScreen(t, m, screenPricing)
	return m
}

func assertOnScreen(t *testing.T, m Model, want screenName) {
	t.Helper()
	if m.screen != want {
		t.Fatalf("screen = %q, want %q", m.screen, want)
	}
}

func TestPricingScreen_NamesTheEffectiveCurrencyOfTheReport(t *testing.T) {
	cases := []struct {
		cur          domain.CurrencyCode
		want, absent string
	}{
		{domain.CurrencyEUR, "EUR", "USD"},
		{domain.CurrencyUSD, "USD", "EUR"},
	}
	for _, c := range cases {
		t.Run(string(c.cur), func(t *testing.T) {
			m := onPricingScreen(t, c.cur, &tuiFakePricingStore{})

			view := plainView(m)

			if !strings.Contains(view, c.want) {
				t.Errorf("the pricing screen must name the currency %s\n%s", c.want, view)
			}
			if strings.Contains(view, c.absent) {
				t.Errorf("the pricing screen must not name %s\n%s", c.absent, view)
			}
		})
	}
}

func TestPricingScreen_NamesTheEffectiveCurrencyNotTheStatedOneOfAFile(t *testing.T) {
	// The global file states EUR but the project file's USD is effective.
	store := &tuiFakePricingStore{globalCurrency: domain.CurrencyEUR, projectCurrency: domain.CurrencyUSD}
	m := onPricingScreen(t, domain.CurrencyUSD, store)

	view := plainView(m)

	if !strings.Contains(view, "USD") || strings.Contains(view, "EUR") {
		t.Errorf("the pricing screen shows the effective currency USD only\n%s", view)
	}
}

// saveAt applies a completed price-edit flow that wrote the file of level.
func saveAt(t *testing.T, m Model, level domain.ConfigLevel, path string) string {
	t.Helper()
	done := pricingEditDoneMsg{result: app.PricingEditResult{
		Outcome:     app.PricingEditSaved,
		Key:         domain.PricingKey{Model: "claude-test-model"},
		Model:       "claude-test-model",
		Report:      m.report,
		ConfigPath:  path,
		ConfigLevel: level,
	}}
	updated, _ := m.handlePricingEditDone(done)
	m2 := asModel(updated)
	if m2.pricingScreen == nil {
		t.Fatal("the pricing screen must keep the save message")
	}
	return escapeRE.ReplaceAllString(m2.pricingScreen.View(savedWidth, 40), "")
}

func TestPriceSavedMessage_StatesTheCurrencyOfTheFileWritten(t *testing.T) {
	// Global file EUR, project file USD: the project currency is effective, so the
	// message must follow the file written, not the report.
	newStore := func() *tuiFakePricingStore {
		return &tuiFakePricingStore{globalCurrency: domain.CurrencyEUR, projectCurrency: domain.CurrencyUSD}
	}
	cases := []struct {
		name         string
		level        domain.ConfigLevel
		path         string
		want, absent string
	}{
		{"global", domain.LevelGlobal, tuiFakeGlobalPricingPath, "rates in EUR", "rates in USD"},
		{"project", domain.LevelProject, projectPricingFile, "rates in USD", "rates in EUR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := onPricingScreen(t, domain.CurrencyUSD, newStore())

			view := saveAt(t, m, c.level, c.path)

			if !strings.Contains(view, c.path) {
				t.Errorf("the message must still name the file %q\n%s", c.path, view)
			}
			if !strings.Contains(view, c.want) || strings.Contains(view, c.absent) {
				t.Errorf("the message must contain %q and not %q\n%s", c.want, c.absent, view)
			}
		})
	}
}

func TestPriceSavedMessage_FileWithoutCurrencyKey_UsesTheEffectiveCurrency(t *testing.T) {
	m := onPricingScreen(t, domain.CurrencyEUR, &tuiFakePricingStore{})

	view := saveAt(t, m, domain.LevelProject, projectPricingFile)

	if !strings.Contains(view, "rates in EUR") {
		t.Errorf("a file that states no currency is written in the effective currency\n%s", view)
	}
}
