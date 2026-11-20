package screens_test

// promote_batch_summary_test.go verifies PromoteBatchSummaryScreen, the final screen of a
// batch promote: the title, the target layer and the counts, one entry per selected agent with
// its status and reason or error, scrolling of a long summary within the terminal height, and
// the final-screen contract (scroll keys scroll, every other key dismisses, help line).

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/tui/screens"
)

const batchTargetLayer = "Team Layer (team)"

// mixedBatchResult has one agent of every status.
func mixedBatchResult() app.PromoteBatchResult {
	return app.PromoteBatchResult{
		HarnessID:       "h",
		AgentsDir:       "/agents",
		TargetCatalogID: "team",
		TargetLayer:     batchTargetLayer,
		Agents: []app.PromoteBatchAgentResult{
			{
				SourcePath: "/agents/alpha.md", Key: "alpha", Status: app.PromoteBatchPromoted,
				Result: app.PromoteResult{DestinationPath: "/team/Subagents/Dev/alpha.md", NumericID: "41", Key: "alpha"},
			},
			{
				SourcePath: "/agents/beta.md", Key: "beta", Status: app.PromoteBatchPromoted,
				ReplacedLayer: "MOSAIC default catalog (default)",
				Result:        app.PromoteResult{DestinationPath: "/team/Subagents/Ops/beta.md", NumericID: "7", Key: "beta"},
			},
			{
				SourcePath: "/agents/gamma.md", Key: "gamma", Status: app.PromoteBatchSkipped,
				Reason: "kept the existing agent in Team Layer (team)",
			},
			{
				SourcePath: "/agents/delta.md", Key: "delta", Status: app.PromoteBatchFailed,
				Reason: "write failed: disk is full", Err: errors.New("write failed: disk is full"),
			},
			{SourcePath: "/agents/epsilon.md", Key: "epsilon", Status: app.PromoteBatchNotProcessed},
		},
	}
}

func newBatchSummary(r app.PromoteBatchResult, w, h int) *screens.PromoteBatchSummaryScreen {
	return screens.NewPromoteBatchSummaryScreen(r, w, h, plainStyles())
}

func firstLine(view string) string { return strings.SplitN(view, "\n", 2)[0] }

func TestPromoteBatchSummary_Title_CompleteOrAborted(t *testing.T) {
	r := mixedBatchResult()

	if got := firstLine(newBatchSummary(r, 120, 40).View()); !strings.Contains(got, "Promote complete") {
		t.Errorf("first line = %q; want the title %q", got, "Promote complete")
	}
	r.Aborted = true
	if got := firstLine(newBatchSummary(r, 120, 40).View()); !strings.Contains(got, "Promote aborted") {
		t.Errorf("first line of an aborted batch = %q; want the title %q", got, "Promote aborted")
	}
}

func TestPromoteBatchSummary_Header_NamesTargetLayerAndCounts(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(mixedBatchResult(), 120, 40).View())

	if !strings.Contains(view, batchTargetLayer) {
		t.Errorf("view does not name the target layer %q:\n%s", batchTargetLayer, view)
	}
	if !strings.Contains(view, "2 promoted, 1 skipped, 1 failed, 1 not processed") {
		t.Errorf("view does not show the counts line:\n%s", view)
	}
}

func TestPromoteBatchSummary_EveryAgentAppearsWithKeyFileAndStatus(t *testing.T) {
	view := newBatchSummary(mixedBatchResult(), 120, 40).View()

	lines := strings.Split(view, "\n")
	for _, want := range []struct{ key, file, status string }{
		{"alpha", "alpha.md", "promoted"},
		{"beta", "beta.md", "promoted"},
		{"gamma", "gamma.md", "skipped"},
		{"delta", "delta.md", "failed"},
		{"epsilon", "epsilon.md", "not processed"},
	} {
		if !entryHas(lines, want.key, want.file, want.status) {
			t.Errorf("no entry holds key %q, file %q and status %q together:\n%s", want.key, want.file, want.status, view)
		}
	}
}

