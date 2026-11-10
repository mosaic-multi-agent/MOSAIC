package tui

// resume_flow_fixtures_test.go builds a TUI resume end to end: a real run
// artifact on disk, a real orchestrator file, the run-selection screen offering
// that run, and a session factory that records every session the model builds.
// The resume-wizard, early-refusal and settings-summary tests drive it with key
// presses and read what the model shows.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

const (
	flowRunID = "20260701T120000Z-a3f9"

	// summaryTitle is the title of the read-only settings summary screen.
	summaryTitle = "Resume: recorded settings"

	// deployedVersion is the version the orchestrator fixture declares for the
	// recorded workflow.
	deployedVersion = "1.0"
)

// flowArtifact describes the run artifact written for a resume.
type flowArtifact struct {
	settings domain.RunSettings
	workflow domain.WorkflowID // "" means recordedWorkflowID
	version  domain.WorkflowVersion
	branch   string // commit branch recorded after commit setup; "" for none
	missing  bool   // write no artifact at all
	garbage  bool   // write an artifact that is not in the canonical format

	damaged            bool // damage the Execution Log; the frontmatter stays intact
	damagedFrontmatter bool // damage the frontmatter itself
	lastGoodCopy       bool // keep the artifact as written as the run's last-good copy before damaging it
}

// recordedSettings is a run that records every setting the wizard used to ask.
func recordedSettings() domain.RunSettings {
	return domain.RunSettings{
		Mode:             domain.ExecutionModeAuto,
		Checkpoints:      true,
		Commits:          true,
		PreConsultation:  true,
		ManualResolution: true,
		ReviewLoopLimit:  5,
		InfraClassSelections: map[string]string{
			"checkpoint": "cp-b",
			"commit":     "git-commit",
		},
	}
}

// fullyRecorded is the artifact of a run that records every setting, with the
// workflow version the orchestrator fixture declares.
func fullyRecorded() flowArtifact {
	return flowArtifact{
		settings: recordedSettings(),
		version:  deployedVersion,
		branch:   "mosaic/run/" + flowRunID,
	}
}

// flowOrchestrator describes the orchestrator file the harness discovers.
type flowOrchestrator struct {
	noInfra bool // declare no infrastructure agents
}

const flowInfraTable = "| Class | Trigger | Param | On Failure | Description |\n" +
	"|-------|---------|-------|------------|-------------|\n"

func flowInfraAgent(name, class string) string {
	return "<InfrastructureAgent type=\"core\" name=\"" + name + "\" version=\"1.0.0\">\n" +
		flowInfraTable + "| " + class + " | ITERATION_END | - | halt | row |\n" +
		"</InfrastructureAgent>\n"
}

func writeFlowOrchestrator(t *testing.T, o flowOrchestrator) string {
	t.Helper()
	content, err := os.ReadFile(writeOrchestratorFixture(t))
	if err != nil {
		t.Fatalf("read orchestrator fixture: %v", err)
	}
	text := string(content)
	if !o.noInfra {
		text += "\n<InfrastructureAgents type=\"managed\">\n" +
			flowInfraAgent("cp-a", "checkpoint") + flowInfraAgent("cp-b", "checkpoint") +
			flowInfraAgent("git-commit", "commit") + "</InfrastructureAgents>\n"
	}
	path := filepath.Join(t.TempDir(), "orchestrator-script.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write orchestrator file: %v", err)
	}
	return path
}

// writeFlowArtifact writes the run folder and, unless it is missing, the run
// artifact, and returns the run folder.
func writeFlowArtifact(t *testing.T, a flowArtifact) string {
	t.Helper()
	folder := filepath.Join(t.TempDir(), "Orchestration-"+flowRunID)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("create run folder: %v", err)
	}
	path := filepath.Join(folder, "Orchestration.md")
	switch {
	case a.missing:
		return folder
	case a.garbage:
		if err := os.WriteFile(path, []byte("this is not a run artifact\n"), 0o600); err != nil {
			t.Fatalf("write garbage artifact: %v", err)
		}
		return folder
	}
	workflow := a.workflow
	if workflow == "" {
		workflow = domain.WorkflowID(recordedWorkflowID)
	}
	store := artifact.NewFileStore(path)
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	info := domain.WorkflowInfo{ID: workflow, Version: a.version}
	if _, err := store.Create(context.Background(), info, "recorded task", a.settings, now, flowRunID); err != nil {
		t.Fatalf("create run artifact: %v", err)
	}
	if a.branch != "" {
		if _, err := store.SetCommitBranch(context.Background(), a.branch, now); err != nil {
			t.Fatalf("record commit branch: %v", err)
		}
	}
	damageFlowArtifact(t, path, a)
	return folder
}

