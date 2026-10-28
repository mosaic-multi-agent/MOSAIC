package main

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"

	"mosaic-run/internal/domain"
)

const uiPath = "C:/telemetry/copilot-otel.jsonl"

func TestGHCPCLIUsageIngest_InvokeIngestsTheProcessConversationAfterExit(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	callsWhileRunning := -1
	rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
		uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
		callsWhileRunning = rig.Rec.count()
	}

	if _, err := rig.invoke(t, mlAgentInstance); err != nil {
		t.Fatal(err)
	}

	if callsWhileRunning != 0 {
		t.Fatalf("%d ingests ran while the process was still running, want 0", callsWhileRunning)
	}
	if len(rig.Rec.Reqs) != 1 {
		t.Fatalf("ingests after Invoke = %d, want exactly 1", len(rig.Rec.Reqs))
	}
	req := rig.Rec.Reqs[0]
	if !req.Final || req.WholeFile {
		t.Errorf("request = %+v, want a final, cursor-based (not whole-file) ingest", req)
	}
	if req.RunDir != rig.RunDir || req.OTelPath != uiPath {
		t.Errorf("request targets (%q, %q), want (%q, %q)", req.RunDir, req.OTelPath, rig.RunDir, uiPath)
	}
	if !reflect.DeepEqual(req.Scope, []string{"sess-research"}) {
		t.Errorf("scope = %v, want only the conversation the process's hook recorded", req.Scope)
	}
}

func TestGHCPCLIUsageIngest_InvokeScopeIsOnlyThisInvocationsSessions(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	uiWriteEvents(t, rig.invocationEvents("Other#1"), "sess-other")
	uiWriteEvents(t, rig.orchestratorEvents(), "sess-orchestrator")
	rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
		uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-a", "sess-b", "sess-a")
	}

	_, _ = rig.invoke(t, mlAgentInstance)

	if len(rig.Rec.Reqs) != 1 {
		t.Fatalf("ingests = %d, want 1", len(rig.Rec.Reqs))
	}
	got := append([]string(nil), rig.Rec.Reqs[0].Scope...)
	if len(got) != 2 || !(got[0] == "sess-a" && got[1] == "sess-b" || got[0] == "sess-b" && got[1] == "sess-a") {
		t.Errorf("scope = %v, want exactly the invocation's distinct sessions sess-a and sess-b", got)
	}
}

func TestGHCPCLIUsageIngest_InvokeWithoutRecordedSessionDoesNotIngest(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	uiWriteEvents(t, rig.invocationEvents("Other#1"), "sess-other")

	_, err := rig.invoke(t, mlAgentInstance)

	if err != nil {
		t.Fatal(err)
	}
	if n := rig.Rec.count(); n != 0 {
		t.Fatalf("ingests = %d, want none: an empty scope would finalise every pending conversation", n)
	}
}

func TestGHCPCLIUsageIngest_InvokeReturnsTheWrappedResultAndErrorUnchanged(t *testing.T) {
	boom := errors.New("process crashed")
	tests := []struct {
		name string
		resp domain.ProtocolResponse
		err  error
	}{
		{"success", mlSuccessResponse(), nil},
		{"error", domain.ProtocolResponse{StatusMessage: "partial"}, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := uiNewRig(t, uiPath)
			rig.Harness.Response, rig.Harness.Err = tt.resp, tt.err
			rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
				uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
			}

			got, err := rig.invoke(t, mlAgentInstance)

			if got != tt.resp || err != tt.err {
				t.Fatalf("returned (%+v, %v), want (%+v, %v)", got, err, tt.resp, tt.err)
			}
			if rig.Rec.count() != 1 {
				t.Errorf("ingests = %d, want 1 even when the process failed", rig.Rec.count())
			}
		})
	}
}

func TestGHCPCLIUsageIngest_InvokeRawIngestsOnlyNewOrchestratorSessionsAfterExit(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	uiWriteEvents(t, rig.orchestratorEvents(), "sess-earlier")
	uiWriteEvents(t, rig.invocationEvents("Other#1"), "sess-other")
	callsWhileRunning := -1
	rig.Harness.OnInvokeRaw = func() {
		uiWriteEvents(t, rig.orchestratorEvents(), "sess-consult")
		callsWhileRunning = rig.Rec.count()
	}

	reply, err := rig.invokeRaw(t)

	if err != nil || string(reply) != "reply" {
		t.Fatalf("returned (%q, %v), want the wrapped reply unchanged", reply, err)
	}
	if callsWhileRunning != 0 {
		t.Fatalf("%d ingests ran while the process was still running, want 0", callsWhileRunning)
	}
	if len(rig.Rec.Reqs) != 1 {
		t.Fatalf("ingests after InvokeRaw = %d, want exactly 1", len(rig.Rec.Reqs))
	}
	req := rig.Rec.Reqs[0]
	if !req.Final || !reflect.DeepEqual(req.Scope, []string{"sess-consult"}) {
		t.Errorf("request = %+v, want a final ingest scoped to the session this process added", req)
	}
}

