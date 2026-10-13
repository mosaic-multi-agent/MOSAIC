package session_test

// Shared helpers for the tests of defective produced schedules: schedule
// documents with a known defect, scripted producer dispatches, readers of the
// dispatch order and of the fix message, and the checks of the consultation a
// defect that outlives the re-dispatch limit leads to.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const (
	sdWorkflow   = "audit-plan-block"
	sdSourceFile = "AuditPlan.md"
	sdBlockName  = "Stage"
	sdSection    = "Stages"
	// sdDuplicateToken is the offending ID of dupSchedule: the later of two
	// IDs that differ only in case.
	sdDuplicateToken = "dup-1"
	// sdProducerMessage is the status message the scripted producers return.
	sdProducerMessage = "wrote the audit plan"
)

// sdAutoModes are the modes in which the engine routes.
var sdAutoModes = []domain.ExecutionMode{domain.ExecutionModeAuto, domain.ExecutionModeAutoReview}

// dupSchedule repeats an ID in different case (rule V2, token sdDuplicateToken).
func dupSchedule() string {
	return scheduleDoc(sdBlockName, sdSection, ids("DUP-1", "B", sdDuplicateToken))
}

// okSchedule is a valid schedule of the given iteration IDs.
func okSchedule(list ...string) string {
	return scheduleDoc(sdBlockName, sdSection, ids(list...))
}

// unsatisfiableDependsSchedule has a Depends On entry that names no earlier
// row (rule V4, token "Z").
func unsatisfiableDependsSchedule() string {
	var b strings.Builder
	b.WriteString("# Working Plan\n\n## Stages\n\n")
	b.WriteString("| Stage | Name | Goal | Depends On | HITL |\n|---|------|------|------------|:----:|\n")
	b.WriteString("| A | Item A | Goal of A | - | FALSE |\n")
	b.WriteString("| B | Item B | Goal of B | Z | FALSE |\n")
	return b.String()
}

// queueProducer queues one SUCCESS dispatch of agent with the given status
// message; each write is applied while that dispatch runs.
func (r *blockRig) queueProducer(agent, message string, writes ...harness.ScriptedWrite) {
	r.adapter.Queue(agent, harness.ScriptedEntry{
		Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#queued",
			StatusCode:      domain.StatusSUCCESS,
			StatusMessage:   message,
		},
		Writes: writes,
	})
}

// queueDefectivePlanner queues n planner dispatches that each write the
// duplicate-ID schedule and return sdProducerMessage.
func (r *blockRig) queueDefectivePlanner(n int) {
	for i := 0; i < n; i++ {
		r.queueProducer("planner", sdProducerMessage, runFileWrite(sdSourceFile, dupSchedule()))
	}
}

// dispatchOrder returns the agent of every dispatch, in dispatch order.
func (r *blockRig) dispatchOrder() []string {
	var out []string
	for _, inv := range r.adapter.Invocations() {
		out = append(out, inv.Agent.Identifier)
	}
	return out
}

// invocationsFor returns the dispatches of agent, in dispatch order.
func (r *blockRig) invocationsFor(agent string) []harness.Invocation {
	var out []harness.Invocation
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent {
			out = append(out, inv)
		}
	}
	return out
}

// requireFixRequest asserts that invs[i] is a re-dispatch asking for a corrected
// schedule: the non-compliant artifact is among its inputs and its task
// description names every part (rule, block, Source, Section, offending ID).
func requireFixRequest(t *testing.T, invs []harness.Invocation, i int, source string, parts ...string) {
	t.Helper()
	if i >= len(invs) {
		t.Fatalf("want a dispatch at position %d, got only %d", i+1, len(invs))
	}
	inv := invs[i]
	if !slices.Contains(inv.Request.InputArtifacts, dispatchPath(source)) {
		t.Errorf("want the non-compliant %s among the input artifacts, got %q", source, inv.Request.InputArtifacts)
	}
	requireNames(t, "task description", inv.Request.TaskDescription, parts...)
}

// requireNames asserts that text contains every part.
func requireNames(t *testing.T, what, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("want the %s to name %q, got %q", what, part, text)
		}
	}
}

// duplicateIDParts are what the fix message and the defect description name
// for a dupSchedule defect.
func duplicateIDParts() []string {
	return []string{"V2", sdBlockName, sdSourceFile, sdSection, sdDuplicateToken}
}

// requireScheduleDefectConsult asserts that req is the consultation after an
// exhausted re-dispatch limit: kind schedule-defect, the defect description in
// last_error_reason and the producer's verbatim message in last_status_message.
func requireScheduleDefectConsult(t *testing.T, req domain.ConsultationRequest, wantReasonParts ...string) {
	t.Helper()
	if req.Deviation == nil || req.Deviation.Kind != domain.DeviationScheduleDefect {
		t.Errorf("want a deviation of kind %q, got %+v", domain.DeviationScheduleDefect, req.Deviation)
	}
	if req.LastErrorReason == nil {
		t.Fatalf("want the defect description in last_error_reason, got nil")
	}
	requireNames(t, "last_error_reason", *req.LastErrorReason, wantReasonParts...)
	if req.LastStatusMessage == nil || *req.LastStatusMessage != sdProducerMessage {
		t.Errorf("want last_status_message %q, got %v", sdProducerMessage, req.LastStatusMessage)
	}
}

// newNoConsultantRig is newBlockRig for a session with no routing consultant.
func newNoConsultantRig(t *testing.T, fixture string, agents ...string) *blockRig {
	t.Helper()
	parent := t.TempDir()
	orchPath := copyOrchestratorFile(t, parent, fixture)
	for _, a := range agents {
		writeAgentFile(t, parent, a)
	}
	runFolder := filepath.Join(parent, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("newNoConsultantRig: create run folder: %v", err)
	}
	f := harness.NewMockAdapter()
	f.SetWriteRoot(parent)
	store := &memStore{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    store,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	return &blockRig{ses: ses, adapter: f, store: store, orchPath: orchPath, runFolder: runFolder}
}

// resumeWithLog records a run of workflowID whose Execution Log is entries; the
// last entry is the step the run stopped after.
func (r *blockRig) resumeWithLog(workflowID string, mode domain.ExecutionMode, entries []domain.ExecutionLogEntry) {
	r.resumeFrom(workflowID, mode)
	last := entries[len(entries)-1]
	r.store.state.GlobalSequence = last.Seq
	r.store.state.CurrentState = domain.CurrentState{
		Phase:      last.Phase,
		Stage:      last.Stage,
		LastStatus: last.Status,
		LastAgent:  last.Agent,
	}
	r.store.state.ExecutionLog = entries
}

// logStep is one Execution Log entry of agent at the 1-based workflow row.
func logStep(seq int, agent, phase, stage string, row int, status domain.StatusCode, summary string) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{
		Seq:         seq,
		Agent:       fmt.Sprintf("%s#%d", agent, seq),
		Phase:       phase,
		Stage:       stage,
		WorkflowRow: domain.WorkflowRow(row),
		Status:      status,
		Summary:     summary,
	}
}

// plannerSuccess is a SUCCESS entry of the planner row (row 1, run scope).
func plannerSuccess(seq int, summary string) domain.ExecutionLogEntry {
	return logStep(seq, "planner", "PLANNING", "", 1, domain.StatusSUCCESS, summary)
}
