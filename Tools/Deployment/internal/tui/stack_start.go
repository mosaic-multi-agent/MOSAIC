package tui

// stack_start.go decides which stack the stack screen starts from for the confirmed path: the
// launch stack when it came from stack input, otherwise the persisted stack of the workspace that
// applies to the path in the current mode, otherwise the launch stack (the default layer alone
// without launch input). Persisting a changed stack is left to the write runs.

import (
	"os"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// stackStart is the starting point of the stack screen.
type stackStart struct {
	desc      stackconfig.Description
	origin    stackconfig.Origin
	byRoot    map[string]domain.CatalogIdentity
	defaultID domain.CatalogIdentity
	errText   string // set when the start stack cannot be used as it is
}

// prepareStack restarts the stack screen from the start stack of the confirmed path. Going back
// and confirming the same mode and path keeps the screen's state.
func (m *rootModel) prepareStack() {
	st := &m.stack
	key := string(m.selections.mode) + "\x00" + m.selections.workspacePath
	if st.key != key {
		start := m.startStack()
		st.key = key
		st.startOrigin = start.origin
		st.startErr = start.errText
		st.screen.Reset(start.desc)
		st.screen.SetIdentities(start.byRoot, start.defaultID)
	}
	if st.startErr != "" && !st.screen.Changed() {
		st.screen.SetExternalError(st.startErr)
	}
}

// startStack computes the start stack for the confirmed path.
func (m *rootModel) startStack() stackStart {
	active := m.svc.ActiveStack()
	byRoot, defaultID := layerIdentities(active.Layers)
	origin := stackconfig.OriginDefault
	if m.stack.launchGiven {
		origin = stackconfig.OriginStackInput
	}
	launch := stackStart{desc: m.stack.launch, origin: origin, byRoot: byRoot, defaultID: defaultID}
	if m.stack.launchGiven {
		return launch
	}
	workspace, ok := m.persistedWorkspace()
	if !ok {
		return launch
	}
	stored, found, err := m.svc.ReadPersistedStack(workspace)
	if err != nil {
		launch.errText = err.Error()
		return launch
	}
	if !found {
		return launch
	}
	start := stackStart{
		desc:      stored.Description(),
		origin:    stackconfig.OriginPersisted,
		byRoot:    persistedIdentities(stored, byRoot),
		defaultID: defaultID,
	}
	if err := m.svc.ValidatePersistedStack(stored); err != nil {
		start.errText = err.Error()
	}
	return start
}

// persistedWorkspace returns the workspace whose persisted stack applies to the confirmed path:
// for promote and transform the workspace containing the path, for the other modes the path
// itself when it is a workspace.
func (m *rootModel) persistedWorkspace() (string, bool) {
	path := m.selections.workspacePath
	workspace, ok := m.svc.FindWorkspace(path)
	if !ok {
		return "", false
	}
	switch m.selections.mode {
	case domain.ModePromoteToGeneric, domain.ModeTransformHarness:
		return workspace, true
	}
	return workspace, sameDirectory(workspace, path)
}

// sameDirectory reports whether a and b name the same existing directory.
func sameDirectory(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	return err == nil && os.SameFile(ai, bi)
}

// persistedIdentities gives each persisted user layer its stored catalog id, replaced by the full
// identity where the active stack contains the same root.
func persistedIdentities(stored stackconfig.Stored, active map[string]domain.CatalogIdentity) map[string]domain.CatalogIdentity {
	out := make(map[string]domain.CatalogIdentity)
	for _, l := range stored.Layers {
		if l.Default {
			continue
		}
		out[l.Path] = domain.CatalogIdentity{ID: l.CatalogID}
		if id, ok := active[l.Path]; ok {
			out[l.Path] = id
		}
	}
	return out
}
