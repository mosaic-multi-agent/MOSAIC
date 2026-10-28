package main

import (
	"context"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"

	"mosaic-run/internal/domain"
)

func TestNewRunLogConfig_OnlyTheGHCPCLIAdapterContributesItsUsageIngest(t *testing.T) {
	uiSetEnv(t, uiPath)
	tests := []struct {
		harness string
		want    bool
	}{
		{commonharness.HarnessIDGHCPCLI, true},
		{commonharness.HarnessIDClaudeCode, false},
		{commonharness.HarnessIDOpenCode, false},
		{"fake", false},
	}
	for _, tt := range tests {
		t.Run(tt.harness, func(t *testing.T) {
			runFolder, _ := mlRunFolder(t)
			adapter := buildAdapter(runFolder, tt.harness, "", "", 0)
			logger := domain.NopDebugLogger{}

			cfg := newRunLogConfig(adapter, runFolder, tt.harness, logger, mainTestClock{t: mlFixedNow})

			if (cfg.UsageIngest != nil) != tt.want {
				t.Fatalf("UsageIngest set = %v, want %v", cfg.UsageIngest != nil, tt.want)
			}
			if cfg.UsageIngest != usageIngestOf(adapter) {
				t.Errorf("UsageIngest = %p, want the adapter's own config %p", cfg.UsageIngest, usageIngestOf(adapter))
			}
			if cfg.RunFolder != runFolder || cfg.HarnessID != tt.harness || cfg.Debug != logger {
				t.Errorf("config = %+v, want the folder, harness and logger passed through", cfg)
			}
		})
	}
}

func TestNewRunLogConfig_ConfigDrivesTheRealSessionWrapperForGHCPCLI(t *testing.T) {
	rig := uiNewRig(t, uiWriteOTelFile(t, 21))
	cfg := newRunLogConfig(rig.Adapter, rig.RunFolder, commonharness.HarnessIDGHCPCLI, domain.NopDebugLogger{}, mainTestClock{t: mlFixedNow})

	_, _ = newRunLifecycleSession(&mlSession{}, cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Cfg.StartOffset == nil || *rig.Cfg.StartOffset != 21 {
		t.Fatalf("start offset = %v, want 21 recorded through the composed session", rig.Cfg.StartOffset)
	}
}

func TestRunUsageSession_NewRunWithMissingFileReportsOneDiagnosticAndStartsAtZero(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Cfg.Ingest = otelingest.Ingest
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		uiRunProcess(t, rig)
		return domain.RunOutcome{Status: domain.RunCompleted}, nil
	}}

	_, _ = newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Cfg.StartOffset == nil || *rig.Cfg.StartOffset != 0 {
		t.Errorf("start offset = %v, want a recorded 0", rig.Cfg.StartOffset)
	}
	if n := rig.Log.naming(string(diag.KindFileMissing)); n != 1 {
		t.Errorf("missing-file diagnostics = %d (%q), want exactly 1 for the run", n, rig.Log.Entries)
	}
}

func TestRunUsageSession_ResumedRunProcessIngestsCarryNoStartOffset(t *testing.T) {
	rig := uiNewRig(t, uiWriteOTelFile(t, 50))
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		uiRunProcess(t, rig)
		return domain.RunOutcome{Status: domain.RunCompleted}, nil
	}}

	_, _ = newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if rig.Rec.count() != 2 {
		t.Fatalf("ingests = %d, want the catch-up and the process ingest", rig.Rec.count())
	}
	for i, req := range rig.Rec.Reqs {
		if req.StartOffset != nil {
			t.Errorf("ingest %d StartOffset = %d, want none on a resumed run", i, *req.StartOffset)
		}
	}
}

func TestGHCPCLIUsageIngest_InvokeRawIngestsWhenTheOrchestratorStreamDidNotExistBefore(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Harness.OnInvokeRaw = func() { uiWriteEvents(t, rig.orchestratorEvents(), "sess-first") }

	_, err := rig.invokeRaw(t)

	if err != nil {
		t.Fatal(err)
	}
	if len(rig.Rec.Reqs) != 1 || len(rig.Rec.Reqs[0].Scope) != 1 || rig.Rec.Reqs[0].Scope[0] != "sess-first" {
		t.Fatalf("requests = %+v, want one ingest scoped to sess-first", rig.Rec.Reqs)
	}
}

func TestUsageIngestConfig_ConversationNotTiedDiagnosticReachesTheDebugLog(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Rec.Result = otelingest.Result{
		Status:      otelingest.StatusIngested,
		Diagnostics: []diag.Diagnostic{{Kind: diag.KindConversationNotTied, Message: "usage spans without a run session"}},
	}

	uiRunProcess(t, rig)

	if n := rig.Log.naming(string(diag.KindConversationNotTied)); n == 0 {
		t.Fatalf("conversation_not_tied diagnostic missing from the debug log: %q", rig.Log.Entries)
	}
}
