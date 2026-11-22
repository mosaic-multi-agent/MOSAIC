package opencodev2_test

// Behavioural tests for the generated interceptor plugin, executed under node
// with a fake host and a fake interceptor (see plugin_exec_helpers_test.go).
// They cover which calls are forwarded, how a reply is applied, and which
// payload and bridge each phase uses.

import (
	"reflect"
	"strings"
	"testing"
)

const denyReply = `{"decision":"deny","reason":"blocked by policy"}`

type obj = map[string]any

func dispatchInput() obj {
	return obj{"agent": "worker", "description": "do the work", "prompt": "the prompt"}
}

// beforeEvent builds an execute.before hook event for the subagent tool.
func beforeEvent(session, id string, input obj) obj {
	return obj{"tool": "subagent", "sessionID": session, "agent": "orchestrator", "messageID": "msg_1", "id": id, "input": input}
}

// afterEvent builds an execute.after hook event that launched child.
func afterEvent(session, id, child string, input obj) obj {
	return obj{
		"tool": "subagent", "sessionID": session, "agent": "orchestrator", "messageID": "msg_1", "id": id, "input": input,
		"status": "completed",
		"result": obj{
			"output":   obj{"sessionID": child, "status": "running", "output": "launched"},
			"metadata": obj{"sessionID": child, "status": "running"},
		},
	}
}

// wakeUp builds a session.inbox.enqueued event delivering a child's result.
func wakeUp(parent, child, state, text string) obj {
	return obj{"type": "session.inbox.enqueued", "data": obj{
		"sessionID": parent, "inboxID": "inbox_" + child + state,
		"item": obj{"type": "synthetic", "payload": obj{
			"text":     text,
			"metadata": obj{"source": "subagent", "childID": child, "state": state},
		}},
	}}
}

func before(event obj, forwards int) pluginStep {
	return pluginStep{Kind: "before", Event: event, Forwards: forwards}
}
func after(event obj, forwards int) pluginStep {
	return pluginStep{Kind: "after", Event: event, Forwards: forwards}
}
func event(e obj, forwards int) pluginStep {
	return pluginStep{Kind: "event", Event: e, Forwards: forwards}
}

func scenario(steps ...pluginStep) pluginScenario {
	if steps == nil {
		steps = []pluginStep{} // marshal as [] so the driver can iterate
	}
	return pluginScenario{Steps: steps}
}

func assertForwardedCounts(t *testing.T, out pluginOutcome, want ...int) {
	t.Helper()
	if len(out.Steps) != len(want) {
		t.Fatalf("driver ran %d steps, want %d", len(out.Steps), len(want))
	}
	for i, w := range want {
		if out.Steps[i].Forwarded != w {
			t.Errorf("step %d (%s) forwarded %d calls, want %d", i, out.Steps[i].Kind, out.Steps[i].Forwarded, w)
		}
	}
}

func assertThrew(t *testing.T, step stepResult, want string) {
	t.Helper()
	if step.Threw == nil || *step.Threw != want {
		got := "<no throw>"
		if step.Threw != nil {
			got = *step.Threw
		}
		t.Errorf("hook threw %q, want %q", got, want)
	}
}

func assertNoThrow(t *testing.T, step stepResult) {
	t.Helper()
	if step.Threw != nil {
		t.Errorf("hook threw %q, want it to resolve without error", *step.Threw)
	}
}

func assertSubset(t *testing.T, want, got obj) {
	t.Helper()
	for k, w := range want {
		if g, ok := got[k]; !ok || !reflect.DeepEqual(w, g) {
			t.Errorf("payload[%q] = %v, want %v (payload %v)", k, g, w, got)
		}
	}
}