// entryHas reports whether some line holds key, file and status together (an entry's header line).
func entryHas(lines []string, key, file, status string) bool {
	for _, l := range lines {
		if strings.Contains(l, key) && strings.Contains(l, file) && strings.Contains(l, status) {
			return true
		}
	}
	return false
}

func TestPromoteBatchSummary_PromotedEntry_ShowsDestinationAndNumericID(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(mixedBatchResult(), 120, 40).View())

	for _, want := range []string{"/team/Subagents/Dev/alpha.md", "41", "/team/Subagents/Ops/beta.md"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not show %q of a promoted agent:\n%s", want, view)
		}
	}
}

func TestPromoteBatchSummary_ReplacedAgent_NamesTheReplacedLayer(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(mixedBatchResult(), 120, 40).View())

	if !strings.Contains(view, "replaced the agent in MOSAIC default catalog (default)") {
		t.Errorf("view does not say which layer's agent was replaced:\n%s", view)
	}
}

func TestPromoteBatchSummary_SkippedAndFailedEntries_ShowReasonAndError(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(mixedBatchResult(), 120, 40).View())

	for _, want := range []string{"kept the existing agent in Team Layer (team)", "write failed: disk is full"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not show %q:\n%s", want, view)
		}
	}
}

func TestPromoteBatchSummary_OneAgentBatch_ShowsThatAgentsResult(t *testing.T) {
	r := mixedBatchResult()
	r.Agents = r.Agents[2:3] // only the skipped agent

	view := collapseWhitespace(newBatchSummary(r, 120, 40).View())

	for _, want := range []string{"gamma", "gamma.md", "skipped", "0 promoted, 1 skipped, 0 failed, 0 not processed"} {
		if !strings.Contains(view, want) {
			t.Errorf("view of a one-agent batch does not contain %q:\n%s", want, view)
		}
	}
}

// ---------------------------------------------------------------------------
// Scrolling
// ---------------------------------------------------------------------------

func longBatchResult(n int) app.PromoteBatchResult {
	r := app.PromoteBatchResult{TargetLayer: batchTargetLayer}
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("agent-%02d", i+1)
		r.Agents = append(r.Agents, app.PromoteBatchAgentResult{
			SourcePath: "/agents/" + key + ".md", Key: key, Status: app.PromoteBatchPromoted,
			Result: app.PromoteResult{DestinationPath: "/team/Subagents/Dev/" + key + ".md", NumericID: fmt.Sprint(100 + i)},
		})
	}
	return r
}

func TestPromoteBatchSummary_LongSummary_FitsTheTerminalHeight(t *testing.T) {
	for _, size := range []layoutSize{{80, 12}, {40, 10}, {120, 40}} {
		s := newBatchSummary(longBatchResult(30), size.w, size.h)
		view := s.View()
		// Content must be present, so an empty view cannot satisfy the fit check.
		if !strings.Contains(firstLine(view), "Promote complete") {
			t.Errorf("%dx%d: first line = %q; want the title", size.w, size.h, firstLine(view))
		}
		if !strings.Contains(view, "agent-01") {
			t.Errorf("%dx%d: the first agent is not visible:\n%s", size.w, size.h, view)
		}
		if size.w >= 80 && !strings.Contains(collapseWhitespace(lastNonEmptyLine(view)), "ctrl+c: quit") {
			t.Errorf("%dx%d: last line is not the help line:\n%s", size.w, size.h, view)
		}
		for step := 0; step < 3; step++ {
			if n := strings.Count(s.View(), "\n") + 1; n > size.h {
				t.Errorf("%dx%d after %d scroll steps: view has %d lines; want at most %d", size.w, size.h, step, n, size.h)
			}
			s.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}
}

func TestPromoteBatchSummary_LongSummary_ScrollsToTheLastAgent(t *testing.T) {
	s := newBatchSummary(longBatchResult(30), 80, 12)
	if strings.Contains(s.View(), "agent-30") {
		t.Fatal("precondition: the last agent is already visible before scrolling")
	}

	for i := 0; i < 200; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if !strings.Contains(s.View(), "agent-30") {
		t.Errorf("the last agent is not reachable by scrolling down:\n%s", s.View())
	}
	if s.Done() {
		t.Error("Done() = true after scrolling; scroll keys must not dismiss")
	}
}

func TestPromoteBatchSummary_ScrollKeys_ScrollAndNeverDismiss(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"up": {Type: tea.KeyUp}, "down": {Type: tea.KeyDown},
		"k": {Type: tea.KeyRunes, Runes: []rune{'k'}}, "j": {Type: tea.KeyRunes, Runes: []rune{'j'}},
	}
	for name, key := range keys {
		s := newBatchSummary(longBatchResult(30), 80, 12)
		top := s.View()
		// Scroll down first so that the up keys have something to scroll back.
		down := tea.KeyMsg{Type: tea.KeyDown}
		if name == "j" || name == "down" {
			down = key
		}
		for i := 0; i < 5; i++ {
			if s.Update(down) || s.Done() {
				t.Errorf("scroll key %q dismissed the batch summary", name)
			}
		}
		scrolled := s.View()
		if scrolled == top {
			t.Errorf("view unchanged after scrolling down (key %q)", name)
		}
		if name == "up" || name == "k" {
			for i := 0; i < 5; i++ {
				if s.Update(key) || s.Done() {
					t.Errorf("scroll key %q dismissed the batch summary", name)
				}
			}
			if s.View() != top {
				t.Errorf("view after 5 presses of %q is not the top of the summary again", name)
			}
		}
	}
}

