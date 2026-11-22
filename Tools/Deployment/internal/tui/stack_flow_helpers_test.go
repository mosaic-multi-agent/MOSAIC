package tui

// stack_flow_helpers_test.go holds the shared fixtures for the stack-screen flow tests: a
// recording service double, a recording reload capability, and helpers that advance the root
// model through the entry flow (mode -> harness -> path screen -> stack screen). Nothing in
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
		stubFlowService: stubFlowService{harnesses: []domain.HarnessRef{flowTestHarness, flowTargetHarness}},
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

func (s *stackFlowSvc) PromoteBatch(ctx context.Context, r app.PromoteBatchRequest) (app.PromoteBatchResult, error) {
	s.log.add(s.id + ":promote")
	return s.stubFlowService.PromoteBatch(ctx, r)
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

// entryPath returns a path the path screen accepts for mode: a directory (for promote, an
// agents folder).
func entryPath(t *testing.T, mode domain.RunMode) string {
	t.Helper()
	dir := t.TempDir()
	if mode == domain.ModePromoteToGeneric {
		return createAgentsFolder(t, dir)
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

// toMode requires the mode screen, which is the first screen of the entry flow.
func (f *stackFlow) toMode(t *testing.T) {
	t.Helper()
	requireScreen(t, f.m, screenMode)
}

// toPath advances through the entry screens to the path screen of the flow's mode (through the
// harness screen first in the harness-first modes).
func (f *stackFlow) toPath(t *testing.T) {
	t.Helper()
	driveToPath(t, f.m, f.mode, "")
}

// confirmPath confirms the path and, in the path-first modes, chooses the first listed harness
// when the harness screen follows. It returns the command of the last key pressed.
func (f *stackFlow) confirmPath(t *testing.T) tea.Cmd {
	t.Helper()
	return confirmPath(t, f.m, "")
}

// backToPath presses Esc on the stack screen and, in the path-first modes, once more on each
// harness screen (the source, and in Transform the target) that precedes it, so the model shows
// the path screen.
func (f *stackFlow) backToPath(t *testing.T) {
	t.Helper()
	f.key(tea.KeyEsc)
	for i := 0; i < 2 && f.m.screen == screenHarness; i++ {
		f.key(tea.KeyEsc)
	}
	requireScreen(t, f.m, screenWorkspace)
}

// toStack advances through the path screen (and the harness screen after it, in the path-first
// modes) to the stack screen.
func (f *stackFlow) toStack(t *testing.T) {
	t.Helper()
	f.toPath(t)
	f.confirmPath(t)
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
