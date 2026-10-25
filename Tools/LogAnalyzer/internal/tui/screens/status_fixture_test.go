package screens_test

// Fixtures and helpers for the by-status and tools view tests. The group and
// tool figures are set directly so each test verifies that the screen renders
// the analysed figures. Every amount in statusRun is unique, so a row can be
// found by its amount.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"mosaic-log-analyzer/internal/domain"
)

func statusGroup(kind domain.StatusKind, code string, n int, cents int64) domain.StatusGroup {
	return domain.StatusGroup{
		Kind: kind, Code: code,
		Figures: domain.GroupFigures{Invocations: n, Totals: moneyTotals(cents)},
	}
}

func openStatusGroup(n int, cents int64) domain.StatusGroup {
	g := statusGroup(domain.StatusOpen, "", n, cents)
	g.Figures.Open = n
	return g
}

// statusRun has five run-level groups in contract order (two enum codes, one
// non-enum code, no status, open) and three agent types.
func statusRun() domain.RunReport {
	run := overviewRun()
	run.ByStatus = domain.StatusBreakdown{
		Run: []domain.StatusGroup{
			statusGroup(domain.StatusReported, "SUCCESS", 7, 1234),
			statusGroup(domain.StatusReported, "BLOCKED", 2, 250),
			statusGroup(domain.StatusReported, "WEIRD_CODE", 1, 77),
			statusGroup(domain.StatusNone, "", 3, 456),
			openStatusGroup(2, 111),
		},
		ByAgentType: []domain.AgentTypeStatus{
			{AgentType: "builder", Groups: []domain.StatusGroup{
				statusGroup(domain.StatusReported, "SUCCESS", 3, 707),
				statusGroup(domain.StatusNone, "", 1, 40),
			}},
			{AgentType: "reviewer", Groups: []domain.StatusGroup{
				statusGroup(domain.StatusReported, "SUCCESS", 4, 303),
				openStatusGroup(1, 9),
			}},
			{AgentType: "", Groups: []domain.StatusGroup{
				statusGroup(domain.StatusReported, "BLOCKED", 1, 88),
			}},
		},
	}
	return run
}

// manyStatusTypesRun has n agent types type-01 ... with one SUCCESS group each.
func manyStatusTypesRun(n int) domain.RunReport {
	run := overviewRun()
	for i := 1; i <= n; i++ {
		run.ByStatus.ByAgentType = append(run.ByStatus.ByAgentType, domain.AgentTypeStatus{
			AgentType: fmt.Sprintf("type-%02d", i),
			Groups:    []domain.StatusGroup{statusGroup(domain.StatusReported, "SUCCESS", 1, int64(i))},
		})
	}
	return run
}

// hasToken reports whether the line holds the whitespace-separated token.
func hasToken(line, token string) bool {
	for _, f := range strings.Fields(line) {
		if f == token {
			return true
		}
	}
	return false
}

// rowFor returns the line holding needle and fails the test when there is none.
func rowFor(t *testing.T, view, needle string) string {
	t.Helper()
	line := lineWith(view, needle)
	if line == "" {
		t.Fatalf("no line contains %q\n%s", needle, view)
	}
	return line
}

func assertFitsWidth(t *testing.T, view string, width int) {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > width {
			t.Errorf("line is %d columns wide, want at most %d: %q", w, width, l)
		}
	}
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }
