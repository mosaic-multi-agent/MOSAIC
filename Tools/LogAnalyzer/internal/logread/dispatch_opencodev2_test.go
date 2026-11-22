package logread_test

// Tests for the dispatched instance of opencode-v2 tool_call_start events.
//
// Rules under test:
//   - A "subagent" tool call whose tool_input has a string "prompt" exposes the
//     structured agent instance id of the prompt as DispatchedInstance.
//   - A re-call of an existing child by session id (prompt without an id) and
//     other tools, even when their input mentions an id, expose none.
//   - Other harnesses' launch tools keep working in the same stream.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

const openCodeV2Fixture = "../../testdata/opencodev2.jsonl"

func TestReader_OpenCodeV2Fixture_SubagentDispatchExposesTheInstanceId(t *testing.T) {
	want := map[string]domain.AgentInstanceID{
		"v2-sub":    "coder#7",
		"v2-recall": "",
		"v2-shell":  "",
	}

	events, findings, err := logread.NewReader().ReadAll(context.Background(), openCodeV2Fixture)

	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none for an opencode-v2 log", findings)
	}
	seen := 0
	for _, ev := range events {
		if ev.ToolCallStart == nil {
			continue
		}
		seen++
		id := ev.ToolCallStart.CallID
		if got := ev.ToolCallStart.DispatchedInstance; got != want[id] {
			t.Errorf("call %q: DispatchedInstance = %q, want %q", id, got, want[id])
		}
		if ev.Harness != domain.HarnessOpenCodeV2 {
			t.Errorf("call %q: Harness = %q, want opencode-v2", id, ev.Harness)
		}
	}
	if seen != len(want) {
		t.Errorf("saw %d tool_call_start events, want %d", seen, len(want))
	}
}

func TestReader_OpenCodeV2SubagentCallWithoutPrompt_ExposesNoInstance(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("tool_call_start", "2026-10-01T10:00:02.000Z",
			`"call_id":"c1","tool_name":"subagent","tool_input":{"agent":"coder","description":"implement"}`))

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if got := events[0].ToolCallStart.DispatchedInstance; got != "" {
		t.Errorf("DispatchedInstance = %q, want none for a call without a prompt", got)
	}
}

func TestReader_OpenCodeV2SubagentDispatch_OtherHarnessLaunchToolsStillWork(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("tool_call_start", "2026-10-01T10:00:02.000Z",
			`"call_id":"c1","tool_name":"Agent","tool_input":{"prompt":"agent_instance_id: planner#3"}`),
		rmLine("tool_call_start", "2026-10-01T10:00:03.000Z",
			`"call_id":"c2","tool_name":"subagent","tool_input":{"prompt":"agent_instance_id: coder#4"}`))

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if got := events[0].ToolCallStart.DispatchedInstance; got != "planner#3" {
		t.Errorf("Agent dispatch: DispatchedInstance = %q, want planner#3", got)
	}
	if got := events[1].ToolCallStart.DispatchedInstance; got != "coder#4" {
		t.Errorf("subagent dispatch: DispatchedInstance = %q, want coder#4", got)
	}
}
