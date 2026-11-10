package cli_test

// Tests that the text report shows money in the effective currency of the report
// ($ for USD, the euro sign for EUR, two decimals, '.' separator) and that it
// prints the reason next to each model left unpriced by a currency mismatch.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const euro = "€"

func cents(c int64) domain.MoneyValue {
	return domain.KnownMoney(domain.Money(c * 10_000_000))
}

func currencyTotals(c int64) domain.Totals {
	return domain.Totals{Money: domain.CategoryMoney{Total: cents(c), Complete: true}}
}

// currencyReport is a report with one run, an agent and an unattributable bucket,
// each carrying money, in the given currency.
func currencyReport(cur domain.CurrencyCode) domain.Report {
	run := domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Totals:       currencyTotals(1234),
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: currencyTotals(234)},
		Agents: []domain.ActorReport{
			{Actor: domain.AgentInstance("builder#1"), Totals: currencyTotals(1000)},
		},
	}
	run.Cache.Rewrites = domain.RewriteFigures{
		ColdIdle: 1, Tokens: domain.Tokens(5000),
		Cost: cents(250), ExtraCost: cents(100), Complete: true,
	}
	return domain.Report{
		Source:   domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"},
		Currency: cur,
		AllRuns:  currencyTotals(1234),
		Runs:     []domain.RunReport{run},
		Unattributable: &domain.RunReport{
			Run:    domain.UnattributableRun(),
			Totals: currencyTotals(99),
		},
	}
}

func textOf(t *testing.T, r domain.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeText(&buf, r); err != nil {
		t.Fatalf("EncodeText: %v", err)
	}
	return buf.String()
}

