package main

// An opencode-v2 Runner run gets the same Runner-owned MOSAIC log events as a
// V1 opencode run: run_start/run_end around the session and the invocation_end
// safety net after a dispatch whose plugin did not close the invocation.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/mosaiclog"
)

const v2HarnessID = "opencode-v2"

func TestRunLifecycle_OpenCodeV2_WritesRunStartAndRunEndWithItsHarness(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	sess := newRunLifecycleSession(&mlSession{}, mlLogConfig(runFolder, v2HarnessID))

	_, _ = sess.Start(context.Background(), domain.RunConfig{})

	events := mlReadEvents(t, mlRunLogPath(ws))
	mlAssertPairs(t, events, 1)
	for _, ev := range events {
		if ev["harness"] != v2HarnessID || ev["run_id"] != mlRunID {
			t.Errorf("envelope wrong for an opencode-v2 run: %v", ev)
		}
	}
	if events[0]["cwd"] != ws {
		t.Errorf("run_start.cwd = %v, want %q", events[0]["cwd"], ws)
	}
}

func TestBuildAdapter_OpenCodeV2WithRunFolderAppliesTheSafetyNet(t *testing.T) {
	runFolder, _ := mlRunFolder(t)

	events := mlInvokeFailingHarness(t, runFolder, v2HarnessID)

	evs := mlReadEvents(t, events)
	if len(evs) != 1 || evs[0]["event"] != "invocation_end" {
		t.Fatalf("events = %v, want the safety net's one invocation_end", evs)
	}
	ev := evs[0]
	if ev["harness"] != v2HarnessID || ev["run_id"] != mlRunID || ev["agent_instance_id"] != mlAgentInstance {
		t.Errorf("event = %v, want harness %s, the run and the agent instance", ev, v2HarnessID)
	}
	if ev["schema_version"] != mosaiclog.SchemaVersion {
		t.Errorf("schema_version = %v, want the Runner's %s", ev["schema_version"], mosaiclog.SchemaVersion)
	}
	for _, k := range []string{"status_code", "response"} {
		if _, ok := ev[k]; ok {
			t.Errorf("%s must be omitted after an error return", k)
		}
	}
}

func TestBuildAdapter_OpenCodeV2SafetyNetKeepsAnInvocationEndThePluginWrote(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	events := filepath.Join(ws, "OrchestrationLogs", mlRunID, mlAgentInstance, "03_events.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginLine := `{"event":"invocation_end","agent_instance_id":"` + mlAgentInstance + `","status_code":"SUCCESS"}` + "\n"
	if err := os.WriteFile(events, []byte(pluginLine), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = mlInvokeFailingHarness(t, runFolder, v2HarnessID)

	evs := mlReadEvents(t, events)
	if len(evs) != 1 {
		t.Fatalf("events = %v, want only the plugin's invocation_end", mlEventNames(evs))
	}
	if _, runner := evs[0]["harness"]; runner {
		t.Error("the surviving invocation_end is the Runner's, want the plugin's")
	}
}

func TestBuildAdapter_OpenCodeV2WithoutRunFolderGetsNoSafetyNet(t *testing.T) {
	for name, folder := range map[string]string{"empty": "", "not a run folder": filepath.Join(t.TempDir(), "scratch")} {
		t.Run(name, func(t *testing.T) {
			ws := t.TempDir()
			events := filepath.Join(ws, "OrchestrationLogs", mlRunID, mlAgentInstance, "03_events.jsonl")
			if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(ws)
			h := buildAdapter(folder, v2HarnessID, mlMissingBinary, "", nonHITLTimeouts(time.Minute), nil)

			_, err := h.Invoke(context.Background(), domain.AgentReference{Identifier: "Research"},
				domain.ProtocolRequest{AgentInstanceID: mlAgentInstance, RunID: mlRunID})

			if err == nil {
				t.Fatal("Invoke succeeded, want a launch failure")
			}
			if evs := mlReadEvents(t, events); len(evs) != 0 {
				t.Fatalf("events = %v, want none without a resolvable run folder", evs)
			}
		})
	}
}

func TestBuildAdapter_DecoratedOpenCodeV2KeepsOptionalCapabilities(t *testing.T) {
	runFolder, _ := mlRunFolder(t)

	h := buildAdapter(runFolder, v2HarnessID, mlMissingBinary, "", nonHITLTimeouts(time.Minute), nil)

	if _, ok := h.(domain.RawInvoker); !ok {
		t.Fatal("decorated opencode-v2 adapter lost domain.RawInvoker")
	}
	rev, ok := h.(domain.ExecutableRevealer)
	if !ok || rev.ExecutablePath() != mlMissingBinary {
		t.Fatalf("ExecutableRevealer = %v (ok=%v), want path %q", rev, ok, mlMissingBinary)
	}
	rep, ok := h.(runIDReporter)
	if !ok || rep.RunID() != mlRunID {
		t.Fatalf("runIDReporter = %v (ok=%v), want run id %s", rep, ok, mlRunID)
	}
}

func TestBuildAdapter_DecoratedOpenCodeV2InvokeRawNeverWritesInvocationEnd(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	events := filepath.Join(ws, "OrchestrationLogs", mlRunID, mlAgentInstance, "03_events.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	h := buildAdapter(runFolder, v2HarnessID, mlMissingBinary, "", nonHITLTimeouts(time.Minute), nil)
	raw, ok := h.(domain.RawInvoker)
	if !ok {
		t.Fatal("adapter is not a RawInvoker")
	}

	_, err := raw.InvokeRaw(context.Background(), domain.AgentReference{Identifier: mlAgentInstance}, []byte("{}"))

	if err == nil {
		t.Fatal("InvokeRaw succeeded, want a launch failure")
	}
	if evs := mlReadEvents(t, events); len(evs) != 0 {
		t.Fatalf("events = %v, want none: raw calls are not invocations", evs)
	}
}
