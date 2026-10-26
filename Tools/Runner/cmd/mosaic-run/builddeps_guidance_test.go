package main

// CLI-style tests of saved pre-consultation guidance through the shared
// dependency builder, over a real run folder on disk. The Interaction is the
// CLI's, which cannot ask: the run must never wait for a review, uses the saved
// guidance file when there is one (a hand edit included), and runs
// pre-consultation only when there is none.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const (
	cliAdviceSentinel = "cli-advice-sentinel-m2d"
	cliSavedSentinel  = "cli-hand-edited-sentinel-p6h"
	guidanceFileName  = "RunnerGuidance.txt"
)

// cliGuidanceRun is one CLI-style session over a real run folder.
type cliGuidanceRun struct {
	ses       session.Session
	harness   *harness.MockAdapter
	invoker   *fakeRawInvoker
	orchPath  string
	runFolder string
}

// newCLIGuidanceRun assembles a session from the shared builder with the CLI's
// Interaction. recorded is nil for a new run, otherwise the recorded settings of
// a resumable linear run. advice is the raw pre-consultation reply, if any.
func newCLIGuidanceRun(t *testing.T, dir string, recorded *domain.RunSettings, advice ...string) *cliGuidanceRun {
	t.Helper()
	orchPath := copyOrchestratorFileForMain(t, dir, "linear-orch.md")
	writeAgentFileForMain(t, dir, "agent-a")
	writeAgentFileForMain(t, dir, "agent-b")
	runFolder := filepath.Join(dir, domain.RunScopedFolder(resumeTestRunID))
	if err := os.MkdirAll(runFolder, 0o755); err != nil {
		t.Fatalf("setup: create run folder: %v", err)
	}
	r := &cliGuidanceRun{harness: harness.NewMockAdapter(), invoker: &fakeRawInvoker{}, orchPath: orchPath, runFolder: runFolder}
	for _, a := range advice {
		r.invoker.responses = append(r.invoker.responses, []byte(a))
	}
	interact := cli.NewInteraction(io.Discard)
	deps := buildDepsFromTransports(r.invoker, interact, nil, nil)
	deps.Harness = r.harness
	deps.Clock = mainTestClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	deps.Interact = interact
	if recorded != nil {
		deps.Store = recordedResumeStore(*recorded)
	} else {
		deps.Store = &mainTestMemStore{}
	}
	r.ses = session.New(deps)
	return r
}

func (r *cliGuidanceRun) queueSuccess(agents ...string) {
	for _, a := range agents {
		r.harness.Queue(a, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: a + "#x", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
}

func (r *cliGuidanceRun) newConfig(pre bool) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: r.orchPath, WorkflowID: "linear", Task: "test task", IsNewRun: true,
		RunFolder:   r.runFolder,
		RunSettings: domain.RunSettings{Mode: domain.ExecutionModeAuto, PreConsultation: pre},
	}
}

func (r *cliGuidanceRun) resumeCfg(pre bool) domain.RunConfig {
	cfg := resumeConfig(r.orchPath, domain.RunSettings{PreConsultation: pre})
	cfg.RunFolder = r.runFolder
	return cfg
}

func (r *cliGuidanceRun) rawCalls() int {
	r.invoker.mu.Lock()
	defer r.invoker.mu.Unlock()
	return r.invoker.callCount
}

func (r *cliGuidanceRun) guidanceFile() (string, bool) {
	data, err := os.ReadFile(filepath.Join(r.runFolder, guidanceFileName))
	return string(data), err == nil
}

func (r *cliGuidanceRun) requestOf(agent string) domain.ProtocolRequest {
	for _, inv := range r.harness.Invocations() {
		if inv.Agent.Identifier == agent {
			return inv.Request
		}
	}
	return domain.ProtocolRequest{}
}

func adviceReply(task string) string {
	return `{"task_description":"` + task + `","constraints":""}`
}

func TestCLIGuidance_NoSavedFile_RunsPreConsultationAndSavesTheResult(t *testing.T) {
	r := newCLIGuidanceRun(t, t.TempDir(), nil, adviceReply(cliAdviceSentinel))
	r.queueSuccess("agent-a", "agent-b")

	got, err := startSessionGuarded(t, r.ses, r.newConfig(true))

	if err != nil || got.Status != domain.RunCompleted {
		t.Fatalf("want the run completed without waiting for a review, got %q err %v (%s)", got.Status, err, got.Message)
	}
	if r.rawCalls() != 1 {
		t.Errorf("want pre-consultation run once, got %d raw calls", r.rawCalls())
	}
	file, ok := r.guidanceFile()
	if !ok || !strings.Contains(file, cliAdviceSentinel) {
		t.Errorf("want RunnerGuidance.txt in the run folder holding the advice, got found=%v %q", ok, file)
	}
	if !strings.Contains(r.requestOf("agent-a").TaskDescription, cliAdviceSentinel) {
		t.Error("want the advice appended to the first dispatch")
	}
}

