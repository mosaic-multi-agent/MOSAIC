"""Edge cases of subagentStart identity resolution.

- An explicit agent_instance_id line in the task message loses to a pending
  dispatch of the same type.
- A start whose type matches nothing pending keeps every pending dispatch.
- The adapter source no longer contains the 'unknown-agent' literal.

Driven through the hook entry point (mosaic_logger.dispatch).
"""

import json
import os
import pathlib
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, SESSION_ID, CorrelationCase, dispatch_prompt, fallback_pattern,
)
from locked_append_support import hold_lock  # noqa: E402


class TestExplicitLineAgainstPendingDispatch(CorrelationCase):

    def test_pending_dispatch_of_the_same_type_wins_over_an_explicit_line(self):
        self.dispatch_agent("Research#1")
        self.start("agt-a", "Research", prompt=dispatch_prompt("Research#9"))

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual([], self.queued_ids())
        self.assertNotIn("Research#9", self.folders())


class TestStartsThatMatchNothing(CorrelationCase):

    def test_a_different_type_leaves_all_pending_dispatches_in_order(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("Planner#2")

        self.start("agt-a", "Reviewer")

        self.assertRegex(self.mapped_instance("agt-a"), fallback_pattern("Reviewer"))
        self.assertEqual(["Research#1", "Planner#2"], self.queued_ids())

    def test_two_starts_of_one_type_for_one_dispatch_use_the_dispatch_once(self):
        self.dispatch_agent("Research#1")

        self.start("agt-a", "Research")
        self.start("agt-b", "Research")

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertRegex(self.mapped_instance("agt-b"), fallback_pattern("Research"))
        self.assertEqual(1, self.names("Research#1").count("invocation_start"))


class TestStartWhileTheQueueLockIsHeld(CorrelationCase):

    def test_a_start_falls_back_to_a_unique_id_and_consumes_nothing(self):
        self.dispatch_agent("Research#1")
        hold_lock(self.paths.pending_dispatch_entry(RUN_ID, SESSION_ID))

        self.start("agt-a", "Research")

        self.assertRegex(self.mapped_instance("agt-a"), fallback_pattern("Research"))
        self.assertEqual(["Research#1"], self.queued_ids())
        self.assert_no_unknown_agent_folder()


class TestSpawnCallIdOfOtherInstances(CorrelationCase):

    def _start_event(self, instance):
        return [e for e in self.events(instance) if e["event"] == "invocation_start"][0]

    def test_a_fallback_start_does_not_take_the_spawn_call_of_a_dispatched_agent(self):
        self.dispatch_agent("Planner#2", use_id="toolu_planner")

        self.start("agt-a", "Reviewer")

        start = self._start_event(self.mapped_instance("agt-a"))
        self.assertNotEqual("toolu_planner", start.get("spawn_call_id"))

    def test_an_explicit_line_start_does_not_take_the_spawn_call_of_a_dispatched_agent(self):
        self.dispatch_agent("Planner#2", use_id="toolu_planner")

        self.start("agt-a", "Reviewer", prompt=dispatch_prompt("Reviewer#5"))

        start = self._start_event(self.mapped_instance("agt-a"))
        self.assertNotEqual("toolu_planner", start.get("spawn_call_id"))


class TestDispatchCaptureStoresNoType(CorrelationCase):

    def test_the_queued_entry_carries_no_agent_type_key(self):
        self.dispatch_agent("Research#1")

        entries = [json.loads(ln) for ln in self.queue_lines()]

        self.assertEqual(["Research#1"], [e["agent_instance_id"] for e in entries])
        self.assertNotIn("agent_type", entries[0])


class TestAdapterSourceHasNoUnknownAgentLiteral(unittest.TestCase):

    def test_no_adapter_module_contains_the_literal(self):
        adapter_dir = pathlib.Path(__file__).resolve().parent.parent
        offenders = [p.name for p in adapter_dir.glob("mosaic_logger*.py")
                     if "unknown-agent" in p.read_text("utf-8")]

        self.assertEqual([], offenders)


if __name__ == "__main__":
    unittest.main()