func TestEncodeText_EURReport_ShowsEverySumInEuro(t *testing.T) {
	out := textOf(t, currencyReport(domain.CurrencyEUR))

	for _, want := range []string{
		"Money: " + euro + "12.34", // run and all-runs total
		"Money: " + euro + "2.34",  // orchestrator
		"Money: " + euro + "10.00", // agent
		"Money: " + euro + "0.99",  // unattributable
		"cost " + euro + "2.50",    // rewrite cost
		"extra " + euro + "1.00",   // rewrite extra cost
	} {
		if !strings.Contains(out, want) {
			t.Errorf("EUR text report must contain %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "$") {
		t.Errorf("EUR text report must not contain a dollar sign\n%s", out)
	}
}

func TestEncodeText_USDReport_ShowsDollars(t *testing.T) {
	out := textOf(t, currencyReport(domain.CurrencyUSD))

	for _, want := range []string{"Money: $12.34", "Money: $10.00", "cost $2.50", "extra $1.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("USD text report must contain %q\n%s", want, out)
		}
	}
	if strings.Contains(out, euro) {
		t.Errorf("USD text report must not contain the euro sign\n%s", out)
	}
}

func TestEncodeText_EURReport_NeverUsesTheAsciiForm(t *testing.T) {
	out := textOf(t, currencyReport(domain.CurrencyEUR))

	if strings.Contains(out, "EUR ") {
		t.Errorf("the text report has no glyph mode and must always use the euro sign\n%s", out)
	}
}

func TestEncodeText_EURReport_NegativeAmountKeepsTheSignBeforeTheSymbol(t *testing.T) {
	r := currencyReport(domain.CurrencyEUR)
	r.AllRuns = currencyTotals(-123)

	out := textOf(t, r)

	if !strings.Contains(out, "Money: -"+euro+"1.23") {
		t.Errorf("a negative amount must read -%s1.23\n%s", euro, out)
	}
}

func TestEncodeText_EURReport_UnpricedAndNoDataMarkersAreUnchanged(t *testing.T) {
	r := currencyReport(domain.CurrencyEUR)
	r.AllRuns = domain.Totals{Money: domain.CategoryMoney{Total: domain.UnpricedMoney()}}
	r.Runs[0].Totals = domain.Totals{}

	out := textOf(t, r)

	if !strings.Contains(out, "Money: unpriced") || !strings.Contains(out, "Money: -\n") {
		t.Errorf("unpriced and no-data markers must stay as they are\n%s", out)
	}
}

func TestEncodeText_ReportWithoutCurrency_ReadsAsDollars(t *testing.T) {
	out := textOf(t, currencyReport(""))

	if !strings.Contains(out, "Money: $12.34") {
		t.Errorf("a report that sets no currency is USD\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Unpriced models and their reasons
// ---------------------------------------------------------------------------

const mismatchReason = "priced only in the global file (/home/u/.config/MOSAIC/LogAnalyzer/pricing.yaml) " +
	"in USD; the project file's currency is EUR"

func unpricedReport(reasons ...domain.UnpricedReason) domain.Report {
	r := currencyReport(domain.CurrencyEUR)
	r.UnpricedModels = []domain.PricingKey{
		{Model: "claude-a"},
		{Provider: "github-copilot", Model: "gpt-b"},
		{Model: "claude-c"},
	}
	r.UnpricedReasons = reasons
	return r
}

func unpricedList(out string) []string {
	const heading = "Unpriced models (no pricing entry found):"
	idx := strings.Index(out, heading)
	if idx < 0 {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(out[idx+len(heading):], "\n")[1:] {
		if !strings.HasPrefix(l, "  ") {
			break
		}
		lines = append(lines, l)
	}
	return lines
}

func TestEncodeText_UnpricedModelWithReason_ShowsKeyAndReasonOnOneLine(t *testing.T) {
	r := unpricedReport(domain.UnpricedReason{Key: domain.PricingKey{Model: "claude-a"}, Reason: mismatchReason})

	lines := unpricedList(textOf(t, r))

	want := "  claude-a: " + mismatchReason
	if len(lines) != 3 || lines[0] != want {
		t.Errorf("unpriced list = %q, want first line %q and three lines", lines, want)
	}
}

func TestEncodeText_UnpricedModelsWithoutReason_AreListedAsBefore(t *testing.T) {
	r := unpricedReport(domain.UnpricedReason{Key: domain.PricingKey{Model: "claude-a"}, Reason: mismatchReason})

	lines := unpricedList(textOf(t, r))

	if len(lines) != 3 {
		t.Fatalf("unpriced list = %q, want three lines", lines)
	}
	if lines[1] != "  "+r.UnpricedModels[1].String() || lines[2] != "  claude-c" {
		t.Errorf("models without a reason must print their key only, in order; got %q", lines[1:])
	}
}

func TestEncodeText_UnpricedReasons_AppearOnlyBesideTheirOwnModel(t *testing.T) {
	r := unpricedReport(
		domain.UnpricedReason{Key: domain.PricingKey{Model: "claude-c"}, Reason: "reason-for-c"},
		domain.UnpricedReason{Key: domain.PricingKey{Provider: "github-copilot", Model: "gpt-b"}, Reason: "reason-for-b"},
	)

	lines := unpricedList(textOf(t, r))

	want := []string{
		"  claude-a",
		"  " + r.UnpricedModels[1].String() + ": reason-for-b",
		"  claude-c: reason-for-c",
	}
	if len(lines) != len(want) {
		t.Fatalf("unpriced list = %q, want %q", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestEncodeText_UnpricedModelsWithoutAnyReason_PrintNoColon(t *testing.T) {
	r := unpricedReport()

	lines := unpricedList(textOf(t, r))

	if len(lines) != 3 {
		t.Fatalf("unpriced list = %q, want three lines", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, ":") {
			t.Errorf("line %q must carry no reason", l)
		}
	}
}

func TestEncodeText_ReasonForAModelNotInTheUnpricedList_IsNotPrinted(t *testing.T) {
	r := unpricedReport(domain.UnpricedReason{Key: domain.PricingKey{Model: "other-model"}, Reason: "stray-reason"})

	out := textOf(t, r)

	if strings.Contains(out, "stray-reason") {
		t.Errorf("a reason belongs beside its model in the unpriced list only\n%s", out)
	}
}
