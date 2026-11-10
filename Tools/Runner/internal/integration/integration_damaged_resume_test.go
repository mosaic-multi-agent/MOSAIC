package integration_test

// A resume of a run whose Orchestration.md is damaged, from the command line
// through the early resume checks into the session: the run is not refused
// before it starts, and the session makes the repair consultation at start. The
// workflow, the damaged artifact and the repaired artifact are tool-local
// fixtures under testdata/damaged.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/artifact"
	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// Resolved when the test binary starts, before any test changes directory.
var damagedFixtureDir = mustAbs("../../testdata/damaged")

var linearOrchFixture = mustAbs("../../testdata/session/linear-orch.md")

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}

// repairingConsultant answers the routing consultations of a damaged start: on
// each call it records the request, writes the repaired artifact over the
// damaged one, and replies with reply.
type repairingConsultant struct {
	artifactPath string
	repaired     []byte
	reply        domain.RoutingInstruction
	requests     []domain.ConsultationRequest
}

func (c *repairingConsultant) ConsultRouting(_ context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.requests = append(c.requests, req)
	if err := os.WriteFile(c.artifactPath, c.repaired, 0o600); err != nil {
		return domain.RoutingInstruction{}, err
	}
	return c.reply, nil
}

// damagedResumeRun is a workspace holding a deployed orchestrator file for an
// enabled CLI harness and the damaged run, set as the working directory.
type damagedResumeRun struct {
	harnessID    string
	artifactPath string
	repaired     []byte
	agentsDir    string
}

func enabledCLIHarnessEntry(t *testing.T) commonharness.CLIHarness {
	t.Helper()
	for _, entry := range commonharness.CLIHarnesses() {
		if _, disabled := harness.DisabledReason(entry.ID); !disabled {
			return entry
		}
	}
	t.Fatal("test setup: no CLI harness is enabled")
	return commonharness.CLIHarness{}
}

func newDamagedResumeRun(t *testing.T) *damagedResumeRun {
	t.Helper()
	entry := enabledCLIHarnessEntry(t)
	dir := t.TempDir()
	t.Chdir(dir)

	agentsDir := filepath.Join(dir, entry.AgentsDir)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", agentsDir, err)
	}
	copyFile(t, agentsDir, "orchestrator-script.md", linearOrchFixture)
	writeAgentFile(t, agentsDir, "agent-a")
	writeAgentFile(t, agentsDir, "agent-b")

	runFolder := scopedRunFolder(t, dir)
	artifactPath := copyFile(t, runFolder, "Orchestration.md", filepath.Join(damagedFixtureDir, "linear_resume_damaged.md"))
	repaired, err := os.ReadFile(filepath.Join(damagedFixtureDir, "linear_resume_repaired.md"))
	if err != nil {
		t.Fatalf("read repaired fixture: %v", err)
	}
	if _, err := artifact.NewFileStore(artifactPath).Read(context.Background()); err == nil {
		t.Fatal("test setup: the damaged fixture reads cleanly")
	}
	return &damagedResumeRun{harnessID: entry.ID, artifactPath: artifactPath, repaired: repaired, agentsDir: agentsDir}
}

// resumeWithRepair resumes the damaged run from the command line with a real
// session whose routing consultant repairs the artifact and dispatches agent-b
// again. It returns the exit code, stderr, the consultant and the adapter.
func (r *damagedResumeRun) resumeWithRepair(t *testing.T) (int, string, *repairingConsultant, *harness.MockAdapter) {
	t.Helper()
	routing := &repairingConsultant{
		artifactPath: r.artifactPath,
		repaired:     r.repaired,
		reply: domain.RoutingInstruction{Dispatch: &domain.DispatchInstruction{
			Agent: "agent-b", RowIndex: 1, TaskDescription: "run agent-b again",
		}},
	}
	adapter := harness.NewMockAdapter()
	adapter.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#3", StatusCode: domain.StatusSUCCESS, StatusMessage: "done after the repair",
	}})
	sess := newSessionWithRouting(adapter, r.artifactPath, routing)
	var out, errOut bytes.Buffer

	code := cli.Run(context.Background(), []string{"run", "--run", integrationRunID, "--harness", r.harnessID},
		artifact.NewFileStore(r.artifactPath), nil, sess, &out, &errOut)
	return code, errOut.String(), routing, adapter
}

func TestIntegration_DamagedRunResumedFromTheCLI_ReachesTheSessionsStartRepair(t *testing.T) {
	r := newDamagedResumeRun(t)

	code, stderr, routing, adapter := r.resumeWithRepair(t)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess: the damaged run must reach the session; stderr: %q", code, stderr)
	}
	if len(routing.requests) == 0 {
		t.Fatal("the session made no repair consultation for the damaged artifact")
	}
	report := routing.requests[0].ArtifactDamage
	if report == nil || len(report.Problems) == 0 {
		t.Fatalf("the first consultation carries no damage report: %+v", routing.requests[0])
	}
	invs := adapter.Invocations()
	if len(invs) != 1 || invs[0].Request.AgentInstanceID != "agent-b#3" {
		t.Fatalf("invocations = %d, want the repair's dispatch (agent-b#3) carried out once", len(invs))
	}
}

func TestIntegration_DamagedRunResumedFromTheCLI_ArtifactReadsCleanlyAfterTheRun(t *testing.T) {
	r := newDamagedResumeRun(t)

	code, stderr, _, _ := r.resumeWithRepair(t)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, stderr)
	}
	state, err := artifact.NewFileStore(r.artifactPath).Read(context.Background())
	if err != nil {
		t.Fatalf("the artifact does not read cleanly after the run: %v", err)
	}
	var agents []string
	for _, e := range state.ExecutionLog {
		agents = append(agents, e.Agent)
	}
	if got := strings.Join(agents, ","); got != "agent-a#1,agent-b#2,agent-b#3" {
		t.Errorf("Execution Log agents = %s, want the repaired rows followed by the dispatched step", got)
	}
}

func TestIntegration_DamagedRunWithoutARoutingConsultant_IsNotRefusedBeforeTheSession(t *testing.T) {
	r := newDamagedResumeRun(t)
	adapter := harness.NewMockAdapter()
	sess := newSession(adapter, r.artifactPath)
	var out, errOut bytes.Buffer

	code := cli.Run(context.Background(), []string{"run", "--run", integrationRunID, "--harness", r.harnessID},
		artifact.NewFileStore(r.artifactPath), nil, sess, &out, &errOut)

	// With no routing consultant the session cannot repair and fails the run; a
	// refusal here would mean the early checks ended it before the session.
	if code == cli.ExitRefused {
		t.Fatalf("the damaged run was refused by the early checks instead of reaching the session; stderr: %q", errOut.String())
	}
	if code == cli.ExitSuccess {
		t.Errorf("exit code = ExitSuccess for a damaged run that could not be repaired")
	}
	if len(adapter.Invocations()) != 0 {
		t.Errorf("%d dispatches were made on a damaged artifact that could not be repaired", len(adapter.Invocations()))
	}
	data, err := os.ReadFile(r.artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	original, err := os.ReadFile(filepath.Join(damagedFixtureDir, "linear_resume_damaged.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Error("the artifact was changed although no repair was made")
	}
}
