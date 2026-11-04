package tui

// Tests that the help overlay lists the configuration files of both levels (the
// pricing and the analysis settings file of each) and whether each exists.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	helpPricingProject  = "/work/MosaicLogAnalyzer/config/pricing.yaml"
	helpPricingGlobal   = "/home/u/.config/MOSAIC/LogAnalyzer/pricing.yaml"
	helpSettingsProject = markSettingsPath
	helpSettingsGlobal  = markGlobalSettingsPath
)

// helpModel is a runs-list model on a tall terminal over stores whose global
// pricing file exists, project pricing file is absent, global settings file is
// absent and project settings file exists.
func helpModel(t *testing.T, globalUnavailable bool) Model {
	t.Helper()
	pricing := &tuiFakePricingStore{
		path: helpPricingProject, global: helpPricingGlobal, globalExists: true, globalUnavailable: globalUnavailable,
	}
	settings := &tuiFakeSettingsStore{projectExists: true, globalUnavailable: globalUnavailable}
	m := newConfigModel(t, pricing, settings, &tuiFakeInteraction{}, markDigestRun())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 80})
	return asModel(updated)
}

// lineHolding returns the first line of view containing label, or "".
func lineHolding(view, label string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, label) {
			return line
		}
	}
	return ""
}

// assertConfigFilesListed checks the Configuration files group of an open overlay.
func assertConfigFilesListed(t *testing.T, view string) {
	t.Helper()
	if !strings.Contains(view, "Configuration files") {
		t.Fatalf("the overlay must have a Configuration files group\n%s", view)
	}
	want := []struct{ label, path, marker string }{
		{"pricing global", helpPricingGlobal, "(exists)"},
		{"pricing project", helpPricingProject, "(absent)"},
		{"settings global", helpSettingsGlobal, "(absent)"},
		{"settings project", helpSettingsProject, "(exists)"},
	}
	for _, w := range want {
		line := lineHolding(view, w.label)
		if line == "" {
			t.Errorf("the overlay must have a %q line\n%s", w.label, view)
			continue
		}
		if !strings.Contains(line, w.path) || !strings.Contains(line, w.marker) {
			t.Errorf("line %q must hold the path %q and %q", line, w.path, w.marker)
		}
	}
}

func TestHelpOverlay_RunsList_ListsBothLevelsOfBothFilesWithExistence(t *testing.T) {
	m := send(helpModel(t, false), screens.KeyHelp)

	assertOn(t, m, screens.ScreenRuns)
	assertConfigFilesListed(t, m.View())
}

func TestHelpOverlay_PerRunView_ListsBothLevelsOfBothFilesWithExistence(t *testing.T) {
	m := send(helpModel(t, false), "enter", screens.KeyViewTools, screens.KeyHelp)

	assertOn(t, m, screens.ScreenTools)
	assertConfigFilesListed(t, m.View())
}

func TestHelpOverlay_PricingScreen_ListsBothLevelsOfBothFilesWithExistence(t *testing.T) {
	m := send(helpModel(t, false), "p", screens.KeyHelp)

	assertOn(t, m, screens.ScreenPricing)
	assertConfigFilesListed(t, m.View())
}

func TestHelpOverlay_GlobalLevelUnavailable_IsMarkedUnavailable(t *testing.T) {
	m := send(helpModel(t, true), screens.KeyHelp)

	view := m.View()

	for _, label := range []string{"pricing global", "settings global"} {
		if line := lineHolding(view, label); !strings.Contains(line, "(unavailable)") {
			t.Errorf("line %q must be marked (unavailable)\n%s", line, view)
		}
	}
	if line := lineHolding(view, "pricing project"); !strings.Contains(line, helpPricingProject) {
		t.Errorf("the project line must still show its path, got %q", line)
	}
}
