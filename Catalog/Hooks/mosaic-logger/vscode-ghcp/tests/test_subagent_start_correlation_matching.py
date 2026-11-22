"""Which instance id a SubagentStart receives.

Order of sources: the oldest pending dispatch of the started agent's type; else a
unique fallback id {prefix}_{timestamp}-{4 hex} with the agent type as prefix (the
VS Code SubagentStart carries no task message, so there is no explicit-line source).
The started agent's type is agent_type, else agent_name. A dispatch's type is the
prefix of its instance id (the dispatch tool's type key is not verified, so a
tool_input agentName is not used). The literal 'unknown-agent' is never produced.

Driven through the hook entry point (mosaic_logger.dispatch).
"""

import os
import pathlib
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, CorrelationCase, fallback_pattern,
)


class TestTypeMatchedStarts(CorrelationCase):

    def assert_folder_prefixes_match_agent_types(self):
        for instance in self.folders():
            started = next(e for e in self.events(instance)
                           if e["event"] == "invocation_start")
            self.assertEqual(instance.split("#")[0], started["agent_type"])

    def test_parallel_dispatches_of_different_types_started_in_reverse_order(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("TestWriter#2")
        self.start("toolu_writer", "TestWriter")
        self.start("toolu_research", "Research")

        self.assertEqual("TestWriter#2", self.mapped_instance("toolu_writer"))
        self.assertEqual("Research#1", self.mapped_instance("toolu_research"))
        self.assertEqual(["Research#1", "TestWriter#2"], self.folders())
        self.assert_folder_prefixes_match_agent_types()
        self.assertEqual([], self.queued_ids())

    def test_three_dispatches_of_two_types_in_mixed_start_order(self):
        # Start order (TestWriter, Research, Research) differs from dispatch order.
        self.dispatch_agent("Research#1")
        self.dispatch_agent("TestWriter#2")
        self.dispatch_agent("Research#3")
        self.start("toolu_a", "TestWriter")
        self.start("toolu_b", "Research")
        self.start("toolu_c", "Research")

        self.assertEqual("TestWriter#2", self.mapped_instance("toolu_a"))
        self.assertEqual("Research#1", self.mapped_instance("toolu_b"))
        self.assertEqual("Research#3", self.mapped_instance("toolu_c"))
        self.assert_folder_prefixes_match_agent_types()

    def test_spawn_index_holds_the_dispatched_id_for_the_matched_start(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("TestWriter#2")
        self.start("toolu_writer", "TestWriter")

        self.assertEqual("TestWriter#2", self.instance_of("toolu_writer"))

    def test_dispatch_of_another_type_is_never_consumed(self):
        self.dispatch_agent("TestWriter#2")
        self.start("toolu_reviewer", "Reviewer")

        self.assertRegex(self.mapped_instance("toolu_reviewer"), fallback_pattern("Reviewer"))
        self.assertEqual(["TestWriter#2"], self.queued_ids())

    def test_skipped_dispatch_is_still_available_to_its_own_type(self):
        self.dispatch_agent("TestWriter#2")
        self.start("toolu_reviewer", "Reviewer")
        self.start("toolu_writer", "TestWriter")

        self.assertEqual("TestWriter#2", self.mapped_instance("toolu_writer"))
        self.assertEqual([], self.queued_ids())

    def test_oldest_dispatch_of_the_type_goes_first(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("Research#2")
        self.start("toolu_a", "Research")
        self.start("toolu_b", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))
        self.assertEqual("Research#2", self.mapped_instance("toolu_b"))

    def test_dispatch_type_is_the_instance_id_prefix(self):
        # An older entry of another type is queued first, so oldest-first
        # consumption would pick the wrong entry.
        self.dispatch_agent("Planner#2")
        self.dispatch_agent("Research#1")
        self.start("toolu_a", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))
        self.assertEqual(["Planner#2"], self.queued_ids())

    def test_type_match_ignores_case_and_separators(self):
        self.dispatch_agent("Reviewer#3")
        self.dispatch_agent("test-writer-tdd#4")
        self.start("toolu_a", "TestWriterTDD")

        self.assertEqual("test-writer-tdd#4", self.mapped_instance("toolu_a"))
        self.assertEqual(["Reviewer#3"], self.queued_ids())

    def test_every_dispatch_tool_name_is_matched_alike(self):
        self.dispatch_agent("Planner#2", tool_name="Task")
        self.dispatch_agent("Research#1", tool_name="Agent")
        self.start("toolu_a", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))
        self.assertEqual(["Planner#2"], self.queued_ids())


