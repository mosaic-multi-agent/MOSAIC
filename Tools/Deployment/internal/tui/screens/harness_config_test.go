package screens_test

// harness_config_test.go verifies the list variants of the configurable HarnessScreen: an
// offered subset, a harness shown disabled with a reason, unusable harnesses staying
// disabled, and restoring the highlight to a previously chosen harness.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

var thirdHarness = domain.HarnessRef{
	ID:          "third-one",
	DisplayName: "Third One",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// configuredScreen builds a screen whose constructor list is only the descriptor harness, so a
// selection or view that matches the configured list cannot come from the constructor's list.
func configuredScreen(cfg screens.HarnessScreenConfig) *screens.HarnessScreen {
	s := screens.NewHarnessScreen(
		[]domain.HarnessRef{descriptorHarness},
		100, 30, plainStyles(),
	)
	s.Configure(cfg)
	return s
}

// enterSelection presses Enter and returns the selected ID, or "" when nothing was selected.
func enterSelection(s *screens.HarnessScreen) string {
	s.Update(harnessEnterKey())
	if !s.Done() {
		return ""
	}
	return s.SelectedID()
}

func TestHarnessScreen_Configure_OffersExactlyTheGivenSubset(t *testing.T) {
	// Arrange / Act
	s := configuredScreen(screens.HarnessScreenConfig{
		Role:      screens.HarnessRoleDeployment,
		Harnesses: []domain.HarnessRef{thirdHarness},
	})
	view := s.View()

	// Assert
	if !strings.Contains(view, "Third One") {
		t.Errorf("offered harness missing from view:\n%s", view)
	}
	for _, other := range []string{"Claude Code", "Custom Descriptor"} {
		if strings.Contains(view, other) {
			t.Errorf("harness %q is not in the offered subset but is shown:\n%s", other, view)
		}
	}
	if got := enterSelection(s); got != "third-one" {
		t.Errorf("selected = %q, want third-one", got)
	}
}

func TestHarnessScreen_Configure_KeepsGivenOrder(t *testing.T) {
	// Arrange / Act
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses: []domain.HarnessRef{thirdHarness, builtinHarness},
	})
	view := s.View()

	// Assert
	if strings.Index(view, "Third One") > strings.Index(view, "Claude Code") {
		t.Errorf("harnesses are not shown in the given order:\n%s", view)
	}
	if got := enterSelection(s); got != "third-one" {
		t.Errorf("first selectable = %q, want third-one", got)
	}
}

func TestHarnessScreen_Configure_UnusableHarnessInSubset_IsDisabledWithReason(t *testing.T) {
	// Arrange / Act
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses: []domain.HarnessRef{inactiveExternalHarness},
	})
	view := s.View()
	got := enterSelection(s)

	// Assert
	if !strings.Contains(view, "requires explicit opt-in") {
		t.Errorf("unusable reason not shown:\n%s", view)
	}
	if got != "" {
		t.Errorf("unusable harness was selected: %q", got)
	}
}

func TestHarnessScreen_Configure_DisabledID_ShowsReasonAndCannotBeSelected(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Role:           screens.HarnessRoleTransformTarget,
		Harnesses:      []domain.HarnessRef{builtinHarness, descriptorHarness},
		DisabledID:     "claude-code",
		DisabledReason: "already the transform source",
	})

	// Act
	view := s.View()
	got := enterSelection(s)

	// Assert
	if !strings.Contains(view, "already the transform source") {
		t.Errorf("disabled reason not shown:\n%s", view)
	}
	if got != "custom-desc" {
		t.Errorf("selected = %q, want custom-desc (the disabled harness must be skipped)", got)
	}
}

func TestHarnessScreen_Configure_DisabledID_OnlyEntry_EnterDoesNothing(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:      []domain.HarnessRef{builtinHarness},
		DisabledID:     "claude-code",
		DisabledReason: "already the transform source",
	})

	// Act
	got := enterSelection(s)

	// Assert
	if got != "" || s.Done() {
		t.Errorf("Enter selected %q on a list with no enabled item", got)
	}
}

func TestHarnessScreen_Configure_DisabledReason_ReplacesUnusableReason(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:      []domain.HarnessRef{builtinHarness, inactiveExternalHarness},
		DisabledID:     "gated-external",
		DisabledReason: "already the transform source",
	})

	// Act
	view := s.View()

	// Assert
	if !strings.Contains(view, "already the transform source") {
		t.Errorf("given reason not shown:\n%s", view)
	}
	if strings.Contains(view, "requires explicit opt-in") {
		t.Errorf("the harness's own reason is shown in place of the given one:\n%s", view)
	}
}

