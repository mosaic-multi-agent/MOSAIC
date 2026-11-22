"""A SubagentStart/SubagentStop pair re-fired for an agent that already ran.

VS Code can fire the lifecycle again for a finished agent. The logger continues the
existing invocation: it keeps the agent map entry and the spawn index entry (and the
stopped/claimed markers beside it), consumes no pending dispatch, writes nothing new
into the invocation folder and adds no second invocation_end. A new agent dispatched
meanwhile keeps its own instance id.

Driven through the hook entry point (mosaic_logger.dispatch) with VS Code-shaped
payloads.
"""

import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import RUN_ID, SESSION_ID, SUFFIX, CorrelationCase  # noqa: E402

_OLD_CALL = "toolu_old0001"
_NEW_CALL = "toolu_new0002"
_OLD = "Research#1"
_NEW = "TestWriter#2"


class _RefireCase(CorrelationCase):

    def first_cycle(self, completed=True):
        """Dispatch, start and stop the old agent; its spawning PostToolUse completes it."""
        self.dispatch_agent(_OLD, call_id=_OLD_CALL)
        self.start(_OLD_CALL + SUFFIX, "Research")
        self.stop_agent(_OLD_CALL + SUFFIX, "Research")
        if completed:
            self.post_dispatch(_OLD_CALL, "first answer")

    def refire(self):
        time.sleep(0.05)  # a rewritten state file would carry a later timestamp
        self.start(_OLD_CALL + SUFFIX, "Research")
        self.stop_agent(_OLD_CALL + SUFFIX, "Research")

    def spawn_file(self, extension="json"):
        return self.paths.subagent_index_file(_OLD_CALL, extension)


class TestRefiredStartOfFinishedAgent(_RefireCase):

    def test_agent_map_entry_is_not_overwritten(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        before = self.mapping_path(_OLD_CALL).read_bytes()

        self.refire()

        self.assertEqual(before, self.mapping_path(_OLD_CALL).read_bytes())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_CALL))

    def test_spawn_index_entry_is_not_overwritten(self):
        self.first_cycle()
        before = self.spawn_file().read_bytes()

        self.refire()

        self.assertEqual(before, self.spawn_file().read_bytes())
        self.assertEqual(_OLD, self.instance_of(_OLD_CALL))

    def test_completion_markers_survive_the_refire(self):
        self.first_cycle()

        self.refire()

        self.assertTrue(self.spawn_file("stopped").exists())
        self.assertTrue(self.spawn_file("claimed").exists())

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
        self.assertEqual(_OLD, self.mapped_instance(_OLD_CALL))

    def test_new_agent_still_gets_its_own_id_after_the_refire(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        self.refire()

        self.start(_NEW_CALL + SUFFIX, "TestWriter")

        self.assertEqual(_NEW, self.mapped_instance(_NEW_CALL))
        self.assertEqual(_NEW, self.instance_of(_NEW_CALL))
        self.assertEqual([], self.queued_ids())
        self.assertEqual(sorted([_OLD, _NEW]), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_refire_without_a_pending_dispatch_creates_no_new_folder(self):
        self.first_cycle()

        self.refire()

        self.assertEqual([_OLD], self.folders())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_CALL))
        self.assert_no_unknown_agent_folder()

    def test_refire_with_a_different_suffix_still_finds_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)
        time.sleep(0.05)

        self.start(_OLD_CALL + "__vscode-1793098999999", "Research")

        self.assertEqual([_NEW], self.queued_ids())
        self.assertEqual([_OLD], self.folders())