func TestPluginRuns_SetupRegistersBothHooksAndSubscribesWithoutIO(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{Scenario: scenario()})

	if out.ID != "mosaic-interceptor" {
		t.Errorf("plugin id = %q, want mosaic-interceptor", out.ID)
	}
	if out.SetupError != "" {
		t.Errorf("setup failed: %s", out.SetupError)
	}
	if !reflect.DeepEqual(sortedCopy(out.Hooks), []string{"execute.after", "execute.before"}) {
		t.Errorf("registered hooks = %v, want execute.before and execute.after", out.Hooks)
	}
	if out.SubscribeCount != 1 {
		t.Errorf("event subscriptions = %d, want 1", out.SubscribeCount)
	}
	if out.LogAfterSetup != 0 {
		t.Errorf("setup started %d interceptor processes, want none before any call", out.LogAfterSetup)
	}
}

func TestPluginRuns_ThrowingHookRegistrationDoesNotStopTheOthers(t *testing.T) {
	t.Parallel()
	sc := scenario(before(beforeEvent("ses_p", "call_1", dispatchInput()), 0), after(afterEvent("ses_p", "call_1", "ses_c", dispatchInput()), 1))
	sc.FailHooks = []string{"execute.before"}
	out := runPlugin(t, pluginRun{Scenario: sc})

	if out.SetupError != "" {
		t.Errorf("setup rejected: %s", out.SetupError)
	}
	assertForwardedCounts(t, out, 0, 1)
}

func TestPluginRuns_SetupToleratesAHostWithoutHooksOrEvents(t *testing.T) {
	t.Parallel()
	sc := scenario()
	sc.NoToolHook = true
	sc.NoSubscribe = true
	out := runPlugin(t, pluginRun{Scenario: sc})

	if out.SetupError != "" {
		t.Errorf("setup rejected on a bare host: %s", out.SetupError)
	}
}

func TestPluginRuns_ForwardsADispatchOnThePreBridgeWithWireFields(t *testing.T) {
	t.Parallel()
	ev := beforeEvent("ses_p", "call_1", dispatchInput())
	out := runPlugin(t, pluginRun{Scenario: scenario(before(ev, 1))})

	assertForwardedCounts(t, out, 1)
	assertNoThrow(t, out.Steps[0])
	call := out.Calls[0]
	assertSubset(t, obj{
		"hook_event_name": "execute.before", "tool": "subagent", "sessionID": "ses_p", "agent": "orchestrator",
		"messageID": "msg_1", "id": "call_1", "input": dispatchInput(),
	}, call.Payload)
	if call.Phase != "pre" {
		t.Errorf("dispatch went to the %q bridge, want pre", call.Phase)
	}
	tail := call.Args[len(call.Args)-6:]
	wantArgs := []string{"--control-dir", out.ControlDir, "--harness", "opencode-v2", "--phase", "pre"}
	if !reflect.DeepEqual(tail, wantArgs) {
		t.Errorf("bridge arguments end %v, want %v", tail, wantArgs)
	}
	if !reflect.DeepEqual(out.Steps[0].Event["input"], obj(dispatchInput())) {
		t.Errorf("an empty reply changed the call input to %v", out.Steps[0].Event["input"])
	}
}

func TestPluginRuns_DeniedDispatchIsRejectedWithTheReason(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{
		Replies:  map[string]string{"pre": denyReply},
		Scenario: scenario(before(beforeEvent("ses_p", "call_1", dispatchInput()), 1)),
	})

	assertThrew(t, out.Steps[0], "blocked by policy")
}

func TestPluginRuns_DenyWithoutAReasonUsesTheDefaultMessage(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{"absent": `{"decision":"deny"}`, "empty": `{"decision":"deny","reason":""}`} {
		reply := reply
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := runPlugin(t, pluginRun{
				Replies:  map[string]string{"pre": reply},
				Scenario: scenario(before(beforeEvent("ses_p", "call_1", dispatchInput()), 1)),
			})
			assertThrew(t, out.Steps[0], "This operation cannot proceed.")
		})
	}
}

