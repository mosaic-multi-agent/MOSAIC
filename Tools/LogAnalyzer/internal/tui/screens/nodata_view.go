package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

// noDataKeys are the short help keys of the no-data page.
var noDataKeys = []tuicommon.HelpEntry{
	{Key: "s", Desc: "choose a different source"}, {Key: "esc", Desc: "quit"}, {Key: "ctrl+c", Desc: "quit"},
}

// noDataOverlayGroups are the keys listed by the help overlay of the no-data page.
func noDataOverlayGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		{Title: "This screen", Entries: []tuicommon.HelpEntry{
			{Key: "s", Desc: "choose a different log source"},
			{Key: "enter", Desc: "same as s"},
		}},
		{Title: "Navigation", Entries: []tuicommon.HelpEntry{
			{Key: "esc", Desc: "quit"},
			{Key: "q", Desc: "same as esc"},
			{Key: KeyHelp, Desc: "show or hide this help"},
			{Key: "ctrl+c", Desc: "quit"},
		}},
	}
}

// noDataExplanation says why there is nothing to show; it is not an error.
const noDataExplanation = "The logs directory is empty — no run folders or unattributable bucket were found.\n" +
	"This is a normal state when the tool has not yet been run against this directory."