func TestCLIGuidance_ResumeWithSavedFile_DoesNotRunPreConsultationAndUsesTheFile(t *testing.T) {
	recorded := domain.RunSettings{Mode: domain.ExecutionModeAuto, PreConsultation: true}
	r := newCLIGuidanceRun(t, t.TempDir(), &recorded, adviceReply(cliAdviceSentinel))
	hand := "# edited by hand\n[task_description]\n" + cliSavedSentinel + "\n\n[constraints]\n"
	if err := os.WriteFile(filepath.Join(r.runFolder, guidanceFileName), []byte(hand), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	r.queueSuccess("agent-b")

	got, err := startSessionGuarded(t, r.ses, r.resumeCfg(true))

	if err != nil || got.Status != domain.RunCompleted {
		t.Fatalf("want the resumed run completed, got %q err %v (%s)", got.Status, err, got.Message)
	}
	if r.rawCalls() != 0 {
		t.Errorf("want no pre-consultation when the file exists, got %d raw calls", r.rawCalls())
	}
	task := r.requestOf("agent-b").TaskDescription
	if !strings.Contains(task, cliSavedSentinel) || strings.Contains(task, cliAdviceSentinel) {
		t.Errorf("want the hand-edited file used, got %q", task)
	}
}

func TestCLIGuidance_ResumeWithoutFile_RunsPreConsultationAndSaves(t *testing.T) {
	recorded := domain.RunSettings{Mode: domain.ExecutionModeAuto, PreConsultation: true}
	r := newCLIGuidanceRun(t, t.TempDir(), &recorded, adviceReply(cliAdviceSentinel))
	r.queueSuccess("agent-b")

	got, err := startSessionGuarded(t, r.ses, r.resumeCfg(true))

	if err != nil || got.Status != domain.RunCompleted {
		t.Fatalf("want the resumed run completed, got %q err %v (%s)", got.Status, err, got.Message)
	}
	if r.rawCalls() != 1 {
		t.Errorf("want pre-consultation run once, got %d raw calls", r.rawCalls())
	}
	if file, ok := r.guidanceFile(); !ok || !strings.Contains(file, cliAdviceSentinel) {
		t.Errorf("want the result saved, got found=%v %q", ok, file)
	}
}

func TestCLIGuidance_SavedByFirstSession_ReusedByTheResumeWithoutPreConsultation(t *testing.T) {
	dir := t.TempDir()
	first := newCLIGuidanceRun(t, dir, nil, adviceReply(cliAdviceSentinel))
	first.queueSuccess("agent-a", "agent-b")
	if _, err := startSessionGuarded(t, first.ses, first.newConfig(true)); err != nil {
		t.Fatalf("setup: first session failed: %v", err)
	}
	recorded := domain.RunSettings{Mode: domain.ExecutionModeAuto, PreConsultation: true}
	second := newCLIGuidanceRun(t, dir, &recorded)
	second.queueSuccess("agent-b")

	got, err := startSessionGuarded(t, second.ses, second.resumeCfg(true))

	if err != nil || got.Status != domain.RunCompleted {
		t.Fatalf("want the resumed run completed, got %q err %v (%s)", got.Status, err, got.Message)
	}
	if second.rawCalls() != 0 {
		t.Errorf("want no pre-consultation on the resume, got %d raw calls", second.rawCalls())
	}
	if !strings.Contains(second.requestOf("agent-b").TaskDescription, cliAdviceSentinel) {
		t.Error("want the first session's saved guidance appended on the resume")
	}
}

func TestCLIGuidance_PreConsultationOff_RunsWithoutPreConsultationAndWritesNoFile(t *testing.T) {
	r := newCLIGuidanceRun(t, t.TempDir(), nil)
	r.queueSuccess("agent-a", "agent-b")

	got, err := startSessionGuarded(t, r.ses, r.newConfig(false))

	if err != nil || got.Status != domain.RunCompleted {
		t.Fatalf("want the run completed, got %q err %v (%s)", got.Status, err, got.Message)
	}
	if r.rawCalls() != 0 {
		t.Errorf("want no pre-consultation, got %d raw calls", r.rawCalls())
	}
	if _, ok := r.guidanceFile(); ok {
		t.Error("want no guidance file written when pre-consultation is off")
	}
}

func TestCLIGuidance_PreConsultationFailure_LeavesNoFile(t *testing.T) {
	r := newCLIGuidanceRun(t, t.TempDir(), nil) // no scripted reply: the raw call fails
	r.queueSuccess("agent-a", "agent-b")

	got, _ := startSessionGuarded(t, r.ses, r.newConfig(true))

	if got.Status != domain.RunStartFailed {
		t.Errorf("want RunStartFailed as before, got %q (%s)", got.Status, got.Message)
	}
	if _, ok := r.guidanceFile(); ok {
		t.Error("want no guidance file after a failed pre-consultation")
	}
}
