package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// OverviewScreen is the run overview view (V1): total, orchestrator versus
// subagents, token categories, models and the cache hit ratio.
type OverviewScreen struct {
	run    domain.RunReport
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
}

// NewOverviewScreen creates the overview screen for one run.
func NewOverviewScreen(run domain.RunReport, width, height int, styles Styles) *OverviewScreen {
	return &OverviewScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenOverview}}
}

// ID implements Screen.
func (s *OverviewScreen) ID() ScreenID { return ScreenOverview }

// Init implements Screen.
func (s *OverviewScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *OverviewScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch k := key.String(); k {
	case "esc", "q":
		s.back = true
	case "up", "k", "down", "j":
		s.scroll.handle(k, len(s.body()), s.height)
	default:
		s.nav.request(k)
	}
	return s, nil
}

// View implements Screen.
func (s *OverviewScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	return renderPage(s.styles, width, "Log Analyzer — Run Overview", runLabel(s.run),
		body[w.Offset:w.End()], w, len(body), s.Help())
}

// body renders every line of the overview; the caller scrolls through it.
func (s *OverviewScreen) body() []string {
	ov := s.run.Overview
	st, width := s.styles, s.width
	line := func(text string) string { return clip(st.Body, width, text) }

	out := []string{
		line("Total             " + formatMoneyTotal(ov.Totals.Money)),
		line(fmt.Sprintf("Orchestrator      %s  %s", formatMoneyTotal(ov.Orchestrator.Money), FormatRatio(ov.OrchestratorShare))),
		line(fmt.Sprintf("Subagents         %s  %s  (%d invocations)",
			formatMoneyTotal(ov.Subagents.Totals.Money), FormatRatio(ov.SubagentShare), ov.Subagents.Invocations)),
	}
	if ov.Unattributed != nil {
		out = append(out, line(fmt.Sprintf("Unattributed      %s  %s",
			formatMoneyTotal(ov.Unattributed.Money), FormatRatio(ov.UnattributedShare))))
	}
	out = append(out, line("Cache hit ratio  "+FormatRatio(ov.CacheHitRatio)))
	for _, l := range incompleteness(s.run.UnpricedModels, ov.Totals.AssumedTierCacheWrite,
		ov.Subagents.MissingUsage, ov.Subagents.Open) {
		out = append(out, clip(st.Warning, width, "  "+l))
	}

	out = append(out, "", clip(st.Subtitle, width, "Token categories"))
	for _, cat := range domain.BillableCategories() {
		count, _ := ov.Totals.Tokens.Get(cat)
		out = append(out, line(fmt.Sprintf("  %-12s %12s  %s", cat.Label(), FormatTokens(count),
			FormatMoney(categoryMoney(ov.Totals.Money, cat)))))
		if cat == domain.CategoryOutput {
			// Reasoning is part of output: shown beneath it, never as a cost line.
			out = append(out, line(fmt.Sprintf("    %-10s %12s", domain.CategoryReasoning.Label(),
				FormatTokens(ov.Totals.Tokens.Reasoning))))
		}
	}

	out = append(out, "", clip(st.Subtitle, width, "By model"))
	for _, m := range ov.ByModel {
		out = append(out, line(modelRow(m)))
	}
	return out
}

func modelRow(m domain.ModelFigures) string {
	name := domain.PricingKey{Provider: m.Provider, Model: m.Model}.String()
	if m.Model == "" {
		name = UnknownModelLabel
	}
	money := FormatMoney(m.Totals.Money.Total)
	if !m.Priced && m.Totals.Money.Total.State != domain.MoneyUnpriced {
		money += " " + UnpricedMarker
	}
	return fmt.Sprintf("  %-24s %6d requests  %s", name, m.Requests, money)
}

// categoryMoney selects the money of one billable token category.
func categoryMoney(c domain.CategoryMoney, cat domain.TokenCategory) domain.MoneyValue {
	switch cat {
	case domain.CategoryInput:
		return c.Input
	case domain.CategoryCacheRead:
		return c.CacheRead
	case domain.CategoryCacheCreation:
		return c.CacheCreation
	case domain.CategoryOutput:
		return c.Output
	}
	return domain.NoMoneyData()
}

// Help implements Screen.
func (s *OverviewScreen) Help() string {
	return "↑/↓ scroll  " + viewKeysHelp(ScreenOverview) + "  esc back  ctrl+c quit"
}

// Back reports whether the user asked to leave the per-run views.
func (s *OverviewScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *OverviewScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *OverviewScreen) Reset() {
	s.back = false
	s.nav.reset()
}
