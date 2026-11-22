package mosaiclog

// The harness vocabulary of the envelope admits opencode-v2: the writer must
// not silently drop its run events or its invocation_end fallback.

import "testing"

func TestWriter_OpenCodeV2_RunStartAndRunEndAreWritten(t *testing.T) {
	w, root := newTestWriter(t, "opencode-v2")

	w.RunStart(testRunID, `C:\work\project`)
	w.RunEnd(testRunID, OutcomeCompleted)

	events := readEvents(t, orchestratorLogPath(root, testRunID))
	if len(events) != 2 {
		t.Fatalf("got %d events, want run_start and run_end for opencode-v2", len(events))
	}
	if events[0]["event"] != "run_start" || events[1]["event"] != "run_end" {
		t.Errorf("events = %v, want run_start then run_end", events)
	}
	for _, ev := range events {
		if ev["harness"] != "opencode-v2" || ev["run_id"] != testRunID {
			t.Errorf("envelope wrong for opencode-v2: %v", ev)
		}
	}
}

func TestEnsureInvocationEnd_OpenCodeV2AppendsLikeOpenCode(t *testing.T) {
	w, root := newTestWriter(t, "opencode-v2")
	makeInvocationFolder(t, root, "Research#3")

	res := w.EnsureInvocationEnd(testRunID, InvocationEnd{AgentInstanceID: "Research#3", StatusCode: "SUCCESS", Response: "done"})

	if res != EnsureAppended {
		t.Fatalf("result = %q, want %q", res, EnsureAppended)
	}
	events := readEvents(t, invocationLogPath(root, testRunID, "Research#3"))
	if len(events) != 1 || events[0]["event"] != "invocation_end" || events[0]["harness"] != "opencode-v2" {
		t.Fatalf("events = %v, want one invocation_end with harness opencode-v2", events)
	}
	for k := range events[0] {
		if !invocationEndKeys[k] {
			t.Errorf("unexpected field %q in the fallback invocation_end", k)
		}
	}
}

func TestEnsureInvocationEnd_OpenCodeV2NeverWritesSecondEnd(t *testing.T) {
	w, root := newTestWriter(t, "opencode-v2")
	makeInvocationFolder(t, root, "Research#3")
	w.EnsureInvocationEnd(testRunID, InvocationEnd{AgentInstanceID: "Research#3"})

	res := w.EnsureInvocationEnd(testRunID, InvocationEnd{AgentInstanceID: "Research#3"})

	if res == EnsureAppended {
		t.Errorf("second call appended again, want the existing end kept")
	}
	if got := len(readEvents(t, invocationLogPath(root, testRunID, "Research#3"))); got != 1 {
		t.Errorf("got %d events, want exactly 1", got)
	}
}
