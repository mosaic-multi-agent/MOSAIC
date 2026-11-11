package session_test

// Start repair when a later start-sequence write finds the damage: the settings
// adoption and the commit-branch write of a resumed run each re-read the
// artifact and refuse when it is damaged. A store double damages the artifact
// just before delegating to the real file store, so the real re-read refuses.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const laterBranch = "mosaic/run/later-read"

// damagingStore wraps the real file store. The first `remaining` calls of
// AdoptRunnerSettings and SetCommitBranch write a stray line into the Execution
// Log before delegating, so the delegate's own re-read finds damage.
type damagingStore struct {
	domain.ArtifactStore
	t           *testing.T
	path        string
	remaining   int
	adoptCalls  int
	branchCalls int
}

func (d *damagingStore) damage() {
	d.t.Helper()
	if d.remaining <= 0 {
		return
	}
	d.remaining--
	data, err := os.ReadFile(d.path)
	if err != nil {
		d.t.Fatalf("damaging store: %v", err)
	}
	if err := os.WriteFile(d.path, []byte(damageLog(d.t, string(data))), 0o600); err != nil {
		d.t.Fatalf("damaging store: %v", err)
	}
}

func (d *damagingStore) SetCommitBranch(ctx context.Context, branch string, now time.Time) (domain.ArtifactState, error) {
	d.branchCalls++
	d.damage()
	return d.ArtifactStore.SetCommitBranch(ctx, branch, now)
}

func (d *damagingStore) AdoptRunnerSettings(ctx context.Context, mode domain.ExecutionMode, pre, manual bool, now time.Time) (domain.ArtifactState, error) {
	d.adoptCalls++
	d.damage()
	return d.ArtifactStore.AdoptRunnerSettings(ctx, mode, pre, manual, now)
}

var runnerSettingLines = regexp.MustCompile(`(?m)^runner_[a-z_]+: .*\n`)

// withLaterReadResume turns the rig into a resume of a valid run, behind a
// damaging store that damages the artifact on its first `damages` guarded
// writes. withCommits runs the commit-agent workflow with commit setup pending;
// otherwise the artifact is native-created, without runner settings, so the
// resume adopts the supplied ones.
func withLaterReadResume(t *testing.T, withCommits bool, damages int, store **damagingStore) func(*repairRig) {
	return func(r *repairRig) {
		t.Helper()
		dir := filepath.Dir(r.run.orchPath)
		settings := domain.RunSettings{Mode: domain.ExecutionModeOrchestrated}
		if withCommits {
			copyOrchestratorFile(t, dir, "commit-agent-orch.md")
			writeAgentFile(t, dir, "commit-manager-git")
			settings.Commits = true
			settings.CommitBranchVariant = domain.CommitBranchMOSAICOwned
			r.f.Queue("commit-manager-git", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
				AgentInstanceID: "commit-manager-git#1",
				StatusCode:      domain.StatusSUCCESS,
				StatusMessage:   "branch ready [branch:" + laterBranch + "]",
			}})
		}
		real := artifact.NewFileStore(r.run.artifactPath)
		if _, err := real.Create(context.Background(), domain.WorkflowInfo{ID: "linear", Version: "1.0"}, "test task", settings, epoch, testRunID); err != nil {
			t.Fatalf("test setup: %v", err)
		}
		if !withCommits {
			doc := readText(t, r.run.artifactPath)
			writeFile(t, r.run.artifactPath, runnerSettingLines.ReplaceAllString(doc, ""))
			r.cfg.Supplied.Mode = true
		} else {
			r.cfg.Commits = true
			r.cfg.CommitBranchVariant = domain.CommitBranchMOSAICOwned
			r.cfg.Supplied.CommitBranchVariant = true
		}
		r.cfg.IsNewRun = false
		*store = &damagingStore{ArtifactStore: real, t: t, path: r.run.artifactPath, remaining: damages}
		r.deps.Store = *store
	}
}

