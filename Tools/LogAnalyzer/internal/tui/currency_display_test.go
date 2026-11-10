package tui

// Money in the TUI follows the effective currency of the report: "$" for USD, the
// euro sign for EUR (two decimals, '.' separator), and "EUR " in ASCII glyph mode.
// Every screen that shows money is checked: the runs list, the per-run views, the
// across-runs view and the comparison, including cache, rewrite and compaction money.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	euroSign      = "€"
	currencyWidth = 200
)

func centsValue(c int64) domain.MoneyValue { return domain.KnownMoney(domain.Money(c * 10_000_000)) }

// currencyReportIn is the two-run comparison report in the given currency, with
// cache, rewrite and compaction-call money on the first run so every money display
// of the per-run views is reached.
func currencyReportIn(cur domain.CurrencyCode) domain.Report {
	report := acrossNavReport()
	report.Currency = cur
	run := report.Runs[0]
	run.Cache = domain.CacheFigures{
		NetBenefit:        centsValue(400),
		UnreadWriteCost:   centsValue(125),
		UnreadWriteTokens: domain.Tokens(2_000),
		Complete:          true,
		Rewrites: domain.RewriteFigures{
			ColdIdle: 1, Tokens: domain.Tokens(5_000),
			Cost: centsValue(250), ExtraCost: centsValue(100), Complete: true,
		},
	}
	run.Context = domain.RunContext{Orchestrator: domain.ActorContext{
		Actor: domain.Orchestrator(),
		Calls: []domain.ContextCall{{
			Model:   "model-o",
			Usage:   domain.TokenUsage{Input: domain.Tokens(1_000)},
			Context: domain.Tokens(1_000),
			Cost:    centsValue(10),
		}},
		CompactionCalls: domain.CompactionCallFigures{
			Count: 1, Cost: centsValue(75), UnappliedCount: 1, UnappliedCost: centsValue(75),
		},
	}}
	report.Runs[0] = run
	return report
}

func sizedModel(report domain.Report, width, height int) Model {
	updated, _ := onRunsList(report).Update(tea.WindowSizeMsg{Width: width, Height: height})
	return asModel(updated)
}

func plainView(m Model) string { return escapeRE.ReplaceAllString(m.View(), "") }

func asciiMode(t *testing.T) {
	t.Helper()
	t.Cleanup(tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphASCII}))
}

func unicodeMode(t *testing.T) {
	t.Helper()
	t.Cleanup(tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphUnicode}))
}

// currencyViews opens each money-bearing view of the first run and returns its
// plain text keyed by a name.
func currencyViews(t *testing.T, report domain.Report, width, height int) map[string]string {
	t.Helper()
	views := map[string]string{}
	list := sizedModel(report, width, height)
	views["runs list"] = plainView(list)

	detail := send(list, "enter")
	assertOn(t, detail, screens.ScreenRunDetail)
	views["run detail"] = plainView(detail)
	for name, key := range map[string]string{
		"overview": screens.KeyViewOverview, "agent types": screens.KeyViewAgentTypes,
		"status": screens.KeyViewStatus, "tools": screens.KeyViewTools,
		"timeline": screens.KeyViewTimeline, "context": screens.KeyViewContext,
	} {
		views[name] = plainView(send(detail, key))
	}
	views["context calls"] = plainView(send(detail, screens.KeyViewContext, "down", "enter"))
	views["across runs"] = plainView(send(list, screens.KeyAcrossRuns))
	views["compare"] = plainView(send(list, screens.KeyMarkRun, "down", screens.KeyMarkRun, screens.KeyCompare))
	return views
}

func TestMoneyDisplay_EURReport_EveryViewShowsEurosAndNoDollars(t *testing.T) {
	unicodeMode(t)

	for name, view := range currencyViews(t, currencyReportIn(domain.CurrencyEUR), currencyWidth, 60) {
		if strings.Contains(view, "$") {
			t.Errorf("%s: an EUR report must show no dollar sign\n%s", name, view)
		}
	}
}

