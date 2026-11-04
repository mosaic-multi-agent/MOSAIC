package app_test

// Fakes for the usage-ingest ports and a service builder shared by the
// ingest-otel and pending-usage tests.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const ingestTestRunDir = "/test-logs/20260101T000000Z-abcd"

// fakeUsageIngester records its requests and returns a scripted result.
type fakeUsageIngester struct {
	result domain.UsageIngestResult
	err    error
	calls  []domain.UsageIngestRequest
}

func (f *fakeUsageIngester) IngestRun(_ context.Context, req domain.UsageIngestRequest) (domain.UsageIngestResult, error) {
	f.calls = append(f.calls, req)
	return f.result, f.err
}

// fakeIngestProbe answers per run directory and records which directories were asked about.
type fakeIngestProbe struct {
	states map[string]domain.UsageIngestState
	calls  []string
}

func (f *fakeIngestProbe) PendingUsage(runDir string) domain.UsageIngestState {
	f.calls = append(f.calls, runDir)
	return f.states[runDir]
}

// ingestTestService wires one named run (Dir ingestTestRunDir) whose source
// classifies as src, with the given ingest ports (either may be nil).
func ingestTestService(t *testing.T, src domain.Source, ing domain.UsageIngester, probe domain.IngestStateProbe) (*app.Service, *fakeEventReader) {
	t.Helper()
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			return domain.Inventory{Source: s, Runs: []domain.RunEntry{{
				Run: appTestRunRef, Dir: ingestTestRunDir, OrchestratorFile: appTestOrchFile,
			}}}, nil
		},
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile, appTestUsageRecord("o-1", 1_000_000), appTestRunEndEvent())
	store := newFakePricingStore()
	store.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing(appTestUnattrModel)})
	svc := app.New(app.Deps{
		Source: logSource, Reader: reader, Pricing: store, Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
		Ingester: ing, IngestProbe: probe,
	})
	return svc, reader
}

func logsRootSource() domain.Source {
	return domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
}
