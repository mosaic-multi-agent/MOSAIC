package screens_test

// The label of the tools-view row that groups calls with no tool name: an end
// whose start is missing and which logged no name itself.

import "testing"

func TestTools_NamelessToolGroup_ReadsStartMissing(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	assertContains(t, view, "(start missing)")
	assertNotContains(t, view, "(unnamed tool)")
	assertFields(t, rowFor(t, view, "(start missing)"), "3", "2")
}