class TestStartedAgentType(CorrelationCase):
    """The started agent's type is agent_type, else agent_name."""

    def test_agent_name_alone_is_the_type_for_matching(self):
        self.dispatch_agent("Planner#2")
        self.dispatch_agent("Research#1")
        self.start("toolu_a", agent_name="Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))
        self.assertEqual(["Planner#2"], self.queued_ids())

    def test_agent_type_wins_over_agent_name(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("Planner#2")
        self.start("toolu_a", agent_type="Planner", agent_name="Research")

        self.assertEqual("Planner#2", self.mapped_instance("toolu_a"))
        self.assertEqual(["Research#1"], self.queued_ids())

    def test_invocation_start_records_the_agent_name_when_it_is_the_type(self):
        self.dispatch_agent("Research#1")
        self.start("toolu_a", agent_name="Research")

        started = next(e for e in self.events("Research#1") if e["event"] == "invocation_start")
        self.assertEqual("Research", started["agent_type"])

    def test_name_only_fallback_folder_prefix_equals_the_recorded_agent_type(self):
        self.start("toolu_a", agent_name="Reviewer")

        instance = self.mapped_instance("toolu_a")
        started = next(e for e in self.events(instance) if e["event"] == "invocation_start")

        self.assertRegex(instance, fallback_pattern("Reviewer"))
        self.assertEqual("Reviewer", started["agent_type"])

    def test_empty_agent_type_falls_back_to_the_agent_name(self):
        self.dispatch_agent("Research#1")
        self.start("toolu_a", agent_type="", agent_name="Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))


class TestStartWithoutMatchingDispatch(CorrelationCase):

    def test_unique_fallback_id_with_the_agent_type_as_prefix(self):
        self.start("toolu_a", "Research")

        self.assertRegex(self.mapped_instance("toolu_a"), fallback_pattern("Research"))
        self.assertEqual([self.mapped_instance("toolu_a")], self.folders())
        self.assert_no_unknown_agent_folder()

    def test_fallback_id_is_what_the_spawn_index_holds(self):
        self.start("toolu_a", "Research")

        self.assertEqual(self.mapped_instance("toolu_a"), self.instance_of("toolu_a"))

    def test_two_unmatched_starts_get_distinct_ids_and_folders(self):
        self.start("toolu_a", "Research")
        self.start("toolu_b", "Research")

        first, second = self.mapped_instance("toolu_a"), self.mapped_instance("toolu_b")
        self.assertNotEqual(first, second)
        self.assertEqual(sorted([first, second]), self.folders())

    def test_fallback_prefix_is_a_valid_path_component_for_an_odd_type(self):
        self.start("toolu_a", "my agent#2")

        instance = self.mapped_instance("toolu_a")

        self.assertRegex(instance, r"^[^#\s]+_\d{8}T\d{6}Z-[0-9a-f]{4}$")
        self.assertTrue(self.paths.invocation_dir(RUN_ID, instance).is_dir())

    def test_invocation_start_carries_the_fallback_id(self):
        self.start("toolu_a", "Research")

        instance = self.mapped_instance("toolu_a")
        started = next(e for e in self.events(instance) if e["event"] == "invocation_start")

        self.assertEqual(instance, started["agent_instance_id"])
        self.assertEqual("Research", started["agent_type"])

    def test_fallback_start_still_writes_one_start_and_the_input_file(self):
        self.start("toolu_a", "Research")

        instance = self.mapped_instance("toolu_a")

        self.assertEqual(1, self.names(instance).count("invocation_start"))
        self.assertTrue(self.paths.invocation_input(RUN_ID, instance).exists())


class TestStartWithoutAgentType(CorrelationCase):

    def test_untyped_start_consumes_no_dispatch(self):
        self.dispatch_agent("Research#1")
        self.start("toolu_a")

        self.assertRegex(self.mapped_instance("toolu_a"), fallback_pattern("agent"))
        self.assertEqual(["Research#1"], self.queued_ids())
        self.assert_no_unknown_agent_folder()

    def test_empty_type_and_name_are_treated_like_an_absent_type(self):
        self.dispatch_agent("Research#1")
        self.start("toolu_a", agent_type="", agent_name="")

        self.assertRegex(self.mapped_instance("toolu_a"), fallback_pattern("agent"))
        self.assertEqual(["Research#1"], self.queued_ids())

    def test_pending_dispatch_stays_available_to_a_later_typed_start(self):
        self.dispatch_agent("Research#1")
        self.start("toolu_a")
        self.start("toolu_b", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_b"))


class TestDispatchCapture(CorrelationCase):
    """Only an explicit agent_instance_id line is queued."""

    def test_an_explicit_line_is_queued_and_matches_its_type(self):
        self.dispatch_agent("Research#1")

        self.assertEqual(["Research#1"], self.queued_ids())

    def test_dispatch_with_only_a_bare_mention_is_not_queued(self):
        self.dispatch_agent("Research#9", prompt="Please hand this to Research#9 when ready.")

        self.assertEqual([], self.queue_lines())

    def test_a_bare_mention_beside_an_explicit_line_does_not_win(self):
        prompt = ('Follow Other#7 first. {"agent_instance_id": "Research#3", '
                  f'"run_id": "{RUN_ID}"}}')
        self.dispatch_agent("ignored#0", prompt=prompt)

        self.assertEqual(["Research#3"], self.queued_ids())


class TestQueueAddressing(CorrelationCase):
    """The start finds the dispatch under the run bucket the dispatch was queued in."""

    bind_run = False

    def test_dispatch_that_binds_the_session_is_found_by_the_start(self):
        self.dispatch_agent("Planner#2")
        self.dispatch_agent("Research#1")

        self.start("toolu_a", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a"))
        self.assertEqual(["Planner#2"], self.queued_ids())
        self.assertEqual([], self.folders("unknown-run"))

    def test_dispatch_without_a_run_id_is_found_in_the_unknown_run_bucket(self):
        self.dispatch_agent("Planner#2", run_id=None)
        self.dispatch_agent("Research#1", run_id=None)

        self.start("toolu_a", "Research")

        self.assertEqual("Research#1", self.mapped_instance("toolu_a", "unknown-run"))
        self.assertEqual(["Planner#2"], self.queued_ids(run_id="unknown-run"))
        self.assertEqual(["Research#1"], self.folders("unknown-run"))

    def test_other_sessions_dispatches_are_not_consumed(self):
        self.dispatch_agent("Research#1", session_id="other-session")

        self.start("toolu_a", "Research")

        self.assertRegex(self.mapped_instance("toolu_a", "unknown-run") or "",
                         fallback_pattern("Research"))
        self.assertEqual(["Research#1"], self.queued_ids("other-session"))


class TestAdapterSourceHasNoUnknownAgentLiteral(unittest.TestCase):

    def test_no_adapter_module_contains_the_literal(self):
        adapter_dir = pathlib.Path(__file__).resolve().parent.parent
        offenders = [p.name for p in adapter_dir.glob("mosaic_logger*.py")
                     if "unknown-agent" in p.read_text("utf-8")]

        self.assertEqual([], offenders)


if __name__ == "__main__":
    unittest.main()
