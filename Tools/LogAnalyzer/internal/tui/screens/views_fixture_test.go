package screens_test

// Shared fixtures and key helpers for the per-run view screen tests.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	wideW, tallH = 120, 60
	shortH       = 8
)

// Compile-time proof that the per-run view screens satisfy RunView.
var (
	_ screens.RunView = (*screens.RunDetailScreen)(nil)
	_ screens.RunView = (*screens.OverviewScreen)(nil)
	_ screens.RunView = (*screens.AgentTypesScreen)(nil)
)

var scrollHintRE = regexp.MustCompile(`\d+-\d+ of \d+`)

func plain() screens.Styles { return screens.Styles{} }

// usd converts whole cents to Money.
func usd(cents int64) domain.Money { return domain.Money(cents * 10_000_000) }

func known(cents int64) domain.MoneyValue { return domain.KnownMoney(usd(cents)) }

func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

func press(s screens.Screen, names ...string) screens.Screen {
	for _, n := range names {
		s, _ = s.Update(keyMsg(n))
	}
	return s
}

func repeat(name string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = name
	}
	return out
}

func lineCount(view string) int { return strings.Count(view, "\n") + 1 }

func assertContains(t *testing.T, view string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(view, w) {
			t.Errorf("view does not contain %q\n%s", w, view)
		}
	}
}

func assertNotContains(t *testing.T, view string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(view, u) {
			t.Errorf("view must not contain %q\n%s", u, view)
		}
	}
}

func assertFits(t *testing.T, view string, height int) {
	t.Helper()
	if n := lineCount(view); n > height {
		t.Errorf("view has %d lines, want at most %d\n%s", n, height, view)
	}
}

// overviewRun returns a fully priced run whose V1 figures are all distinct:
// run total $10.00 (orchestrator $4.00 = 40.0%, subagents $6.00 = 60.0%), cache
// hit ratio 3000/4500 = 66.7%, two priced models and one request without a model.
func overviewRun() domain.RunReport {
	total := domain.Totals{
		Tokens: domain.TokenUsage{
			Input:         domain.Tokens(1000),
			CacheRead:     domain.Tokens(3000),
			CacheCreation: domain.Tokens(500),
			Output:        domain.Tokens(200),
		},
		Money: domain.CategoryMoney{
			Input: known(111), CacheRead: known(222), CacheCreation: known(333), Output: known(334),
			Total: known(1000), Complete: true,
		},
	}
	moneyOnly := func(cents int64) domain.Totals {
		return domain.Totals{Money: domain.CategoryMoney{Total: known(cents), Complete: true}}
	}
	run := domain.RunReport{
		Run:          domain.NamedRun("20260101T120000Z-abcd"),
		Totals:       total,
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: moneyOnly(400)},
	}
	run.Overview = domain.RunOverview{
		Totals:            total,
		Orchestrator:      moneyOnly(400),
		Subagents:         domain.GroupFigures{Invocations: 3, Totals: moneyOnly(600)},
		OrchestratorShare: domain.Ratio{Numerator: 400_000_000_0, Denominator: 1_000_000_000_0},
		SubagentShare:     domain.Ratio{Numerator: 600_000_000_0, Denominator: 1_000_000_000_0},
		CacheHitRatio:     domain.Ratio{Numerator: 3000, Denominator: 4500},
		ByModel: []domain.ModelFigures{
			{Model: "model-a", Priced: true, Requests: 3, Totals: moneyOnly(650)},
			{Model: "model-b", Priced: true, Requests: 2, Totals: moneyOnly(275)},
			{Model: "", Priced: true, Requests: 1, Totals: moneyOnly(75)},
		},
	}
	return run
}

// manyModelsRun returns an overview run with n models named model-01 ...
func manyModelsRun(n int) domain.RunReport {
	run := overviewRun()
	run.Overview.ByModel = nil
	for i := 1; i <= n; i++ {
		run.Overview.ByModel = append(run.Overview.ByModel, domain.ModelFigures{
			Model: domain.ModelID(fmt.Sprintf("model-%02d", i)), Priced: true, Requests: 1,
			Totals: domain.Totals{Money: domain.CategoryMoney{Total: known(int64(i)), Complete: true}},
		})
	}
	return run
}
