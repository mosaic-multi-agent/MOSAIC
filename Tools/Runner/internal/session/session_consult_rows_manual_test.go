package session_test

// The manual routing dialogue (the user acting as resolver) announces no
// consultation: not the in-run request, not the stop-screen manual dispatch and
// not the manual-resolution fallback after a failed orchestrator consultation.
// A failed orchestrator consultation has ended before the dialogue is asked.
// Non-interactive output does not change when the announcements are wired.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const evDialogue = "dialogue"

// traceDialogue makes the user's questions show up in the trace.
func traceDialogue(u *manualUser, tr *consultTrace) *manualUser {
	u.onAsk = func(interaction.QuestionID) { tr.add(evDialogue) }
	return u
}

func TestConsultAnnounce_InRunManualRequest_AtAConsultationDecision_AnnouncesOnlyTheOrchestratorCalls(t *testing.T) {
	tr := &consultTrace{}
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueStop("after the manual step")
	rig := newInRunRig(t, traceDialogue(linearDialogue(t, "2", "do b now"), tr), inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, consultant: consultant, consultations: tr,
	})
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if consultant.CallCount != 2 {
		t.Fatalf("scenario did not run as intended: want the consultant asked twice, got %d", consultant.CallCount)
	}
	requireNotOpenAt(t, tr, evDialogue, 1)
	requireBalanced(t, tr)
	if got := tr.count(evStart); got != consultant.CallCount {
		t.Errorf("announced %d consultations for %d orchestrator calls: the dialogue must not be announced", got, consultant.CallCount)
	}
}

func TestConsultAnnounce_InRunManualRequest_InPlaceOfAnEngineStep_AnnouncesNothing(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			tr := &consultTrace{}
			rig := newInRunRig(t, traceDialogue(linearDialogue(t, "1", "redo the plan"), tr), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents, consultations: tr,
			})
			rig.queueOK("agent-a", 2)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if tr.count(evDialogue) == 0 {
				t.Fatal("scenario did not run as intended: the manual dialogue never opened")
			}
			if n := tr.count(evStart) + tr.count(evEnd); n != 0 {
				t.Errorf("announced %d consultation events for a manual dialogue: %v", n, tr.snapshot())
			}
		})
	}
}

func TestConsultAnnounce_StopScreenManualDispatch_AnnouncesOnlyTheOrchestratorCalls(t *testing.T) {
	tr := &consultTrace{}
	user := traceDialogue(newManualUser(t).completeDialogue("3", "docs/extra.md"), tr)
	rig := newManualRig(t, user, nil, tr)
	queueSuccess(rig.f, "build-review", "build-review#3")

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if rig.consultant.CallCount != 1 {
		t.Fatalf("scenario did not run as intended: want the consultant asked once after the dialogue, got %d", rig.consultant.CallCount)
	}
	requireNotOpenAt(t, tr, evDialogue, 1)
	requireBalanced(t, tr)
	if got := tr.count(evStart); got != 1 {
		t.Errorf("announced %d consultations, want only the one orchestrator consultation: %v", got, tr.snapshot())
	}
	if events := tr.snapshot(); events[0] != evDialogue {
		t.Errorf("want the manual dialogue first with nothing announced before it, got %v", events)
	}
}

func TestConsultAnnounce_ManualFallback_FailedConsultationEndedBeforeTheDialogueIsAsked(t *testing.T) {
	manual := &scriptedRoutingConsultant{}
	manual.queueDispatch("agent-a", "manual resolution fallback", 0)
	manual.queueStop("done")
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, tr *consultTrace) {
		d.Manual = &tracedRouting{inner: manual, tr: tr, evt: evManualCall}
	})
	rig.consult.queueError(domain.ConsultFailTransport)
	successAgentEntries(rig.f, "agent-a", 1)
	cfg := baseOrchestratedConfig(rig.orchPath)
	cfg.RunSettings.ManualResolution = true

	rig.ses.Start(context.Background(), cfg) //nolint:errcheck

	if rig.tr.count(evManualCall) == 0 {
		t.Fatal("scenario did not run as intended: the manual resolver was never asked")
	}
	requireBalanced(t, rig.tr)
	events := rig.tr.snapshot()
	for i, e := range events {
		if e == evManualCall && (i == 0 || events[i-1] != evEnd) {
			t.Errorf("manual resolver asked at event %d without the failed consultation having ended first: %v", i, events)
		}
	}
	requireNotOpenAt(t, rig.tr, evManualCall, 1)
	if got, want := rig.tr.count(evStart), rig.tr.count(evCall); got != want {
		t.Errorf("announced %d consultations for %d orchestrator calls: the manual resolver must not be announced", got, want)
	}
}

func TestConsultAnnounce_WiringTheObserver_LeavesNoticeOutputUnchanged(t *testing.T) {
	run := func(withObserver bool) []interaction.Notice {
		notices := &noticeCapturingInteraction{}
		rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, _ *consultTrace) {
			d.Interact = notices
			if !withObserver {
				d.Consultations = nil
			}
		})
		rig.consult.queueDispatch("agent-a", "first", 0)
		rig.consult.queueDispatch("agent-b", "second", 1)
		rig.consult.queueStop("done")
		successAgentEntries(rig.f, "agent-a", 1)
		successAgentEntries(rig.f, "agent-b", 1)
		rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath)) //nolint:errcheck
		if withObserver && rig.tr.count(evStart) != 3 {
			t.Fatalf("scenario did not run as intended: want 3 announced consultations, got %v", rig.tr.snapshot())
		}
		return notices.allNotices()
	}

	without, with := run(false), run(true)

	if len(without) == 0 {
		t.Fatal("scenario did not run as intended: the run produced no notice")
	}
	if !reflect.DeepEqual(with, without) {
		t.Errorf("notices with the observer =\n%+v\nwant exactly the notices without it\n%+v", with, without)
	}
}
