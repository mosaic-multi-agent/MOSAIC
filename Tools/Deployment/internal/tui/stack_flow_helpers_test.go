package tui

// stack_flow_helpers_test.go holds the shared fixtures for the stack-screen flow tests: a
// recording service double, a recording reload capability, and helpers that advance the root
// model through the entry flow (harness -> mode -> path screen -> stack screen). Nothing in
// this file is a test itself.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// stackScreenTitle is the title the stack screen renders; it identifies the screen in a view.
const stackScreenTitle = "Catalogue Stack"

// entryModes lists the modes in the order the mode screen offers them.
var entryModes = []domain.RunMode{
	domain.ModeDeployWorkspace,
	domain.ModeUpdateWorkspace,
	domain.ModeUpdateWorkflows,
	domain.ModeDeployAgents,
	domain.ModeDeployHooks,
	domain.ModePromoteToGeneric,
	domain.ModeTransformHarness,
}

// runLog records which service ran which operation, as "<service id>:<operation>".
type runLog struct{ entries []string }

func (l *runLog) add(entry string) { l.entries = append(l.entries, entry) }

// stackFlowSvc is an app.Service double that records the run it receives and the harness ids
// whose injections were checked.
type stackFlowSvc struct {
	stubFlowService
	id           string
	log          *runLog
	injectionErr error
	checked      []string
}

func newStackFlowSvc(id string, log *runLog) *stackFlowSvc {
	return &stackFlowSvc{
		stubFlowService: stubFlowService{harnesses: []domain.HarnessRef{flowTestHarness}},
		id:              id,
		log:             log,
	}
}

func (s *stackFlowSvc) CheckHarnessInjections(harnessID string) error {
	s.checked = append(s.checked, harnessID)
	return s.injectionErr
}

func (s *stackFlowSvc) DeployNew(ctx context.Context, r app.DeployRequest) (domain.RunSummary, error) {
	s.log.add(s.id + ":deploy")
	return s.stubFlowService.DeployNew(ctx, r)
}

func (s *stackFlowSvc) Update(ctx context.Context, r app.UpdateRequest) (domain.RunSummary, error) {
	s.log.add(s.id + ":update")
	return s.stubFlowService.Update(ctx, r)
}

func (s *stackFlowSvc) UpdateWorkflows(ctx context.Context, r app.WorkflowUpdateRequest) (domain.RunSummary, error) {
	s.log.add(s.id + ":workflows")
	return s.stubFlowService.UpdateWorkflows(ctx, r)
}

func (s *stackFlowSvc) DeployAgents(ctx context.Context, r app.DeployAgentsRequest) (domain.RunSummary, error) {
	s.log.add(s.id + ":agents")
	return s.stubFlowService.DeployAgents(ctx, r)
}

func (s *stackFlowSvc) DeployHooks(ctx context.Context, r app.DeployHooksRequest) (domain.RunSummary, error) {
	s.log.add(s.id + ":hooks")
	return s.stubFlowService.DeployHooks(ctx, r)
}

func (s *stackFlowSvc) Promote(ctx context.Context, r app.PromoteRequest) (app.PromoteResult, error) {
	s.log.add(s.id + ":promote")
	return s.stubFlowService.Promote(ctx, r)
}

func (s *stackFlowSvc) TransformHarness(ctx context.Context, r app.TransformHarnessRequest) (app.TransformHarnessResult, error) {
	s.log.add(s.id + ":transform")
	return s.stubFlowService.TransformHarness(ctx, r)
}

// operationOf names the operation the run of a mode records in the runLog.
var operationOf = map[domain.RunMode]string{
	domain.ModeDeployWorkspace:  "deploy",
	domain.ModeUpdateWorkspace:  "update",
	domain.ModeUpdateWorkflows:  "workflows",
	domain.ModeDeployAgents:     "agents",
	domain.ModeDeployHooks:      "hooks",
	domain.ModePromoteToGeneric: "promote",
	domain.ModeTransformHarness: "transform",
}

// reloadCall is one call of the reload capability.
type reloadCall struct {
	d      stackconfig.Description
	origin stackconfig.Origin
}

// reloadRecorder is a CatalogReloadFunc that records its calls and returns svc, or err when set.
type reloadRecorder struct {
	calls []reloadCall
	svc   app.Service
	err   error
}

func (r *reloadRecorder) reload(d stackconfig.Description, origin stackconfig.Origin) (app.Service, error) {
	r.calls = append(r.calls, reloadCall{d: d, origin: origin})
	if r.err != nil {
		return nil, r.err
	}
	return r.svc, nil
}

// stackFlow is a root model wired for the stack flow, with the doubles that observe it.
type stackFlow struct {
	m        *rootModel
	orig     *stackFlowSvc
	reloaded *stackFlowSvc
	rec      *reloadRecorder
	log      *runLog
	mode     domain.RunMode
	path     string
}

