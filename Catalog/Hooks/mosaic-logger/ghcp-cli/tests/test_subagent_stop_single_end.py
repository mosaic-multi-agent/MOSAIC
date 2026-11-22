"""One invocation_end per invocation on subagentStop.

The first subagentStop that resolves to an invocation writes its invocation_end,
final assistant turn and 02_output.md. A later subagentStop resolving to the
same invocation (a re-fired stop, or a resumed cycle) writes none of these, still
logs the usage that is new into the same stream, and leaves the first output in
place. Concurrent stops for one invocation write at most one invocation_end. A
different invocation is never suppressed by another's completion.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, ConcurrentHooksCase, CorrelationCase, invocation, ts, usage_event,
)

_AGENT = "agt-stop-0001"
_INSTANCE = "Research#1"


class _StopCase(CorrelationCase):

    def started_agent(self, agent_id=_AGENT, instance=_INSTANCE):
        self.dispatch_agent(instance)
        self.start(agent_id, instance.split("#")[0])

    def end_events(self, instance=_INSTANCE):
        return [e for e in self.events(instance) if e["event"] == "invocation_end"]

    def assistant_turns(self, instance=_INSTANCE):
        return [e for e in self.events(instance)
                if e["event"] == "turn" and e.get("role") == "assistant"]


class TestRepeatedStopOfOneInvocation(_StopCase):

    def test_a_second_stop_writes_no_second_end_or_final_turn(self):
        self.started_agent()
        self.stop_agent(_AGENT, "first answer")

        self.stop_agent(_AGENT, "second answer")

        self.assertEqual(1, len(self.end_events()))
        self.assertEqual(1, len(self.assistant_turns()))
        self.assertEqual("first answer", self.end_events()[0]["response"])

    def test_a_second_stop_keeps_the_first_output_file(self):
        self.started_agent()
        self.stop_agent(_AGENT, "first answer")

        self.stop_agent(_AGENT, "second answer")

        output = self.paths.invocation_output(RUN_ID, _INSTANCE).read_text("utf-8")
        self.assertIn("first answer", output)
        self.assertNotIn("second answer", output)

    def test_a_second_stop_still_logs_new_usage_into_the_same_stream_once(self):
        self.started_agent()
        self.stop_agent(_AGENT, "first answer")
        self.append([usage_event("late-1", ts=ts(12), agent_id=_AGENT, event_id="e-late")])

        self.stop_agent(_AGENT, "second answer")
        self.stop_agent(_AGENT, "third answer")

        self.assertEqual([invocation(RUN_ID, _INSTANCE)], self.streams_of_record("late-1"))

    def test_the_first_stop_of_each_of_two_invocations_is_written(self):
        self.started_agent("agt-a", "Research#1")
        self.stop_agent("agt-a", "answer a")
        self.started_agent("agt-b", "Research#2")

        self.stop_agent("agt-b", "answer b")

        self.assertEqual(1, len(self.end_events("Research#1")))
        self.assertEqual(1, len(self.end_events("Research#2")))
        self.assertEqual("answer b", self.end_events("Research#2")[0]["response"])


class TestConcurrentStopsOfOneInvocation(ConcurrentHooksCase):

    def test_simultaneous_stops_write_one_end(self):
        self.dispatch_agent(_INSTANCE)
        self.start(_AGENT, "Research")
        payload = {"agentId": _AGENT, "response": "the answer",
                   "transcriptPath": str(self.transcript)}

        self.run_concurrently("subagentStop", [dict(payload) for _ in range(4)])

        ends = [e for e in self.events(_INSTANCE) if e["event"] == "invocation_end"]
        self.assertEqual(1, len(ends))
        turns = [e for e in self.events(_INSTANCE)
                 if e["event"] == "turn" and e.get("role") == "assistant"]
        self.assertEqual(1, len(turns))


if __name__ == "__main__":
    import unittest
    unittest.main()
