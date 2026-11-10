package tui

// Tests for the written-file display of the pricing-gap flow: the TUI runs the
// analysis with the gap flow, and when the flow saved a price the Runs view shows
// the file written in its status line. When nothing was saved, no save status is
// shown.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// gapInteraction answers the gap flow: "Enter prices now" (or skip when
// skipAction is set), the level as tuiFakeInteraction does, and every rate "3.00".
type gapInteraction struct {
	ratesInteraction
	skipAction bool
}

func (g *gapInteraction) SelectOne(ctx context.Context, q interaction.ChoiceQuestion) (interaction.ChoiceAnswer, error) {
	if q.ID == app.QuestionPricingAction {
		if g.skipAction {
			return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: app.PricingActionSkip}, nil
		}
		return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: app.PricingActionEnter}, nil
	}
	return g.tuiFakeInteraction.SelectOne(ctx, q)
}

// runGapAnalysis runs the TUI's analysis (gap flow enabled) over a run with an
// unpriced model and feeds its result back, returning the model on the Runs view.
func runGapAnalysis(t *testing.T, ia *gapInteraction) Model {
	t.Helper()
	digests := &tuiFakeDigestStore{digest: domain.Digest{
		Header:    domain.DigestHeader{Version: "1.0.0"},
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{markDigestRun()}},
	}}
	svc := app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     &tuiFakePricingStore{path: "/test/pricing.yaml"},
		Digests:     digests,
		Settings:    &tuiFakeSettingsStore{},
		Clock:       &tuiFakeClock{},
		Interaction: ia,
		ToolVersion: "test",
	})
	m := newNavModel()
	m.ctx = context.Background()
	m.width = 200
	m.opts.Service = svc
	m.analysisPath = exportSource

	updated, _ := m.Update(m.buildAnalysisCmd()())
	m = asModel(updated)
	if m.screen != screenRuns {
		t.Fatalf("setup: screen = %q after the analysis, want runs", m.screen)
	}
	return m
}

func TestGapFlow_SavedPrice_RunsViewShowsTheFileOfTheChosenLevel(t *testing.T) {
	cases := []struct {
		option string
		shown  string
		other  string
	}{
		{"global", tuiFakeGlobalPricingPath, "/test/pricing.yaml"},
		{"project", "/test/pricing.yaml", tuiFakeGlobalPricingPath},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			ia := &gapInteraction{ratesInteraction: ratesInteraction{tuiFakeInteraction{levelOption: c.option}}}

			m := runGapAnalysis(t, ia)

			view := m.View()
			if !strings.Contains(view, c.shown) {
				t.Errorf("the Runs view must show the written file %q\n%s", c.shown, view)
			}
			if strings.Contains(view, c.other) {
				t.Errorf("the Runs view must not show the file %q that was not written\n%s", c.other, view)
			}
		})
	}
}

func TestGapFlow_NothingSaved_RunsViewShowsNoSaveStatus(t *testing.T) {
	cases := []struct {
		name string
		ia   *gapInteraction
	}{
		{"skipped", &gapInteraction{skipAction: true}},
		{"level question skipped", &gapInteraction{ratesInteraction: ratesInteraction{tuiFakeInteraction{skipLevel: true}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := runGapAnalysis(t, c.ia)

			view := m.View()
			for _, path := range []string{"/test/pricing.yaml", tuiFakeGlobalPricingPath} {
				if strings.Contains(view, path) {
					t.Errorf("no save status may be shown when nothing was saved; the view names %q\n%s", path, view)
				}
			}
		})
	}
}
