package tui

// narrowed_harness_helpers_test.go holds the shared fixtures for the tests of the path-first
// modes (Update workspace, Change workflows, Transform harness), whose harness screen offers only
// the harnesses the workspace manifest at the entered path records. A service double answers
// RecordedHarnesses per path, standing in for the manifest. Nothing here is a test itself.

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// ghostHarnessID is a harness ID a manifest may record although the registry lists no such harness.
const ghostHarnessID = "ghost"

// recordedHarnessSvc is the launch service double: it answers RecordedHarnesses from a table keyed
// by path and remembers which paths were asked.
type recordedHarnessSvc struct {
	*stackFlowSvc
	recorded map[string][]string
	asked    []string
}

func (s *recordedHarnessSvc) RecordedHarnesses(workspace string) []string {
	s.asked = append(s.asked, filepath.Clean(workspace))
	return s.recorded[filepath.Clean(workspace)]
}

// record sets the harnesses the manifest at path records.
func (s *recordedHarnessSvc) record(path string, ids ...string) {
	s.recorded[filepath.Clean(path)] = ids
}

// narrowFlow is a stackFlow whose launch service answers RecordedHarnesses.
type narrowFlow struct {
	*stackFlow
	svc *recordedHarnessSvc
}

// withPreset presets the harness of the session.
func withPreset(id string) flowOption {
	return func(o *Options) { o.InitialRequest.HarnessID = id }
}

// newNarrowFlow builds a root model for mode over a service that lists alpha, bravo and the
// unusable charlie, whose manifest at the pre-filled path records recordedIDs (none = no record),
// and that has a reload capability. The window is wide enough that no line wraps.
func newNarrowFlow(t *testing.T, mode domain.RunMode, recordedIDs []string, opts ...flowOption) *narrowFlow {
	t.Helper()
	f := &stackFlow{log: &runLog{}, mode: mode, path: entryPath(t, mode)}
	f.orig = newStackFlowSvc("original", f.log)
	f.orig.harnesses = []domain.HarnessRef{entryAlpha, entryBravo, entryCharlie}
	f.reloaded = newStackFlowSvc("reloaded", f.log)
	f.reloaded.harnesses = f.orig.harnesses
	svc := &recordedHarnessSvc{stackFlowSvc: f.orig, recorded: map[string][]string{}}
	svc.record(f.path, recordedIDs...)
	// The service rebuilt from the confirmed stack reads the same manifests as the launch service.
	rebuilt := &recordedHarnessSvc{stackFlowSvc: f.reloaded, recorded: svc.recorded}
	f.rec = &reloadRecorder{svc: rebuilt}
	o := Options{
		Theme:          DefaultTheme(),
		ReloadCatalog:  f.rec.reload,
		LaunchStack:    stackconfig.Description{IncludeDefault: true},
		InitialRequest: &app.DeployRequest{WorkspacePath: f.path},
	}
	for _, opt := range opts {
		opt(&o)
	}
	f.m = newRootModel(t.Context(), svc, o)
	f.m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	return &narrowFlow{stackFlow: f, svc: svc}
}

// toHarnessScreen chooses mode and confirms the pre-filled path. It requires the harness screen.
func (f *narrowFlow) toHarnessScreen(t *testing.T) {
	t.Helper()
	pickMode(t, f.m, f.mode)
	requireScreen(t, f.m, screenWorkspace)
	pressEnter(f.m)
	requireScreen(t, f.m, screenHarness)
}

// offers reports whether the harness screen lists h (by display name, or by ID when it has none).
func offers(m *rootModel, h domain.HarnessRef) bool {
	name := h.DisplayName
	if name == "" {
		name = h.ID
	}
	return strings.Contains(m.View(), name)
}

// requireOffered fails the test unless the harness screen lists exactly the wanted harnesses out
// of alpha, bravo, charlie and the ghost harness.
func requireOffered(t *testing.T, m *rootModel, want ...domain.HarnessRef) {
	t.Helper()
	requireScreen(t, m, screenHarness)
	wanted := map[string]bool{}
	for _, h := range want {
		wanted[h.ID] = true
	}
	candidates := []domain.HarnessRef{entryAlpha, entryBravo, entryCharlie, {ID: ghostHarnessID}}
	for _, h := range candidates {
		if offers(m, h) != wanted[h.ID] {
			t.Errorf("harness %q offered = %v, want %v:\n%s", h.ID, offers(m, h), wanted[h.ID], m.View())
		}
	}
}

// requireMessageOnScreen fails the test unless the current view, with white space collapsed and
// in lower case, contains every fragment.
func requireMessageOnScreen(t *testing.T, m *rootModel, fragments ...string) {
	t.Helper()
	text := strings.ToLower(collapseSpaces(m.View()))
	for _, fragment := range fragments {
		if !strings.Contains(text, strings.ToLower(fragment)) {
			t.Errorf("the view lacks %q:\n%s", fragment, m.View())
		}
	}
}
