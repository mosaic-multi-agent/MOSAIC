package tui

// The activity-panel toggle key as the root model handles it: it toggles the
// panel and nothing else, and the keys that already had a meaning keep it.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

func TestActivityKey_L_TogglesThePanelOnTheProgressScreen(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
	if !m.progressScreen.ActivityVisible() {
		t.Fatal("panel not visible when the progress screen first appears")
	}

	pressRune(m, "l")
	hidden := !m.progressScreen.ActivityVisible()
	pressRune(m, "l")

	if !hidden || !m.progressScreen.ActivityVisible() {
		t.Errorf("toggle did not hide then show the panel (hidden after first press: %v)", hidden)
	}
}

func TestActivityKey_L_RaisesNoRequestAndOpensNoGate(t *testing.T) {
	rec := &tuiRecordingLogger{}
	reqs := session.NewInRunRequests()
	m := newRequestsModel(rec, reqs)

	pressRune(m, "l")

	if m.progressScreen.ActivityVisible() {
		t.Error("'l' did not toggle the panel")
	}
	for _, kind := range []domain.InRunRequest{domain.InRunRequestManualDispatch, domain.InRunRequestHITLNext} {
		if reqs.Pending(kind) {
			t.Errorf("'l' made request %q pending", kind)
		}
	}
	if m.progressScreen.ConfirmPending() || m.progressScreen.GracefulStop() {
		t.Error("'l' opened or resolved the stop gate")
	}
	if n := requestLogCount(rec); n != 0 {
		t.Errorf("'l' wrote %d request log entries, want none", n)
	}
	if n := len(rec.eventsNamed(domain.EventTUIStopGateEntered)); n != 0 {
		t.Errorf("'l' wrote %d stop-gate entries, want none", n)
	}
	if m.progressScreen.ArtifactViewRequested() || m.screen != screenProgress {
		t.Error("'l' left the progress screen")
	}
}

func TestActivityKeys_ExistingKeys_KeepTheirBehaviourWithThePanelHidden(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)
	pressRune(m, "l")
	if m.progressScreen.ActivityVisible() {
		t.Fatal("panel still visible after 'l'; the hidden state was not established")
	}

	pressRune(m, "m")
	pressRune(m, "h")
	pressRune(m, "s")

	if !reqs.Pending(domain.InRunRequestManualDispatch) || !reqs.Pending(domain.InRunRequestHITLNext) {
		t.Error("request keys 'm' and 'h' stopped working with the panel hidden")
	}
	if !m.progressScreen.ConfirmPending() {
		t.Error("'s' no longer opens the stop gate with the panel hidden")
	}
}