func TestHarnessScreen_Configure_UnknownOrEmptyDisabledID_HasNoEffect(t *testing.T) {
	for _, id := range []string{"", "not-in-list"} {
		t.Run("id="+id, func(t *testing.T) {
			// Arrange
			s := configuredScreen(screens.HarnessScreenConfig{
				Harnesses:      []domain.HarnessRef{thirdHarness, builtinHarness},
				DisabledID:     id,
				DisabledReason: "should never appear",
			})

			// Act
			view := s.View()
			got := enterSelection(s)

			// Assert: the configured list is shown, nothing is disabled, and its first item is selected.
			if !strings.Contains(view, "Third One") {
				t.Errorf("configured list not shown:\n%s", view)
			}
			if strings.Contains(view, "should never appear") {
				t.Errorf("reason shown for an unmatched DisabledID:\n%s", view)
			}
			if got != "third-one" {
				t.Errorf("selected = %q, want third-one", got)
			}
		})
	}
}

func TestHarnessScreen_Configure_HighlightID_PutsCursorOnThatHarness(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:   []domain.HarnessRef{builtinHarness, descriptorHarness, thirdHarness},
		HighlightID: "third-one",
	})

	// Act
	got := enterSelection(s)

	// Assert
	if got != "third-one" {
		t.Errorf("selected = %q, want third-one", got)
	}
}

func TestHarnessScreen_Configure_HighlightID_NotInList_FallsBackToFirstEnabled(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:   []domain.HarnessRef{inactiveExternalHarness, thirdHarness, builtinHarness},
		HighlightID: "custom-desc",
	})

	// Act
	got := enterSelection(s)

	// Assert
	if got != "third-one" {
		t.Errorf("selected = %q, want third-one (first enabled item)", got)
	}
}

func TestHarnessScreen_Configure_HighlightID_Disabled_FallsBackToFirstEnabled(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:      []domain.HarnessRef{thirdHarness, builtinHarness, descriptorHarness},
		DisabledID:     "third-one",
		DisabledReason: "already the transform source",
		HighlightID:    "third-one",
	})

	// Act
	got := enterSelection(s)

	// Assert
	if got != "claude-code" {
		t.Errorf("selected = %q, want claude-code (first enabled item)", got)
	}
}

func TestHarnessScreen_Configure_NoEnabledItem_EnterDoesNothing(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses: []domain.HarnessRef{inactiveExternalHarness},
	})

	// Act
	got := enterSelection(s)

	// Assert
	if got != "" || s.Done() {
		t.Errorf("Enter selected %q with no enabled item", got)
	}
}

func TestHarnessScreen_Configure_ClearsDoneAndBack(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{Harnesses: []domain.HarnessRef{builtinHarness}})
	s.Update(harnessEnterKey())
	if !s.Done() {
		t.Fatal("fixture error: screen not done after Enter")
	}

	// Act
	s.Configure(screens.HarnessScreenConfig{Harnesses: []domain.HarnessRef{builtinHarness, descriptorHarness}})

	// Assert
	if s.Done() || s.Back() {
		t.Errorf("Done=%v Back=%v after Configure, want both false", s.Done(), s.Back())
	}
}

func TestHarnessScreen_Configure_ReplacesEarlierListAndHighlight(t *testing.T) {
	// Arrange
	s := configuredScreen(screens.HarnessScreenConfig{
		Harnesses:   []domain.HarnessRef{builtinHarness, descriptorHarness, thirdHarness},
		HighlightID: "third-one",
	})

	// Act
	s.Configure(screens.HarnessScreenConfig{Harnesses: []domain.HarnessRef{thirdHarness, builtinHarness}})
	view := s.View()
	got := enterSelection(s)

	// Assert
	if strings.Contains(view, "Custom Descriptor") {
		t.Errorf("earlier list still shown:\n%s", view)
	}
	if got != "third-one" {
		t.Errorf("selected = %q, want third-one (first item of the new list)", got)
	}
}

func TestHarnessScreen_Configure_NavigatingOntoDisabledItem_EnterDoesNotSelectIt(t *testing.T) {
	cases := map[string]screens.HarnessScreenConfig{
		"disabled-id": {
			Harnesses:      []domain.HarnessRef{thirdHarness, builtinHarness},
			DisabledID:     "claude-code",
			DisabledReason: "already the transform source",
		},
		"unusable": {
			Harnesses: []domain.HarnessRef{thirdHarness, inactiveExternalHarness},
		},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			s := configuredScreen(cfg)
			disabledID := cfg.Harnesses[1].ID

			// Act
			s.Update(harnessDownKey())
			s.Update(harnessEnterKey())

			// Assert: the configured list is in effect and the disabled item was not chosen.
			view := s.View()
			if !strings.Contains(view, "Third One") {
				t.Errorf("configured list not shown:\n%s", view)
			}
			if s.Done() && s.SelectedID() == disabledID {
				t.Errorf("the disabled harness %q was selected after navigating onto it", disabledID)
			}
		})
	}
}
