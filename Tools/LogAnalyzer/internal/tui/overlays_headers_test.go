package tui

// The option list of a select-one overlay is a table: its column titles are full
// names, and its row count accounts for the real header height, so the cursor
// row and the skip entry stay on screen however tall the header is.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
)

// describedSelectModel opens a select overlay over n options that all carry a
// description, so the list has both columns.
func describedSelectModel(t *testing.T, width, height, n int) Model {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("opt-%02d", i+1)
	}
	p := makePendingSelect(app.QuestionPricingAction, ids, make(chan answerMsg, 1))
	p.choiceQ.AllowSkip = true
	for i := range p.choiceQ.Options {
		p.choiceQ.Options[i].Description = fmt.Sprintf("what option %d does", i+1)
	}
	updated, _ := modelAt(width, height).Update(p)
	m := asModel(updated)
	m.interact.Close()
	if m.screen != screenQuestion || m.selectOverlay == nil {
		t.Fatalf("setup: screen = %q, want the select overlay", m.screen)
	}
	return m
}

func TestSelectOverlayHeaders_ColumnsAreTitledWithFullNames(t *testing.T) {
	colourOff(t)
	m := describedSelectModel(t, 80, 20, 5)

	view := escapeRE.ReplaceAllString(m.View(), "")

	if !strings.Contains(view, "option") || !strings.Contains(view, "description") {
		t.Errorf("the option list must be titled option and description\n%s", view)
	}
}

func TestSelectOverlayHeaders_CursorRowAndSkipEntryStayOnScreenAtEveryShortHeight(t *testing.T) {
	colourOff(t)
	for height := 7; height <= 12; height++ {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			m := describedSelectModel(t, 60, height, 20)

			for i := 0; i < 25; i++ {
				m = press(m, "j")
				assertViewWithin(t, m.View(), 60, height)
			}

			line := rawLineWith(t, m.View(), "Skip")
			if !strings.HasPrefix(escapeRE.ReplaceAllString(line, ""), ">") {
				t.Errorf("the skip entry must be on screen under the cursor: %q", line)
			}
		})
	}
}
