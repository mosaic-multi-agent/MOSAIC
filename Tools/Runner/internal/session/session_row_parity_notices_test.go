package session_test

// The non-interactive CLI prints every Interact.Notify call as one line, so the
// notices of a run must not change when row information is announced to the
// step-record observer. This pins the notice sequence of a run with a failed
// harness attempt and a HITL-rejected attempt with its re-dispatch.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

func TestRowParity_Notices_RunWithFailedAndRejectedAttempts_AreUnchanged(t *testing.T) {
	rig := newRowRig(t, "hitl-linear-orch.md", rowLinearAgents, func(d *session.Deps) {
		d.Approvals = firstDisapprovedThenApproved()
		d.Routing = &scriptedRoutingConsultant{}
	})
	rig.f.Queue("agent-a", harness.ScriptedEntry{Err: errors.New("harness: subprocess timed out")},
		successEntry("plan v1"), successEntry("plan v2"))
	rig.f.Queue("agent-b", successEntry("done"))

	rig.start(configForMode(rig.orchPath, domain.ExecutionModeAuto))

	type line struct{ title, message string }
	var got []line
	for _, ev := range rig.rec.snapshot() {
		if ev.kind == "notice" {
			got = append(got, line{ev.notice.Title, ev.notice.Message})
		}
	}
	want := []line{
		{"agent-a#1", `phase=PLANNING iteration="" status=running`},
		{"agent-a#2", `phase=PLANNING iteration="" status=running`},
		{"agent-a#3", `phase=PLANNING iteration="" status=SUCCESS`},
		{"agent-b#4", `phase=PLANNING iteration="" status=running`},
		{"agent-b#4", `phase=PLANNING iteration="" status=SUCCESS`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("notices =\n%+v\nwant the unchanged sequence\n%+v", got, want)
	}
}
