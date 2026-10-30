package screens

import (
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// footerSeparator joins the entries of a key-hint footer.
const footerSeparator = "  "

// exportKeyEntry is the footer entry every per-run view lists for its export key.
var exportKeyEntry = tuicommon.HelpEntry{Key: KeyExportRun, Desc: "export"}

// quitKeyEntry is the footer entry of the runs list's quit key.
var quitKeyEntry = tuicommon.HelpEntry{Key: "esc", Desc: "quit"}

// Drop tiers of a footer entry: higher tiers are dropped later.
const (
	footerTierPlain = iota
	footerTierQuit
	footerTierExport
)

// footerTier ranks an entry; the export entry outlasts the quit entry, which
// outlasts every other entry.
func footerTier(entry string) int {
	switch entry {
	case exportKeyEntry.Key + " " + exportKeyEntry.Desc:
		return footerTierExport
	case quitKeyEntry.Key + " " + quitKeyEntry.Desc:
		return footerTierQuit
	}
	return footerTierPlain
}

// footerEllipsis marks the place where entries were dropped.
func footerEllipsis() string {
	if tuicommon.CurrentRenderMode().Glyphs == tuicommon.GlyphASCII {
		return "..."
	}
	return "…"
}

// footerEntries lists the scroll hint and the keys as whole entries, in order.
func footerEntries(hint string, keys []tuicommon.HelpEntry) []string {
	var entries []string
	if hint != "" {
		entries = append(entries, hint)
	}
	for _, k := range keys {
		entries = append(entries, k.Key+" "+k.Desc)
	}
	return entries
}

// leadHelp joins a scroll hint and keys without the help hint or any fitting.
func leadHelp(hint string, keys []tuicommon.HelpEntry) string {
	return strings.Join(footerEntries(hint, keys), footerSeparator)
}

// fitFooter joins entries to fit width. Whole entries are dropped when it does
// not: the rightmost first, the export entry last. An ellipsis marks the drop. When
// withHelp is true the help hint always closes the line, even if nothing else fits.
func fitFooter(entries []string, withHelp bool, width int) string {
	// drop order: plain entries from the right, then the quit entry, then export
	var order []int
	for tier := 0; tier <= footerTierExport; tier++ {
		for i := len(entries) - 1; i >= 0; i-- {
			if footerTier(entries[i]) == tier {
				order = append(order, i)
			}
		}
	}

	dropped := make([]bool, len(entries))
	for n := 0; ; n++ {
		line := composeFooter(entries, dropped, n > 0, withHelp)
		if textwidth.Of(line) <= width || n == len(order) {
			if textwidth.Of(line) > width {
				if withHelp {
					return textwidth.Truncate(helpHintText, width)
				}
				return textwidth.Truncate(line, width)
			}
			return line
		}
		dropped[order[n]] = true
	}
}

// composeFooter joins the entries that are not dropped, an ellipsis when
// anything was dropped, and the help hint.
func composeFooter(entries []string, dropped []bool, ellipsis, withHelp bool) string {
	var parts []string
	for i, e := range entries {
		if !dropped[i] {
			parts = append(parts, e)
		}
	}
	if ellipsis {
		parts = append(parts, footerEllipsis())
	}
	if withHelp {
		parts = append(parts, helpHintText)
	}
	return strings.Join(parts, footerSeparator)
}
