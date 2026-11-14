package logread_test

// Tests for reading the dispatch prompt of subagent-launch tool_call_start
// events.
//
// Rules under test:
//   - For a subagent-launch tool whose tool_input is an object with a string
//     "prompt", the agent instance id named in it by the structured form is
//     exposed as ToolCallStart.DispatchedInstance.
//   - Every other case (other tools, input absent / not an object, prompt absent
//     or not a string, no structured id) leaves it empty and raises no finding.
//   - No other part of tool_input reaches the domain event.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

const dispatchFixture = "../../testdata/dispatch_orchestrator.jsonl"

func TestReader_DispatchFixture_ExposesTheInstanceIdPerCall(t *testing.T) {
	want := map[string]domain.AgentInstanceID{
		"cc-agent":        "planner#3",
		"cc-task":         "reviewer#12",
		"oc-task":         "coder#7",
		"cli-agent":       "tester#4",
		"vs-run":          "writer#5",
		"vs-exec":         "runner#6",
		"bare-id":         "",
		"not-launch":      "",
		"no-input":        "",
		"input-string":    "",
		"input-array":     "",
		"input-null":      "",
		"prompt-number":   "",
		"prompt-missing":  "",
		"escaped-quotes":  "planner#8",
		"escaped-unicode": "planner#99",
		"no-id":           "",
		"last-call":       "planner#10",
	}

	events, findings, err := logread.NewReader().ReadAll(context.Background(), dispatchFixture)

	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("odd or absent tool_input must raise no finding, got %v", findings)
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d", len(events), len(want))
	}
	for _, ev := range events {
		if ev.ToolCallStart == nil {
			t.Fatalf("event %q has no ToolCallStart block", ev.RawType)
		}
		got, ok := want[ev.ToolCallStart.CallID]
		if !ok {
			t.Fatalf("unexpected call id %q", ev.ToolCallStart.CallID)
		}
		if ev.ToolCallStart.DispatchedInstance != got {
			t.Errorf("call %q: DispatchedInstance = %q, want %q", ev.ToolCallStart.CallID, ev.ToolCallStart.DispatchedInstance, got)
		}
	}
}

func TestReader_DispatchPrompt_OtherInputMembersAndPayloadsNeverReachTheEvent(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("tool_call_start", "2026-10-01T10:00:02.000Z",
			`"call_id":"c1","tool_name":"Agent","tool_input":{"description":"DESC-MK1","prompt":"TEXT-MK2 agent_instance_id: planner#3 TAIL-MK3","extra":{"x":"EXTRA-MK4"}}`),
		rmLine("tool_call_start", "2026-10-01T10:00:03.000Z",
			`"call_id":"c2","tool_name":"Read","tool_input":{"prompt":"READ-MK5","file_path":"/PATH-MK6"}`))

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if got := events[0].ToolCallStart.DispatchedInstance; got != "planner#3" {
		t.Fatalf("DispatchedInstance = %q, want planner#3", got)
	}
	for _, ev := range events {
		assertNoMarkers(t, ev, "DESC-MK1", "TEXT-MK2", "TAIL-MK3", "EXTRA-MK4", "READ-MK5", "PATH-MK6")
	}
}

func TestReader_DispatchPrompt_OnlyToolCallStartCarriesIt(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("tool_call_end", "2026-10-01T10:00:02.000Z",
			`"call_id":"c1","tool_name":"Agent","status":"success","tool_input":{"prompt":"agent_instance_id: planner#3"}`))

	if len(events) != 1 || events[0].ToolCallEnd == nil {
		t.Fatalf("events = %+v, want one tool_call_end", events)
	}
	if events[0].ToolCallStart != nil {
		t.Error("a tool_call_end must not carry a ToolCallStart block")
	}
	assertNoMarkers(t, events[0], "planner#3")
}

func TestReader_DispatchPrompt_LargePrompt_StillFindsTheId(t *testing.T) {
	prompt := strings.Repeat("filler text ", 50_000) + "\\nagent_instance_id: planner#21\\n" + strings.Repeat("more ", 1000)

	ev, findings := readOne(t, rmLine("tool_call_start", "2026-10-01T10:00:02.000Z",
		`"call_id":"c1","tool_name":"Agent","tool_input":{"prompt":"`+prompt+`"}`))

	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
	if ev.ToolCallStart == nil || ev.ToolCallStart.DispatchedInstance != "planner#21" {
		t.Errorf("ToolCallStart = %+v, want DispatchedInstance planner#21", ev.ToolCallStart)
	}
}