func TestPromoteBatchSummary_JAndK_ScrollTheContent(t *testing.T) {
	s := newBatchSummary(longBatchResult(30), 80, 12)
	top := s.View()

	for i := 0; i < 5; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	scrolled := s.View()
	for i := 0; i < 5; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	}

	if scrolled == top {
		t.Error("view unchanged after 'j' presses on a summary taller than the window")
	}
	if s.View() != top {
		t.Error("view after 'k' presses is not the top of the summary again")
	}
}

// ---------------------------------------------------------------------------
// Final-screen contract
// ---------------------------------------------------------------------------

func TestPromoteBatchSummary_AnyOtherKey_Dismisses(t *testing.T) {
	for keyName, key := range dismissKeys() {
		s := newBatchSummary(mixedBatchResult(), 120, 40)

		if !s.Update(key) || !s.Done() {
			t.Errorf("key %q did not dismiss the batch summary", keyName)
		}
	}
}

func TestPromoteBatchSummary_NonKeyMessage_DoesNotDismiss(t *testing.T) {
	s := newBatchSummary(mixedBatchResult(), 120, 40)

	if s.Update(tea.WindowSizeMsg{Width: 100, Height: 30}) || s.Done() {
		t.Error("a non-key message dismissed the batch summary")
	}
}

func TestPromoteBatchSummary_HelpLine_NamesScrollKeysAndBackToStart(t *testing.T) {
	s := newBatchSummary(longBatchResult(30), 120, 40)

	help := collapseWhitespace(lastNonEmptyLine(s.View()))

	for _, want := range []string{"k up", "j down", "other keys: back to start", "ctrl+c: quit"} {
		if !strings.Contains(help, want) {
			t.Errorf("help line %q does not contain %q", help, want)
		}
	}
}

func TestPromoteBatchSummary_HelpLine_StaysVisibleWhenScrolled(t *testing.T) {
	s := newBatchSummary(longBatchResult(30), 80, 12)
	for i := 0; i < 10; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	help := collapseWhitespace(lastNonEmptyLine(s.View()))

	if !strings.Contains(help, "ctrl+c: quit") {
		t.Errorf("last line %q is not the help line after scrolling", help)
	}
}

func TestPromoteBatchSummary_Resize_KeepsFit(t *testing.T) {
	s := newBatchSummary(longBatchResult(30), 120, 40)

	s.Resize(40, 10)

	view := s.View()
	if n := strings.Count(view, "\n") + 1; n > 10 {
		t.Errorf("view has %d lines after Resize to 40x10; want at most 10", n)
	}
	if !strings.Contains(firstLine(view), "Promote complete") || !strings.Contains(view, "agent-01") {
		t.Errorf("view after Resize lacks the title or the first agent:\n%s", view)
	}
}
