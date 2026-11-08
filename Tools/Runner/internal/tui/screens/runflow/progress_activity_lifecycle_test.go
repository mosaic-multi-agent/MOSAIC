package runflow

// Activity panel lifecycle and layout: a new invocation replaces the old one,
// stale keys are ignored, ended invocations are marked, and the panel stays
// within the screen height.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestActivityPanel_NewInvocation_ReplacesThePreviousOnesActivity(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, toolItem("OldTool", "old-input"))

	s.BeginActivity(2, domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "agent-b#4"})
	view := s.View()

	if strings.Contains(view, "OldTool") || strings.Contains(view, "old-input") {
		t.Errorf("previous invocation's activity still shown:\n%s", view)
	}
	if !strings.Contains(view, placeholderText) {
		t.Errorf("placeholder missing for the new invocation:\n%s", view)
	}
}

func TestActivityPanel_StaleKey_IsIgnored(t *testing.T) {
	s := newActivityScreen(t)
	s.BeginActivity(2, dispatchInv)

	s.AppendActivity(1, toolItem("StaleTool", "stale-input"))
	s.EndActivity(1)
	view := s.View()

	if strings.Contains(view, "StaleTool") {
		t.Errorf("an item of a superseded invocation was shown:\n%s", view)
	}
	if !strings.Contains(view, placeholderText) {
		t.Errorf("a stale item ended the placeholder of the current invocation:\n%s", view)
	}
	if strings.Contains(strings.ToLower(view), "ended") {
		t.Errorf("a stale end marked the current invocation as ended:\n%s", view)
	}
}

func TestActivityPanel_ItemsBeforeAnyInvocation_AreIgnored(t *testing.T) {
	s := NewProgressScreen(120, 30, progressStyles())

	s.AppendActivity(1, toolItem("Orphan", "x"))
	s.EndActivity(1)

	if view := s.View(); strings.Contains(view, "Orphan") {
		t.Errorf("an item without a begun invocation was shown:\n%s", view)
	}
}

func TestActivityPanel_Ended_KeepsItemsAndMarksTheInvocationEnded(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, toolItem("Read", "final.txt"))
	before := strings.Contains(strings.ToLower(s.View()), "ended")

	s.EndActivity(1)
	view := s.View()

	if before {
		t.Error("a running invocation is marked as ended")
	}
	if !strings.Contains(view, "final.txt") {
		t.Errorf("ended invocation's items disappeared:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "ended") {
		t.Errorf("ended invocation is not marked as ended:\n%s", view)
	}
}

func TestActivityPanel_EndedMark_IsGoneWithTheNextInvocation(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, textItem("done"))
	s.EndActivity(1)
	if !strings.Contains(strings.ToLower(s.View()), "ended") {
		t.Fatalf("ended invocation is not marked as ended:\n%s", s.View())
	}

	s.BeginActivity(2, dispatchInv)

	if view := s.View(); strings.Contains(strings.ToLower(view), "ended") {
		t.Errorf("the next invocation inherits the ended mark:\n%s", view)
	}
}

func TestActivityPanel_ManyItems_ShowTheMostRecentOnesWithinTheScreenHeight(t *testing.T) {
	const height = 16
	s := NewProgressScreen(120, height, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#3", Phase: "EXECUTION", Status: "running"})
	s.BeginActivity(1, dispatchInv)
	for i := 0; i < 60; i++ {
		s.AppendActivity(1, toolItem("Tool", fmt.Sprintf("input-%02d", i)))
	}

	view := s.View()

	if lines := strings.Count(view, "\n") + 1; lines > height {
		t.Errorf("view has %d lines, want <= %d", lines, height)
	}
	if !strings.Contains(view, "input-59") {
		t.Errorf("most recent item missing:\n%s", view)
	}
	if strings.Contains(view, "input-00") {
		t.Errorf("oldest item still shown although far more than fit:\n%s", view)
	}
}

func TestActivityPanel_SmallScreen_KeepsTheStepRowsAndShrinksThePanel(t *testing.T) {
	const height = 9
	build := func(withActivity bool) *ProgressScreen {
		s := NewProgressScreen(160, height, progressStyles())
		s.AppendRow(ProgressRow{AgentInstance: "first#1", Phase: "PLANNING", Status: "SUCCESS"})
		s.AppendRow(ProgressRow{AgentInstance: "second#2", Phase: "EXECUTION", Status: "running"})
		if withActivity {
			s.BeginActivity(1, dispatchInv)
			for i := 0; i < 20; i++ {
				s.AppendActivity(1, textItem(fmt.Sprintf("line-%02d", i)))
			}
		}
		return s
	}
	today := strings.Count(build(false).View(), "\n") + 1

	view := build(true).View()

	for _, want := range []string{"first#1", "second#2"} {
		if !strings.Contains(view, want) {
			t.Errorf("step row %q squeezed out by the panel:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "line-19") {
		t.Errorf("the panel shows nothing of the activity on a small screen:\n%s", view)
	}
	// The rows are never squeezed out, so a screen that is already taller than
	// its height without a panel may not grow by the panel.
	limit := height
	if today > limit {
		limit = today
	}
	if lines := strings.Count(view, "\n") + 1; lines > limit {
		t.Errorf("view has %d lines, want <= %d", lines, limit)
	}
}
