package opencodev2_test

// Behavioural tests for the completion wake-up forwarding and the plugin's
// lifecycle (cleanup, resubscription), executed under node.

import (
	"reflect"
	"testing"
)

// launched returns the steps of one dispatch that started child.
func launched(session, id, agent, child string) []pluginStep {
	in := obj{"agent": agent, "description": "do the work", "prompt": "p"}
	return []pluginStep{before(beforeEvent(session, id, in), 1), after(afterEvent(session, id, child, in), 1)}
}

func withSteps(prefix []pluginStep, rest ...pluginStep) pluginScenario {
	return pluginScenario{Steps: append(append([]pluginStep(nil), prefix...), rest...)}
}

func TestPluginRuns_CompletionIsForwardedOnceOnTheCompletionBridge(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{
		Replies: map[string]string{"completion": denyReply},
		Scenario: withSteps(launched("ses_p", "call_1", "worker", "ses_c"),
			event(wakeUp("ses_p", "ses_c", "completed", "final text"), 1),
			event(wakeUp("ses_p", "ses_c", "completed", "again"), 0),
		),
	})

	completions := out.callsForPhase("completion")
	if len(completions) != 1 {
		t.Fatalf("completion bridge calls = %d, want exactly 1 per dispatch", len(completions))
	}
	assertSubset(t, obj{
		"hook_event_name": "subagent.completed", "sessionID": "ses_p", "id": "call_1", "agent": "worker",
		"childSessionID": "ses_c", "state": "completed", "text": "final text",
	}, completions[0].Payload)
	if tail := completions[0].Args[len(completions[0].Args)-2:]; !reflect.DeepEqual(tail, []string{"--phase", "completion"}) {
		t.Errorf("completion bridge arguments end %v, want --phase completion", tail)
	}
}

func TestPluginRuns_RunningWakeUpsAreSkippedAndTheFirstFinishedOneIsForwarded(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{Scenario: withSteps(launched("ses_p", "call_1", "worker", "ses_c"),
		event(wakeUp("ses_p", "ses_c", "running", "still going"), 0),
		event(wakeUp("ses_p", "ses_c", "completed", "done"), 1),
	)})

	completions := out.callsForPhase("completion")
	if len(completions) != 1 {
		t.Fatalf("completion bridge calls = %d, want 1", len(completions))
	}
	assertSubset(t, obj{"state": "completed", "text": "done"}, completions[0].Payload)
}

func TestPluginRuns_ChildRecordedFromOutputSessionIDFallbackCompletes(t *testing.T) {
	t.Parallel()
	in := dispatchInput()
	post := afterEvent("ses_p", "call_1", "ses_c", in)
	post["result"] = obj{"output": obj{"sessionID": "ses_c", "status": "running"}}
	out := runPlugin(t, pluginRun{Scenario: scenario(
		before(beforeEvent("ses_p", "call_1", in), 1), after(post, 1),
		event(wakeUp("ses_p", "ses_c", "completed", "done"), 1),
	)})

	assertForwardedCounts(t, out, 1, 1, 1)
}

func TestPluginRuns_WakeUpsThatAreNotSubagentResultsAreSkipped(t *testing.T) {
	t.Parallel()
	notSubagent := wakeUp("ses_p", "ses_c", "completed", "x")
	notSubagent["data"].(obj)["item"].(obj)["payload"].(obj)["metadata"].(obj)["source"] = "other"
	userItem := wakeUp("ses_p", "ses_c", "completed", "x")
	userItem["data"].(obj)["item"].(obj)["type"] = "user"
	otherType := wakeUp("ses_p", "ses_c", "completed", "x")
	otherType["type"] = "session.synthetic.created"
	unknownChild := wakeUp("ses_p", "ses_other", "completed", "x")

	out := runPlugin(t, pluginRun{Scenario: withSteps(launched("ses_p", "call_1", "worker", "ses_c"),
		event(notSubagent, 0), event(userItem, 0), event(otherType, 0), event(unknownChild, 0),
		event(wakeUp("ses_p", "ses_c", "completed", "real"), 1),
	)})

	completions := out.callsForPhase("completion")
	if len(completions) != 1 {
		t.Fatalf("completion bridge calls = %d, want only the real wake-up", len(completions))
	}
	assertSubset(t, obj{"text": "real"}, completions[0].Payload)
}

func TestPluginRuns_ConcurrentDispatchesCompleteUnderTheirOwnIdentity(t *testing.T) {
	t.Parallel()
	steps := append(launched("ses_p", "call_1", "worker_a", "ses_c1"), launched("ses_p", "call_2", "worker_b", "ses_c2")...)
	out := runPlugin(t, pluginRun{Scenario: withSteps(steps,
		event(wakeUp("ses_p", "ses_c2", "completed", "b done"), 1),
		event(wakeUp("ses_p", "ses_c1", "completed", "a done"), 1),
	)})

	completions := out.callsForPhase("completion")
	if len(completions) != 2 {
		t.Fatalf("completion bridge calls = %d, want 2", len(completions))
	}
	assertSubset(t, obj{"id": "call_2", "agent": "worker_b", "childSessionID": "ses_c2", "text": "b done"}, completions[0].Payload)
	assertSubset(t, obj{"id": "call_1", "agent": "worker_a", "childSessionID": "ses_c1", "text": "a done"}, completions[1].Payload)
}

func TestPluginRuns_CleanupStopsAllForwarding(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{Scenario: withSteps(launched("ses_p", "call_1", "worker", "ses_c"),
		pluginStep{Kind: "cleanup"},
		before(beforeEvent("ses_p", "call_2", dispatchInput()), 0),
		after(afterEvent("ses_p", "call_2", "ses_c2", dispatchInput()), 0),
		event(wakeUp("ses_p", "ses_c", "completed", "late"), 0),
	)})

	n := len(out.Steps)
	for i := 3; i < n; i++ {
		if out.Steps[i].Forwarded != 0 {
			t.Errorf("step %d (%s) forwarded %d calls after cleanup, want 0", i, out.Steps[i].Kind, out.Steps[i].Forwarded)
		}
	}
	if out.SubscribeCount != 1 {
		t.Errorf("subscriptions = %d, want no resubscription after cleanup", out.SubscribeCount)
	}
}

func TestPluginRuns_ResubscribesWhenTheSubscriptionEndsAndKeepsForwarding(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, pluginRun{Scenario: withSteps(launched("ses_p", "call_1", "worker", "ses_c"),
		pluginStep{Kind: "end"},
		event(wakeUp("ses_p", "ses_c", "completed", "after resubscribe"), 1),
	)})

	if out.SubscribeCount < 2 {
		t.Fatalf("subscriptions = %d, want the plugin to subscribe again after the stream ended", out.SubscribeCount)
	}
	completions := out.callsForPhase("completion")
	if len(completions) != 1 {
		t.Fatalf("completion bridge calls = %d, want 1 on the new subscription", len(completions))
	}
	assertSubset(t, obj{"text": "after resubscribe"}, completions[0].Payload)
}
