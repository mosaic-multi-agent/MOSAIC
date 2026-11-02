package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// RunQuality is the finding summary of one run shown in the frame header.
type RunQuality struct {
	Findings int  // findings whose Run equals the run
	DataLoss bool // any of them is a data-loss kind
}

// RunQualityOf summarises the findings of the given run.
func RunQualityOf(q domain.QualitySummary, run domain.RunRef) RunQuality {
	var own []domain.Finding
	for _, f := range q.Findings {
		if f.Run == run {
			own = append(own, f)
		}
	}
	if len(own) == 0 {
		return RunQuality{}
	}
	return RunQuality{Findings: len(own), DataLoss: domain.NewQualitySummary(own).Incomplete()}
}

// FrameChromeLines is the fixed line count of the frame: header, tab bar,
// separator, help.
const FrameChromeLines = 4

// FrameBodyHeight is the number of body lines available at the given height.
func FrameBodyHeight(height int) int { return max(height-FrameChromeLines, 0) }

// Frame describes one per-run view's chrome.
type Frame struct {
	Run     domain.RunReport
	Quality RunQuality
	Active  ScreenID              // highlighted tab
	Keys    []tuicommon.HelpEntry // this screen's own keys for the short help line
	Hint    string                // scroll hint, may be ""
	Sort    string                // active sort label of a table view, may be ""
}

// frameTabs are the per-run views in tab order.
var frameTabs = []struct {
	id  ScreenID
	tab chrome.Tab
}{
	{ScreenRunDetail, chrome.Tab{Key: KeyViewDispatches, Label: "Dispatches"}},
	{ScreenOverview, chrome.Tab{Key: KeyViewOverview, Label: "Overview"}},
	{ScreenAgentTypes, chrome.Tab{Key: KeyViewAgentTypes, Label: "By type"}},
	{ScreenStatus, chrome.Tab{Key: KeyViewStatus, Label: "By status"}},
	{ScreenTools, chrome.Tab{Key: KeyViewTools, Label: "Tools"}},
	{ScreenTimeline, chrome.Tab{Key: KeyViewTimeline, Label: "Timeline"}},
	{ScreenContext, chrome.Tab{Key: KeyViewContext, Label: "Context"}},
}

// RenderFrame returns exactly min(height, FrameChromeLines+len(body)) lines,
// none wider than width; body is cut to FrameBodyHeight(height).
func RenderFrame(f Frame, body []string, width, height int, st Styles) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	header := frameHeader(f, width, st)
	tabs := frameTabBar(f, width, st.Theme)
	help := st.Help.Render(frameHelpLine(f, width))
	if height < FrameChromeLines {
		return strings.Join([]string{header, tabs, help}[:height], "\n")
	}
	lines := []string{header, tabs, chrome.SectionHeader("", width, st.Theme)}
	for _, l := range body[:min(len(body), FrameBodyHeight(height))] {
		lines = append(lines, clip(lipgloss.NewStyle(), width, l))
	}
	return strings.Join(append(lines, help), "\n")
}

// frameTabBar renders the per-run views with the active one highlighted.
func frameTabBar(f Frame, width int, theme tuicommon.Theme) string {
	tabs := make([]chrome.Tab, len(frameTabs))
	active := -1
	for i, t := range frameTabs {
		tabs[i] = t.tab
		if t.id == f.Active {
			active = i
		}
	}
	if line, ok := compactTabBar(tabs, active, width, theme); ok {
		return line
	}
	return chrome.TabBar(tabs, active, width, theme)
}

// compactTabBar draws every tab with a single space between them when the padded
// bar is too wide for width but the compact one fits, so all views stay listed
// at 80 columns. ok is false when the padded bar fits or the compact one does not.
func compactTabBar(tabs []chrome.Tab, active, width int, theme tuicommon.Theme) (line string, ok bool) {
	cells := make([]string, len(tabs))
	padded, compact := 0, len(tabs)-1
	for i, t := range tabs {
		text := t.Key + " " + t.Label
		padded += textwidth.Of(text) + 2
		compact += textwidth.Of(text)
		cells[i] = text
		if i == active {
			cells[i] = theme.Style(tuicommon.RoleSelected).Render("[" + text + "]")
			compact += 2
		}
	}
	if padded <= width || compact > width {
		return "", false
	}
	return strings.Join(cells, " "), true
}

type headerBadge struct {
	text string
	role tuicommon.StyleRole
}

// frameHeader renders the run label followed by the summary badges that fit.
func frameHeader(f Frame, width int, st Styles) string {
	return badgeHeader(f.Run.Run.Label(), headerBadges(f, st.Currency), width, st.Theme)
}

// headerBadges lists the header badges in display order: the in-progress marker,
// total cost, run duration, invocation count and, when present, the findings and
// the active sort of a table view.
func headerBadges(f Frame, cur domain.CurrencyCode) []headerBadge {
	var out []headerBadge
	if f.Run.Provisional {
		out = append(out, headerBadge{ProvisionalLabel, tuicommon.RoleSignalWarning})
	}
	money := f.Run.Totals.Money
	out = append(out, headerBadge{FormatTotal(cur, money), MoneyRole(money.Total, money.Complete)})

	tl := f.Run.Timeline
	durationRole := tuicommon.RoleDataTime
	if !tl.HasAxis {
		durationRole = tuicommon.RoleSignalMuted
	}
	out = append(out,
		headerBadge{FormatDuration(tl.Axis.End.Sub(tl.Axis.Start), tl.HasAxis), durationRole},
		headerBadge{countLabel(f.Run.Overview.Subagents.Invocations, "invocation"), tuicommon.RoleDataActor})

	if q := f.Quality; q.Findings > 0 {
		role := tuicommon.RoleSignalWarning
		if q.DataLoss {
			role = tuicommon.RoleSignalBad
		}
		out = append(out, headerBadge{countLabel(q.Findings, "finding"), role})
	}
	if f.Sort != "" {
		out = append(out, headerBadge{f.Sort, tuicommon.RoleSignalMuted})
	}
	return out
}

// countLabel renders "<n> <noun>" with the noun pluralised.
func countLabel(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// helpHintText is the closing entry of every short help line.
const helpHintText = KeyHelp + " help"

// shortHelpText joins a scroll hint, a screen's own keys and the help hint.
func shortHelpText(hint string, keys []tuicommon.HelpEntry) string {
	return fitFooter(footerEntries(hint, keys), true, 1<<30)
}

// frameHelpLine fits the short help into width, dropping whole entries by
// priority; the help hint stays.
func frameHelpLine(f Frame, width int) string {
	return fitFooter(footerEntries(f.Hint, f.Keys), true, width)
}
