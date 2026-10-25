package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// AcrossRunsScreen shows the figures across all named runs (V6): the runs side
// by side with their totals, then each agent type summed over the runs with its
// per-run averages. The unattributable bucket is never shown.
type AcrossRunsScreen struct {
	across domain.AcrossRuns
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
}

// NewAcrossRunsScreen builds the across-runs screen for the report.
func NewAcrossRunsScreen(report domain.Report, width, height int, styles Styles) *AcrossRunsScreen {
	return &AcrossRunsScreen{across: report.AcrossRuns, width: width, height: height, styles: styles}
}

// ID implements Screen.
func (s *AcrossRunsScreen) ID() ScreenID { return ScreenAcrossRuns }

// Init implements Screen.
func (s *AcrossRunsScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *AcrossRunsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch k := key.String(); k {
	case "esc", "q":
		s.back = true
	case "up", "k", "down", "j":
		s.scroll.handle(k, len(s.body()), s.height)
	}
	return s, nil
}

// View implements Screen.
func (s *AcrossRunsScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	return renderPage(s.styles, width, "Log Analyzer — Across Runs", fmt.Sprintf("%d named run(s)", len(s.across.Runs)),
		body[w.Offset:w.End()], w, len(body), s.Help())
}

// body renders every line of the view; the caller scrolls through it.
func (s *AcrossRunsScreen) body() []string {
	st, width := s.styles, s.width
	if len(s.across.Runs) == 0 {
		return []string{clip(st.Muted, width, "No named runs.")}
	}
	out := []string{clip(st.Subtitle, width, "Runs")}
	for _, r := range s.across.Runs {
		out = append(out, clip(st.Body, width, "  "+runSummaryRow(r)))
	}
	out = append(out, "", clip(st.Subtitle, width, "Agent types across runs"))
	if len(s.across.ByAgentType) == 0 {
		out = append(out, clip(st.Muted, width, "  No subagent invocations."))
	}
	for _, g := range s.across.ByAgentType {
		row, perRun := acrossTypeRows(g)
		out = append(out, clip(st.Body, width, "  "+row), clip(st.Muted, width, perRun))
	}
	return out
}

// runSummaryRow renders one run: id, total and subagent count, followed by the
// labels of any incomplete data.
func runSummaryRow(r domain.RunSummary) string {
	label := r.Run.Label()
	if r.Provisional {
		label += " (" + ProvisionalLabel + ")"
	}
	row := fmt.Sprintf("%s  total %s  %d subagents", label, formatMoneyTotal(r.Totals.Money), r.Subagents.Invocations)
	return appendLabels(row, incompleteness(r.UnpricedModels, r.Totals.AssumedTierCacheWrite,
		r.Subagents.MissingUsage, r.Subagents.Open))
}

// acrossTypeRows renders one agent type as a figures row, with the labels of
// any incomplete data, and a line of per-run averages.
func acrossTypeRows(g domain.AgentTypeAcrossRuns) (row, perRun string) {
	name := g.AgentType
	if name == "" {
		name = UnknownTypeLabel
	}
	avg := FormatMoney(g.PerRunMoney.Average)
	if g.PerRunMoney.Basis > 0 && !g.PerRunMoney.Complete && g.Figures.Totals.Money.Complete {
		avg += " (partial)"
	}
	row = fmt.Sprintf("%-14s %3d inv, %d runs  total %s  avg/run %s",
		name, g.Figures.Invocations, g.Runs, formatMoneyTotal(g.Figures.Totals.Money), avg)
	row = appendLabels(row, incompleteness(g.Figures.UnpricedModels, g.Figures.Totals.AssumedTierCacheWrite,
		g.Figures.MissingUsage, g.Figures.Open))
	perRun = fmt.Sprintf("      per run: %s invocations, %s tokens",
		formatPerRun(g.PerRunInvocations, 1), formatPerRun(g.PerRunTokens, 0))
	if g.ProvisionalRuns > 0 {
		perRun += fmt.Sprintf("; %d of %d runs %s", g.ProvisionalRuns, g.Runs, ProvisionalLabel)
	}
	return row, perRun
}

// formatPerRun renders a per-run ratio as a decimal number; AbsentMarker when undefined.
func formatPerRun(r domain.Ratio, decimals int) string {
	if r.Denominator <= 0 {
		return AbsentMarker
	}
	return fmt.Sprintf("%.*f", decimals, float64(r.Numerator)/float64(r.Denominator))
}

// Help implements Screen.
func (s *AcrossRunsScreen) Help() string { return "↑/↓ scroll  esc back  ctrl+c quit" }

// Back reports whether the user asked to leave the view.
func (s *AcrossRunsScreen) Back() bool { return s.back }

// Requested implements RunView; the across-runs view has no view keys.
func (s *AcrossRunsScreen) Requested() (ScreenID, bool) { return "", false }

// Reset clears Back.
func (s *AcrossRunsScreen) Reset() { s.back = false }
