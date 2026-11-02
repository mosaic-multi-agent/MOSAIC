package tui

// Tests for the TUI side of saving to a chosen configuration level.
//
// Rules under test:
//   - Marking a tool and entering a price ask the level question; the answer
//     decides the file written and is passed through to the store.
//   - The file written is shown afterwards (status line of the Tools view, status
//     line of the pricing screen).
//   - A skipped level question saves nothing.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// markWith returns a Tools-view model over a settings store and interaction.
func markWith(t *testing.T, store *tuiFakeSettingsStore, ia *tuiFakeInteraction) Model {
	t.Helper()
	m := newConfigModel(t, &tuiFakePricingStore{path: "/test/pricing.yaml"}, store, ia, markDigestRun())
	m = send(m, "enter", screens.KeyViewTools)
	assertOn(t, m, screens.ScreenTools)
	return m
}

func TestMarkTool_AsksTheLevelQuestion(t *testing.T) {
	ia := &tuiFakeInteraction{}

	markAndFinish(t, markWith(t, &tuiFakeSettingsStore{}, ia))

	if len(ia.levelQuestions) != 1 {
		t.Errorf("level questions asked = %d, want 1", len(ia.levelQuestions))
	}
}

func TestMarkTool_TheAnswerDecidesTheFileSavedAndShownAfterwards(t *testing.T) {
	cases := []struct {
		option string
		level  domain.ConfigLevel
		shown  string
		other  string
	}{
		{"global", domain.LevelGlobal, markGlobalSettingsPath, markSettingsPath},
		{"project", domain.LevelProject, markSettingsPath, markGlobalSettingsPath},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			store := &tuiFakeSettingsStore{}
			m := markWith(t, store, &tuiFakeInteraction{levelOption: c.option})

			m = markAndFinish(t, m)

			if len(store.levels) != 1 || store.levels[0] != c.level {
				t.Errorf("saved at levels %v, want [%v]", store.levels, c.level)
			}
			view := m.View()
			if !strings.Contains(view, c.shown) {
				t.Errorf("the view must show the written file %q\n%s", c.shown, view)
			}
			if strings.Contains(view, c.other) {
				t.Errorf("the view must not show the file %q that was not written\n%s", c.other, view)
			}
		})
	}
}

func TestMarkTool_SkippedLevelQuestion_SavesNothing(t *testing.T) {
	store := &tuiFakeSettingsStore{}
	m := markWith(t, store, &tuiFakeInteraction{skipLevel: true})

	markAndFinish(t, m)

	if len(store.adds) != 0 {
		t.Errorf("nothing may be saved after the level question is skipped; saved %v", store.adds)
	}
}

// ratesInteraction answers the level question as tuiFakeInteraction does and
// every rate question with "3.00".
type ratesInteraction struct{ tuiFakeInteraction }

func (r *ratesInteraction) AskText(_ context.Context, q interaction.TextQuestion) (interaction.TextAnswer, error) {
	if q.ID == app.QuestionPricingRate {
		return interaction.TextAnswer{Status: interaction.Answered, Text: "3.00"}, nil
	}
	return interaction.TextAnswer{Status: interaction.Answered, Text: ""}, nil
}

// editPriceOf opens the pricing screen of the only model, presses enter and feeds
// the editor's result back.
func editPriceOf(t *testing.T, pricing *tuiFakePricingStore, ia *ratesInteraction) Model {
	t.Helper()
	m := newConfigModel(t, pricing, &tuiFakeSettingsStore{}, ia, markDigestRun())
	m = send(m, "p")
	assertOn(t, m, screens.ScreenPricing)
	updated, cmd := pressKeyType(m, tea.KeyEnter)
	return runCmd(t, asModel(updated), cmd)
}

func TestPricingEdit_AsksTheLevelQuestion(t *testing.T) {
	ia := &ratesInteraction{}

	editPriceOf(t, &tuiFakePricingStore{path: "/test/pricing.yaml"}, ia)

	if len(ia.levelQuestions) != 1 {
		t.Errorf("level questions asked = %d, want 1", len(ia.levelQuestions))
	}
}

func TestPricingEdit_TheAnswerDecidesTheFileSavedAndShownAfterwards(t *testing.T) {
	cases := []struct {
		option string
		level  domain.ConfigLevel
		shown  string
	}{
		{"global", domain.LevelGlobal, tuiFakeGlobalPricingPath},
		{"project", domain.LevelProject, "/test/pricing.yaml"},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			pricing := &tuiFakePricingStore{path: "/test/pricing.yaml"}
			ia := &ratesInteraction{tuiFakeInteraction{levelOption: c.option}}

			m := editPriceOf(t, pricing, ia)

			if len(pricing.puts) != 1 || pricing.puts[0] != c.level {
				t.Errorf("saved at levels %v, want [%v]", pricing.puts, c.level)
			}
			if m.pricingScreen == nil {
				t.Fatal("the pricing screen must keep the confirmation of the save")
			}
			view := m.pricingScreen.View(m.width, m.height)
			if !strings.Contains(view, c.shown) {
				t.Errorf("the pricing screen must show the written file %q\n%s", c.shown, view)
			}
		})
	}
}

func TestPricingEdit_SkippedLevelQuestion_SavesNothing(t *testing.T) {
	pricing := &tuiFakePricingStore{path: "/test/pricing.yaml"}
	ia := &ratesInteraction{tuiFakeInteraction{skipLevel: true}}

	editPriceOf(t, pricing, ia)

	if len(pricing.puts) != 0 {
		t.Errorf("nothing may be saved after the level question is skipped; saved at %v", pricing.puts)
	}
}
