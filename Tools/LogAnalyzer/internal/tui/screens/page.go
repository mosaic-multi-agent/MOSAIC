package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
	"mosaic-common/tui/kit/textwidth"
)

// PageChromeLines is the fixed line count of the page frame: header, separator,
// help.
const PageChromeLines = 3

// PageBodyHeight is the number of body lines available at the given height.
func PageBodyHeight(height int) int { return max(height-PageChromeLines, 0) }

// PageBadge is one badge shown after the page title.
type PageBadge struct {
	Text string
	Role tuicommon.StyleRole
}

// Page describes the chrome of a screen that is not a per-run view: no run, no
// tab bar.
type Page struct {
	Title  string                // existing screen title text, RoleTitle
	Badges []PageBadge           // after the title, rendered as badges
	Keys   []tuicommon.HelpEntry // this screen's own keys for the short help line
	Help   bool                  // true: "? help" ends the help line
	Hint   string                // scroll hint, may be ""
}

// RenderPage returns exactly min(height, PageChromeLines+len(body)) lines, none
// wider than width: header line, separator, body cut to PageBodyHeight, help
// line.
func RenderPage(p Page, body []string, width, height int, st Styles) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	badges := make([]headerBadge, len(p.Badges))
	for i, b := range p.Badges {
		badges[i] = headerBadge{b.Text, b.Role}
	}
	header := badgeHeader(p.Title, badges, width, st.Theme)
	separator := chrome.SectionHeader("", width, st.Theme)
	help := st.Help.Render(pageHelpLine(p, width))
	if height < PageChromeLines {
		return strings.Join([]string{header, separator, help}[:height], "\n")
	}
	lines := []string{header, separator}
	for _, l := range body[:min(len(body), PageBodyHeight(height))] {
		lines = append(lines, clip(lipgloss.NewStyle(), width, l))
	}
	return strings.Join(append(lines, help), "\n")
}

// badgeHeader renders the title followed by the badges that fit, in order.
func badgeHeader(title string, badges []headerBadge, width int, theme tuicommon.Theme) string {
	if textwidth.Of(title) >= width {
		return theme.Style(tuicommon.RoleTitle).Render(textwidth.Truncate(title, width))
	}
	line := theme.Style(tuicommon.RoleTitle).Render(title)
	remaining := width - textwidth.Of(title)
	for _, b := range badges {
		need := textwidth.Of(b.text) + 3 // brackets and the separating space
		if need > remaining {
			continue
		}
		line += " " + chrome.Badge(b.text, b.role, need-1, theme)
		remaining -= need
	}
	return line
}

// pageHelpLine fits the page's short help into width, dropping whole entries by
// priority; the help hint, if the page offers one, stays.
func pageHelpLine(p Page, width int) string {
	return fitFooter(footerEntries(p.Hint, p.Keys), p.Help, width)
}

// pageChrome is the state every page screen shares: the help overlay.
type pageChrome struct {
	configHelp
	help helpOverlay
}

// HelpOpen reports whether the help overlay is showing.
func (c *pageChrome) HelpOpen() bool { return c.help.open }

// render draws the page's body, or the help overlay in its place.
func (c *pageChrome) render(p Page, groups []chrome.HelpGroup, body []string, width, height int, st Styles) string {
	if c.help.open {
		p.Keys, p.Hint = overlayKeys, ""
		body = c.help.linesIn(c.withConfigGroup(groups), width, PageBodyHeight(height), st.Theme)
	}
	return RenderPage(p, body, width, height, st)
}
