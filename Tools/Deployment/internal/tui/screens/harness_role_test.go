package screens_test

// harness_role_test.go verifies the role wording of the configurable HarnessScreen: the title
// and subtitle state what the chosen harness is for in each mode, the tool-version branding
// stays, and HarnessRoleForMode maps each mode to its role.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// newConfiguredHarnessScreen builds a screen showing the fixture harnesses with the given role.
func newConfiguredHarnessScreen(role screens.HarnessRole, sourceName string) *screens.HarnessScreen {
	s := screens.NewHarnessScreen([]domain.HarnessRef{builtinHarness}, 100, 30, plainStyles())
	s.Configure(screens.HarnessScreenConfig{
		Role:       role,
		Harnesses:  []domain.HarnessRef{builtinHarness, descriptorHarness},
		SourceName: sourceName,
	})
	return s
}

func assertViewHasAll(t *testing.T, view string, words ...string) {
	t.Helper()
	lower := strings.ToLower(view)
	for _, w := range words {
		if !strings.Contains(lower, strings.ToLower(w)) {
			t.Errorf("view lacks %q:\n%s", w, view)
		}
	}
}

func TestHarnessScreen_RoleWording_StatesTheRole(t *testing.T) {
	cases := []struct {
		name       string
		role       screens.HarnessRole
		sourceName string
		want       []string
	}{
		{"deploy target", screens.HarnessRoleDeployTarget, "", []string{"deploy to"}},
		{"deployment", screens.HarnessRoleDeployment, "", []string{"deployment", "updated", "changed"}},
		{"promote source", screens.HarnessRolePromoteSource, "", []string{"written for", "generic", "catalog"}},
		{"transform source", screens.HarnessRoleTransformSource, "", []string{"transform from"}},
		{"transform target", screens.HarnessRoleTransformTarget, "Source Harness Name", []string{"transform to", "Source Harness Name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			s := newConfiguredHarnessScreen(tc.role, tc.sourceName)

			// Act
			view := s.View()

			// Assert
			assertViewHasAll(t, view, tc.want...)
		})
	}
}

func TestHarnessScreen_RoleWording_RolesAreDistinct(t *testing.T) {
	// Arrange
	deploy := newConfiguredHarnessScreen(screens.HarnessRoleDeployTarget, "").View()
	transformSrc := newConfiguredHarnessScreen(screens.HarnessRoleTransformSource, "").View()

	// Act / Assert: each screen states its own role, and the other role's wording does not leak in.
	assertViewHasAll(t, deploy, "deploy to")
	assertViewHasAll(t, transformSrc, "transform from")
	if strings.Contains(strings.ToLower(transformSrc), "deploy to") {
		t.Errorf("transform source screen states a deploy role:\n%s", transformSrc)
	}
	if strings.Contains(strings.ToLower(deploy), "transform from") {
		t.Errorf("deploy screen states a transform role:\n%s", deploy)
	}
}

func TestHarnessScreen_TransformTarget_WithoutSourceName_StillStatesRole(t *testing.T) {
	// Arrange
	s := newConfiguredHarnessScreen(screens.HarnessRoleTransformTarget, "")

	// Act
	view := s.View()

	// Assert
	assertViewHasAll(t, view, "transform to")
}

func TestHarnessScreen_Subtitle_IsOnLineDirectlyBelowTitle(t *testing.T) {
	// Arrange
	s := newConfiguredHarnessScreen(screens.HarnessRoleDeployTarget, "")

	// Act
	lines := strings.Split(s.View(), "\n")

	// Assert
	if len(lines) < 2 {
		t.Fatalf("view has fewer than two lines:\n%s", s.View())
	}
	if !strings.Contains(strings.ToLower(lines[0]), "harness") {
		t.Errorf("first line is not the title: %q", lines[0])
	}
	if !strings.Contains(strings.ToLower(lines[1]), "deploy to") {
		t.Errorf("second line does not hold the subtitle: %q", lines[1])
	}
}

func TestHarnessScreen_RoleNone_KeepsTodaysTitleWithoutSubtitle(t *testing.T) {
	// Arrange
	s := newConfiguredHarnessScreen(screens.HarnessRoleNone, "")

	// Act
	lines := strings.Split(s.View(), "\n")

	// Assert
	if !strings.Contains(lines[0], "Select Harness") {
		t.Errorf("title = %q, want it to contain %q", lines[0], "Select Harness")
	}
	if strings.Contains(lines[1], "harness to") || strings.Contains(lines[1], "written for") {
		t.Errorf("a subtitle line is present for HarnessRoleNone: %q", lines[1])
	}
}

func TestHarnessScreen_RoleWording_KeepsToolVersionBranding(t *testing.T) {
	// Arrange
	s := newConfiguredHarnessScreen(screens.HarnessRoleDeployTarget, "")
	s.SetToolVersion("2.3.4")

	// Act
	title := strings.Split(s.View(), "\n")[0]

	// Assert
	assertViewHasAll(t, title, "Deploy", "v2.3.4", "Target")
}

func TestHarnessScreen_Configure_ReWordsOnEachCall(t *testing.T) {
	// Arrange
	s := newConfiguredHarnessScreen(screens.HarnessRoleDeployTarget, "")

	// Act
	s.Configure(screens.HarnessScreenConfig{
		Role:      screens.HarnessRolePromoteSource,
		Harnesses: []domain.HarnessRef{builtinHarness},
	})
	view := s.View()

	// Assert
	assertViewHasAll(t, view, "written for")
	if strings.Contains(strings.ToLower(view), "deploy to") {
		t.Errorf("old wording remains after reconfiguration:\n%s", view)
	}
}

func TestHarnessRoleForMode_MapsEachModeToItsRole(t *testing.T) {
	cases := []struct {
		mode domain.RunMode
		want screens.HarnessRole
	}{
		{domain.ModeDeployWorkspace, screens.HarnessRoleDeployTarget},
		{domain.ModeDeployAgents, screens.HarnessRoleDeployTarget},
		{domain.ModeDeployHooks, screens.HarnessRoleDeployTarget},
		{domain.ModeUpdateWorkspace, screens.HarnessRoleDeployment},
		{domain.ModeUpdateWorkflows, screens.HarnessRoleDeployment},
		{domain.ModePromoteToGeneric, screens.HarnessRolePromoteSource},
		{domain.ModeTransformHarness, screens.HarnessRoleTransformSource},
		{domain.RunMode("no-such-mode"), screens.HarnessRoleNone},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			if got := screens.HarnessRoleForMode(tc.mode); got != tc.want {
				t.Errorf("HarnessRoleForMode(%q) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}
