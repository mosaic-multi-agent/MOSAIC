package session_test

// How an infrastructure dispatch is recorded: the sent inputs and the error
// code of a BLOCKED outcome reach the Execution Log row, and a harness-level
// failure of an infrastructure dispatch is a BLOCKED row with E501.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// blockedCommit is a commit agent response BLOCKED with the given code.
func blockedCommit(code domain.ErrorCode) harness.ScriptedEntry {
	return harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCommitAgent + "#q",
		StatusCode:      domain.StatusBLOCKED,
		StatusMessage:   "nothing to commit against",
		ErrorCode:       code,
		ErrorReason:     "scripted block",
	}}
}

// runProductExpectingCommitRow runs one Product iteration whose commit
// dispatch ends as entry, and returns the commit's applied row. The run uses
// cfg, which enables commits.
func runProductExpectingCommitRow(t *testing.T, rig *blockRig, cfg domain.RunConfig, entry *harness.ScriptedEntry) domain.CompletedStep {
	t.Helper()
	queueProductIteration(rig, "A")
	if entry != nil {
		rig.adapter.Queue(infraCommitAgent, *entry)
	}

	// The commit agent's on_failure is halt, so a failing dispatch stops the
	// run; the row is recorded before the stop.
	if _, err := rig.ses.Start(context.Background(), cfg); err != nil {
		t.Fatalf("want nil error from Start, got %v", err)
	}
	rows := infraRowsOf(rig.store, infraCommitAgent)
	if len(rows) != 1 {
		t.Fatalf("want 1 commit infrastructure row, got %d", len(rows))
	}
	return rows[0]
}

func TestSession_Start_InfraDispatchBlockedWithErrorCode_RowCarriesTheCode(t *testing.T) {
	rig := newProductRig(t, "iteration-end-product-block-orch.md", infraCommitAgent, "A")
	rig.queueCommitSetup()
	entry := blockedCommit(domain.ErrorINPUT_NOT_FOUND)

	row := runProductExpectingCommitRow(t, rig, rig.commitsConfig("product-block", domain.ExecutionModeAuto), &entry)

	if row.Status != domain.StatusBLOCKED {
		t.Errorf("want row status BLOCKED, got %q", row.Status)
	}
	if row.ErrorCode != domain.ErrorINPUT_NOT_FOUND {
		t.Errorf("want row error code %q, got %q", domain.ErrorINPUT_NOT_FOUND, row.ErrorCode)
	}
}

func TestSession_Start_InfraDispatchHarnessInvocationError_RowIsBlockedE501(t *testing.T) {
	rig := newProductRig(t, "iteration-end-product-block-orch.md", infraCommitAgent, "A")
	rig.queueCommitSetup()
	entry := harness.ScriptedEntry{Err: errors.New("harness transport failed")}

	row := runProductExpectingCommitRow(t, rig, rig.commitsConfig("product-block", domain.ExecutionModeAuto), &entry)

	if row.Status != domain.StatusBLOCKED {
		t.Errorf("want row status BLOCKED, got %q", row.Status)
	}
	if row.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("want row error code %q, got %q", domain.ErrorTOOL_UNAVAILABLE, row.ErrorCode)
	}
}

func TestSession_Start_InfraDispatchAgentDefinitionUnresolvable_RowIsBlockedE501(t *testing.T) {
	// The commit agent's definition file is not written, so the trigger
	// dispatch cannot resolve it. The run is a resume that recorded its commit
	// branch, so commit setup (which would need the definition) does not run.
	rig := newBlockRig(t, "iteration-end-product-block-orch.md", nil, "planner", "builder", "checker")
	rig.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids("A"))))
	rig.resumeFrom("product-block", domain.ExecutionModeAuto)
	rig.recordCommitsEnabled()

	row := runProductExpectingCommitRow(t, rig, rig.resumeConfig("product-block", domain.ExecutionModeAuto), nil)

	if row.Status != domain.StatusBLOCKED {
		t.Errorf("want row status BLOCKED, got %q", row.Status)
	}
	if row.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("want row error code %q, got %q", domain.ErrorTOOL_UNAVAILABLE, row.ErrorCode)
	}
	if n := len(rig.requestsOf(infraCommitAgent)); n != 0 {
		t.Errorf("want no dispatch of an agent whose definition cannot be resolved, got %d", n)
	}
}

// ---- rendered through the real artifact file store ----

// runStageGroupedOnFileStore runs the grouped Stage fixture as a new run on the
// real artifact file store, the commit agent answering with commitEntry, and
// returns the commit's Execution Log row text.
func runStageGroupedOnFileStore(t *testing.T, commitEntry harness.ScriptedEntry) string {
	t.Helper()
	rig := newStageGroupedRig(t, "2")
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{"test-writer": "Stage-2/PlanProgress.md"})
	rig.queueCommitSetup()
	rig.adapter.Queue(infraCommitAgent, commitEntry)
	path := filepath.Join(rig.runFolder, "Orchestration.md")
	rig.ses = session.New(session.Deps{
		Harness:  rig.adapter,
		Store:    artifact.NewFileStore(path),
		Routing:  rig.consult,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	cfg := rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto)
	cfg.RunID = testRunID

	if _, err := rig.ses.Start(context.Background(), cfg); err != nil {
		t.Fatalf("want nil error from Start, got %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run artifact: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, infraCommitAgent+"#") && !strings.Contains(line, commitSetupSummary) {
			return line
		}
	}
	t.Fatalf("no Execution Log row of %s in the artifact:\n%s", infraCommitAgent, data)
	return ""
}

func TestSession_Start_CommitRowOnFileStore_ListsSentInputsWithoutRunPrefix(t *testing.T) {
	row := runStageGroupedOnFileStore(t, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCommitAgent + "#q",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "committed",
	}})

	if !strings.Contains(row, "Stage-2/Plan.md, Stage-2/PlanProgress.md") {
		t.Errorf("want the row's Inputs to list the sent artifacts, got %q", row)
	}
	if strings.Contains(row, domain.RunScopedFolder(testRunID)) {
		t.Errorf("want the run prefix removed from the recorded Inputs, got %q", row)
	}
}

func TestSession_Start_BlockedCommitRowOnFileStore_ShowsTheErrorMarker(t *testing.T) {
	row := runStageGroupedOnFileStore(t, blockedCommit(domain.ErrorINPUT_NOT_FOUND))

	if !strings.Contains(row, "[error:E101]") {
		t.Errorf("want the Summary to end with the [error:E101] marker, got %q", row)
	}
}
