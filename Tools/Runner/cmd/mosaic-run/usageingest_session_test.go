package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/otelingest"

	"mosaic-run/internal/domain"
)

func TestRunUsageSession_NewRunRecordsTheOTelFileSizeBeforeTheFirstProcess(t *testing.T) {
	otel := uiWriteOTelFile(t, 137)
	rig := uiNewRig(t, otel)
	var offsetDuringStart *int64
	var callsDuringStart int
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		offsetDuringStart = rig.Cfg.StartOffset
		callsDuringStart = rig.Rec.count()
		return domain.RunOutcome{Status: domain.RunCompleted}, nil
	}}

	_, err := newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if err != nil {
		t.Fatal(err)
	}
	if offsetDuringStart == nil || *offsetDuringStart != 137 {
		t.Fatalf("start offset when the first process could run = %v, want the file size 137", offsetDuringStart)
	}
	if callsDuringStart != 0 {
		t.Errorf("a new run ran %d ingests before its first process, want none", callsDuringStart)
	}
}

func TestRunUsageSession_NewRunWithMissingFileStartsAtZero(t *testing.T) {
	rig := uiNewRig(t, uiPath)

	_, _ = newRunUsageSession(&mlSession{}, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Cfg.StartOffset == nil || *rig.Cfg.StartOffset != 0 {
		t.Fatalf("start offset = %v, want a recorded 0 for a missing file", rig.Cfg.StartOffset)
	}
}

func TestRunUsageSession_NewRunProcessIngestsCarryTheStartOffset(t *testing.T) {
	otel := uiWriteOTelFile(t, 50)
	rig := uiNewRig(t, otel)
	rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
		uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
	}
	inner := &mlSession{start: func(ctx context.Context, _ domain.RunConfig) (domain.RunOutcome, error) {
		_, err := rig.invoke(t, mlAgentInstance)
		return domain.RunOutcome{Status: domain.RunCompleted}, err
	}}

	_, _ = newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Rec.count() == 0 {
		t.Fatal("no ingest ran for the process")
	}
	for i, req := range rig.Rec.Reqs {
		if req.StartOffset == nil || *req.StartOffset != 50 {
			t.Errorf("ingest %d StartOffset = %v, want 50 so earlier usage is excluded", i, req.StartOffset)
		}
	}
}

func TestRunUsageSession_ResumeRunsAFinalCatchUpBeforeAnyProcess(t *testing.T) {
	otel := uiWriteOTelFile(t, 10)
	rig := uiNewRig(t, otel)
	rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
		uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
	}
	var callsBeforeProcess int
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		callsBeforeProcess = rig.Rec.count()
		_, err := rig.invoke(t, mlAgentInstance)
		return domain.RunOutcome{Status: domain.RunCompleted}, err
	}}

	_, err := newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if err != nil {
		t.Fatal(err)
	}
	if callsBeforeProcess != 1 {
		t.Fatalf("ingests before the first process = %d, want exactly the catch-up", callsBeforeProcess)
	}
	catchUp := rig.Rec.Reqs[0]
	if !catchUp.Final || len(catchUp.Scope) != 0 || catchUp.WholeFile || catchUp.StartOffset != nil {
		t.Errorf("catch-up = %+v, want a final, unscoped, cursor-based ingest without a start offset", catchUp)
	}
	if catchUp.RunDir != rig.RunDir || catchUp.OTelPath != otel {
		t.Errorf("catch-up targets (%q, %q), want (%q, %q)", catchUp.RunDir, catchUp.OTelPath, rig.RunDir, otel)
	}
	if rig.Cfg.StartOffset != nil {
		t.Errorf("a resumed run recorded start offset %d, want none", *rig.Cfg.StartOffset)
	}
}

func TestRunUsageSession_ReturnsStartResultAndErrorUnchanged(t *testing.T) {
	boom := errors.New("start failed")
	want := domain.RunOutcome{Status: domain.RunFailed, Message: "no go"}
	for _, isNew := range []bool{true, false} {
		rig := uiNewRig(t, uiWriteOTelFile(t, 5))
		rig.Rec.Result = otelingest.Result{Status: otelingest.StatusFailed, Err: errors.New("ingest broke")}
		inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) { return want, boom }}

		got, err := newRunUsageSession(inner, rig.Cfg).Start(context.Background(), domain.RunConfig{IsNewRun: isNew})

		if !reflect.DeepEqual(got, want) || err != boom {
			t.Errorf("isNew=%v: returned (%+v, %v), want (%+v, %v)", isNew, got, err, want, boom)
		}
		if isNew && rig.Cfg.StartOffset == nil {
			t.Errorf("a new run recorded no start offset")
		}
		if !isNew && rig.Rec.count() != 1 {
			t.Errorf("a resumed run ran %d catch-up ingests, want 1 even though it failed", rig.Rec.count())
		}
	}
}

func TestRunLifecycleSession_AppliesTheUsageSessionOnlyWhenConfigured(t *testing.T) {
	otel := uiWriteOTelFile(t, 77)
	rig := uiNewRig(t, otel)
	cfg := mlLogConfig(rig.RunFolder, commonharness.HarnessIDGHCPCLI)
	cfg.UsageIngest = rig.Cfg

	_, _ = newRunLifecycleSession(&mlSession{}, cfg).Start(context.Background(), domain.RunConfig{IsNewRun: true})

	if rig.Cfg.StartOffset == nil || *rig.Cfg.StartOffset != 77 || rig.Rec.count() != 0 {
		t.Fatalf("offset=%v ingests=%d, want offset 77 and no ingest (no process ran)", rig.Cfg.StartOffset, rig.Rec.count())
	}
	mlAssertPairs(t, mlReadEvents(t, mlRunLogPath(rig.Workspace)), 1)

	// Without the field the lifecycle decorator behaves as before.
	other := uiNewRig(t, otel)
	plain := mlLogConfig(other.RunFolder, commonharness.HarnessIDGHCPCLI)
	_, _ = newRunLifecycleSession(&mlSession{}, plain).Start(context.Background(), domain.RunConfig{IsNewRun: true})
	if other.Rec.count() != 0 {
		t.Errorf("ingests without UsageIngest = %d, want 0", other.Rec.count())
	}
}

func TestBuildAdapter_OnlyTheGHCPCLIHarnessGetsUsageIngest(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
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
			h := buildAdapter(runFolder, tt.harness, "", "", 0)

			if got := usageIngestOf(h) != nil; got != tt.want {
				t.Fatalf("usage ingest applied = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAdapter_GHCPCLIWithoutRunFolderStaysUndecorated(t *testing.T) {
	uiSetEnv(t, uiPath)

	h := buildAdapter("", commonharness.HarnessIDGHCPCLI, "", "", 0)

	if usageIngestOf(h) != nil {
		t.Fatal("an adapter built without a run folder must not be decorated")
	}
}

