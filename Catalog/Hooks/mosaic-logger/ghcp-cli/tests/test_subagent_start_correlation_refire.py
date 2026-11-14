"""A subagentStart/subagentStop pair re-fired for an agent that already ran.

The CLI can fire the lifecycle again for a finished agent. The logger continues
the existing invocation: it keeps the mapping and the agent-to-run association,
consumes no pending dispatch, takes no remembered spawn call id, writes nothing
new into the invocation folder, and never logs the agent's usage a second time;
usage that is new is logged into the same stream. A new agent dispatched
meanwhile keeps its own instance id.

Driven through the hook entry point (mosaic_logger.dispatch) with harness-shaped
payloads.
"""

import importlib
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, SESSION_ID, CorrelationCase, invocation, ts, usage_event,
)

_OLD_AGENT = "agt-old-0001"
_NEW_AGENT = "agt-new-0002"
_OLD = "Research#1"
_NEW = "TestWriter#2"


class _RefireCase(CorrelationCase):

    def old_usage(self, record_id, second):
        return usage_event(record_id, ts=ts(second), agent_id=_OLD_AGENT,
                           event_id="e-" + record_id)

    def first_cycle(self):
        """Dispatch, start and stop the old agent once; its first usage call is m1."""
        self.write([self.old_usage("m1", 10)])
        self.dispatch_agent(_OLD)
        self.start(_OLD_AGENT, "Research")
        self.stop_agent(_OLD_AGENT, "first answer")

    def refire(self):
        time.sleep(0.05)  # a rewritten state file would carry a later timestamp
        self.start(_OLD_AGENT, "Research")
        self.stop_agent(_OLD_AGENT, "again")

    def end_events(self, instance=_OLD):
        return [e for e in self.events(instance) if e["event"] == "invocation_end"]


class TestRefiredStartOfFinishedAgent(_RefireCase):

    def test_mapping_is_not_overwritten(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire()

        self.assertEqual(before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))

    def test_agent_to_run_association_is_not_overwritten(self):
        self.first_cycle()
        path = self.paths.agent_run_entry(_OLD_AGENT)
        before = path.read_bytes()

        self.refire()

        self.assertEqual(before, path.read_bytes())

    def test_remembered_spawn_call_id_is_not_taken_by_the_refire(self):
        self.first_cycle()
        spawn_key = importlib.import_module("mosaic_logger_spawn_key")
        # A second dispatch of the old id queues it again and remembers its spawn
        # call; a re-fire that consumed it (or took its spawn call) would show here.
        self.dispatch_agent(_OLD, use_id="toolu_later")

        self.refire()

        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))
        self.assertEqual([_OLD], self.queued_ids())
        self.assertEqual([_OLD], self.folders())
        self.assertEqual("toolu_later", spawn_key.take_spawn_call(
            self.paths, RUN_ID, SESSION_ID, _OLD))

    def test_pending_dispatch_of_another_agent_is_not_consumed(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.refire()

        self.assertEqual([_NEW], self.queued_ids())

    def test_pending_dispatch_of_the_same_type_is_not_consumed(self):
        self.first_cycle()
        self.dispatch_agent("Research#7")

        self.refire()

        self.assertEqual(["Research#7"], self.queued_ids())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))

    def test_new_agent_still_gets_its_own_id_after_the_refire(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        self.refire()

        self.start(_NEW_AGENT, "TestWriter")

        self.assertEqual(_NEW, self.mapped_instance(_NEW_AGENT))
        self.assertEqual([], self.queued_ids())
        self.assertEqual(sorted([_OLD, _NEW]), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_refire_without_a_pending_dispatch_creates_no_new_folder(self):
        self.first_cycle()

        self.refire()

        self.assertEqual([_OLD], self.folders())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))
        self.assert_no_unknown_agent_folder()


