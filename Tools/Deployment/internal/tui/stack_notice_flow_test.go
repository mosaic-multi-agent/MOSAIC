package tui

// stack_notice_flow_test.go verifies the migration notice on the stack screen: it tells the user
// that custom-catalog behaviour has changed, and it appears only when the session was launched
// with a user catalog. The notice is recognised by the word "custom", which the stack screen's
// regular text does not use; a second phrase, "default", is required alongside it so that the
// notice also tells the user the default layer can be switched on.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// hasNotice reports whether the current view carries the migration notice.
func hasNotice(f *stackFlow) bool {
	view := strings.ToLower(f.m.View())
	return strings.Contains(view, "custom") && strings.Contains(view, "default")
}

func TestMigrationNotice_UserCatalogLaunch_ShowsTheNoticeInEveryMode(t *testing.T) {
	user := t.TempDir()
	launches := map[string]stackconfig.Description{
		"without the default-layer flag": {UserLayers: []string{user}},
		"with the default-layer flag":    {UserLayers: []string{user}, IncludeDefault: true},
	}
	for name, launch := range launches {
		for _, mode := range entryModes {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				f := newStackFlow(t, mode, withLaunch(launch, true))

				f.toStack(t)

				if !hasNotice(f) {
					t.Errorf("no migration notice on the stack screen:\n%s", f.m.View())
				}
				requireInView(t, f, user, "(default)")
			})
		}
	}
}

func TestMigrationNotice_UserCatalogLaunch_OffersTheDefaultLayerSwitchedOff(t *testing.T) {
	user := t.TempDir()
	f := newStackFlow(t, entryModes[0], withLaunch(stackconfig.Description{UserLayers: []string{user}}, true))

	f.toStack(t)

	requireInView(t, f, user, "(default)", "[ ] left out")
	if !hasNotice(f) {
		t.Errorf("no migration notice on the stack screen:\n%s", f.m.View())
	}
}

func TestMigrationNotice_NoUserCatalog_ShowsNoNotice(t *testing.T) {
	cases := map[string]struct {
		launch stackconfig.Description
		given  bool
	}{
		"no stack input":                 {stackconfig.Description{IncludeDefault: true}, false},
		"default-layer flag given alone": {stackconfig.Description{IncludeDefault: true}, true},
	}
	for name, tc := range cases {
		for _, mode := range entryModes {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				f := newStackFlow(t, mode, withLaunch(tc.launch, tc.given))

				f.toStack(t)

				if hasNotice(f) {
					t.Errorf("a migration notice is shown without a user catalog:\n%s", f.m.View())
				}
			})
		}
	}
}

func TestMigrationNotice_StaysWhenTheUserEditsTheStack(t *testing.T) {
	user := t.TempDir()
	f := newStackFlow(t, entryModes[0], withLaunch(stackconfig.Description{UserLayers: []string{user}}, true))
	f.toStack(t)

	f.addLayerOnStack(t.TempDir())

	if !hasNotice(f) {
		t.Errorf("the migration notice disappeared after the user edited the stack:\n%s", f.m.View())
	}
}

func TestMigrationNotice_NoLaunchCatalog_PersistedUserLayersShowNoNotice(t *testing.T) {
	store := newPersistStore()
	persisted := t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(true, persisted))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{})

	f.toStack(t)

	requireInView(t, f, persisted)
	if hasNotice(f) {
		t.Errorf("a migration notice is shown although the session was launched without a user catalog:\n%s", f.m.View())
	}
}

func TestMigrationNotice_LaunchCatalogWinningOverAPersistedStack_ShowsTheNotice(t *testing.T) {
	store := newPersistStore()
	persisted, launch := t.TempDir(), t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(true, persisted))
	f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{},
		withLaunch(stackconfig.Description{UserLayers: []string{launch}}, true))

	f.toStack(t)

	requireInView(t, f, launch)
	if !hasNotice(f) {
		t.Errorf("no migration notice although the session was launched with a user catalog:\n%s", f.m.View())
	}
}
