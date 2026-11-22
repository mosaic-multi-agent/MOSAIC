package tui

// stack_promote_agents_folder_test.go verifies that the stack step of the promote mode does not
// depend on the agents folder: the layers offered are the launch stack (or the default layer
// alone) whatever folder was confirmed, a persisted stack of a workspace that happens to contain
// the folder is never read, and its problems never show up.

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

func TestPromoteStack_StartsFromLaunchOrDefaultWhateverTheAgentsFolder(t *testing.T) {
	user := t.TempDir()
	launches := []struct {
		name       string
		launch     stackconfig.Description
		given      bool
		wantOrigin stackconfig.Origin
	}{
		{"default layer alone", stackconfig.Description{IncludeDefault: true}, false, stackconfig.OriginDefault},
		{"launch stack", stackconfig.Description{UserLayers: []string{user}}, true, stackconfig.OriginStackInput},
	}
	for _, tc := range launches {
		for _, where := range []string{"inside a workspace with a persisted stack", "outside any workspace"} {
			t.Run(tc.name+"/"+where, func(t *testing.T) {
				store := newPersistStore()
				persisted := t.TempDir()
				ws := newWorkspaceDir(t, store, storedOf(false, persisted))
				folder := subDir(t, ws, "agents")
				if where == "outside any workspace" {
					folder = t.TempDir()
				}
				f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, folder, persistFlowOpts{},
					withLaunch(tc.launch, tc.given))

				f.toStack(t)
				requireNotInView(t, f, persisted)
				f.confirmStack(t)

				call := requireOneReload(t, f)
				if !sameDescription(call.d, tc.launch) {
					t.Errorf("rebuilt from %+v, want the launch stack %+v", call.d, tc.launch)
				}
				if call.origin != tc.wantOrigin {
					t.Errorf("origin = %q, want %q", call.origin, tc.wantOrigin)
				}
			})
		}
	}
}

func TestPromoteStack_SameLayersAreOfferedForDifferentAgentsFolders(t *testing.T) {
	store := newPersistStore()
	persistedA, persistedB := t.TempDir(), t.TempDir()
	wsA := newWorkspaceDir(t, store, storedOf(false, persistedA))
	wsB := newWorkspaceDir(t, store, storedOf(false, persistedB))
	folders := []string{subDir(t, wsA, "agents"), subDir(t, wsB, "agents"), t.TempDir()}

	var views []string
	for _, folder := range folders {
		f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, folder, persistFlowOpts{})
		f.toStack(t)
		views = append(views, f.m.View())
	}

	for i := 1; i < len(views); i++ {
		if views[i] != views[0] {
			t.Errorf("the stack step for agents folder %q differs from the one for %q:\n%s\n--- versus ---\n%s",
				folders[i], folders[0], views[i], views[0])
		}
	}
}

func TestPromoteStack_BackAndOtherAgentsFolder_OffersNoPersistedLayer(t *testing.T) {
	store := newPersistStore()
	persistedA, persistedB := t.TempDir(), t.TempDir()
	wsA := newWorkspaceDir(t, store, storedOf(false, persistedA))
	wsB := newWorkspaceDir(t, store, storedOf(false, persistedB))
	folderB := subDir(t, wsB, "agents")
	f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, subDir(t, wsA, "agents"), persistFlowOpts{})
	f.toStack(t)

	f.key(tea.KeyEsc)
	f.m.wsScreen.SetPrefilledPath(folderB)
	f.key(tea.KeyEnter)

	if !f.onStack() {
		t.Fatalf("not on the stack screen after confirming another agents folder:\n%s", f.m.View())
	}
	requireNotInView(t, f, persistedA, persistedB)
}

func TestPromoteStack_PersistedStackProblemsOfTheFoldersWorkspace_AreNeverShown(t *testing.T) {
	store := newPersistStore()
	ws := newWorkspaceDir(t, store, storedOf(false, goneLayer))
	f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, subDir(t, ws, "agents"),
		persistFlowOpts{validateErr: errGoneLayer, readErr: errors.New("unreadable-persisted-stack")})

	f.toStack(t)
	requireNotInView(t, f, unresolvedText, goneLayer, "unreadable-persisted-stack")
	f.confirmStack(t)

	if len(f.rec.calls) != 1 {
		t.Errorf("the service was rebuilt %d time(s); want once: the persisted stack of the folder's workspace is not involved", len(f.rec.calls))
	}
}

// TestPromoteStack_PersistedStackRuleHoldsOnItsOwn confirms the promote folder directly (no path
// screen), so the persisted-stack rule is exercised independently of the path-kind rule: the
// folder's workspace has a persisted stack with a problem, and none of it may reach the stack step.
func TestPromoteStack_PersistedStackRuleHoldsOnItsOwn(t *testing.T) {
	store := newPersistStore()
	persisted := t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, subDir(t, ws, "agents"),
		persistFlowOpts{validateErr: errGoneLayer, readErr: errors.New("unreadable-persisted-stack")})
	f.m.selections.mode = domain.ModePromoteToGeneric
	f.m.selections.workspacePath = subDir(t, ws, "agents")

	f.m.afterPath()

	if !f.onStack() {
		t.Fatalf("not on the stack screen:\n%s", f.m.View())
	}
	requireNotInView(t, f, persisted, unresolvedText, goneLayer, "unreadable-persisted-stack")
}

func TestPromoteStack_ConfirmedStackReachesTheBatchRun(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	f := newStackFlow(t, domain.ModePromoteToGeneric)
	f.m.wsScreen.SetPrefilledPath(folder)
	f.toStack(t)

	cmd := f.confirmStack(t)
	runStarted(cmd)

	if len(f.log.entries) != 1 || f.log.entries[0] != "reloaded:promote" {
		t.Errorf("run log = %v; want the batch promote run on the service rebuilt from the confirmed stack", f.log.entries)
	}
}
