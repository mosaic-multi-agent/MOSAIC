package tui

// stack_flow_test.go verifies where the stack screen sits in the entry flow:
//
//	harness -> mode -> path screen -> stack -> running
//
// It is shown in every mode when the reload capability is configured and omitted in every mode
// when it is not. The path screen keeps the kind of its mode, a pre-filled path is still shown,
// back-navigation keeps the stack screen's state, and the initial default-layer choice follows
// the way the session was launched.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

func TestStackFlow_EveryMode_StackScreenFollowsThePathScreen(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)
			f.toPath(t)

			f.key(tea.KeyEnter) // confirm the pre-filled path

			if !f.onStack() {
				t.Errorf("after confirming the path in %s the screen is not the stack screen (screen = %v):\n%s",
					mode, f.m.screen, f.m.View())
			}
			if len(f.rec.calls) != 0 {
				t.Errorf("the service was rebuilt %d time(s) before the stack was confirmed", len(f.rec.calls))
			}
		})
	}
}

func TestStackFlow_NoReloadCapability_PathScreenLeadsStraightToRunning(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode, withoutReload())
			f.toPath(t)

			cmd := f.key(tea.KeyEnter)

			if f.m.screen != screenRunning {
				t.Fatalf("screen = %v after the path in %s without a reload capability, want screenRunning:\n%s",
					f.m.screen, mode, f.m.View())
			}
			runStarted(cmd)
			if want := "original:" + operationOf[mode]; len(f.log.entries) != 1 || f.log.entries[0] != want {
				t.Errorf("run log = %v, want [%s]", f.log.entries, want)
			}
		})
	}
}

func TestStackFlow_ModeSelection_LeadsToThePathScreenNotACatalogueFolderScreen(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode, withLaunch(stackconfig.Description{UserLayers: []string{t.TempDir()}}, true))

			f.toPath(t)

			if f.onStack() {
				t.Errorf("the stack screen shows straight after mode selection in %s; the path screen must come first", mode)
			}
			if strings.Contains(strings.ToLower(f.m.View()), "catalogue folder") {
				t.Errorf("a catalogue folder screen shows after mode selection in %s:\n%s", mode, f.m.View())
			}
		})
	}
}

func TestStackFlow_PathScreenKind_FollowsTheMode(t *testing.T) {
	file := createAgentFile(t, t.TempDir())
	dir := t.TempDir()
	cases := []struct {
		label       string
		mode        domain.RunMode
		path        string
		wantOnStack bool
	}{
		{"deploy-workspace takes a directory", domain.ModeDeployWorkspace, dir, true},
		{"deploy-workspace rejects a file", domain.ModeDeployWorkspace, file, false},
		{"update-workspace takes a directory", domain.ModeUpdateWorkspace, dir, true},
		{"update-workflows takes a directory", domain.ModeUpdateWorkflows, dir, true},
		{"deploy-agents takes a directory", domain.ModeDeployAgents, dir, true},
		{"deploy-hooks takes a directory", domain.ModeDeployHooks, dir, true},
		{"promote takes a file", domain.ModePromoteToGeneric, file, true},
		{"promote rejects a directory", domain.ModePromoteToGeneric, dir, false},
		{"transform takes a file", domain.ModeTransformHarness, file, true},
		{"transform takes a directory", domain.ModeTransformHarness, dir, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			f := newStackFlow(t, tc.mode)
			f.m.wsScreen.SetPrefilledPath(tc.path)
			f.toPath(t)

			f.key(tea.KeyEnter)

			if got := f.onStack(); got != tc.wantOnStack {
				t.Errorf("%s with path %q: on the stack screen = %v, want %v:\n%s",
					tc.mode, tc.path, got, tc.wantOnStack, f.m.View())
			}
		})
	}
}

func TestStackFlow_PrefilledPath_IsShownOnThePathScreenBeforeTheStackScreen(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)

			f.toPath(t)

			if !strings.Contains(f.m.View(), f.path) {
				t.Errorf("the path screen does not show the pre-filled path %q:\n%s", f.path, f.m.View())
			}
			if f.onStack() {
				t.Error("the stack screen is showing instead of the pre-filled path screen")
			}
		})
	}
}

func TestStackFlow_PreselectedHarness_StartsAtModeSelectionAndStillShowsTheStackScreen(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace, withPreselectedHarness())

	if f.m.screen != screenMode {
		t.Fatalf("a pre-selected harness starts on screen %v, want the mode screen", f.m.screen)
	}
	f.toStack(t)
}