func TestPluginRuns_AllowWithUpdatedInputAssignsEachKeyOntoTheInput(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{
		Replies:  map[string]string{"pre": `{"decision":"allow","updatedInput":{"prompt":"rewritten","extra":"x"}}`},
		Scenario: scenario(before(beforeEvent("ses_p", "call_1", dispatchInput()), 1)),
	})

	assertNoThrow(t, out.Steps[0])
	want := obj{"agent": "worker", "description": "do the work", "prompt": "rewritten", "extra": "x"}
	if got := out.Steps[0].Event["input"]; !reflect.DeepEqual(got, want) {
		t.Errorf("input after the hook = %v, want %v", got, want)
	}
}

func TestPluginRuns_RepliesThatDoNotDecideLeaveTheCallUnchanged(t *testing.T) {
	t.Parallel()
	cases := map[string]pluginRun{
		"empty object":      {Replies: map[string]string{"pre": `{}`}},
		"allow no update":   {Replies: map[string]string{"pre": `{"decision":"allow"}`}},
		"no stdout":         {Replies: map[string]string{}},
		"non-JSON stdout":   {Replies: map[string]string{"pre": "not json at all"}},
		"non-zero exit":     {Replies: map[string]string{"pre": denyReply}, Exits: map[string]int{"pre": 3}},
		"unknown decision":  {Replies: map[string]string{"pre": `{"decision":"maybe","reason":"x"}`}},
		"JSON array stdout": {Replies: map[string]string{"pre": `[1,2]`}},
	}
	for name, run := range cases {
		run := run
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			run.Scenario = scenario(before(beforeEvent("ses_p", "call_1", dispatchInput()), 1))
			out := runPlugin(t, run)
			assertNoThrow(t, out.Steps[0])
			if got := out.Steps[0].Event["input"]; !reflect.DeepEqual(got, obj(dispatchInput())) {
				t.Errorf("input after the hook = %v, want it unchanged", got)
			}
		})
	}
}

func TestPluginRuns_CallsThatAreNotDispatchesAreNotForwarded(t *testing.T) {
	t.Parallel()
	other := beforeEvent("ses_p", "call_1", obj{"command": "ls"})
	other["tool"] = "bash"
	reCall := beforeEvent("ses_p", "call_2", obj{"sessionID": "ses_c", "prompt": "continue"})
	emptySession := beforeEvent("ses_p", "call_3", obj{"agent": "worker", "prompt": "p", "sessionID": ""})
	out := runPlugin(t, pluginRun{
		Replies:  map[string]string{"pre": denyReply},
		Scenario: scenario(before(other, 0), before(reCall, 0), before(emptySession, 1)),
	})

	assertForwardedCounts(t, out, 0, 0, 1)
	assertNoThrow(t, out.Steps[0])
	assertNoThrow(t, out.Steps[1])
	assertThrew(t, out.Steps[2], "blocked by policy")
}

func TestPluginRuns_ReCallsAreNotForwardedEvenWhenTheReplyWouldDenyThem(t *testing.T) {
	t.Parallel()
	reCall := beforeEvent("ses_p", "call_2", obj{"sessionID": "ses_c", "agent": "worker", "prompt": "continue"})
	out := runPlugin(t, pluginRun{
		Replies:  map[string]string{"pre": denyReply},
		Scenario: scenario(before(reCall, 0), after(afterEvent("ses_p", "call_2", "ses_c", reCall["input"].(obj)), 0)),
	})

	assertForwardedCounts(t, out, 0, 0)
	assertNoThrow(t, out.Steps[0])
	if len(out.Calls) != 0 {
		t.Errorf("a re-call reached the interceptor: %v", out.Calls)
	}
}