// damageFlowArtifact damages the artifact written at path as a describes:
// a stray line ends the Execution Log table early, or a frontmatter value is
// left unclosed. The recorded values are those the artifact was created with.
func damageFlowArtifact(t *testing.T, path string, a flowArtifact) {
	t.Helper()
	if !a.damaged && !a.damagedFrontmatter {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact to damage: %v", err)
	}
	text := string(data)
	if a.lastGoodCopy {
		writeFlowLastGoodCopy(t, path, data)
	} else if a.damagedFrontmatter {
		// The real store keeps a last-good copy current; a run without one has none.
		removeFlowLastGoodCopy(t, path)
	}
	switch {
	case a.damagedFrontmatter:
		text = strings.Replace(text, "workflow: ", "workflow: [", 1)
	default:
		text = strings.Replace(text, "</ExecutionLog>", "a stray line that is not a table row\n</ExecutionLog>", 1)
	}
	if text == string(data) {
		t.Fatalf("test setup: nothing to damage in the artifact:\n%s", text)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write damaged artifact: %v", err)
	}
	if _, err := artifact.ParseAt(path, []byte(text)); err == nil {
		t.Fatal("test setup: the damaged artifact still reads cleanly")
	}
}

// removeFlowLastGoodCopy deletes any last-good copy the store kept for the run
// whose artifact is at artifactPath.
func removeFlowLastGoodCopy(t *testing.T, artifactPath string) {
	t.Helper()
	evidenceDir, ok := artifact.EvidenceDir(artifactPath)
	if !ok {
		return
	}
	err := os.Remove(filepath.Join(evidenceDir, "Orchestration.last-good.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove last-good copy: %v", err)
	}
}

// writeFlowLastGoodCopy stores data as the last-good copy of the run whose
// artifact is at artifactPath, where the Runner keeps its run evidence.
func writeFlowLastGoodCopy(t *testing.T, artifactPath string, data []byte) {
	t.Helper()
	evidenceDir, ok := artifact.EvidenceDir(artifactPath)
	if !ok {
		t.Fatalf("test setup: no evidence location for %s", artifactPath)
	}
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "Orchestration.last-good.md"), data, 0o600); err != nil {
		t.Fatalf("write last-good copy: %v", err)
	}
}

// resumeFlow is a TUI standing on the run-selection screen, offering one run to
// resume.
type resumeFlow struct {
	t        *testing.T
	m        *rootModel
	folder   string
	orchPath string

	discovered []string // harness ids the orchestrator discoverer was asked for
	sessions   []flowSession
}

// flowSession is one session the model built through the session factory.
type flowSession struct {
	runFolder string
	isNewRun  bool
	orchFile  string
	cfg       runconfig.ConfigSelection
}

func newResumeFlow(t *testing.T, a flowArtifact, o flowOrchestrator) *resumeFlow {
	t.Helper()
	f := &resumeFlow{t: t, orchPath: writeFlowOrchestrator(t, o), folder: writeFlowArtifact(t, a)}
	workflow := a.workflow
	if workflow == "" {
		workflow = domain.WorkflowID(recordedWorkflowID)
	}
	candidate := newTestCandidate(flowRunID, f.folder)
	candidate.RunInfo.Workflow = string(workflow)
	candidate.RunInfo.LastUpdated = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	candidate.RunInfo.FolderPath = f.folder
	candidate.RunInfo.Damaged = a.damaged || a.damagedFrontmatter
	sess := newCapturingSession()
	f.m = newRootModel(context.Background(), sess, Options{
		Theme:      tuicommon.DefaultTheme(),
		ScanResult: &runscan.ScanResult{Candidates: []runscan.RunCandidate{candidate}},
		OrchestratorDiscoverer: func(_ string, harnessID string) (string, error) {
			f.discovered = append(f.discovered, harnessID)
			return f.orchPath, nil
		},
		SessionFactory: func(runFolder string, isNewRun bool, orchFile string, cfg runconfig.ConfigSelection) session.Session {
			f.sessions = append(f.sessions, flowSession{runFolder: runFolder, isNewRun: isNewRun, orchFile: orchFile, cfg: cfg})
			return sess
		},
	})
	f.m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	if f.m.screen != screenRunSelect {
		t.Fatalf("precondition: screen = %v, want screenRunSelect", f.m.screen)
	}
	return f
}

