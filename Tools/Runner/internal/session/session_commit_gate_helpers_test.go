package session_test

// Shared setup for session tests that run with commits enabled. A new run with
// commits enabled dispatches the commit agent once for commit setup before the
// first workflow row; the helpers script that reply, enable the setting, and
// let a test read the trigger-fired dispatches and rows without the setup ones.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

const (
	// commitSetupBranch is the branch the scripted commit setup reply reports.
	commitSetupBranch = "mosaic/run/test-branch"

	// commitSetupSummary is the status message of the scripted setup reply.
	commitSetupSummary = "branch ready [branch:" + commitSetupBranch + "]"

	// commitSetupTaskPrefix starts the task description of the setup dispatch.
	commitSetupTaskPrefix = "commit setup:"
)

// commitSetupEntry is the commit agent's scripted reply to the setup dispatch.
func commitSetupEntry() harness.ScriptedEntry {
	return harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCommitAgent + "#setup",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   commitSetupSummary,
	}}
}

// enableCommits turns commits on in cfg and queues the setup reply on f. Call
// it before queueing any other reply for the commit agent: replies are consumed
// in queue order and the setup dispatch comes first.
func enableCommits(f *harness.MockAdapter, cfg *domain.RunConfig) {
	cfg.Commits = true
	cfg.CommitBranchVariant = domain.CommitBranchMOSAICOwned
	f.Queue(infraCommitAgent, commitSetupEntry())
}

// commitsRun is the settings of a new run in mode with commits enabled. The
// test queues the commit setup reply itself, ahead of the commit agent's other
// replies.
func commitsRun(mode domain.ExecutionMode) domain.RunSettings {
	return domain.RunSettings{
		Mode:                mode,
		Commits:             true,
		CommitBranchVariant: domain.CommitBranchMOSAICOwned,
	}
}

// queueCommitSetup queues the setup reply of the rig's commit agent. Call it
// before queueing any other reply for the commit agent.
func (r *blockRig) queueCommitSetup() {
	r.adapter.Queue(infraCommitAgent, commitSetupEntry())
}

// commitsConfig is config with commits enabled. The test queues the setup
// reply itself (queueCommitSetup) before the commit agent's other replies.
func (r *blockRig) commitsConfig(workflowID string, mode domain.ExecutionMode) domain.RunConfig {
	cfg := r.config(workflowID, mode)
	cfg.Commits = true
	cfg.CommitBranchVariant = domain.CommitBranchMOSAICOwned
	return cfg
}

// commitsRecorded returns rs as a run records it after commit setup succeeded:
// commits enabled with the setup branch recorded. A resume of such a run
// dispatches no setup.
func commitsRecorded(rs domain.RunSettings) domain.RunSettings {
	rs.Commits = true
	rs.CommitBranch = commitSetupBranch
	rs.CommitBranchVariant = domain.CommitBranchMOSAICOwned
	return rs
}

// recordCommitsDisabled marks the recorded run of resumeFrom as one with
// commits disabled.
func (r *blockRig) recordCommitsDisabled() {
	r.store.state.Commits = false
	r.store.state.CommitBranch = ""
}

// recordCommitsEnabled marks the recorded run of resumeFrom as one with
// commits enabled and the setup branch recorded.
func (r *blockRig) recordCommitsEnabled() {
	r.store.state.RunSettings = commitsRecorded(r.store.state.RunSettings)
}

// isCommitSetup reports whether req is the commit setup dispatch.
func isCommitSetup(req domain.ProtocolRequest) bool {
	return strings.HasPrefix(req.TaskDescription, commitSetupTaskPrefix)
}

// triggerInvocations returns the invocations of f in dispatch order, without
// the commit setup dispatch of a commits-enabled run.
func triggerInvocations(f *harness.MockAdapter) []harness.Invocation {
	var out []harness.Invocation
	for _, inv := range f.Invocations() {
		if !isCommitSetup(inv.Request) {
			out = append(out, inv)
		}
	}
	return out
}

// requireNoCommitActivity fails the test when the commit agent was dispatched
// by a trigger or has an infrastructure row other than commit setup.
func requireNoCommitActivity(t *testing.T, rig *blockRig) {
	t.Helper()
	if n := len(rig.requestsOf(infraCommitAgent)); n != 0 {
		t.Errorf("want no trigger dispatch of %s, got %d", infraCommitAgent, n)
	}
	if rows := infraRowsOf(rig.store, infraCommitAgent); len(rows) != 0 {
		t.Errorf("want no Execution Log row of %s, got %+v", infraCommitAgent, rows)
	}
}