func TestMoneyDisplay_EURReport_FiguresCarryTheEuroSign(t *testing.T) {
	unicodeMode(t)
	views := currencyViews(t, currencyReportIn(domain.CurrencyEUR), currencyWidth, 60)

	cases := []struct{ view, want string }{
		{"runs list", euroSign + "7.00"},
		{"runs list", euroSign + "3.00"},
		{"run detail", euroSign + "7.00"},
		{"run detail", euroSign + "3.50"},
		{"overview", euroSign + "7.00"},
		{"overview", euroSign + "3.50"},
		{"overview", euroSign + "4.00"}, // net cache benefit
		{"overview", euroSign + "1.25"}, // unread cache writes
		{"overview", euroSign + "2.50"}, // rewrite cost
		{"overview", euroSign + "1.00"}, // rewrite extra cost
		{"agent types", euroSign + "3.50"},
		{"context", euroSign + "0.75"},       // compaction call cost
		{"context calls", euroSign + "0.10"}, // per-call cost
		{"across runs", euroSign + "3.00"},
		{"across runs", euroSign + "5.00"},
		{"across runs", euroSign + "7.00"},
		{"compare", euroSign + "3.50"},
		{"compare", euroSign + "1.50"},
		{"compare", "-" + euroSign + "2.00"},
	}
	for _, c := range cases {
		if !strings.Contains(views[c.view], c.want) {
			t.Errorf("%s must show %q\n%s", c.view, c.want, views[c.view])
		}
	}
}

func TestMoneyDisplay_USDReport_EveryViewShowsDollarsAndNoEuros(t *testing.T) {
	unicodeMode(t)
	views := currencyViews(t, currencyReportIn(domain.CurrencyUSD), currencyWidth, 60)

	for name, view := range views {
		if strings.Contains(view, euroSign) || strings.Contains(view, "EUR") {
			t.Errorf("%s: a USD report must show no euro sign\n%s", name, view)
		}
	}
	for _, want := range []struct{ view, text string }{
		{"runs list", "$7.00"}, {"overview", "$2.50"}, {"context", "$0.75"},
		{"across runs", "$5.00"}, {"compare", "-$2.00"},
	} {
		if !strings.Contains(views[want.view], want.text) {
			t.Errorf("%s must show %q\n%s", want.view, want.text, views[want.view])
		}
	}
}

func TestMoneyDisplay_EURReportInAsciiMode_UsesTheEURPrefix(t *testing.T) {
	asciiMode(t)
	views := currencyViews(t, currencyReportIn(domain.CurrencyEUR), currencyWidth, 60)

	for name, view := range views {
		if strings.Contains(view, euroSign) || strings.Contains(view, "$") {
			t.Errorf("%s: ASCII mode must show neither the euro sign nor a dollar sign\n%s", name, view)
		}
	}
	for _, want := range []struct{ view, text string }{
		{"runs list", "EUR 7.00"}, {"overview", "EUR 2.50"}, {"context", "EUR 0.75"},
		{"across runs", "EUR 5.00"}, {"compare", "-EUR 2.00"},
	} {
		if !strings.Contains(views[want.view], want.text) {
			t.Errorf("%s must show %q\n%s", want.view, want.text, views[want.view])
		}
	}
}

func TestMoneyDisplay_USDReportInAsciiMode_KeepsTheDollarSign(t *testing.T) {
	asciiMode(t)
	views := currencyViews(t, currencyReportIn(domain.CurrencyUSD), currencyWidth, 60)

	if !strings.Contains(views["runs list"], "$7.00") || strings.Contains(views["runs list"], "EUR") {
		t.Errorf("USD stays $ in ASCII mode\n%s", views["runs list"])
	}
}

func TestMoneyDisplay_EURPrefixInAsciiMode_NeverWidensALineBeyondTheTerminal(t *testing.T) {
	asciiMode(t)

	for _, width := range []int{80, 100, 140} {
		for _, view := range currencyViews(t, currencyReportIn(domain.CurrencyEUR), width, 60) {
			assertViewWithin(t, view, width, 60)
		}
	}
}

func TestMoneyDisplay_EuroSignInUnicodeMode_NeverWidensALineBeyondTheTerminal(t *testing.T) {
	unicodeMode(t)

	for _, width := range []int{80, 100, 140} {
		for _, view := range currencyViews(t, currencyReportIn(domain.CurrencyEUR), width, 60) {
			assertViewWithin(t, view, width, 60)
		}
	}
}

func TestMoneyDisplay_UnpricedAndMissingMarkersAreCurrencyFree(t *testing.T) {
	unicodeMode(t)
	report := currencyReportIn(domain.CurrencyEUR)
	report.Runs[0].Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	report.Runs[1].Totals.Money = domain.CategoryMoney{Total: domain.MoneyValue{State: domain.MoneyNoData}}

	view := plainView(sizedModel(report, currencyWidth, 40))

	if !strings.Contains(view, screens.UnpricedMarker) || !strings.Contains(view, screens.AbsentMarker) {
		t.Errorf("unpriced and no-data money keep their markers\n%s", view)
	}
}