// launched returns the sessions built to run (not the placeholder built when a
// run is chosen): they are given the orchestrator file.
func (f *resumeFlow) launched() []flowSession {
	var out []flowSession
	for _, s := range f.sessions {
		if s.orchFile != "" {
			out = append(out, s)
		}
	}
	return out
}

func (f *resumeFlow) key(k tea.KeyType) { f.m.Update(tea.KeyMsg{Type: k}) }

func (f *resumeFlow) typeText(text string) {
	for _, r := range text {
		f.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// selectRun chooses the offered run on the run-selection screen.
func (f *resumeFlow) selectRun() {
	f.key(tea.KeyDown)
	f.key(tea.KeyEnter)
}

// answerHarness accepts the harness question's first option.
func (f *resumeFlow) answerHarness() { f.key(tea.KeyEnter) }

// toWizard chooses the run and answers the harness question.
func (f *resumeFlow) toWizard() {
	f.selectRun()
	if f.m.screen != screenSetupHarness {
		f.t.Fatalf("precondition: screen = %v after choosing the run, want screenSetupHarness", f.m.screen)
	}
	f.answerHarness()
	if f.m.screen != screenSetupConfig {
		f.t.Fatalf("precondition: screen = %v after the harness question, want screenSetupConfig", f.m.screen)
	}
}

// view returns the whitespace-normalised view the model currently shows.
func (f *resumeFlow) view() string { return collapseSpaces(f.m.View()) }

func collapseSpaces(s string) string { return strings.Join(strings.Fields(s), " ") }

// showing reports whether the current view contains text, ignoring runs of
// whitespace.
func (f *resumeFlow) showing(text string) bool {
	return strings.Contains(f.view(), collapseSpaces(text))
}

// Wizard prompts, by the text the configuration screen shows for them.
var flowPrompts = []struct{ marker, name string }{
	{hitlTimeoutPromptMarker, "hitl-timeout"},
	{"invocation timeout", "timeout"},
	{"allow workflow version drift", "drift"},
	{"execution mode", "mode"},
	{"commit branch variant", "commit-branch"},
	{"checkpoints:", "checkpoints"},
	{"commits:", "commits"},
	{"pre-consultation", "pre-consultation"},
	{"manual resolution", "manual-resolution"},
	{"review loop limit", "review-limit"},
	{"agent for this run", "infra-class"},
}

func (f *resumeFlow) prompt() string {
	view := collapseSpaces(strings.ToLower(f.m.configScreen.View()))
	for _, p := range flowPrompts {
		if strings.Contains(view, p.marker) {
			return p.name
		}
	}
	return "unknown"
}

// answerWizard answers every configuration prompt the wizard shows and returns
// their names in order. With alternative false each prompt gets its default; the
// timeout is typed, the mode is auto, the commit branch is the first variant.
// With alternative true every choice prompt gets its second option instead.
func (f *resumeFlow) answerWizard(alternative bool) []string {
	f.t.Helper()
	var names []string
	for i := 0; i < 30 && f.m.screen == screenSetupConfig; i++ {
		name := f.prompt()
		names = append(names, name)
		switch name {
		case "timeout":
			f.typeText("45m")
			f.key(tea.KeyEnter)
		case "hitl-timeout":
			for j := 0; j < 12; j++ {
				f.key(tea.KeyBackspace)
			}
			f.typeText("3h")
			f.key(tea.KeyEnter)
		case "mode":
			f.key(tea.KeyDown)
			f.key(tea.KeyDown)
			f.key(tea.KeyEnter)
		case "commit-branch":
			f.key(tea.KeyDown)
			if alternative {
				f.key(tea.KeyDown)
			}
			f.key(tea.KeyEnter)
		case "review-limit":
			f.key(tea.KeyEnter)
		default:
			if alternative {
				f.key(tea.KeyDown)
			}
			f.key(tea.KeyEnter)
		}
	}
	return names
}