class TestRefireWritesNothingIntoTheFolder(_RefireCase):

    def test_refire_writes_no_second_start_turn_or_input(self):
        self.first_cycle()
        input_path = self.paths.invocation_input(RUN_ID, _OLD)
        input_before = input_path.read_bytes()
        events_before = self.events(_OLD)

        self.refire()

        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        self.assertEqual(events_before, self.events(_OLD))
        self.assertEqual(input_before, input_path.read_bytes())
        self.assertEqual([_OLD], self.folders())

    def test_refire_and_a_second_completion_add_no_second_end(self):
        self.first_cycle()

        self.refire()
        self.post_dispatch(_OLD_CALL, "again")

        ends = self.ends(_OLD)
        self.assertEqual(1, len(ends))
        self.assertEqual("first answer", ends[0]["response"])
        output = self.paths.invocation_output(RUN_ID, _OLD).read_text("utf-8")
        self.assertIn("first answer", output)
        self.assertNotIn("again", output)

    def test_refire_before_completion_continues_and_ends_once(self):
        self.first_cycle(completed=False)
        entry_before = self.spawn_file().read_bytes()

        self.refire()
        self.post_dispatch(_OLD_CALL, "the answer")

        self.assertEqual(entry_before, self.spawn_file().read_bytes())
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        ends = self.ends(_OLD)
        self.assertEqual(1, len(ends))
        self.assertEqual("the answer", ends[0]["response"])

    def test_second_refire_changes_nothing_further(self):
        self.first_cycle()
        self.refire()
        events_before = self.events(_OLD)
        mapping_before = self.mapping_path(_OLD_CALL).read_bytes()
        spawn_before = self.spawn_file().read_bytes()

        self.refire()

        self.assertEqual(events_before, self.events(_OLD))
        self.assertEqual(mapping_before, self.mapping_path(_OLD_CALL).read_bytes())
        self.assertEqual(spawn_before, self.spawn_file().read_bytes())
        self.assertEqual([_OLD], self.folders())

    def test_orchestrator_stop_after_the_refire_adds_no_end(self):
        self.first_cycle()
        self.refire()

        self.orchestrator_stop()

        self.assertEqual(1, len(self.ends(_OLD)))


class TestRefireOrchestratorSideEffectsStillRun(_RefireCase):

    def test_orchestrator_raw_export_is_refreshed_by_the_refire(self):
        self.first_cycle()
        raw = self.paths.orchestrator_raw(RUN_ID, SESSION_ID)
        self.assertTrue(raw.exists(), "the first start must export the orchestrator transcript")
        raw.unlink()

        self.refire()

        self.assertTrue(raw.exists())
        self.assertEqual([_OLD], self.folders())


class TestRefireOfFallbackIdAgent(_RefireCase):

    def test_refire_of_an_agent_that_got_a_fallback_id_continues_it(self):
        self.start(_OLD_CALL + SUFFIX, "Reviewer")
        self.stop_agent(_OLD_CALL + SUFFIX, "Reviewer")
        instance = self.mapped_instance(_OLD_CALL)
        mapping_before = self.mapping_path(_OLD_CALL).read_bytes()
        spawn_before = self.spawn_file().read_bytes()

        self.refire()

        self.assertEqual(mapping_before, self.mapping_path(_OLD_CALL).read_bytes())
        self.assertEqual(spawn_before, self.spawn_file().read_bytes())
        self.assertEqual([instance], self.folders())
        self.assertEqual(1, self.names(instance).count("invocation_start"))
        self.assert_no_unknown_agent_folder()


class TestRefireWithoutAgentType(_RefireCase):
    """The existing-invocation check comes before any type handling."""

    def assert_old_invocation_continued(self):
        self.assertEqual(_OLD, self.mapped_instance(_OLD_CALL))
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        self.assert_no_unknown_agent_folder()

    def test_refire_with_absent_agent_type_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.start(_OLD_CALL + SUFFIX)

        self.assert_old_invocation_continued()
        self.assertEqual([_NEW], self.queued_ids())

    def test_refire_with_empty_agent_type_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent(_NEW)

        self.start(_OLD_CALL + SUFFIX, agent_type="", agent_name="")

        self.assert_old_invocation_continued()
        self.assertEqual([_NEW], self.queued_ids())

    def test_refire_with_only_an_agent_name_continues_the_old_invocation(self):
        self.first_cycle()
        self.dispatch_agent("Planner#9")

        self.start(_OLD_CALL + SUFFIX, agent_name="Planner")

        self.assert_old_invocation_continued()
        self.assertEqual(["Planner#9"], self.queued_ids())



if __name__ == "__main__":
    import unittest
    unittest.main()