func TestStackFlow_Back_StackToPathToMode(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)

	f.key(tea.KeyEsc)
	if f.m.screen != screenWorkspace {
		t.Fatalf("screen = %v after Esc on the stack screen, want the path screen", f.m.screen)
	}
	f.key(tea.KeyEsc)

	if f.m.screen != screenMode {
		t.Errorf("screen = %v after Esc on the path screen, want the mode screen", f.m.screen)
	}
}

func TestStackFlow_Back_KeepsTheStackScreensState(t *testing.T) {
	user := t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace, withLaunch(stackconfig.Description{UserLayers: []string{user}}, true))
	f.toStack(t)
	f.key(tea.KeyDown) // the default row
	f.space()          // keep the default layer
	if !strings.Contains(f.m.View(), "[x] kept") {
		t.Fatalf("precondition: the default layer is not kept after the toggle:\n%s", f.m.View())
	}

	f.key(tea.KeyEsc)   // back to the path screen
	f.key(tea.KeyEnter) // forward to the stack screen again

	if !f.onStack() {
		t.Fatalf("not back on the stack screen:\n%s", f.m.View())
	}
	if !strings.Contains(f.m.View(), "[x] kept") {
		t.Errorf("the stack screen lost its state across back-navigation; the default layer is no longer kept:\n%s", f.m.View())
	}
}

func TestStackFlow_Back_KeepsAnAddedFolder(t *testing.T) {
	added := t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)
	f.typeText("a")
	f.typeText(added)
	f.key(tea.KeyEnter)
	if !strings.Contains(f.m.View(), added) {
		t.Fatalf("precondition: the added folder is not listed:\n%s", f.m.View())
	}

	f.key(tea.KeyEsc)
	f.key(tea.KeyEnter)

	if !strings.Contains(f.m.View(), added) {
		t.Errorf("the folder added on the stack screen is gone after back-navigation:\n%s", f.m.View())
	}
}

func TestStackFlow_InitialDefaultLayer_FollowsTheLaunchCase(t *testing.T) {
	user := t.TempDir()
	cases := []struct {
		name       string
		launch     stackconfig.Description
		given      bool
		wantKept   bool
		wantLayers bool
	}{
		{"no stack input", stackconfig.Description{IncludeDefault: true}, false, true, false},
		{"user catalog without the default-layer flag", stackconfig.Description{UserLayers: []string{user}}, true, false, true},
		{"user catalog with the default-layer flag", stackconfig.Description{UserLayers: []string{user}, IncludeDefault: true}, true, true, true},
		{"default-layer flag alone", stackconfig.Description{IncludeDefault: true}, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStackFlow(t, domain.ModeDeployWorkspace, withLaunch(tc.launch, tc.given))

			f.toStack(t)

			view := f.m.View()
			if !strings.Contains(view, "(default)") {
				t.Errorf("the default layer is not listed as a row:\n%s", view)
			}
			if tc.wantKept && !strings.Contains(view, "[x] kept") {
				t.Errorf("the default layer toggle is not on:\n%s", view)
			}
			if !tc.wantKept && !strings.Contains(view, "[ ] left out") {
				t.Errorf("the default layer toggle is not off:\n%s", view)
			}
			if tc.wantLayers && !strings.Contains(view, user) {
				t.Errorf("the launch catalog %q is not listed:\n%s", user, view)
			}
		})
	}
}

func TestStackFlow_StackScreen_ShowsTheLaunchLayersInOrder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace,
		withLaunch(stackconfig.Description{UserLayers: []string{first, second}}, true))

	f.toStack(t)

	view := f.m.View()
	i, j := strings.Index(view, first), strings.Index(view, second)
	if i < 0 || j < 0 || i > j {
		t.Errorf("launch layers are not listed highest first (first at %d, second at %d):\n%s", i, j, view)
	}
}

func TestStackFlow_CtrlC_FromTheStackScreen_CancelsAndQuits(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)

	cmd := f.key(tea.KeyCtrlC)

	if f.m.ctx.Err() == nil {
		t.Error("the context is not cancelled after ctrl+c on the stack screen")
	}
	if cmd == nil {
		t.Error("no quit command after ctrl+c on the stack screen")
	}
}

func TestStackFlow_WindowResize_OnTheStackScreen_UpdatesTheModel(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)

	f.m.Update(tea.WindowSizeMsg{Width: 140, Height: 44})

	if f.m.width != 140 || f.m.height != 44 {
		t.Errorf("size = %dx%d after resize, want 140x44", f.m.width, f.m.height)
	}
	if !f.onStack() {
		t.Error("the stack screen is no longer showing after a resize")
	}
}
