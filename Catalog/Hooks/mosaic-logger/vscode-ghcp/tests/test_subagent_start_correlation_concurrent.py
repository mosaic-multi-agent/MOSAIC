"""Concurrent SubagentStart hooks, each a separate process.

Parallel dispatches are queued, then the starts run at the same time through the real
hook entry point (mosaic_logger.py as a subprocess). Every start must get its own
dispatch, no dispatch may go to two starts, and the queue must be empty afterwards.
Two same-named spawns without an agent id, started together, never share a key or id.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import ConcurrentHooksCase  # noqa: E402


class TestConcurrentStartsThroughTheHookEntryPoint(ConcurrentHooksCase):

    def run_starts(self, specs, **extra):
        self.run_concurrently([self.start_payload(a, t, **extra) for a, t in specs])

    def test_parallel_dispatches_of_one_type_each_get_their_own_id(self):
        instances = [f"Research#{n}" for n in range(1, 5)]
        for instance in instances:
            self.dispatch_agent(instance)

        self.run_starts([(f"toolu_{n}", "Research") for n in range(1, 5)])

        ids = [self.mapped_instance(f"toolu_{n}") for n in range(1, 5)]
        self.assertEqual(sorted(instances), sorted(ids))
        self.assertEqual(sorted(ids), sorted(self.instance_of(f"toolu_{n}") for n in range(1, 5)))
        self.assertEqual([], self.queue_lines())
        self.assertEqual(sorted(ids), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_repeated_rounds_never_hand_one_dispatch_to_two_starts(self):
        for round_no in range(3):
            session = f"corr-session-round-{round_no}"
            instances = [f"Research#{round_no}{n}" for n in range(3)]
            for instance in instances:
                self.dispatch_agent(instance, session_id=session)

            self.run_concurrently([
                self.start_payload(f"toolu_{round_no}_{n}", "Research", session_id=session)
                for n in range(3)])

            ids = [self.mapped_instance(f"toolu_{round_no}_{n}") for n in range(3)]
            self.assertEqual(sorted(instances), sorted(ids), f"round {round_no}")
            self.assertEqual([], self.queue_lines(session), f"round {round_no}")

    def test_parallel_dispatches_of_two_types_land_in_matching_folders(self):
        for instance in ("Research#1", "TestWriter#2", "Research#3", "TestWriter#4"):
            self.dispatch_agent(instance)

        self.run_starts([("toolu_w1", "TestWriter"), ("toolu_r1", "Research"),
                         ("toolu_w2", "TestWriter"), ("toolu_r2", "Research")])

        writers = {self.mapped_instance("toolu_w1"), self.mapped_instance("toolu_w2")}
        researchers = {self.mapped_instance("toolu_r1"), self.mapped_instance("toolu_r2")}
        self.assertEqual({"TestWriter#2", "TestWriter#4"}, writers)
        self.assertEqual({"Research#1", "Research#3"}, researchers)
        self.assertEqual([], self.queue_lines())

    def test_more_starts_than_dispatches_leave_the_surplus_on_unique_fallbacks(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("Research#2")

        self.run_starts([(f"toolu_{n}", "Research") for n in range(1, 5)])

        ids = [self.mapped_instance(f"toolu_{n}") for n in range(1, 5)]
        self.assertEqual(4, len(set(ids)))
        self.assertEqual({"Research#1", "Research#2"}, {i for i in ids if "#" in i})
        self.assertEqual([], self.queue_lines())
        self.assert_no_unknown_agent_folder()


class TestConcurrentSameNamedSpawnsWithoutAgentId(ConcurrentHooksCase):

    def test_same_named_name_only_starts_get_distinct_keys_and_instance_ids(self):
        self.run_concurrently([self.start_payload(agent_name="Research") for _ in range(3)])

        entries = self.spawn_entries()
        keys = [e["key"] for e in entries]
        instances = [e["agent_instance_id"] for e in entries]
        self.assertEqual(3, len(set(keys)), keys)
        self.assertEqual(3, len(set(instances)), instances)
        self.assertEqual(sorted(instances), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_same_named_name_only_starts_each_consume_their_own_dispatch(self):
        for instance in ("Research#1", "Research#2"):
            self.dispatch_agent(instance)

        self.run_concurrently([self.start_payload(agent_name="Research") for _ in range(2)])

        instances = sorted(e["agent_instance_id"] for e in self.spawn_entries())
        self.assertEqual(["Research#1", "Research#2"], instances)
        self.assertEqual(["Research#1", "Research#2"], self.folders())
        self.assertEqual([], self.queue_lines())


if __name__ == "__main__":
    import unittest
    unittest.main()
