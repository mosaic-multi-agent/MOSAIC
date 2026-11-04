package cli_test

// The rewrite line of the text report (EncodeText).
//
// Rules under test:
//   - A run with rewrites gets one line after its totals and before its
//     orchestrator section:
//     "Context rewrites: <n> (model changed <a>, cold after idle <b>, partial <c>),
//     <tokens> tokens, cost <money>, extra <money>".
//   - A run without rewrites gets no such line.
//   - Unpriced rewrites show the unpriced word, never $0.00.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func textRewriteRun(f domain.RewriteFigures) domain.RunReport {
	run := domain.RunReport{
		Run:    domain.NamedRun(cliTestRunID),
		Totals: domain.Totals{Money: domain.CategoryMoney{Total: domain.KnownMoney(domain.Money(2_000_000_000)), Complete: true}},
	}
	run.Cache.Rewrites = f
	return run
}

func textRewriteLine(t *testing.T, out string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Context rewrites:") {
			return strings.TrimSpace(l)
		}
	}
	t.Fatalf("no \"Context rewrites:\" line in the output:\n%s", out)
	return ""
}

func TestEncodeText_RunWithRewrites_ShowsCountsTokensCostAndExtraOnOneLine(t *testing.T) {
	f := domain.RewriteFigures{
		ModelChanged: 1, ColdIdle: 2, Partial: 3,
		Tokens: domain.Tokens(123_456),
		Cost:   domain.KnownMoney(domain.Money(6_370_000_000)), ExtraCost: domain.KnownMoney(domain.Money(5_910_000_000)),
		Complete: true,
	}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{textRewriteRun(f)}}

	line := textRewriteLine(t, encodeTextString(t, report))

	want := "Context rewrites: 6 (model changed 1, cold after idle 2, partial 3), 123,456 tokens, cost $6.37, extra $5.91"
	if line != want {
		t.Errorf("line = %q\nwant   %q", line, want)
	}
}

func TestEncodeText_RewriteLineFollowsTheRunTotalsAndPrecedesTheOrchestrator(t *testing.T) {
	f := domain.RewriteFigures{
		ColdIdle: 1, Tokens: domain.Tokens(10),
		Cost: domain.KnownMoney(domain.Money(1_000_000_000)), ExtraCost: domain.KnownMoney(domain.Money(900_000_000)),
		Complete: true,
	}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{textRewriteRun(f)}}

	out := encodeTextString(t, report)

	runAt := strings.Index(out, "Run: ")
	if runAt < 0 {
		t.Fatalf("no run section:\n%s", out)
	}
	moneyAt := strings.Index(out[runAt:], "Money:") + runAt
	rewriteAt := strings.Index(out, "Context rewrites:")
	orchAt := strings.Index(out, "Orchestrator:")
	if rewriteAt < 0 || orchAt < 0 || !(runAt < moneyAt && moneyAt < rewriteAt && rewriteAt < orchAt) {
		t.Errorf("order wrong (run %d, money %d, rewrites %d, orchestrator %d):\n%s", runAt, moneyAt, rewriteAt, orchAt, out)
	}
}

func TestEncodeText_RunWithoutRewrites_HasNoRewriteLine(t *testing.T) {
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{textRewriteRun(domain.RewriteFigures{})}}

	out := encodeTextString(t, report)

	if strings.Contains(out, "Context rewrites") {
		t.Errorf("output names rewrites for a run without any:\n%s", out)
	}
}

func TestEncodeText_UnpricedRewrites_ShowUnpricedNotZeroDollars(t *testing.T) {
	f := domain.RewriteFigures{
		ColdIdle: 1, Tokens: domain.Tokens(100_000),
		Cost: domain.UnpricedMoney(), ExtraCost: domain.UnpricedMoney(),
	}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{textRewriteRun(f)}}

	line := textRewriteLine(t, encodeTextString(t, report))

	if !strings.Contains(line, "cost unpriced") || !strings.Contains(line, "extra unpriced") || strings.Contains(line, "$") {
		t.Errorf("line = %q, want cost and extra shown as unpriced", line)
	}
}
