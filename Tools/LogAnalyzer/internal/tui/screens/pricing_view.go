package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/app"
)

// pricingTitle is the title of the pricing page.
const pricingTitle = "Log Analyzer — Pricing"

// pricingKeys are the short help keys of the pricing page.
var pricingKeys = []tuicommon.HelpEntry{
	{Key: "↑/↓", Desc: "select"}, {Key: "enter", Desc: "price"}, {Key: "esc", Desc: "back"}, {Key: "ctrl+c", Desc: "quit"},
}

// pricingOverlayGroups are the keys listed by the help overlay of the pricing page.
func pricingOverlayGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		{Title: "This screen", Entries: []tuicommon.HelpEntry{
			{Key: "enter", Desc: "set or overwrite the price of the selected model"},
		}},
		{Title: "Navigation", Entries: []tuicommon.HelpEntry{
			{Key: "↑/↓ j/k", Desc: "move the selection"},
			{Key: "esc", Desc: "back to the runs"},
			{Key: "q", Desc: "same as esc"},
			{Key: KeyHelp, Desc: "show or hide this help"},
			{Key: "ctrl+c", Desc: "quit"},
		}},
	}
}

// pricingColumns are the columns of the model table: the model keeps its room and
// the priced state follows it.
func pricingColumns() []table.Column {
	return []table.Column{
		{Title: "model", Flex: true, MinWidth: 12},
		{Title: "status"},
	}
}

// pricingCells is the row of one model: an unpriced model asks for a look.
func pricingCells(m app.ModelPricingStatus) []table.Cell {
	name := table.Cell{Text: statusKey(m).String()}
	if m.Priced {
		return []table.Cell{name, {Text: "(priced)"}}
	}
	warn := tuicommon.RoleSignalWarning
	name.Role = warn
	return []table.Cell{name, {Text: AbsentMarker + " unpriced", Role: warn}}
}
