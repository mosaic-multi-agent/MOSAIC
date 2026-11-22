package tui

// mode_first_flow_test.go verifies the order of the entry screens of the root model: the mode
// screen comes first, the harness screen follows it and is worded for the chosen mode, the path
// screen follows the harness screen, and the stack screen is the last entry screen with its
// injection check still running for the chosen harness.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

var (
	entryAlpha   = domain.HarnessRef{ID: "alpha", DisplayName: "Alpha Harness", Tier: domain.TierBuiltin, Usable: true}
	entryBravo   = domain.HarnessRef{ID: "bravo", DisplayName: "Bravo Harness", Tier: domain.TierBuiltin, Usable: true}
	entryCharlie = domain.HarnessRef{
		ID: "charlie", DisplayName: "Charlie Harness", Tier: domain.TierBuiltin,
		Usable: false, UnusableReason: "not installed",
	}
)

// newEntryModel builds a root model over a service that lists alpha, bravo and the unusable
// charlie, sized wide enough that no screen wraps its title lines.
func newEntryModel() *rootModel {
	svc := &stubNavService{harnesses: []domain.HarnessRef{entryAlpha, entryBravo, entryCharlie}}
	m := newRootModel(context.Background(), svc, Options{Theme: DefaultTheme()})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	return m
}

// newEntryFlowModel builds a root model whose service can run every mode with alpha and bravo
// as harnesses and whose path screen is pre-filled with an existing directory.
func newEntryFlowModel(t *testing.T) *rootModel {
	t.Helper()
	ws := t.TempDir()
	svc := newFlowSvc(ws)
	svc.harnesses = []domain.HarnessRef{entryAlpha, entryBravo}
	return newFlowModel(svc, ws)
}

// harnessScreenText returns the lower-case text of the harness screen with white space
// collapsed, so wording is found even where the screen wrapped a line.
func harnessScreenText(t *testing.T, m *rootModel) string {
	t.Helper()
	requireScreen(t, m, screenHarness)
	return strings.ToLower(collapseSpaces(m.View()))
}

// showHarnessScreen chooses mode and, in the path-first modes, confirms the pre-filled path, so
// the model shows the harness screen of mode (the manifest records no harness: the full list).
func showHarnessScreen(t *testing.T, m *rootModel, mode domain.RunMode) {
	t.Helper()
	pickMode(t, m, mode)
	if isPathFirst(mode) {
		requireScreen(t, m, screenWorkspace)
		pressEnter(m)
	}
	requireScreen(t, m, screenHarness)
}

func TestModeFirst_FirstScreenIsTheModeScreen(t *testing.T) {
	m := newEntryModel()

	if m.screen != screenMode {
		t.Fatalf("first screen = %v, want screenMode", m.screen)
	}
	if !strings.Contains(m.View(), "Select Mode") {
		t.Errorf("the first view is not the mode selection:\n%s", m.View())
	}
}

func TestModeFirst_HarnessFirstModes_HarnessScreenFollowsTheModeScreen(t *testing.T) {
	for _, mode := range harnessFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			m := newEntryModel()

			pickMode(t, m, mode)

			if m.screen != screenHarness {
				t.Errorf("screen = %v after choosing %s, want screenHarness", m.screen, mode)
			}
		})
	}
}

func TestModeFirst_PathFirstModes_PathScreenFollowsTheModeScreen(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			m := newEntryModel()

			pickMode(t, m, mode)

			if m.screen != screenWorkspace {
				t.Errorf("screen = %v after choosing %s, want screenWorkspace", m.screen, mode)
			}
		})
	}
}

func TestModeFirst_HarnessFirstModes_PathScreenFollowsTheHarnessScreen(t *testing.T) {
	for _, mode := range harnessFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			m := newEntryModel()
			pickMode(t, m, mode)

			pickHarness(t, m, entryBravo.ID)

			if m.screen != screenWorkspace {
				t.Errorf("screen = %v after choosing the harness in %s, want screenWorkspace", m.screen, mode)
			}
			if m.selections.harnessID != entryBravo.ID || m.selections.mode != mode {
				t.Errorf("selections = %+v, want mode %s with harness %s", m.selections, mode, entryBravo.ID)
			}
		})
	}
}