func TestSession_CommitBranchWriteThatFindsDamage_IsRepairedAndWrittenOnceMore(t *testing.T) {
	var store *damagingStore
	rig := newRepairRig(t, []repairStep{
		{edit: fixLog, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, withLaterReadResume(t, true, 1, &store))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	report := requireDamageReport(t, rig.cons.request(0))
	if len(report.Problems) == 0 {
		t.Error("want the damage report to list the problem the refused write found")
	}
	requireStartTriggersNull(t, rig.cons.request(0))
	if store.branchCalls != 2 {
		t.Errorf("want the branch written once more after the repair (2 calls), got %d", store.branchCalls)
	}
	want := []string{"commit-manager-git#1", "agent-a#2"}
	if ids := invokedInstances(rig.f); len(ids) != 2 || ids[0] != want[0] || ids[1] != want[1] {
		t.Errorf("want the commit setup agent invoked once and the repair's dispatch carried out %v, got %v", want, ids)
	}
	if doc := readText(t, rig.run.artifactPath); !strings.Contains(doc, laterBranch) {
		t.Error("want the commit branch recorded in the artifact after the repair")
	}
}

func TestSession_SettingsAdoptionThatFindsDamage_IsRepairedAndAdoptedOnceMore(t *testing.T) {
	var store *damagingStore
	rig := newRepairRig(t, []repairStep{
		{edit: fixLog, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, withLaterReadResume(t, false, 1, &store))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	requireDamageReport(t, rig.cons.request(0))
	requireStartTriggersNull(t, rig.cons.request(0))
	if store.adoptCalls != 2 {
		t.Errorf("want the settings adopted once more after the repair (2 calls), got %d", store.adoptCalls)
	}
	if ids := invokedInstances(rig.f); len(ids) != 1 || ids[0] != "agent-a#1" {
		t.Errorf("want the repair's dispatch carried out (agent-a#1), got %v", ids)
	}
	if doc := readText(t, rig.run.artifactPath); !strings.Contains(doc, "runner_mode: orchestrated") {
		t.Error("want the supplied runner settings recorded in the artifact after the repair")
	}
}

func TestSession_SecondDamageInTheSameStart_EndsTheRunWithoutAnotherRepairConsultation(t *testing.T) {
	var store *damagingStore
	rig := newRepairRig(t, []repairStep{
		{edit: fixLog, reply: dispatchReply("agent-a", 0)},
		{edit: fixLog, reply: dispatchReply("agent-a", 0)},
	}, withLaterReadResume(t, true, 2, &store))

	got, startErr := rig.startAllowingError()

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	if !errors.Is(got.Cause, domain.ErrWriteAfterDamage) && !errors.Is(startErr, domain.ErrWriteAfterDamage) {
		t.Errorf("want the refused write's error as the cause or returned error, got %v / %v", got.Cause, startErr)
	}
	if rig.cons.calls() != 1 {
		t.Errorf("want one repair consultation in the start, got %d", rig.cons.calls())
	}
	for _, id := range invokedInstances(rig.f) {
		if strings.HasPrefix(id, "agent-") {
			t.Errorf("want no workflow dispatch after the second damage, got %v", invokedInstances(rig.f))
		}
	}
}

func TestSession_NewRunWhoseCommitBranchWriteFindsDamage_FailsWithoutRepairConsultation(t *testing.T) {
	var store *damagingStore
	rig := newRepairRig(t, []repairStep{
		{edit: fixLog, reply: dispatchReply("agent-a", 0)},
	}, func(r *repairRig) {
		withLaterReadResume(t, true, 1, &store)(r)
		// Undo the seeded artifact: a new run creates its own.
		if err := os.Remove(r.run.artifactPath); err != nil {
			t.Fatalf("test setup: %v", err)
		}
		r.cfg.IsNewRun = true
	})

	got, startErr := rig.startAllowingError()

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	if !errors.Is(got.Cause, domain.ErrWriteAfterDamage) && !errors.Is(startErr, domain.ErrWriteAfterDamage) {
		t.Errorf("want the refused write's error as the cause or returned error, got %v / %v", got.Cause, startErr)
	}
	if rig.cons.calls() != 0 {
		t.Errorf("want no repair consultation for a new run, got %d", rig.cons.calls())
	}
}

// startAllowingError runs the session to its end. A write failure ends the run
// as RunFailed and may also be returned as the error, as it is today.
func (r *repairRig) startAllowingError() (domain.RunOutcome, error) {
	return session.New(r.deps).Start(context.Background(), r.cfg)
}
