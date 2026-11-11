package integration_test

// A resume of a run whose Orchestration.md has damaged frontmatter, from the
// command line through the early resume checks. With a usable last-good copy the
// run passes into the session, which makes the repair consultation at start;
// without one the run is refused before the session starts. The artifacts are
// tool-local fixtures under testdata/damaged.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// newFrontmatterDamagedRun lays out a deployed workspace for an enabled CLI
// harness and a run whose frontmatter is damaged. With lastGood, the run's
// last-good copy is placed where the Runner keeps it.
func newFrontmatterDamagedRun(t *testing.T, lastGood bool) *damagedResumeRun {
	t.Helper()
	entry := enabledCLIHarnessEntry(t)
	dir := t.TempDir()
	t.Chdir(dir)

	agentsDir := filepath.Join(dir, entry.AgentsDir)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", agentsDir, err)
	}
	writeOrchWithFrontmatter(t, agentsDir, "orchestrator-script", linearOrchFixture)
	writeAgentFile(t, agentsDir, "agent-a")
	writeAgentFile(t, agentsDir, "agent-b")

	runFolder := scopedRunFolder(t, dir)
	artifactPath := copyFile(t, runFolder, "Orchestration.md",
		filepath.Join(damagedFixtureDir, "linear_resume_damaged_frontmatter.md"))
	repaired, err := os.ReadFile(filepath.Join(damagedFixtureDir, "linear_resume_repaired.md"))
	if err != nil {
		t.Fatalf("read repaired fixture: %v", err)
	}
	_, readErr := artifact.NewFileStore(artifactPath).Read(context.Background())
	if !domain.IsArtifactDamage(readErr) {
		t.Fatalf("test setup: want a damage error reading the fixture, got %v", readErr)
	}
	if lastGood {
		evidenceDir := filepath.Join(dir, "RunnerLogs", integrationRunID)
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", evidenceDir, err)
		}
		copyFile(t, evidenceDir, "Orchestration.last-good.md", filepath.Join(damagedFixtureDir, "linear_resume_repaired.md"))
	}
	return &damagedResumeRun{harnessID: entry.ID, artifactPath: artifactPath, repaired: repaired, agentsDir: agentsDir}
}

func TestIntegration_DamagedFrontmatterWithLastGoodCopy_ReachesTheSessionsStartRepair(t *testing.T) {
	r := newFrontmatterDamagedRun(t, true)

	code, stderr, routing, adapter := r.resumeWithRepair(t)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess: the run has a usable last-good copy and must reach the session; stderr: %q", code, stderr)
	}
	if len(routing.requests) == 0 {
		t.Fatal("the session made no repair consultation for the damaged frontmatter")
	}
	if len(adapter.Invocations()) != 1 {
		t.Errorf("invocations = %d, want the repair's dispatch carried out once", len(adapter.Invocations()))
	}
}

func TestIntegration_DamagedFrontmatterWithoutLastGoodCopy_IsRefusedBeforeTheSession(t *testing.T) {
	r := newFrontmatterDamagedRun(t, false)
	routing := &repairingConsultant{artifactPath: r.artifactPath, repaired: r.repaired}
	adapter := harness.NewMockAdapter()
	sess := newSessionWithRouting(adapter, r.artifactPath, routing)
	var out, errOut bytes.Buffer

	code := cli.Run(context.Background(), []string{"run", "--run", integrationRunID, "--harness", r.harnessID},
		artifact.NewFileStore(r.artifactPath), nil, sess, &out, &errOut)

	if code != cli.ExitRefused {
		t.Fatalf("exit code = %d, want ExitRefused; stderr: %q", code, errOut.String())
	}
	if len(routing.requests) != 0 || len(adapter.Invocations()) != 0 {
		t.Errorf("the session started: %d consultations, %d dispatches", len(routing.requests), len(adapter.Invocations()))
	}
}
