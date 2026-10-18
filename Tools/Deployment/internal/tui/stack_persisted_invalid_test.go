package tui

// stack_persisted_invalid_test.go verifies the stack screen when the persisted stack cannot be
// used: an unresolvable layer is shown as an error naming the layer, confirming the unchanged
// stack does not start a run or write anything, and the user can replace the stack. The error is
// recognised by the phrase "no longer resolves", which only the validation error contributes (the
// layer row itself shows just the path).

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

const (
	goneLayer      = "gone-catalog-folder-for-test"
	unresolvedText = "no longer resolves"
)

var errGoneLayer = errors.New("persisted catalog layer " + goneLayer + " " + unresolvedText)

// invalidModes are all modes that start from a persisted stack.
var invalidModes = append(append([]domain.RunMode{}, writeModes...),
	domain.ModePromoteToGeneric, domain.ModeTransformHarness)

// pathForMode returns a confirmable path inside ws for mode.
func pathForMode(t *testing.T, mode domain.RunMode, ws string) string {
	t.Helper()
	if mode == domain.ModePromoteToGeneric {
		return createAgentFile(t, subDir(t, ws, "nested"))
	}
	return ws
}

func TestPersistedStart_UnresolvableLayer_ErrorIsShownNamingTheLayer(t *testing.T) {
	for _, mode := range invalidModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
			f := newPersistedFlow(t, mode, store, pathForMode(t, mode, ws), persistFlowOpts{validateErr: errGoneLayer})

			f.toStack(t)

			requireInView(t, f, unresolvedText, goneLayer)
			if len(f.rec.calls) != 0 {
				t.Errorf("the service was rebuilt %d time(s) although the persisted stack is unresolvable", len(f.rec.calls))
			}
			if got, _ := store.get(ws); len(got.Layers) != 1 || got.Layers[0].Path != goneLayer {
				t.Errorf("the persisted stack was written: %+v", got)
			}
		})
	}
}

func TestPersistedStart_ValidPersistedStack_ShowsNoError(t *testing.T) {
	store := newPersistStore()
	ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{}) // validation passes

	f.toStack(t)

	requireInView(t, f, goneLayer)
	requireNotInView(t, f, unresolvedText)
}

func TestPersistedStart_UnresolvableLayer_ConfirmingUnchangedStartsNoRun(t *testing.T) {
	for _, mode := range invalidModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
			f := newPersistedFlow(t, mode, store, pathForMode(t, mode, ws), persistFlowOpts{validateErr: errGoneLayer})
			f.toStack(t)

			cmd := f.confirmStack(t)
			runStarted(cmd)

			if !f.onStack() {
				t.Errorf("left the stack screen with an unresolvable persisted stack:\n%s", f.m.View())
			}
			requireInView(t, f, unresolvedText, goneLayer)
			if len(f.rec.calls) != 0 || len(f.log.entries) != 0 {
				t.Errorf("a run was started: reloads %d, service calls %v", len(f.rec.calls), f.log.entries)
			}
			if got, _ := store.get(ws); len(got.Layers) != 1 || got.Layers[0].Path != goneLayer {
				t.Errorf("the persisted stack was written: %+v", got)
			}
		})
	}
}

func TestPersistedStart_UnresolvableLayer_UserCanReplaceTheBadLayer(t *testing.T) {
	for _, mode := range invalidModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
			f := newPersistedFlow(t, mode, store, pathForMode(t, mode, ws), persistFlowOpts{validateErr: errGoneLayer})
			f.toStack(t)
			replacement := t.TempDir()
			requireInView(t, f, goneLayer, unresolvedText)

			f.removeTopLayer()
			f.addLayerOnStack(replacement)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if len(call.d.UserLayers) != 1 || call.d.UserLayers[0] != replacement {
				t.Errorf("rebuilt from %+v, want only the replacement layer %q", call.d, replacement)
			}
			if call.origin != stackconfig.OriginStackInput {
				t.Errorf("origin = %q, want the stack input origin for a stack the user changed", call.origin)
			}
		})
	}
}

func TestPersistedStart_LaunchStackWins_UnresolvablePersistedLayerShowsNoError(t *testing.T) {
	store := newPersistStore()
	launch := t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{validateErr: errGoneLayer},
		withLaunch(stackconfig.Description{UserLayers: []string{launch}}, true))

	f.toStack(t)

	requireInView(t, f, launch)
	requireNotInView(t, f, unresolvedText, goneLayer)
	f.confirmStack(t)
	requireOneReload(t, f)
}

func TestPersistedStart_UnreadablePersistedStack_ErrorIsShownWithoutRunning(t *testing.T) {
	store := newPersistStore()
	ws := newWorkspaceDir(t, store, storedOf(false, t.TempDir()))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws,
		persistFlowOpts{readErr: errors.New("READ-FAILURE-SENTINEL manifest is corrupt")})

	f.toStack(t)
	runStarted(f.confirmStack(t))

	requireInView(t, f, "READ-FAILURE-SENTINEL")
	if len(f.rec.calls) != 0 || len(f.log.entries) != 0 {
		t.Errorf("a run was started with an unreadable persisted stack: reloads %d, calls %v", len(f.rec.calls), f.log.entries)
	}
}
