package tui

// The model hands every key to the pricing, no-data and export pages first, so
// their `?` overlay takes every key while it is open and esc closes it before
// leaving the page. Where a text input has focus (export path, source path, text
// question) `?` is typed as text and opens nothing.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// helpOpenIn reports whether a screen shows its help overlay.
func helpOpenIn(s any) bool {
	h, ok := s.(interface{ HelpOpen() bool })
	return ok && h.HelpOpen()
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, quit := cmd().(tea.QuitMsg)
	return quit
}

func noDataModel() Model {
	m := newNavModel()
	m.noDataScreen = screens.NewNoDataScreen(
		domain.Source{Kind: domain.SourceLogsRoot, Path: "/empty"}, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenNoData
	return m
}

func pricingModel() Model {
	return setUpPricingScreen([]app.ModelPricingStatus{{Model: "model-a"}}, new(app.Session))
}

func TestPricingHelp_QuestionMarkOpensTheOverlayAndEscClosesItBeforeLeaving(t *testing.T) {
	m := send(pricingModel(), screens.KeyHelp)

	if m.screen != screenPricing || !helpOpenIn(m.pricingScreen) {
		t.Fatalf("? must open the pricing overlay (screen %q)", m.screen)
	}
	m = send(m, "esc")
	if m.screen != screenPricing || helpOpenIn(m.pricingScreen) {
		t.Fatalf("esc must only close the overlay (screen %q)", m.screen)
	}
	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRuns)
}

func TestPricingHelp_EnterWhileTheOverlayIsOpenStartsNoEdit(t *testing.T) {
	m := send(pricingModel(), screens.KeyHelp)

	updated, cmd := pressKeyType(m, tea.KeyEnter)

	if cmd != nil || asModel(updated).screen != screenPricing {
		t.Errorf("enter under the overlay must start no edit (cmd %v, screen %q)", cmd, asModel(updated).screen)
	}
}

func TestNoDataHelp_QuestionMarkOpensTheOverlayAndEscClosesItWithoutQuitting(t *testing.T) {
	m := send(noDataModel(), screens.KeyHelp)
	if m.screen != screenNoData || !helpOpenIn(m.noDataScreen) {
		t.Fatalf("? must open the no-data overlay (screen %q)", m.screen)
	}

	updated, cmd := pressKeyType(m, tea.KeyEsc)

	if isQuit(cmd) || helpOpenIn(asModel(updated).noDataScreen) {
		t.Error("esc must close the overlay and not quit")
	}
}

func TestNoDataHelp_ChooseSourceKeyIsIgnoredWhileTheOverlayIsOpen(t *testing.T) {
	m := send(noDataModel(), screens.KeyHelp)

	updated, cmd := pressKey(m, "s")

	if asModel(updated).screen != screenNoData || cmd != nil {
		t.Errorf("s under the overlay must not restart the analysis (screen %q, cmd %v)", asModel(updated).screen, cmd)
	}
}

func TestNoDataHelp_EscQuitsOnceTheOverlayIsClosed(t *testing.T) {
	m := send(noDataModel(), screens.KeyHelp, screens.KeyHelp)

	_, cmd := pressKeyType(m, tea.KeyEsc)

	if !isQuit(cmd) {
		t.Error("esc on the closed no-data page must still quit")
	}
}

func TestExportPage_QuestionMarkIsTypedIntoThePathAndRejectsIt(t *testing.T) {
	store := newTestDigestStore(false)
	m := press(newExportModel(t, store), "E")

	m = send(m, screens.KeyHelp)
	m = pressType(m, tea.KeyEnter)

	requireExportScreen(t, m)
	if helpOpenIn(m.export.screen) {
		t.Error("? must not open an overlay while the path input has focus")
	}
	assertContains(t, escapeRE.ReplaceAllString(m.View(), ""), "path must end in .digest.json")
	if len(store.writes) != 0 {
		t.Errorf("a rejected path wrote %d file(s)", len(store.writes))
	}
}

func TestExportPage_OverwriteQuestionOffersTheOverlayAndSwallowsTheAnswerKeys(t *testing.T) {
	store := newTestDigestStore(false)
	store.allExist = true
	m := submitExport(t, press(newExportModel(t, store), "E"))

	m = send(m, screens.KeyHelp)
	if !helpOpenIn(m.export.screen) {
		t.Fatal("? must open the overlay while the overwrite question is shown")
	}
	m = send(m, "y")
	requireExportScreen(t, m)
	if len(store.writes) != 0 {
		t.Fatalf("y under the overlay wrote %d file(s)", len(store.writes))
	}
	m = send(m, screens.KeyHelp)
	updated, cmd := pressKey(m, "y")
	m = runCmd(t, asModel(updated), cmd)

	if len(store.writes) != 1 || !store.writes[0].overwrite {
		t.Errorf("writes = %+v, want one overwrite once the overlay is closed", store.writes)
	}
}

func TestSourcePage_QuestionMarkIsTypedAndOpensNoOverlay(t *testing.T) {
	m, _ := openSourceScreen(t)

	m = send(m, "a", screens.KeyHelp, "b")

	if helpOpenIn(m.sourceScreen) || m.sourceScreen.Path() != "a?b" {
		t.Errorf("typed text = %q, want a?b with no overlay", m.sourceScreen.Path())
	}
}
