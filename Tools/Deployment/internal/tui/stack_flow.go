package tui

// stack_flow.go routes the catalogue-stack screen in the entry flow. The stack screen follows
// the path screen in every mode when the reload capability is configured; confirming it
// rebuilds the session's service from exactly the confirmed stack and then starts the run.

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// stackStep holds the stack screen and the origin of the stack it starts from. The screen is nil
// when no reload capability is configured, which omits the stack step in every mode.
type stackStep struct {
	screen      *screens.StackScreen
	startOrigin stackconfig.Origin
	launch      stackconfig.Description // the stack the session was launched with
	launchGiven bool                    // the launch stack came from stack input
	key         string                  // mode and path the screen was last started for
	startErr    string                  // why the persisted start stack cannot be used, if it cannot
}

// newStackStep builds the stack step from the session's launch stack. The screen starts from
// that stack, and its origin is stack input when the session was launched with stack input and
// the default layer alone otherwise. The migration notice is set when the launch stack has a
// user catalog; entering the step from a confirmed path restarts it from that path's stack.
func newStackStep(svc app.Service, opts Options, width, height int, styles screens.Styles) stackStep {
	if opts.ReloadCatalog == nil {
		return stackStep{}
	}
	origin := stackconfig.OriginDefault
	if opts.LaunchStackGiven {
		origin = stackconfig.OriginStackInput
	}
	scr := screens.NewStackScreen(opts.LaunchStack, width, height, styles)
	byRoot, defaultID := layerIdentities(svc.ActiveStack().Layers)
	scr.SetIdentities(byRoot, defaultID)
	if opts.LaunchStackGiven && len(opts.LaunchStack.UserLayers) > 0 {
		scr.SetNotice(migrationNotice)
	}
	return stackStep{screen: scr, startOrigin: origin, launch: opts.LaunchStack, launchGiven: opts.LaunchStackGiven}
}

// migrationNotice tells a user who launched with a custom catalog that its behaviour has changed.
const migrationNotice = "Note: the behaviour of custom catalogues has changed. Until now a custom catalogue " +
	"got no default agents or workflows. Your folder is now a layer above the default catalogue, which " +
	"is offered as the bottom row. Keep it to include the default agents and workflows (space on the " +
	"default row); leave it out to use only your own catalogues."

// layerIdentities splits the active layers into the identity of each user layer by root and the
// identity of the default layer.
func layerIdentities(layers []domain.CatalogLayer) (map[string]domain.CatalogIdentity, domain.CatalogIdentity) {
	byRoot := make(map[string]domain.CatalogIdentity)
	var defaultID domain.CatalogIdentity
	for _, l := range layers {
		if l.Default {
			defaultID = l.Identity
		} else {
			byRoot[l.Root] = l.Identity
		}
	}
	return byRoot, defaultID
}

// resize forwards a window size to the stack screen, when there is one.
func (f stackStep) resize(width, height int) {
	if f.screen != nil {
		f.screen.Resize(width, height)
	}
}

// afterPath continues from a confirmed path: to the stack screen, or straight to the run when
// no reload capability is configured.
func (m *rootModel) afterPath() (tea.Model, tea.Cmd) {
	if m.stack.screen == nil {
		return m.startRun()
	}
	m.prepareStack()
	m.screen = screenStack
	return m, m.stack.screen.InputInit()
}

// updateStack handles input on the stack screen.
func (m *rootModel) updateStack(msg tea.Msg) (tea.Model, tea.Cmd) {
	scr := m.stack.screen
	cmd := scr.Update(msg)
	if scr.Back() {
		scr.SetExternalError("") // clears the back flag; the screen keeps its state
		m.screen = screenWorkspace
		return m, m.wsScreen.InputInit()
	}
	if scr.Done() {
		return m.confirmStack()
	}
	return m, cmd
}

// confirmStack rebuilds the service from the confirmed stack and checks that the selected
// harness's injections are supplied. A failure is shown on the stack screen, which stays active
// with the session's service unchanged.
func (m *rootModel) confirmStack() (tea.Model, tea.Cmd) {
	scr := m.stack.screen
	if m.stack.startErr != "" && !scr.Changed() {
		scr.SetExternalError(m.stack.startErr)
		return m, nil
	}
	origin := m.stack.startOrigin
	if scr.Changed() {
		origin = stackconfig.OriginStackInput
	}
	svc, err := m.reloadCatalog(scr.Description(), origin)
	if err == nil {
		err = svc.CheckHarnessInjections(m.selections.harnessID)
	}
	if err != nil {
		scr.SetExternalError(err.Error())
		return m, nil
	}
	m.svc = svc
	return m.startRun()
}

// startRun moves to the running screen and launches the service call for the chosen mode.
func (m *rootModel) startRun() (tea.Model, tea.Cmd) {
	m.screen = screenRunning
	switch m.selections.mode {
	case domain.ModePromoteToGeneric:
		m.statusMsg = "Running promote…"
	case domain.ModeTransformHarness:
		m.statusMsg = "Running transform…"
	default:
		m.statusMsg = "Running deployment…"
	}
	return m, m.startService()
}
