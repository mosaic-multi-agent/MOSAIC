package runflow

// progress_requests_view_test.go verifies what the progress screen renders for
// the in-run requests: the help bar entries and the pending lines.
//
// The screen is drawn at 120 columns: the help bar is wider than 80 and would
// wrap, which would split the substrings asserted here.

import (
	"strings"
	"testing"
)

const (
	helpBarWithRequests = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  a artifact-view  ctrl+c force-quit"
	pendingManualLine   = "Requested: manual dispatch at the next routing decision (m to cancel)"
	pendingHITLLine     = "Requested: HITL for the next agent (h to cancel)"
)

func TestProgressScreen_View_HelpBarListsBothRequestKeys(t *testing.T) {
	s := newRequestsScreen()

	view := s.View()

	if !strings.Contains(view, helpBarWithRequests) {
		t.Errorf("View() lacks the help bar %q", helpBarWithRequests)
	}
}

func TestProgressScreen_View_NothingPending_ShowsNoPendingLines(t *testing.T) {
	s := newRequestsScreen()

	view := s.View()

	if strings.Contains(view, "Requested:") {
		t.Error("View() shows a pending line with nothing pending")
	}
}

func TestProgressScreen_View_ShowsEachPendingRequest(t *testing.T) {
	cases := []struct {
		name    string
		pending PendingRequests
		want    []string
		absent  []string
	}{
		{"manual dispatch", PendingRequests{ManualDispatch: true}, []string{pendingManualLine}, []string{pendingHITLLine}},
		{"hitl", PendingRequests{HITLNext: true}, []string{pendingHITLLine}, []string{pendingManualLine}},
		{"both", PendingRequests{ManualDispatch: true, HITLNext: true}, []string{pendingManualLine, pendingHITLLine}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newRequestsScreen()
			s.SetPendingRequests(tc.pending)

			view := s.View()

			for _, w := range tc.want {
				if !strings.Contains(view, w) {
					t.Errorf("View() lacks %q", w)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(view, a) {
					t.Errorf("View() shows %q for a request that is not pending", a)
				}
			}
		})
	}
}

func TestProgressScreen_View_PendingLineDisappearsWhenCleared(t *testing.T) {
	s := newRequestsScreen()
	s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})
	s.SetPendingRequests(PendingRequests{HITLNext: true})

	view := s.View()

	if strings.Contains(view, pendingManualLine) {
		t.Error("View() still shows the manual-dispatch line after it was cleared")
	}
	if !strings.Contains(view, pendingHITLLine) {
		t.Error("View() lost the HITL line that is still pending")
	}
}

func TestProgressScreen_View_PendingLinesSitBelowRowsAndAboveStopBanner(t *testing.T) {
	s := newRequestsScreen()
	s.AppendRow(ProgressRow{AgentInstance: "row-agent#1", Phase: "build", Status: "SUCCESS"})
	s.SetPendingRequests(PendingRequests{ManualDispatch: true})
	pressProgressKey(s, "s")

	view := s.View()

	rowAt := strings.Index(view, "row-agent#1")
	lineAt := strings.Index(view, pendingManualLine)
	bannerAt := strings.Index(view, "Stop after current step?")
	if rowAt < 0 || lineAt < 0 || bannerAt < 0 {
		t.Fatalf("missing element: row=%d pending=%d banner=%d", rowAt, lineAt, bannerAt)
	}
	if !(rowAt < lineAt && lineAt < bannerAt) {
		t.Errorf("order row=%d pending=%d banner=%d, want row < pending line < confirm banner", rowAt, lineAt, bannerAt)
	}
}

func TestProgressScreen_View_PendingLineDoesNotReplaceStoppingNotice(t *testing.T) {
	s := newRequestsScreen()
	s.SetPendingRequests(PendingRequests{HITLNext: true})
	pressProgressKey(s, "s")
	pressProgressKey(s, "y")

	view := s.View()

	if !strings.Contains(view, "Stopping after current step") {
		t.Error("View() lost the Stopping notice")
	}
	if !strings.Contains(view, pendingHITLLine) {
		t.Error("View() hides a pending request once a stop is confirmed")
	}
}

func TestProgressScreen_ResetRequestState_RemovesPendingLines(t *testing.T) {
	s := newRequestsScreen()
	s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})

	s.ResetRequestState()

	if strings.Contains(s.View(), "Requested:") {
		t.Error("View() still shows a pending line after ResetRequestState")
	}
}
