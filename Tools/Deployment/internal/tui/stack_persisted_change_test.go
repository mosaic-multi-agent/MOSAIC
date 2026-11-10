package tui

// stack_persisted_change_test.go verifies what happens to a persisted stack the user changes on
// the stack screen: the run uses the change, a write run persists it (replacing the old stack, so
// the next update starts from it), a declined plan leaves the old stack, promote and transform
// never persist (the persistence rule itself lives in the double; see persistingSvc), and going back to the path screen restarts the stack screen only when the
// confirmed path changed.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// removeTopLayer removes the user layer under the cursor (the top row) on the stack screen.
func (f *stackFlow) removeTopLayer() { f.typeText("d") }

func TestPersistedChange_WriteModes_NextRunStartsFromTheChangedStack(t *testing.T) {
	for _, mode := range writeModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			old, replacement := t.TempDir(), t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(false, old))
			first := newPersistedFlow(t, mode, store, ws, persistFlowOpts{})
			first.toStack(t)
			first.removeTopLayer()
			first.addLayerOnStack(replacement)
			cmd := first.confirmStack(t)
			if call := requireOneReload(t, first); !sameDescription(call.d, stackconfig.Description{UserLayers: []string{replacement}}) {
				t.Fatalf("the run is rebuilt from %+v, want only the replacement layer", call.d)
			}

			runStarted(cmd)

			next := newPersistedFlow(t, mode, store, ws, persistFlowOpts{})
			next.toStack(t)
			requireInView(t, next, replacement)
			requireNotInView(t, next, old)
			next.confirmStack(t)
			if call := requireOneReload(t, next); call.origin != stackconfig.OriginPersisted {
				t.Errorf("the next update's origin = %q, want %q", call.origin, stackconfig.OriginPersisted)
			}
		})
	}
}

func TestPersistedChange_ReplacesTheOldStackAndNeverMergesIt(t *testing.T) {
	store := newPersistStore()
	old, added := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(true, old))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{})
	f.toStack(t)
	requireInView(t, f, old)
	f.removeTopLayer()
	f.addLayerOnStack(added)
	cmd := f.confirmStack(t)

	runStarted(cmd)

	got, ok := store.get(ws)
	if !ok {
		t.Fatal("no persisted stack after the run")
	}
	for _, l := range got.Layers {
		if l.Path == old {
			t.Errorf("the old layer %q survives in the persisted stack %+v", old, got)
		}
	}
	if len(got.Layers) != 2 || got.Layers[0].Path != added || !got.Layers[1].Default {
		t.Errorf("persisted stack = %+v, want [%s, default]", got.Layers, added)
	}
}

func TestPersistedChange_DeclinedPlan_LeavesThePersistedStackUnchanged(t *testing.T) {
	store := newPersistStore()
	old, added := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, old))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{declines: true})
	f.toStack(t)
	f.addLayerOnStack(added)

	cmd := f.confirmStack(t)
	if got, _ := store.get(ws); len(got.Layers) != 1 || got.Layers[0].Path != old {
		t.Errorf("the persisted stack changed when the stack was confirmed: %+v", got.Layers)
	}
	runStarted(cmd) // the plan is declined: the run writes nothing

	next := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{})
	next.toStack(t)
	requireInView(t, next, old)
	requireNotInView(t, next, added)
}

func TestPersistedChange_Promote_UsedByTheRunButNotPersisted(t *testing.T) {
	store := newPersistStore()
	persisted, added := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	file := createAgentFile(t, subDir(t, ws, "nested"))
	f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, file, persistFlowOpts{})
	f.toStack(t)
	f.addLayerOnStack(added)

	cmd := f.confirmStack(t)
	runStarted(cmd)

	call := requireOneReload(t, f)
	if !sameDescription(call.d, stackconfig.Description{UserLayers: []string{persisted, added}}) {
		t.Errorf("the run is rebuilt from %+v, want the changed stack", call.d)
	}
	if call.origin != stackconfig.OriginStackInput {
		t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginStackInput)
	}
	if got, _ := store.get(ws); len(got.Layers) != 1 || got.Layers[0].Path != persisted {
		t.Errorf("promote changed the persisted stack: %+v", got.Layers)
	}
	next := newPersistedFlow(t, domain.ModePromoteToGeneric, store, file, persistFlowOpts{})
	next.toStack(t)
	requireNotInView(t, next, added)
}

