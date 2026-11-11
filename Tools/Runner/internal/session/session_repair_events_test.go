package session_test

// The repair cycle is recorded in the debug log (damage found, damaged copy
// saved, repair consultation, accepted, rejected, stop for unrepaired damage)
// and announced to the consultation observer. Pre-consultation is unaffected.

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// repairEvents returns the names of the repair events logged, in order.
func repairEvents(l *sessionRecordingLogger) []string {
	var out []string
	for _, e := range l.allEvents() {
		if strings.HasPrefix(e, "session.repair.") {
			out = append(out, e)
		}
	}
	return out
}

// eventFields returns the fields of every entry of event, in order.
func eventFields(l *sessionRecordingLogger, event string) []map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]string
	for _, e := range l.entries {
		if e.Event != event {
			continue
		}
		fields := map[string]string{}
		for _, f := range e.Fields {
			fields[f.Key] = f.Value
		}
		out = append(out, fields)
	}
	return out
}

func requireEvents(t *testing.T, l *sessionRecordingLogger, want ...string) {
	t.Helper()
	if got := repairEvents(l); !slices.Equal(got, want) {
		t.Errorf("want repair events %v, got %v", want, got)
	}
}

// requireCount asserts the field holds a positive integer.
func requirePositiveCount(t *testing.T, fields map[string]string, key string) {
	t.Helper()
	if n, err := strconv.Atoi(fields[key]); err != nil || n < 1 {
		t.Errorf("want field %q to be a positive count, got %q", key, fields[key])
	}
}

func TestSession_AcceptedRepair_RecordsDamageCopyConsultationAndAcceptance(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("done")},
	}, nil)

	rig.start(t)

	requireEvents(t, rig.log, domain.EventSessionRepairDamageFound, domain.EventSessionRepairDamagedCopySaved,
		domain.EventSessionRepairConsult, domain.EventSessionRepairAccepted)
	found := eventFields(rig.log, domain.EventSessionRepairDamageFound)[0]
	if found["when"] != "consultation" || found["first"] == "" {
		t.Errorf("want damage found during a consultation naming the first problem, got %v", found)
	}
	requirePositiveCount(t, found, "problems")
	saved := eventFields(rig.log, domain.EventSessionRepairDamagedCopySaved)[0]
	if got := readText(t, saved["path"]); got != rig.cons.leftAfter(1) {
		t.Error("want the logged path to be the damaged copy")
	}
	consult := eventFields(rig.log, domain.EventSessionRepairConsult)[0]
	if consult["attempt"] != "1" || consult["last_good"] == "" || consult["dispatch_log"] != "" {
		t.Errorf("want attempt 1 with a last-good path and no dispatch log, got %v", consult)
	}
	if accepted := eventFields(rig.log, domain.EventSessionRepairAccepted)[0]; accepted["attempt"] != "1" {
		t.Errorf("want attempt 1 accepted, got %v", accepted)
	}
}

func TestSession_RejectedThenAcceptedRepair_RecordsTheRejectionWithItsReasons(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRewriteRow, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRestoreRow, reply: stopReply("done")},
	}, nil)

	rig.start(t)

	requireEvents(t, rig.log, domain.EventSessionRepairDamageFound, domain.EventSessionRepairDamagedCopySaved,
		domain.EventSessionRepairConsult, domain.EventSessionRepairRejected,
		domain.EventSessionRepairConsult, domain.EventSessionRepairAccepted)
	rejected := eventFields(rig.log, domain.EventSessionRepairRejected)[0]
	requirePositiveCount(t, rejected, "problems")
	if rejected["attempt"] != "1" || !strings.Contains(rejected["reasons"], "ExecutionLog") {
		t.Errorf("want attempt 1 rejected with the reasons named, got %v", rejected)
	}
	consults := eventFields(rig.log, domain.EventSessionRepairConsult)
	if len(consults) != 2 || consults[1]["attempt"] != "2" {
		t.Errorf("want a second repair consultation recorded as attempt 2, got %v", consults)
	}
	if accepted := eventFields(rig.log, domain.EventSessionRepairAccepted)[0]; accepted["attempt"] != "2" {
		t.Errorf("want attempt 2 accepted, got %v", accepted)
	}
}

