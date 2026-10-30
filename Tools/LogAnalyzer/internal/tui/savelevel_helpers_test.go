package tui

// Helpers shared by the tests of saving to a chosen configuration level.

import (
	"context"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// newConfigModel returns a model on the runs list whose session was loaded by the
// real service from a scripted digest holding run, wired with the given stores
// and interaction.
func newConfigModel(t *testing.T, pricing *tuiFakePricingStore, settings *tuiFakeSettingsStore, ia interaction.Interaction, run domain.RunAggregate) Model {
	t.Helper()
	digests := &tuiFakeDigestStore{digest: domain.Digest{
		Header:    domain.DigestHeader{Version: "1.0.0"},
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}},
	}}
	svc := app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     pricing,
		Digests:     digests,
		Settings:    settings,
		Clock:       &tuiFakeClock{},
		Interaction: ia,
		ToolVersion: "test",
	})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: exportSource})
	if err != nil || res.Outcome != app.OutcomeReport || res.Session == nil {
		t.Fatalf("setup: Analyze outcome=%v err=%v", res.Outcome, err)
	}
	m := newNavModel()
	m.ctx = context.Background()
	m.width = 200
	m.opts.Service = svc
	updated, _ := m.Update(analysisDoneMsg{result: res})
	return asModel(updated)
}
