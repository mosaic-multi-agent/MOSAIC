package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// scrollKeys are the short help keys of the views that only scroll.
var scrollKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, exportKeyEntry, {Key: "esc", Desc: "back"}}

// bodyBudget returns how many body lines fit on a terminal of the given height.
func bodyBudget(height int) int { return FrameBodyHeight(height) }

// clip renders text in the style, truncated to the width so a row never wraps
// and the view's line count stays predictable.
func clip(style lipgloss.Style, width int, text string) string {
	if width <= 0 {
		return style.Render(text)
	}
	return style.MaxWidth(width).Render(text)
}

// renderPage assembles the plain page of the across-runs screen: title, label,
// the visible body lines, a separator and the help bar with the scroll hint.
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
		help = hint + "  " + help // first, so clipping a long help line keeps it
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
	case KeyViewContext:
		id = ScreenContext
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

// formatMoneyTotal renders a CategoryMoney total; the partial marker is added
// only to a known amount, never to an unpriced or absent one.
func formatMoneyTotal(cur domain.CurrencyCode, c domain.CategoryMoney) string {
	if c.Total.State == domain.MoneyKnown {
		return FormatTotal(cur, c)
	}
	return FormatMoney(cur, c.Total)
}

// incompleteness lists the labels for unpriced models, assumed cache-write tier,
// missing token usage and open invocations that apply to a figure.
func incompleteness(unpriced []domain.PricingKey, assumed int64, tiers []domain.CacheWriteTier, missing, open int) []string {
	var out []string
	if len(unpriced) > 0 {
		names := make([]string, len(unpriced))
		for i, m := range unpriced {
			names[i] = m.String()
		}
		out = append(out, UnpricedMarker+": "+strings.Join(names, ", "))
	}
	if s := FormatAssumedTier(assumed, tiers); s != "" {
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

// appendLabels adds bracketed incompleteness labels to a row.
func appendLabels(row string, labels []string) string {
	if len(labels) == 0 {
		return row
	}
	return row + "  [" + strings.Join(labels, "; ") + "]"
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