func TestSession_UnrepairedDamage_RecordsTheStopWithReasonAndConsultations(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRewriteRow, reply: dispatchReply("agent-b", 1)},
		{reply: dispatchReply("agent-b", 1)},
	}, nil)

	rig.start(t)

	requireEvents(t, rig.log, domain.EventSessionRepairDamageFound, domain.EventSessionRepairDamagedCopySaved,
		domain.EventSessionRepairConsult, domain.EventSessionRepairRejected,
		domain.EventSessionRepairConsult, domain.EventSessionRepairRejected, domain.EventSessionRepairStop)
	stop := eventFields(rig.log, domain.EventSessionRepairStop)[0]
	if stop["reason"] != string(domain.UnrepairedRejected) || stop["consultations"] != "2" {
		t.Errorf("want reason rejected after 2 consultations, got %v", stop)
	}
}

func TestSession_FailedRepairConsultation_RecordsTheStopAndNoVerdict(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{err: transportFailure()},
	}, nil)

	rig.start(t)

	requireEvents(t, rig.log, domain.EventSessionRepairDamageFound, domain.EventSessionRepairDamagedCopySaved,
		domain.EventSessionRepairConsult, domain.EventSessionRepairStop)
	stop := eventFields(rig.log, domain.EventSessionRepairStop)[0]
	if stop["reason"] != string(domain.UnrepairedConsultationFailed) || stop["consultations"] != "1" {
		t.Errorf("want reason consultation-failed after 1 consultation, got %v", stop)
	}
}

// noCopyEvidence is the store's evidence whose damaged-copy save fails.
type noCopyEvidence struct{ domain.ArtifactEvidence }

func (noCopyEvidence) SaveDamagedCopy(time.Time) (string, error) { return "", errors.New("disk full") }

func TestSession_DamagedCopyThatCannotBeSaved_IsLoggedAndTheRepairStillProceeds(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("done")},
	}, func(r *repairRig) { r.deps.Evidence = noCopyEvidence{r.deps.Store.(domain.ArtifactEvidence)} })

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	requireEvents(t, rig.log, domain.EventSessionRepairDamageFound, domain.EventSessionRepairDamagedCopyFailed,
		domain.EventSessionRepairConsult, domain.EventSessionRepairAccepted)
	if failed := eventFields(rig.log, domain.EventSessionRepairDamagedCopyFailed)[0]; failed["error"] == "" {
		t.Errorf("want the save error in the event, got %v", failed)
	}
	requireDamageReport(t, rig.cons.request(2))
}

func TestSession_RepairConsultations_AreAnnouncedWithTheirAttemptAndTheirVerdictFollows(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRewriteRow, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRestoreRow, reply: stopReply("done")},
	}, nil)

	rig.start(t)

	var attempts []int
	for _, a := range rig.trace.started() {
		attempts = append(attempts, a.RepairAttempt)
	}
	if want := []int{0, 0, 1, 2}; !slices.Equal(attempts, want) {
		t.Errorf("want announced repair attempts %v, got %v", want, attempts)
	}
	judged := rig.trace.judged()
	if len(judged) != 2 || judged[0].Attempt != 1 || judged[0].Accepted || judged[0].Problems < 1 ||
		judged[1] != (domain.RepairOutcome{Attempt: 2, Accepted: true}) {
		t.Errorf("want attempt 1 not accepted with problems then attempt 2 accepted, got %+v", judged)
	}
	requireBalanced(t, &rig.trace.consultTrace)
	events := rig.trace.snapshot()
	for i, e := range events {
		if e == evRepairJudged && (i == 0 || events[i-1] != evEnd) {
			t.Errorf("want each verdict announced right after its consultation ended: %v", events)
		}
	}
}

// recordingPreConsultant records the request of its single call.
type recordingPreConsultant struct{ reqs []domain.ConsultationRequest }

func (p *recordingPreConsultant) PreConsult(_ context.Context, req domain.ConsultationRequest) (domain.PreConsultationAdvice, error) {
	p.reqs = append(p.reqs, req)
	return domain.PreConsultationAdvice{}, nil
}

func TestSession_PreConsultation_CarriesNoArtifactDamageAndRecordsNoRepairEvent(t *testing.T) {
	pre := &recordingPreConsultant{}
	rig := newRepairRig(t, nil, func(r *repairRig) {
		r.cfg.RunSettings.Mode = domain.ExecutionModeAuto
		r.cfg.PreConsultation = true
		r.deps.PreConsult = pre
	})

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	if len(pre.reqs) != 1 {
		t.Fatalf("want one pre-consultation, got %d", len(pre.reqs))
	}
	requireNoDamageReport(t, pre.reqs[0], "pre-consultation")
	requireEvents(t, rig.log)
	if rig.cons.calls() != 0 {
		t.Errorf("want no routing consultation in a clean auto run, got %d", rig.cons.calls())
	}
}
