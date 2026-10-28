package screens

import (
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"

	"mosaic-log-analyzer/internal/domain"
)

// helpOverlay is the state of the ? overlay of one per-run view. While it is
// open it takes every key; its scroll position is independent of the body's.
type helpOverlay struct {
	open   bool
	offset int
}

// handle consumes a key meant for the overlay and reports whether it did: the
// help key opens it, and while open every key is consumed so nothing leaks to
// the view underneath.
func (h *helpOverlay) handle(key string) bool {
	if !h.open {
		if key != KeyHelp {
			return false
		}
		h.open, h.offset = true, 0
		return true
	}
	switch key {
	case KeyHelp, "esc":
		h.open = false
	case "up", "k":
		h.offset = max(h.offset-1, 0)
	case "down", "j":
		h.offset++
	}
	return true
}

// overlayGroups lists every key of a per-run view; escDesc says what esc does
// on the current screen and screenKeys are the keys only this screen has.
func overlayGroups(escDesc string, screenKeys []tuicommon.HelpEntry) []chrome.HelpGroup {
	groups := []chrome.HelpGroup{
		{Title: "Views", Entries: []tuicommon.HelpEntry{
			{Key: KeyViewDispatches, Desc: "dispatches of the run"},
			{Key: KeyViewOverview, Desc: "overview"},
			{Key: KeyViewAgentTypes, Desc: "by agent type"},
			{Key: KeyViewStatus, Desc: "by returned status code"},
			{Key: KeyViewTools, Desc: "tools"},
			{Key: KeyViewTimeline, Desc: "timeline"},
			{Key: KeyViewContext, Desc: "context growth per agent"},
		}},
		{Title: "Export", Entries: []tuicommon.HelpEntry{
			{Key: KeyExportRun, Desc: "export this run as a digest file"},
			{Key: KeyExportAll, Desc: "export all runs as a digest file"},
		}},
	}
	if len(screenKeys) > 0 {
		groups = append(groups, chrome.HelpGroup{Title: "This screen", Entries: screenKeys})
	}
	return append(groups, chrome.HelpGroup{Title: "Navigation", Entries: []tuicommon.HelpEntry{
		{Key: "↑/↓ j/k", Desc: "scroll, or move the selection"},
		{Key: "enter", Desc: "open the selected row where the view has one"},
		{Key: "esc", Desc: escDesc},
		{Key: "q", Desc: "same as esc"},
		{Key: KeyHelp, Desc: "show or hide this help"},
		{Key: "ctrl+c", Desc: "quit"},
	}})
}

// sortKeyEntry is the overlay entry of the sort key of a table view.
var sortKeyEntry = tuicommon.HelpEntry{Key: KeySort, Desc: "cycle the sort order"}

// linesIn renders the overlay groups into body lines within bodyHeight lines,
// clamping the scroll offset to the content.
func (h *helpOverlay) linesIn(groups []chrome.HelpGroup, width, bodyHeight int, theme tuicommon.Theme) []string {
	if bodyHeight == 0 {
		return nil
	}
	total := strings.Count(chrome.HelpOverlay(helpTitle, groups, 0, width, 1<<20, theme), "\n") + 1
	h.offset = min(h.offset, max(total-(bodyHeight-1), 0))
	return strings.Split(chrome.HelpOverlay(helpTitle, groups, h.offset, width, bodyHeight, theme), "\n")
}

const helpTitle = "Keys"

// runChrome is the state every per-run view shares: the findings of its run for
// the header and the help overlay.
type runChrome struct {
	quality RunQuality
	help    helpOverlay
}

// SetRunQuality sets the findings shown in the frame header.
func (c *runChrome) SetRunQuality(q RunQuality) { c.quality = q }

// HelpOpen reports whether the help overlay is showing.
func (c *runChrome) HelpOpen() bool { return c.help.open }

// overlayKeys is the short help while the overlay is open.
var overlayKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: "esc", Desc: "close"}}

// frameSpec is what a per-run view hands the shared frame renderer.
type frameSpec struct {
	run        domain.RunReport
	active     ScreenID
	keys       []tuicommon.HelpEntry // short help keys of the view
	screenKeys []tuicommon.HelpEntry // keys only this view has, for the overlay
	escDesc    string                // what esc does, for the overlay
	hint       string                // scroll hint
	sort       string                // active sort label of a table view, may be ""
}

// render draws the view's body, or the help overlay in its place, in the frame.
func (c *runChrome) render(spec frameSpec, body []string, width, height int, st Styles) string {
	f := Frame{Run: spec.run, Quality: c.quality, Active: spec.active, Keys: spec.keys, Hint: spec.hint, Sort: spec.sort}
	if c.help.open {
		f.Keys, f.Hint = overlayKeys, ""
		body = c.help.linesIn(overlayGroups(spec.escDesc, spec.screenKeys), width, FrameBodyHeight(height), st.Theme)
	}
	return RenderFrame(f, body, width, height, st)
}
