package tui

// fakes_digest_test.go provides an in-memory DigestStore and a model builder
// whose session was loaded from a scripted digest, for the export-action tests.

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	exportRunA   = "20260101T000000Z-aaaa"
	exportRunB   = "20260102T000000Z-bbbb"
	exportSource = "/in/source.digest.json"
	exportModel  = domain.ModelID("model-x")
)

// tuiFakeDigestStore serves one scripted digest at exportSource and records writes.
type tuiFakeDigestStore struct {
	digest   domain.Digest
	allExist bool  // every output path already exists
	writeErr error // returned by Write when set (and the path is writable)

	writes []tuiDigestWrite
}

type tuiDigestWrite struct {
	path      string
	d         domain.Digest
	overwrite bool
}

func (f *tuiFakeDigestStore) Discover(p string) []string {
	if p == exportSource {
		return []string{p}
	}
	return nil
}

func (f *tuiFakeDigestStore) Read(_ context.Context, p string) (domain.Digest, []domain.Finding, error) {
	if p == exportSource {
		return f.digest, nil, nil
	}
	return domain.Digest{}, nil, nil
}

func (f *tuiFakeDigestStore) Write(_ context.Context, p string, d domain.Digest, overwrite bool) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.allExist && !overwrite {
		return fmt.Errorf("write %s: %w", p, domain.ErrDigestExists)
	}
	f.writes = append(f.writes, tuiDigestWrite{path: p, d: d, overwrite: overwrite})
	return nil
}

func exportTestRun(id string) domain.RunAggregate {
	return domain.RunAggregate{
		Run: domain.NamedRun(id),
		Orchestrator: domain.ActorAggregate{
			Actor: domain.Orchestrator(),
			Invocations: []domain.InvocationUsage{
				{Model: exportModel, Usage: domain.TokenUsage{Input: domain.Tokens(100)}},
			},
		},
	}
}

func newTestDigestStore(withBucket bool) *tuiFakeDigestStore {
	agg := domain.Aggregate{Runs: []domain.RunAggregate{exportTestRun(exportRunA), exportTestRun(exportRunB)}}
	if withBucket {
		b := exportTestRun("")
		b.Run = domain.UnattributableRun()
		agg.Unattributable = &b
	}
	return &tuiFakeDigestStore{digest: domain.Digest{Header: domain.DigestHeader{Version: "1.0.0"}, Aggregate: agg}}
}

// newExportModel returns a model on the runs screen whose session was loaded
// by the real Service from the scripted digest store.
func newExportModel(t *testing.T, store *tuiFakeDigestStore) Model {
	t.Helper()
	svc := app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     &tuiFakePricingStore{path: "/test/pricing.yaml"},
		Digests:     store,
		Clock:       &tuiFakeClock{},
		Interaction: &tuiFakeInteraction{},
		ToolVersion: "test",
	})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: exportSource})
	if err != nil || res.Outcome != app.OutcomeReport || res.Session == nil {
		t.Fatalf("setup: Analyze outcome=%v err=%v session=%v", res.Outcome, err, res.Session)
	}
	m := newNavModel()
	m.ctx = context.Background()
	m.width = 200 // wide enough that help and status lines do not wrap
	m.opts.Service = svc
	updated, _ := m.Update(analysisDoneMsg{result: res})
	m = asModel(updated)
	if m.screen != screenRuns {
		t.Fatalf("setup: screen = %q, want runs", m.screen)
	}
	return m
}

// runCmd executes cmd and feeds its message back into the model.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	updated, _ := m.Update(cmd())
	return asModel(updated)
}
