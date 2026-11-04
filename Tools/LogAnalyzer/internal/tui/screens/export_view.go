package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

// exportTitle is the title of the export page.
const exportTitle = "Log Analyzer -- Export digest"

// exportPathKeys are the short help keys while the path is being entered.
var exportPathKeys = []tuicommon.HelpEntry{
	{Key: "enter", Desc: "export"}, {Key: "esc", Desc: "cancel"}, {Key: "ctrl+c", Desc: "quit"},
}

// exportConfirmKeys are the short help keys of the overwrite question.
var exportConfirmKeys = []tuicommon.HelpEntry{
	{Key: "y", Desc: "overwrite"}, {Key: "n", Desc: "cancel"}, {Key: "esc", Desc: "cancel"}, {Key: "ctrl+c", Desc: "quit"},
}

// exportOverlayGroups are the keys listed by the help overlay of the overwrite question.
func exportOverlayGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		{Title: "This screen", Entries: []tuicommon.HelpEntry{
			{Key: "y", Desc: "overwrite the existing file"},
			{Key: "n", Desc: "keep the existing file and cancel the export"},
		}},
		{Title: "Navigation", Entries: []tuicommon.HelpEntry{
			{Key: "esc", Desc: "same as n"},
			{Key: KeyHelp, Desc: "show or hide this help"},
			{Key: "ctrl+c", Desc: "quit"},
		}},
	}
}