func TestPersistedChange_Transform_UsedByTheRunButNotPersisted(t *testing.T) {
	store := newPersistStore()
	persisted, added := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	f := newPersistedFlow(t, domain.ModeTransformHarness, store, ws, persistFlowOpts{})
	f.toStack(t)
	f.addLayerOnStack(added)

	cmd := f.confirmStack(t)
	runStarted(cmd)

	call := requireOneReload(t, f)
	if !sameDescription(call.d, stackconfig.Description{UserLayers: []string{persisted, added}}) {
		t.Errorf("the run is rebuilt from %+v, want the changed stack", call.d)
	}
	if got, _ := store.get(ws); len(got.Layers) != 1 || got.Layers[0].Path != persisted {
		t.Errorf("transform changed the persisted stack: %+v", got.Layers)
	}
}

func TestPersistedChange_BackAndSamePath_KeepsTheScreensState(t *testing.T) {
	store := newPersistStore()
	persisted, added := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{})
	f.toStack(t)
	f.addLayerOnStack(added)

	f.key(tea.KeyEsc)
	f.key(tea.KeyEnter)

	if !f.onStack() {
		t.Fatalf("not on the stack screen:\n%s", f.m.View())
	}
	requireInView(t, f, persisted, added)
}

func TestPersistedChange_BackAndOtherPath_RestartsFromTheNewPathsStack(t *testing.T) {
	store := newPersistStore()
	persistedA, persistedB, added := t.TempDir(), t.TempDir(), t.TempDir()
	wsA := newWorkspaceDir(t, store, storedOf(false, persistedA))
	wsB := newWorkspaceDir(t, store, storedOf(false, persistedB))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, wsA, persistFlowOpts{})
	f.toStack(t)
	f.addLayerOnStack(added)

	f.key(tea.KeyEsc)
	f.m.wsScreen.SetPrefilledPath(wsB)
	f.key(tea.KeyEnter)

	if !f.onStack() {
		t.Fatalf("not on the stack screen:\n%s", f.m.View())
	}
	requireInView(t, f, persistedB)
	requireNotInView(t, f, persistedA, added)
	f.confirmStack(t)
	call := requireOneReload(t, f)
	if call.origin != stackconfig.OriginPersisted || !sameDescription(call.d, stackconfig.Description{UserLayers: []string{persistedB}}) {
		t.Errorf("rebuilt as %+v with origin %q, want the new workspace's stack with origin %q",
			call.d, call.origin, stackconfig.OriginPersisted)
	}
}

func TestPersistedChange_BackAndPathWithoutStack_RestartsFromLaunchOrDefault(t *testing.T) {
	store := newPersistStore()
	persistedA := t.TempDir()
	wsA := newWorkspaceDir(t, store, storedOf(false, persistedA))
	wsB := newWorkspaceDir(t, store, stackconfig.Stored{})
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, wsA, persistFlowOpts{})
	f.toStack(t)
	added := t.TempDir()
	f.addLayerOnStack(added)

	f.key(tea.KeyEsc)
	f.m.wsScreen.SetPrefilledPath(wsB)
	f.key(tea.KeyEnter)

	requireNotInView(t, f, persistedA, added)
	if !strings.Contains(f.m.View(), "[x] kept") {
		t.Errorf("the default layer alone is not the start for a workspace without a persisted stack:\n%s", f.m.View())
	}
}

func TestPersistedChange_PrefilledPathIsNotTheLookupKey(t *testing.T) {
	store := newPersistStore()
	persistedPre := t.TempDir()
	pre := newWorkspaceDir(t, store, storedOf(false, persistedPre))
	confirmed := newWorkspaceDir(t, store, stackconfig.Stored{})
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, pre, persistFlowOpts{})
	f.toPath(t)
	f.m.wsScreen.SetPrefilledPath(confirmed)

	f.key(tea.KeyEnter)

	if !f.onStack() {
		t.Fatalf("not on the stack screen:\n%s", f.m.View())
	}
	requireNotInView(t, f, persistedPre)
}

func TestPersistedChange_PromoteAndTransform_BackAndOtherWorkspace_RestartsFromTheNewStack(t *testing.T) {
	for _, mode := range []domain.RunMode{domain.ModePromoteToGeneric, domain.ModeTransformHarness} {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			persistedA, persistedB, added := t.TempDir(), t.TempDir(), t.TempDir()
			wsA := newWorkspaceDir(t, store, storedOf(false, persistedA))
			wsB := newWorkspaceDir(t, store, storedOf(false, persistedB))
			pathA, pathB := wsA, wsB
			if mode == domain.ModePromoteToGeneric {
				pathA, pathB = createAgentFile(t, wsA), createAgentFile(t, wsB)
			}
			f := newPersistedFlow(t, mode, store, pathA, persistFlowOpts{})
			f.toStack(t)
			f.addLayerOnStack(added)

			f.key(tea.KeyEsc)
			f.m.wsScreen.SetPrefilledPath(pathB)
			f.key(tea.KeyEnter)

			requireInView(t, f, persistedB)
			requireNotInView(t, f, persistedA, added)
		})
	}
}