// flowOption adjusts the Options of a stackFlow before the model is built.
type flowOption func(*Options)

// withLaunch sets the stack the session was launched with.
func withLaunch(d stackconfig.Description, given bool) flowOption {
	return func(o *Options) { o.LaunchStack, o.LaunchStackGiven = d, given }
}

// withoutReload leaves the reload capability unconfigured.
func withoutReload() flowOption { return func(o *Options) { o.ReloadCatalog = nil } }

// withPreselectedHarness starts the flow with the harness already chosen.
func withPreselectedHarness() flowOption {
	return func(o *Options) { o.InitialRequest.HarnessID = flowTestHarness.ID }
}

// entryPath returns a path the path screen accepts for mode: a file for promote, otherwise a
// directory.
func entryPath(t *testing.T, mode domain.RunMode) string {
	t.Helper()
	dir := t.TempDir()
	if mode == domain.ModePromoteToGeneric {
		return createAgentFile(t, dir)
	}
	return dir
}

// newStackFlow builds a root model for mode. By default the reload capability is configured, the
// session was launched without stack input, and the path screen is pre-filled.
func newStackFlow(t *testing.T, mode domain.RunMode, opts ...flowOption) *stackFlow {
	t.Helper()
	f := &stackFlow{log: &runLog{}, mode: mode, path: entryPath(t, mode)}
	f.orig = newStackFlowSvc("original", f.log)
	f.reloaded = newStackFlowSvc("reloaded", f.log)
	f.rec = &reloadRecorder{svc: f.reloaded}
	o := Options{
		Theme:          DefaultTheme(),
		ReloadCatalog:  f.rec.reload,
		LaunchStack:    stackconfig.Description{IncludeDefault: true},
		InitialRequest: &app.DeployRequest{WorkspacePath: f.path},
	}
	for _, opt := range opts {
		opt(&o)
	}
	f.m = newRootModel(t.Context(), f.orig, o)
	return f
}

// key sends one key to the model and returns the command it produced.
func (f *stackFlow) key(k tea.KeyType) tea.Cmd {
	_, cmd := f.m.Update(tea.KeyMsg{Type: k})
	return cmd
}

// typeText sends text to the model one character at a time.
func (f *stackFlow) typeText(text string) {
	for _, r := range text {
		f.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// onStack reports whether the stack screen is showing.
func (f *stackFlow) onStack() bool {
	return f.m.screen != screenRunning && strings.Contains(f.m.View(), stackScreenTitle)
}

// toMode advances to the mode screen.
func (f *stackFlow) toMode(t *testing.T) {
	t.Helper()
	if f.m.screen == screenHarness {
		f.key(tea.KeyEnter)
	}
	if f.m.screen != screenMode {
		t.Fatalf("screen = %v, want the mode screen", f.m.screen)
	}
}

// toPath advances to the path screen of the flow's mode.
func (f *stackFlow) toPath(t *testing.T) {
	t.Helper()
	f.toMode(t)
	for i, mode := range entryModes {
		if mode != f.mode {
			continue
		}
		for n := 0; n < i; n++ {
			f.key(tea.KeyDown)
		}
	}
	f.key(tea.KeyEnter)
	if f.m.screen != screenWorkspace {
		t.Fatalf("screen = %v after selecting %s, want the path screen", f.m.screen, f.mode)
	}
}

// toStack advances through the path screen to the stack screen.
func (f *stackFlow) toStack(t *testing.T) {
	t.Helper()
	f.toPath(t)
	f.key(tea.KeyEnter)
	if !f.onStack() {
		t.Fatalf("not on the stack screen after confirming the path; screen = %v, view:\n%s", f.m.screen, f.m.View())
	}
}

// confirmStack presses Enter on the stack screen and returns the command produced.
func (f *stackFlow) confirmStack(t *testing.T) tea.Cmd {
	t.Helper()
	if !f.onStack() {
		t.Fatalf("confirmStack: not on the stack screen:\n%s", f.m.View())
	}
	return f.key(tea.KeyEnter)
}

// space presses the space bar.
func (f *stackFlow) space() { f.m.Update(tea.KeyMsg{Type: tea.KeySpace}) }

// runStarted executes the command a confirmation returned, which performs the service run.
func runStarted(cmd tea.Cmd) {
	if cmd != nil {
		cmd()
	}
}

// sameDescription reports whether a and b hold the same ordered user layers and default choice.
func sameDescription(a, b stackconfig.Description) bool {
	if a.IncludeDefault != b.IncludeDefault || len(a.UserLayers) != len(b.UserLayers) {
		return false
	}
	for i := range a.UserLayers {
		if a.UserLayers[i] != b.UserLayers[i] {
			return false
		}
	}
	return true
}
