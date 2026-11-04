package tui

// Selecting a provider row on the pricing screen starts the edit for exactly
// that (provider, model): the rate questions name the provider.

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

// subjectRecorder is an Interaction that skips every question and records the
// subject of each text question.
type subjectRecorder struct {
	tuiFakeInteraction
	subjects []string
}

func (r *subjectRecorder) AskText(_ context.Context, q interaction.TextQuestion) (interaction.TextAnswer, error) {
	r.subjects = append(r.subjects, q.Subject)
	return interaction.TextAnswer{Status: interaction.SkippedOne}, nil
}

func TestModel_PricingScreen_EnterOnProviderRow_EditsThatProviderKey(t *testing.T) {
	rec := &subjectRecorder{}
	m := newNavModel()
	m.ctx = context.Background()
	m.opts.Service = app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     &tuiFakePricingStore{path: "/test/pricing.yaml"},
		Clock:       &tuiFakeClock{},
		Interaction: rec,
	})
	m.session = new(app.Session)
	m.screen = screenPricing
	m.pricingScreen = screens.NewPricingScreen([]app.ModelPricingStatus{
		{Key: domain.PricingKey{Provider: "p1", Model: "shared-model"}, Model: "shared-model", Priced: true},
		{Key: domain.PricingKey{Provider: "p2", Model: "shared-model"}, Model: "shared-model"},
	}, "", m.width, m.height, stylesFromTheme(m.theme))

	updated, _ := pressKey(m, "j")
	_, cmd := pressKeyType(asModel(updated), tea.KeyEnter)
	if cmd == nil {
		t.Fatal("Enter on a pricing row should start the edit command")
	}
	cmd()

	if len(rec.subjects) == 0 {
		t.Fatal("the edit asked no rate questions")
	}
	for _, s := range rec.subjects {
		if !strings.Contains(s, "shared-model [p2]") {
			t.Errorf("question subject %q does not name shared-model [p2]", s)
		}
	}
}
