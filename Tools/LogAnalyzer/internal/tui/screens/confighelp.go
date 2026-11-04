package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

// configHelp holds the entries of the "Configuration files" group of a help
// overlay: the pricing and the analysis settings file of each level.
type configHelp struct {
	entries []tuicommon.HelpEntry
}

// SetConfigFiles sets the entries of the "Configuration files" help group; an
// empty list hides the group.
func (c *configHelp) SetConfigFiles(entries []tuicommon.HelpEntry) { c.entries = entries }

// withConfigGroup appends the "Configuration files" group to groups when set.
func (c *configHelp) withConfigGroup(groups []chrome.HelpGroup) []chrome.HelpGroup {
	if len(c.entries) == 0 {
		return groups
	}
	return append(groups, chrome.HelpGroup{Title: "Configuration files", Entries: c.entries})
}