class TestRefireUsage(_RefireCase):

    def test_old_agent_usage_appears_exactly_once(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.refire()
        self.start(_NEW_AGENT, "TestWriter")

        self.assertEqual(["m1"], self.record_ids())
        self.assertEqual([invocation(RUN_ID, _OLD)], self.streams_of_record("m1"))

    def test_usage_that_is_new_at_the_refire_goes_into_the_old_stream(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        self.append([self.old_usage("m2", 11)])

        self.refire()

        self.assertEqual(["m1", "m2"], self.record_ids())
        self.assertEqual([invocation(RUN_ID, _OLD)], self.streams_of_record("m1"))
        self.assertEqual([invocation(RUN_ID, _OLD)], self.streams_of_record("m2"))

    def test_the_new_agents_usage_is_not_attributed_to_the_old_one(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        self.refire()
        self.start(_NEW_AGENT, "TestWriter")
        self.append([usage_event("n1", ts=ts(12), agent_id=_NEW_AGENT, event_id="e-n1")])

        self.stop_agent(_NEW_AGENT, "new answer")

        self.assertEqual([invocation(RUN_ID, _NEW)], self.streams_of_record("n1"))
        self.assertEqual([invocation(RUN_ID, _OLD)], self.streams_of_record("m1"))


class TestRefireWritesNothingIntoTheFolder(_RefireCase):

    def test_refire_writes_no_second_start_turn_or_input(self):
        self.first_cycle()
        input_path = self.paths.invocation_input(RUN_ID, _OLD)
        input_before = input_path.read_bytes()

        self.refire()

        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        user_turns = [e for e in self.events(_OLD)
                      if e["event"] == "turn" and e.get("role") == "user"]
        self.assertEqual(1, len(user_turns))
        self.assertEqual(input_before, input_path.read_bytes())
        self.assertEqual([_OLD], self.folders())

    def test_refire_writes_no_second_end_or_final_turn_and_keeps_the_output(self):
        self.first_cycle()

        self.refire()

        ends = self.end_events()
        self.assertEqual(1, len(ends))
        self.assertEqual("first answer", ends[0]["response"])
        final_turns = [e for e in self.events(_OLD)
                       if e["event"] == "turn" and e.get("role") == "assistant"]
        self.assertEqual(1, len(final_turns))
        output = self.paths.invocation_output(RUN_ID, _OLD).read_text("utf-8")
        self.assertIn("first answer", output)
        self.assertNotIn("again", output)
        self.assertEqual([_OLD], self.folders())
        self.assert_no_unknown_agent_folder()

    def test_second_refire_changes_nothing_further(self):
        self.first_cycle()
        self.refire()
        events_before = self.events(_OLD)
        mapping_before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire()

        self.assertEqual(events_before, self.events(_OLD))
        self.assertEqual(mapping_before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual([_OLD], self.folders())


class TestRefireOfFallbackIdAgent(_RefireCase):

    def test_refire_of_an_agent_that_got_a_fallback_id_continues_it(self):
        self.start(_OLD_AGENT, "Reviewer")
        self.stop_agent(_OLD_AGENT, "first answer")
        instance = self.mapped_instance(_OLD_AGENT)
        before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire()

        self.assertEqual(before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual([instance], self.folders())
        self.assertEqual(1, self.names(instance).count("invocation_start"))
        self.assert_no_unknown_agent_folder()


class TestRefireWithoutAgentType(_RefireCase):
    """The mapping check comes before any type handling."""

    def assert_old_invocation_continued(self):
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        self.assert_no_unknown_agent_folder()

    def test_refire_with_absent_agent_type_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.start(_OLD_AGENT)

        self.assert_old_invocation_continued()
        self.assertEqual([_NEW], self.queued_ids())

    def test_refire_with_empty_agent_type_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.start(_OLD_AGENT, agent_type="", agent_name="")

        self.assert_old_invocation_continued()
        self.assertEqual([_NEW], self.queued_ids())

    def test_refire_with_only_an_agent_name_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent("Planner#9")

        self.start(_OLD_AGENT, agent_name="Planner")

        self.assert_old_invocation_continued()
        self.assertEqual(["Planner#9"], self.queued_ids())


class TestRefireWhenTheSessionWasBoundByTheDispatch(_RefireCase):
    """The first start learns its run from the dispatch that bound the session;
    the re-fire must still find the mapping written under that run."""

    bind_run = False

    def test_refire_finds_the_mapping_under_the_dispatched_run(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire()

        self.assertEqual(before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual([_NEW], self.queued_ids())
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(["m1"], self.record_ids())
        self.assertEqual([], self.folders("unknown-run"))


if __name__ == "__main__":
    import unittest
    unittest.main()