func TestPluginRuns_DispatchesFromChildSessionsAreNotForwarded(t *testing.T) {
	t.Parallel()
	childFromMetadata := afterEvent("ses_p", "call_1", "ses_c1", dispatchInput())
	childFromOutput := afterEvent("ses_p", "call_2", "ses_c2", dispatchInput())
	childFromOutput["result"] = obj{"output": obj{"sessionID": "ses_c2", "status": "running"}}
	created := obj{"type": "session.created", "data": obj{"sessionID": "ses_c3", "parentID": "ses_p"}}
	rootCreated := obj{"type": "session.created", "data": obj{"sessionID": "ses_r", "parentID": ""}}

	out := runPlugin(t, pluginRun{
		Scenario: scenario(
			before(beforeEvent("ses_p", "call_1", dispatchInput()), 1), after(childFromMetadata, 1),
			before(beforeEvent("ses_p", "call_2", dispatchInput()), 1), after(childFromOutput, 1),
			event(created, 0), event(rootCreated, 0),
			before(beforeEvent("ses_c1", "call_10", dispatchInput()), 0),
			before(beforeEvent("ses_c2", "call_11", dispatchInput()), 0),
			before(beforeEvent("ses_c3", "call_12", dispatchInput()), 0),
			before(beforeEvent("ses_r", "call_13", dispatchInput()), 1),
			before(beforeEvent("ses_p", "call_14", dispatchInput()), 1),
		),
	})

	assertForwardedCounts(t, out, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 1)
}

func TestPluginRuns_PostIsForwardedOnThePostBridgeAndItsReplyIsIgnored(t *testing.T) {
	t.Parallel()
	pre := beforeEvent("ses_p", "call_1", dispatchInput())
	post := afterEvent("ses_p", "call_1", "ses_c", dispatchInput())
	out := runPlugin(t, pluginRun{
		Replies:  map[string]string{"post": denyReply},
		Scenario: scenario(before(pre, 1), after(post, 1)),
	})

	assertNoThrow(t, out.Steps[1])
	if !reflect.DeepEqual(out.Steps[1].Event, post) {
		t.Errorf("the after event was changed: %v, want %v", out.Steps[1].Event, post)
	}
	posts := out.callsForPhase("post")
	if len(posts) != 1 {
		t.Fatalf("post bridge calls = %d, want 1", len(posts))
	}
	assertSubset(t, obj{
		"hook_event_name": "execute.after", "tool": "subagent", "sessionID": "ses_p", "id": "call_1",
		"status": "completed", "result": post["result"], "input": dispatchInput(),
	}, posts[0].Payload)
	if tail := posts[0].Args[len(posts[0].Args)-2:]; !reflect.DeepEqual(tail, []string{"--phase", "post"}) {
		t.Errorf("post bridge arguments end %v, want --phase post", tail)
	}
}

func TestPluginRuns_AfterHooksOfOtherToolsAreNotForwarded(t *testing.T) {
	t.Parallel()
	post := afterEvent("ses_p", "call_9", "ses_c", dispatchInput())
	post["tool"] = "bash"
	out := runPlugin(t, pluginRun{Scenario: scenario(after(post, 0))})

	assertForwardedCounts(t, out, 0)
}

func TestPluginRuns_ErrorResultsAreForwardedAsPostAndRecordNoChild(t *testing.T) {
	t.Parallel()
	post := obj{
		"tool": "subagent", "sessionID": "ses_p", "agent": "orchestrator", "messageID": "msg_1", "id": "call_1",
		"input": dispatchInput(), "status": "error", "error": obj{"_tag": "Tool.Error", "message": "failed"},
	}
	out := runPlugin(t, pluginRun{Scenario: scenario(
		before(beforeEvent("ses_p", "call_1", dispatchInput()), 1), after(post, 1),
		event(wakeUp("ses_p", "ses_c", "completed", "T"), 0),
	)})

	assertSubset(t, obj{"status": "error", "error": post["error"]}, out.callsForPhase("post")[0].Payload)
	if n := len(out.callsForPhase("completion")); n != 0 {
		t.Errorf("completion forwarded %d times for a child that was never recorded, want 0", n)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if strings.Compare(out[j], out[i]) < 0 {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