// TestModeFirst_HarnessScreen_StatesTheRoleOfTheChosenMode verifies the harness screen says
// what the harness is for in the chosen mode.
func TestModeFirst_HarnessScreen_StatesTheRoleOfTheChosenMode(t *testing.T) {
	cases := []struct {
		mode domain.RunMode
		want string
	}{
		{domain.ModeDeployWorkspace, "harness to deploy to"},
		{domain.ModeDeployAgents, "harness to deploy to"},
		{domain.ModeDeployHooks, "harness to deploy to"},
		{domain.ModeUpdateWorkspace, "harness whose deployment"},
		{domain.ModeUpdateWorkflows, "harness whose deployment"},
		{domain.ModePromoteToGeneric, "generic sources in a catalog"},
		{domain.ModeTransformHarness, "harness to transform from"},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			m := newEntryFlowModel(t)
			showHarnessScreen(t, m, tc.mode)

			text := harnessScreenText(t, m)

			if !strings.Contains(text, tc.want) {
				t.Errorf("the harness screen in %s does not say %q:\n%s", tc.mode, tc.want, m.View())
			}
		})
	}
}

// TestModeFirst_HarnessScreen_WordingDiffersPerRoleOfMode verifies the roles are told apart:
// the wording of one role does not appear on the harness screen of another.
func TestModeFirst_HarnessScreen_WordingDiffersPerRoleOfMode(t *testing.T) {
	m := newEntryFlowModel(t)
	showHarnessScreen(t, m, domain.ModeDeployWorkspace)
	deployText := harnessScreenText(t, m)
	pressEsc(m)
	showHarnessScreen(t, m, domain.ModePromoteToGeneric)
	promoteText := harnessScreenText(t, m)
	pressEsc(m)
	showHarnessScreen(t, m, domain.ModeTransformHarness)
	transformText := harnessScreenText(t, m)

	if deployText == promoteText || promoteText == transformText || deployText == transformText {
		t.Error("the harness screen reads the same in Deploy, Promote and Transform")
	}
	if strings.Contains(promoteText, "to deploy to") || strings.Contains(transformText, "to deploy to") {
		t.Error("the Promote or Transform harness screen still uses the Deploy wording")
	}
}

// TestModeFirst_HarnessScreen_WordingFollowsTheLatestMode verifies that going back and choosing
// a different mode updates the wording of the harness screen.
func TestModeFirst_HarnessScreen_WordingFollowsTheLatestMode(t *testing.T) {
	m := newEntryModel()
	pickMode(t, m, domain.ModeDeployWorkspace)
	pressEsc(m) // back to the mode screen

	pickMode(t, m, domain.ModePromoteToGeneric)

	text := harnessScreenText(t, m)
	if !strings.Contains(text, "generic sources in a catalog") {
		t.Errorf("the harness screen after switching to Promote lacks the Promote wording:\n%s", m.View())
	}
	if strings.Contains(text, "harness to deploy to") {
		t.Errorf("the harness screen after switching to Promote still has the Deploy wording:\n%s", m.View())
	}
}

// TestModeFirst_DeployAndPromoteModes_OfferTheFullHarnessListBeforeThePath verifies the harness
// screen of these modes lists every harness, usable or not, before the path is asked.
func TestModeFirst_DeployAndPromoteModes_OfferTheFullHarnessListBeforeThePath(t *testing.T) {
	modes := []domain.RunMode{
		domain.ModeDeployWorkspace, domain.ModeDeployAgents, domain.ModeDeployHooks, domain.ModePromoteToGeneric,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			m := newEntryModel()
			pickMode(t, m, mode)

			view := m.View()

			if m.screen != screenHarness {
				t.Fatalf("screen = %v, want the harness screen before the path", m.screen)
			}
			for _, h := range []domain.HarnessRef{entryAlpha, entryBravo, entryCharlie} {
				if !strings.Contains(view, h.DisplayName) {
					t.Errorf("the harness screen of %s does not list %q:\n%s", mode, h.DisplayName, view)
				}
			}
		})
	}
}

// TestModeFirst_StackScreen_IsLastAndChecksInjectionsOfTheChosenHarness verifies that in every
// mode the stack screen comes after the path screen and that confirming it checks the harness
// the user chose (not the first listed one).
func TestModeFirst_StackScreen_IsLastAndChecksInjectionsOfTheChosenHarness(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)
			f.orig.harnesses = []domain.HarnessRef{entryAlpha, entryBravo}
			driveToPath(t, f.m, mode, entryBravo.ID)
			if f.onStack() {
				t.Fatal("the stack screen shows before the path was confirmed")
			}

			confirmPath(t, f.m, entryBravo.ID) // confirm the pre-filled path (and the harness after it, where it follows)
			if !f.onStack() {
				t.Fatalf("the stack screen does not follow the path screen:\n%s", f.m.View())
			}
			f.confirmStack(t)

			if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
				t.Errorf("injections checked for %v, want only the chosen harness %q", f.reloaded.checked, entryBravo.ID)
			}
		})
	}
}