func TestGHCPCLIUsageIngest_InvokeRawWithoutNewSessionDoesNotIngest(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	uiWriteEvents(t, rig.orchestratorEvents(), "sess-earlier")

	_, err := rig.invokeRaw(t)

	if err != nil {
		t.Fatal(err)
	}
	if n := rig.Rec.count(); n != 0 {
		t.Fatalf("ingests = %d, want none when the process left no new session", n)
	}
}

func TestGHCPCLIUsageIngest_InvokeRawReturnsTheWrappedErrorUnchanged(t *testing.T) {
	boom := errors.New("raw failure")
	rig := uiNewRig(t, uiPath)
	rig.Harness.RawReply, rig.Harness.RawErr = []byte("partial"), boom
	rig.Harness.OnInvokeRaw = func() { uiWriteEvents(t, rig.orchestratorEvents(), "sess-consult") }

	reply, err := rig.invokeRaw(t)

	if string(reply) != "partial" || err != boom {
		t.Fatalf("returned (%q, %v), want (%q, %v)", reply, err, "partial", boom)
	}
	if rig.Rec.count() != 1 {
		t.Errorf("ingests = %d, want 1 even when the process failed", rig.Rec.count())
	}
}

func TestGHCPCLIUsageIngest_EveryAdapterCapabilityStaysReachable(t *testing.T) {
	rig := uiNewRig(t, uiPath)
	rig.Harness.ExecPath = "/opt/copilot"

	revealer, ok := rig.Adapter.(domain.ExecutableRevealer)
	if !ok || revealer.ExecutablePath() != "/opt/copilot" {
		t.Errorf("ExecutableRevealer lost or not forwarding: %v %v", ok, revealer)
	}
	reporter, ok := rig.Adapter.(runIDReporter)
	if !ok || reporter.RunID() != mlRunID {
		t.Errorf("RunID reporter lost or not forwarding: %v %v", ok, reporter)
	}
	if _, ok := rig.Adapter.(domain.RawInvoker); !ok {
		t.Error("RawInvoker lost")
	}
	if _, ok := rig.Adapter.(ghcpCLIHarness); !ok {
		t.Error("decorated adapter no longer satisfies the full GHCP CLI capability set")
	}
}

func TestGHCPCLIUsageIngest_IngestFailuresNeverChangeWhatTheAdapterReturns(t *testing.T) {
	results := map[string]otelingest.Result{
		"failed":        {Status: otelingest.StatusFailed, Err: errors.New("disk full")},
		"lock held":     {Status: otelingest.StatusLockHeld, Diagnostics: []diag.Diagnostic{{Kind: diag.KindLockHeld}}},
		"unknown state": {Status: otelingest.StatusUnknownState},
	}
	for name, res := range results {
		t.Run(name, func(t *testing.T) {
			rig := uiNewRig(t, uiPath)
			rig.Rec.Result = res
			rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
				uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
			}
			rig.Harness.OnInvokeRaw = func() { uiWriteEvents(t, rig.orchestratorEvents(), "sess-consult") }

			resp, err := rig.invoke(t, mlAgentInstance)
			reply, rawErr := rig.invokeRaw(t)

			if err != nil || resp != rig.Harness.Response {
				t.Errorf("Invoke returned (%+v, %v), want the wrapped result and no error", resp, err)
			}
			if rawErr != nil || string(reply) != "reply" {
				t.Errorf("InvokeRaw returned (%q, %v), want the wrapped reply and no error", reply, rawErr)
			}
			if rig.Rec.count() != 2 {
				t.Errorf("ingests = %d, want both processes ingested", rig.Rec.count())
			}
		})
	}
}

func TestWithGHCPCLIUsageIngest_LeavesAdapterUndecoratedOutsideARunFolder(t *testing.T) {
	h := &uiHarness{}

	got := withGHCPCLIUsageIngest(h, t.TempDir(), domain.NopDebugLogger{})

	if got != domain.HarnessAdapter(h) || usageIngestOf(got) != nil {
		t.Fatalf("got %T, want the adapter returned undecorated for a non-run folder", got)
	}
}

func TestUsageIngestOf_IsNilForAdaptersThatAreNotTheDecorator(t *testing.T) {
	if usageIngestOf(&uiHarness{}) != nil || usageIngestOf(&MockOpenCodeHarness{}) != nil {
		t.Fatal("usageIngestOf must be nil for an undecorated adapter")
	}
}
