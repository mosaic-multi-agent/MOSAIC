package session_test

// Shared rig for the tests of block schedules read from their declared
// Source and Section: a session over a fixture, the run folder beside the
// orchestrator file, scripted agent writes, and the readers of what the
// session dispatched.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// blockRig is a session wired to a fake harness over one fixture workflow.
// The orchestrator file and the agent definitions live in parent; the run
// folder is Orchestration-{testRunID} beneath it, and the adapter writes
// scripted files relative to parent (the dispatched path form).
type blockRig struct {
	ses       session.Session
	adapter   *harness.MockAdapter
	store     *memStore
	consult   *scriptedRoutingConsultant
	orchPath  string
	runFolder string
}

// newBlockRig builds the rig for fixture with an agent definition per agent.
// approvals may be nil.
func newBlockRig(t *testing.T, fixture string, approvals domain.ApprovalReader, agents ...string) *blockRig {
	t.Helper()
	return newBlockRigWith(t, fixture, approvals, nil, agents...)
}

// newBlockRigWith is newBlockRig with an output write detector. A non-nil
// detector makes the process working directory the rig's parent directory, so
// the run-scoped paths the session dispatches resolve against the files the
// scripted agents write.
func newBlockRigWith(t *testing.T, fixture string, approvals domain.ApprovalReader, outputs domain.OutputWriteDetector, agents ...string) *blockRig {
	t.Helper()
	return newBlockRigObserved(t, fixture, approvals, outputs, nil, agents...)
}

// newBlockRigObserved is newBlockRigWith with a consultation observer, which
// may be nil.
func newBlockRigObserved(t *testing.T, fixture string, approvals domain.ApprovalReader, outputs domain.OutputWriteDetector, obs domain.ConsultationObserver, agents ...string) *blockRig {
	t.Helper()
	parent := t.TempDir()
	if outputs != nil {
		t.Chdir(parent)
	}
	orchPath := copyOrchestratorFile(t, parent, fixture)
	for _, a := range agents {
		writeAgentFile(t, parent, a)
	}
	runFolder := filepath.Join(parent, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("newBlockRig: create run folder: %v", err)
	}
	f := harness.NewMockAdapter()
	f.SetWriteRoot(parent)
	store := &memStore{}
	consult := &scriptedRoutingConsultant{}
	ses := session.New(session.Deps{
		Harness:   f,
		Store:     store,
		Routing:   consult,
		Approvals: approvals,
		Outputs:   outputs,
		Clock:     fixedClock{t: epoch},
		Interact:  &noopInteraction{},

		Consultations: obs,
	})
	return &blockRig{ses: ses, adapter: f, store: store, consult: consult, orchPath: orchPath, runFolder: runFolder}
}

// config returns a new-run configuration for the workflow in the given mode.
func (r *blockRig) config(workflowID string, mode domain.ExecutionMode) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: r.orchPath,
		WorkflowID:           domain.WorkflowID(workflowID),
		Task:                 "test task",
		IsNewRun:             true,
		RunFolder:            r.runFolder,
		RunSettings:          domain.RunSettings{Mode: mode},
	}
}

// resumeConfig returns the configuration of a resume of the same run folder.
func (r *blockRig) resumeConfig(workflowID string, mode domain.ExecutionMode) domain.RunConfig {
	cfg := r.config(workflowID, mode)
	markResume(&cfg)
	return cfg
}

// resumeFrom records a run of workflowID that has completed nothing yet, so
// a resume starts at the first row.
func (r *blockRig) resumeFrom(workflowID string, mode domain.ExecutionMode) {
	r.store.exists = true
	r.store.state = domain.ArtifactState{
		Type:            "orchestration-artifact",
		Workflow:        domain.WorkflowID(workflowID),
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  0,
		RunSettings:     domain.RunSettings{Mode: mode},
	}
}

// resumeAfterFirstRow records a run of workflowID whose first routing row
// (agent, in phase) completed successfully, so a resume continues after it.
func (r *blockRig) resumeAfterFirstRow(workflowID string, mode domain.ExecutionMode, agent, phase string) {
	r.resumeFrom(workflowID, mode)
	r.store.state.GlobalSequence = 1
	r.store.state.CurrentState = domain.CurrentState{
		Phase:      phase,
		LastStatus: domain.StatusSUCCESS,
		LastAgent:  agent + "#1",
	}
	r.store.state.ExecutionLog = []domain.ExecutionLogEntry{
		{Seq: 1, Agent: agent + "#1", Phase: phase, WorkflowRow: 1, Status: domain.StatusSUCCESS},
	}
}

