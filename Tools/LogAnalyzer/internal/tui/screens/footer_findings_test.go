package screens_test

// The `f findings` footer entry is dropped after every plain entry and before the
// quit and export entries, on frames, pages, the runs list and every per-run view.

import (
	"testing"
)

const findingsEntry = "f findings"

func TestFooterFit_FindingsEntryIsDroppedAfterPlainEntriesAndBeforeQuitAndExport(t *testing.T) {
	keys := keyEntries("a", "one", "f", "findings", "b", "two", "esc", "quit", "e", "export")
	cases := []struct {
		width int
		want  string
	}{
		{52, "a one  f findings  b two  esc quit  e export  ? help"},
		{51, "a one  f findings  esc quit  e export  …  ? help"},
		{48, "a one  f findings  esc quit  e export  …  ? help"},
		{47, "f findings  esc quit  e export  …  ? help"},
		{41, "f findings  esc quit  e export  …  ? help"},
		{40, "esc quit  e export  …  ? help"},
		{29, "esc quit  e export  …  ? help"},
		{28, "e export  …  ? help"},
		{19, "e export  …  ? help"},
		{18, "…  ? help"},
	}
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for _, c := range cases {
				if got := r.render(t, keys, "", c.width); got != c.want {
					t.Errorf("width %d: footer = %q, want %q", c.width, got, c.want)
				}
			}
		})
	}
}

// plainFooterEntry reports whether an entry is neither the findings, quit, export,
// ellipsis nor help entry, that is whether it is dropped before the findings entry.
func plainFooterEntry(e string) bool {
	switch e {
	case findingsEntry, "esc quit", "e export", footerEllipsis, footerHelp, "...":
		return false
	}
	return true
}

func TestRunsFooter_FindingsEntryOutlastsPlainEntriesAndIsDroppedBeforeQuit(t *testing.T) {
	colourOff(t)
	s := newRuns(wideW, tallH)

	for width := 19; width <= 200; width++ {
		entries := footerEntries(footerLine(s.View(width, tallH)))
		hasFindings := false
		plainKept := false
		hasQuit := false
		for _, e := range entries {
			hasFindings = hasFindings || e == findingsEntry
			plainKept = plainKept || plainFooterEntry(e)
			hasQuit = hasQuit || e == "esc quit"
		}
		if plainKept && !hasFindings {
			t.Errorf("width %d: footer %v keeps a plain entry but dropped %q", width, entries, findingsEntry)
		}
		if hasFindings && !hasQuit {
			t.Errorf("width %d: footer %v keeps %q but dropped the quit entry", width, entries, findingsEntry)
		}
	}
	if line := footerLine(s.View(200, tallH)); !hasEntry(line, findingsEntry) {
		t.Errorf("a wide footer %q must list %q", line, findingsEntry)
	}
}

func TestRunViewFooters_FindingsEntryOutlastsPlainEntriesAndIsDroppedBeforeExport(t *testing.T) {
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.view(t)

			if line := footerLine(s.View(200, tallH)); !hasEntry(line, findingsEntry) {
				t.Fatalf("a wide footer %q must list %q", line, findingsEntry)
			}
			for width := 19; width <= 200; width++ {
				entries := footerEntries(footerLine(s.View(width, tallH)))
				hasFindings, plainKept := false, false
				for _, e := range entries {
					hasFindings = hasFindings || e == findingsEntry
					plainKept = plainKept || plainFooterEntry(e)
				}
				if plainKept && !hasFindings {
					t.Errorf("width %d: footer %v keeps a plain entry but dropped %q", width, entries, findingsEntry)
				}
			}
		})
	}
}
