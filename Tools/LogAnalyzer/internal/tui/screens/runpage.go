package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"mosaic-log-analyzer/internal/domain"
)

// pageChromeLines is the number of lines a per-run view spends on its title,
// run label, separator and help bar; the rest of the terminal is body.
const pageChromeLines = 4

// bodyBudget returns how many body lines fit on a terminal of the given height.
func bodyBudget(height int) int {
	if b := height - pageChromeLines; b > 0 {
		return b
	}
	return 0
}

// clip renders text in the style, truncated to the width so a row never wraps
// and the view's line count stays predictable.
func clip(style lipgloss.Style, width int, text string) string {
	if width <= 0 {
		return style.Render(text)
	}
	return style.MaxWidth(width).Render(text)
}

// renderPage assembles a per-run view: title, run label, the visible body
// lines, a separator and the help bar with the scroll hint.
func renderPage(st Styles, width int, title, label string, visible []string, w Window, total int, help string) string {
	var sb strings.Builder
	sb.WriteString(clip(st.Title, width, title))
	sb.WriteByte('\n')
	sb.WriteString(clip(st.Subtitle, width, label))
	sb.WriteByte('\n')
	for _, l := range visible {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	sb.WriteString(clip(st.Border, width, strings.Repeat("─", max(width, 0))))
	sb.WriteByte('\n')
	if hint := FormatScrollHint(w, total); hint != "" {
		help += "  " + hint
	}
	sb.WriteString(clip(st.Help, width, help))
	return sb.String()
}

// runLabel is the run's label, marked while the run has no end event.
func runLabel(run domain.RunReport) string {
	label := run.Run.Label()
	if run.Provisional {
		label += " (" + ProvisionalLabel + ")"
	}
	return label
}

// viewNav records a request to switch to another per-run view.
type viewNav struct {
	self ScreenID
	req  ScreenID
}

// request records the view bound to key when it differs from the current one.
func (n *viewNav) request(key string) {
	var id ScreenID
	switch key {
	case KeyViewDispatches:
		id = ScreenRunDetail
	case KeyViewOverview:
		id = ScreenOverview
	case KeyViewAgentTypes:
		id = ScreenAgentTypes
	case KeyViewStatus:
		id = ScreenStatus
	case KeyViewTools:
		id = ScreenTools
	case KeyViewTimeline:
		id = ScreenTimeline
	default:
		return
	}
	if id != n.self {
		n.req = id
	}
}

func (n *viewNav) requested() (ScreenID, bool) { return n.req, n.req != "" }

func (n *viewNav) reset() { n.req = "" }

// viewKeysHelp lists the keys that switch to the per-run views other than self.
func viewKeysHelp(self ScreenID) string {
	entries := []struct {
		id   ScreenID
		key  string
		name string
	}{
		{ScreenRunDetail, KeyViewDispatches, "dispatches"},
		{ScreenOverview, KeyViewOverview, "overview"},
		{ScreenAgentTypes, KeyViewAgentTypes, "types"},
		{ScreenStatus, KeyViewStatus, "status"},
		{ScreenTools, KeyViewTools, "tools"},
		{ScreenTimeline, KeyViewTimeline, "timeline"},
	}
	var parts []string
	for _, e := range entries {
		if e.id != self {
			parts = append(parts, e.key+" "+e.name)
		}
	}
	return strings.Join(parts, "  ")
}

// formatMoneyTotal renders a CategoryMoney total; the partial marker is added
// only to a known amount, never to an unpriced or absent one.
func formatMoneyTotal(c domain.CategoryMoney) string {
	if c.Total.State == domain.MoneyKnown {
		return FormatTotal(c)
	}
	return FormatMoney(c.Total)
}

// incompleteness lists the labels for unpriced models, assumed cache-write tier,
// missing token usage and open invocations that apply to a figure.
func incompleteness(unpriced []domain.PricingKey, assumed int64, missing, open int) []string {
	var out []string
	if len(unpriced) > 0 {
		names := make([]string, len(unpriced))
		for i, m := range unpriced {
			names[i] = m.String()
		}
		out = append(out, UnpricedMarker+": "+strings.Join(names, ", "))
	}
	if s := FormatAssumedTier(assumed); s != "" {
		out = append(out, s)
	}
	if missing > 0 {
		out = append(out, fmt.Sprintf("%d %s", missing, MissingUsageLabel))
	}
	if open > 0 {
		out = append(out, fmt.Sprintf("%d %s", open, OpenLabel))
	}
	return out
}

// pageScroller keeps a plain line offset for views without a cursor.
type pageScroller struct{ offset int }

func (p *pageScroller) window(total, height int) Window {
	w := ScrollWindow(p.offset, -1, total, bodyBudget(height))
	p.offset = w.Offset
	return w
}

func (p *pageScroller) handle(key string, total, height int) {
	switch key {
	case "up", "k":
		p.offset--
	case "down", "j":
		p.offset++
	default:
		return
	}
	p.window(total, height)
}
