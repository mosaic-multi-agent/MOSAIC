package cli_test

// Tests for the reasoning line in the text encoder (EncodeText).
//
// Rules under test:
//   - Every token breakdown prints, directly after the output line and indented
//     two spaces deeper, "reasoning: <n> tokens", or "reasoning: -" when absent.
//   - Reasoning never gets a money figure and never changes a money figure.
//   - A partly reported figure shows the present sum with no extra marker.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func reasoningTotals(output int64, reasoning domain.TokenCount) domain.Totals {
	return domain.Totals{
		Tokens: domain.TokenUsage{
			Input:     domain.Tokens(100),
			Output:    domain.Tokens(output),
			Reasoning: reasoning,
		},
		Money: domain.CategoryMoney{
			Output: domain.KnownMoney(domain.Money(2_000_000_000)),
			Total:  domain.KnownMoney(domain.Money(3_000_000_000)),
		},
	}
}

func reasoningReport(totals domain.Totals) domain.Report {
	return domain.Report{
		Currency: domain.Currency,
		AllRuns:  totals,
		Runs: []domain.RunReport{{
			Run:          domain.NamedRun(cliTestRunID),
			Totals:       totals,
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: totals},
			Agents: []domain.ActorReport{{
				Actor:  domain.AgentInstance("Implementer#3"),
				Totals: totals,
			}},
		}},
	}
}

// linesAfterOutput returns, for every "output:" line in out, that line and the
// line that follows it.
func linesAfterOutput(out string) [][2]string {
	lines := strings.Split(out, "\n")
	var pairs [][2]string
	for i, l := range lines {
		if strings.Contains(l, "output:") && i+1 < len(lines) {
			pairs = append(pairs, [2]string{l, lines[i+1]})
		}
	}
	return pairs
}

func TestEncodeText_PresentReasoning_PrintsIndentedLineDirectlyAfterOutput(t *testing.T) {
	out := encodeTextString(t, reasoningReport(reasoningTotals(140, domain.Tokens(40))))

	pairs := linesAfterOutput(out)
	if len(pairs) != 4 {
		t.Fatalf("found %d output lines, want 4 (all runs, run, orchestrator, agent); output:\n%s", len(pairs), out)
	}
	for _, p := range pairs {
		indent := p[0][:strings.Index(p[0], "output:")]
		if want := indent + "  reasoning: 40 tokens"; p[1] != want {
			t.Errorf("line after %q = %q, want %q", p[0], p[1], want)
		}
	}
}

func TestEncodeText_AbsentReasoning_PrintsNotProvidedMarker(t *testing.T) {
	out := encodeTextString(t, reasoningReport(reasoningTotals(140, domain.AbsentTokens())))

	pairs := linesAfterOutput(out)
	if len(pairs) != 4 {
		t.Fatalf("found %d output lines, want 4; output:\n%s", len(pairs), out)
	}
	for _, p := range pairs {
		indent := p[0][:strings.Index(p[0], "output:")]
		if want := indent + "  reasoning: -"; p[1] != want {
			t.Errorf("line after %q = %q, want %q", p[0], p[1], want)
		}
	}
}

func TestEncodeText_PresentZeroReasoning_PrintsZeroNotMarker(t *testing.T) {
	out := encodeTextString(t, reasoningReport(reasoningTotals(30, domain.Tokens(0))))

	for _, p := range linesAfterOutput(out) {
		if !strings.HasSuffix(p[1], "reasoning: 0 tokens") {
			t.Errorf("line after %q = %q, want reasoning: 0 tokens", p[0], p[1])
		}
	}
}

func TestEncodeText_ReasoningGroupsThousands(t *testing.T) {
	out := encodeTextString(t, reasoningReport(reasoningTotals(2_000_000, domain.Tokens(1_234_567))))

	if !strings.Contains(out, "reasoning: 1,234,567 tokens") {
		t.Errorf("reasoning not printed with grouped digits; output:\n%s", out)
	}
}

func TestEncodeText_UnattributableBucket_ShowsReasoningUnderOutput(t *testing.T) {
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: reasoningTotals(140, domain.Tokens(40)),
		Agents: []domain.ActorReport{{
			Actor:  domain.AgentInstance("Orphan#1"),
			Totals: reasoningTotals(70, domain.Tokens(9)),
		}},
	}
	report := domain.Report{Currency: domain.Currency, Unattributable: &unattributable}

	out := encodeTextString(t, report)

	section := out[strings.Index(out, "=== Unattributable ==="):]
	if !strings.Contains(section, "    reasoning: 40 tokens") || !strings.Contains(section, "      reasoning: 9 tokens") {
		t.Errorf("unattributable totals and agent lack their reasoning lines; output:\n%s", out)
	}
}

func TestEncodeText_ReasoningLine_HasNoMoneyFigure(t *testing.T) {
	out := encodeTextString(t, reasoningReport(reasoningTotals(140, domain.Tokens(40))))

	var seen int
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "reasoning") {
			seen++
			if strings.Contains(l, "$") || strings.Contains(l, "unpriced") {
				t.Errorf("reasoning line carries a money figure: %q", l)
			}
		}
	}
	if seen == 0 {
		t.Errorf("no reasoning line printed; output:\n%s", out)
	}
}

func TestEncodeText_MoneyLinesIdenticalWithAndWithoutReasoning(t *testing.T) {
	with := encodeTextString(t, reasoningReport(reasoningTotals(140, domain.Tokens(40))))
	without := encodeTextString(t, reasoningReport(reasoningTotals(140, domain.AbsentTokens())))

	moneyLines := func(s string) []string {
		var out []string
		for _, l := range strings.Split(s, "\n") {
			if strings.Contains(l, "Money:") {
				out = append(out, l)
			}
		}
		return out
	}
	a, b := moneyLines(with), moneyLines(without)
	if len(a) == 0 || strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Errorf("money lines differ with reasoning data:\nwith:\n%v\nwithout:\n%v", a, b)
	}
	if !strings.Contains(with, "reasoning: 40 tokens") {
		t.Errorf("reasoning line missing from the run with reasoning data; output:\n%s", with)
	}
}

func TestEncodeText_PartlyReportedReasoning_ShowsPresentSumWithoutExtraMarker(t *testing.T) {
	partial := domain.SumUsage(
		domain.TokenUsage{Output: domain.Tokens(100), Reasoning: domain.Tokens(25)},
		domain.TokenUsage{Output: domain.Tokens(50)},
	)
	totals := reasoningTotals(0, domain.AbsentTokens())
	totals.Tokens = partial

	out := encodeTextString(t, reasoningReport(totals))

	for _, p := range linesAfterOutput(out) {
		indent := p[0][:strings.Index(p[0], "output:")]
		if want := indent + "  reasoning: 25 tokens"; p[1] != want {
			t.Errorf("line after %q = %q, want exactly %q", p[0], p[1], want)
		}
	}
}