// writeRunFile writes content to rel inside the run folder before the run.
func (r *blockRig) writeRunFile(t *testing.T, rel, content string) {
	t.Helper()
	p := filepath.Join(r.runFolder, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatalf("writeRunFile: mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writeRunFile: write %s: %v", rel, err)
	}
}

// queueOK queues one SUCCESS dispatch of agent. Each write is applied to the
// run folder while that dispatch runs, as an agent's own output would be.
func (r *blockRig) queueOK(agent string, writes ...harness.ScriptedWrite) {
	r.adapter.Queue(agent, harness.ScriptedEntry{
		Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#queued",
			StatusCode:      domain.StatusSUCCESS,
			StatusMessage:   "done",
		},
		Writes: writes,
	})
}

// queueOKTimes queues n plain SUCCESS dispatches of agent.
func (r *blockRig) queueOKTimes(agent string, n int) {
	for i := 0; i < n; i++ {
		r.queueOK(agent)
	}
}

// invocationsOf counts the dispatches of agent, except the commit setup
// dispatch of a commits-enabled run.
func (r *blockRig) invocationsOf(agent string) int {
	n := 0
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent && !isCommitSetup(inv.Request) {
			n++
		}
	}
	return n
}

// runFileWrite is a scripted write of content to rel inside the run folder.
func runFileWrite(rel, content string) harness.ScriptedWrite {
	return harness.ScriptedWrite{Path: dispatchPath(rel), Content: content}
}

// scheduleRow is one iteration of a schedule built by scheduleDoc.
type scheduleRow struct {
	id   string
	hitl bool
}

// ids turns plain iteration IDs into schedule rows with HITL FALSE.
func ids(list ...string) []scheduleRow {
	out := make([]scheduleRow, 0, len(list))
	for _, id := range list {
		out = append(out, scheduleRow{id: id})
	}
	return out
}

// scheduleDoc renders an artifact holding an unrelated Overview table and,
// below it, the schedule: section heading, a table whose first column is
// headed by block, no Depends On column.
func scheduleDoc(block, section string, rows []scheduleRow) string {
	var b strings.Builder
	b.WriteString("# Working Plan\n\n## Overview\n\n")
	b.WriteString("| Item | Note |\n|------|------|\n| scope | prose, not a schedule |\n\n")
	fmt.Fprintf(&b, "## %s\n\n", section)
	fmt.Fprintf(&b, "| %s | Name | Goal | HITL |\n|---|------|------|:----:|\n", block)
	for _, r := range rows {
		flag := "FALSE"
		if r.hitl {
			flag = "TRUE"
		}
		fmt.Fprintf(&b, "| %s | Item %s | Goal of %s | %s |\n", r.id, r.id, r.id, flag)
	}
	return b.String()
}

// dispatchedIterations returns, in dispatch order, the iteration each
// dispatch of agent ran at, read from the output path the dispatch received:
// the text between the run-scoped prefix and the suffix (the block's fill-in
// in the row's declared Output).
func (r *blockRig) dispatchedIterations(agent, prefix, suffix string) []string {
	var out []string
	full := dispatchPath(prefix)
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier != agent {
			continue
		}
		for _, o := range inv.Request.OutputArtifacts {
			if strings.HasPrefix(o, full) && strings.HasSuffix(o, suffix) && len(o) >= len(full)+len(suffix) {
				out = append(out, strings.TrimSuffix(strings.TrimPrefix(o, full), suffix))
				break
			}
		}
	}
	return out
}

// hitlOf returns, in dispatch order, the human_in_the_loop value each
// dispatch of agent carried.
func (r *blockRig) hitlOf(agent string) []bool {
	var out []bool
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent {
			out = append(out, inv.Request.HumanInTheLoop)
		}
	}
	return out
}

// requireSequence asserts two string sequences are equal.
func requireSequence(t *testing.T, what string, got, want []string) {
	t.Helper()
	if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
		t.Errorf("%s: want %q, got %q", what, want, got)
	}
}
