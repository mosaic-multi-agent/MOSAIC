package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"

	"mosaic-run/internal/domain"
)

func TestUsageIngestConfig_UnsetVariableMeansNoIngestAndOneDiagnosticPerRun(t *testing.T) {
	rig := uiNewRig(t, "")
	rig.Harness.OnInvokeRaw = func() { uiWriteEvents(t, rig.orchestratorEvents(), "sess-consult") }

	uiRunProcess(t, rig)
	uiRunProcess(t, rig)
	_, _ = rig.invokeRaw(t)
	_, _ = newRunUsageSession(&mlSession{}, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: false})
	_, _ = newRunUsageSession(&mlSession{}, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if n := rig.Rec.count(); n != 0 {
		t.Errorf("ingests = %d, want none while the variable is unset", n)
	}
	if n := rig.Log.naming(string(diag.KindVariableUnset), otelingest.EnvVar); n != 1 {
		t.Errorf("unset diagnostics = %d (%q), want exactly 1 for the run", n, rig.Log.Entries)
	}
}

func TestUsageIngestConfig_UnsetVariableOnANewRunRecordsNoStartOffsetAndOneDiagnostic(t *testing.T) {
	rig := uiNewRig(t, "")

	_, _ = newRunUsageSession(&mlSession{}, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Cfg.StartOffset != nil {
		t.Errorf("start offset = %d, want none without an OTel file path", *rig.Cfg.StartOffset)
	}
	if n := rig.Log.naming(string(diag.KindVariableUnset), otelingest.EnvVar); n != 1 {
		t.Errorf("unset diagnostics = %d (%q), want exactly 1 for the run", n, rig.Log.Entries)
	}
}

func TestUsageIngestConfig_UnsetDiagnosticIsOncePerRunAcrossWirings(t *testing.T) {
	logger := &uiLogger{}
	runFolder, ws := mlRunFolder(t)
	first := uiWire(t, ws, runFolder, "", logger)
	uiRunProcess(t, first)
	resumed := uiWire(t, ws, runFolder, "", logger)
	uiRunProcess(t, resumed)

	if n := logger.naming(string(diag.KindVariableUnset), otelingest.EnvVar); n != 1 {
		t.Fatalf("unset diagnostics across a resume = %d, want 1 for the run", n)
	}

	otherFolder := filepath.Join(ws, "Orchestration-20261003T190000Z-aaaa")
	other := uiWire(t, ws, otherFolder, "", logger)
	other.Harness.OnInvoke = func(domain.ProtocolRequest) {}
	_, _ = other.invoke(t, mlAgentInstance)
	_, _ = newRunUsageSession(&mlSession{}, other.Cfg).Start(context.Background(), domain.RunConfig{})

	if n := logger.naming(string(diag.KindVariableUnset), otelingest.EnvVar); n != 2 {
		t.Errorf("unset diagnostics after a second run = %d, want 2 (one per run)", n)
	}
}

func TestUsageIngestConfig_MissingFileMeansNoIngestAndOneDiagnosticPerRun(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.jsonl")
	rig := uiNewRig(t, missing)
	rig.Cfg.Ingest = otelingest.Ingest // the real ingest decides "file missing"

	uiRunProcess(t, rig)
	uiRunProcess(t, rig)
	_, _ = newRunUsageSession(&mlSession{}, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if n := rig.Log.naming(string(diag.KindFileMissing)); n != 1 {
		t.Errorf("missing-file diagnostics = %d (%q), want exactly 1 for the run", n, rig.Log.Entries)
	}
	if _, err := os.Stat(filepath.Join(rig.RunDir, ".otel-ingest", "state.json")); err == nil {
		t.Error("an ingest state was written although the OTel file does not exist")
	}
}

func TestUsageIngestConfig_FileMissingFromSeamIsAlsoReportedOnce(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Rec.Result = otelingest.Result{
		Status:      otelingest.StatusFileMissing,
		Diagnostics: []diag.Diagnostic{{Kind: diag.KindFileMissing, Message: "gone"}},
	}

	uiRunProcess(t, rig)
	uiRunProcess(t, rig)

	if n := rig.Log.naming(string(diag.KindFileMissing)); n != 1 {
		t.Fatalf("missing-file diagnostics = %d (%q), want exactly 1", n, rig.Log.Entries)
	}
}

func TestUsageIngestConfig_OtherDiagnosticsReachTheDebugLog(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Rec.Result = otelingest.Result{
		Status:      otelingest.StatusLockHeld,
		Diagnostics: []diag.Diagnostic{{Kind: diag.KindLockHeld, Message: "another ingest holds the lock"}},
	}

	uiRunProcess(t, rig)

	if n := rig.Log.naming(string(diag.KindLockHeld)); n == 0 {
		t.Fatalf("lock_held diagnostic missing from the debug log: %q", rig.Log.Entries)
	}
}

func TestUsageIngestConfig_PathComesFromTheRunnersEnvironmentAtWiring(t *testing.T) {
	rig := uiNewRig(t, uiPath)

	if rig.Cfg.OTelPath != uiPath {
		t.Fatalf("OTelPath = %q, want the value of %s", rig.Cfg.OTelPath, otelingest.EnvVar)
	}
	uiSetEnv(t, filepath.Join(t.TempDir(), "changed.jsonl"))
	uiRunProcess(t, rig)

	if len(rig.Rec.Reqs) != 1 || rig.Rec.Reqs[0].OTelPath != uiPath {
		t.Errorf("requests = %+v, want the path read once at wiring", rig.Rec.Reqs)
	}
}

func TestUsageIngestConfig_RunDirIsTheWorkspaceLogFolderOfTheRun(t *testing.T) {
	rig := uiNewRig(t, uiPath)

	want := filepath.Join(rig.Workspace, "OrchestrationLogs", mlRunID)
	if rig.Cfg.RunDir != want {
		t.Fatalf("RunDir = %q, want %q", rig.Cfg.RunDir, want)
	}
}
