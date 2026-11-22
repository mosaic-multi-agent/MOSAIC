"""Which instance id a SubagentStart receives.

Order of sources: the oldest pending dispatch of the started agent's type; else
an explicit agent_instance_id line in the started agent's task message; else a
unique fallback id {prefix}_{timestamp}-{4 hex} with the agent type as prefix.
A bare Name#Number mention in a task message is never an id source, and the
literal 'unknown-agent' is never produced.

Driven through the hook entry point (mosaic_logger.dispatch).
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (
    RUN_ID, CorrelationCase, dispatch_prompt, fallback_pattern,
    pre_dispatch_payload, start_payload,
)


class TestTypeMatchedStarts(CorrelationCase):

    def assert_folder_prefixes_match_agent_types(self):
        for instance in self.folders():
            started = next(e for e in self.events(instance)
                           if e["event"] == "invocation_start")
            self.assertEqual(instance.split("#")[0], started["agent_type"])

    def test_parallel_dispatches_of_different_types_started_in_reverse_order(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            start_payload("agt-writer", "TestWriter"),
            start_payload("agt-research", "Research"),
        )

        self.assertEqual("TestWriter#2", self.mapped_instance("agt-writer"))
        self.assertEqual("Research#1", self.mapped_instance("agt-research"))
        self.assertEqual(["Research#1", "TestWriter#2"], self.folders())
        self.assert_folder_prefixes_match_agent_types()
        self.assertEqual([], self.queued_ids())

    def test_three_dispatches_of_two_types_in_mixed_start_order(self):
        # Start order (TestWriter, Research, Research) differs from dispatch order.
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            pre_dispatch_payload("Research#3", "Research", "tu-3"),
            start_payload("agt-a", "TestWriter"),
            start_payload("agt-b", "Research"),
            start_payload("agt-c", "Research"),
        )

        self.assertEqual("TestWriter#2", self.mapped_instance("agt-a"))
        self.assertEqual("Research#1", self.mapped_instance("agt-b"))
        self.assertEqual("Research#3", self.mapped_instance("agt-c"))
        self.assert_folder_prefixes_match_agent_types()

    def test_dispatch_of_another_type_is_never_consumed(self):
        self.replay(
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            start_payload("agt-reviewer", "Reviewer"),
        )

        self.assertRegex(self.mapped_instance("agt-reviewer"), fallback_pattern("Reviewer"))
        self.assertEqual(["TestWriter#2"], self.queued_ids())

    def test_skipped_dispatch_is_still_available_to_its_own_type(self):
        self.replay(
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            start_payload("agt-reviewer", "Reviewer"),
            start_payload("agt-writer", "TestWriter"),
        )

        self.assertEqual("TestWriter#2", self.mapped_instance("agt-writer"))
        self.assertEqual([], self.queued_ids())

    def test_oldest_dispatch_of_the_type_goes_first(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            pre_dispatch_payload("Research#2", "Research", "tu-2"),
            start_payload("agt-a", "Research"),
            start_payload("agt-b", "Research"),
        )

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual("Research#2", self.mapped_instance("agt-b"))

    def test_dispatch_capture_records_the_subagent_type(self):
        self.replay(pre_dispatch_payload("Alias#1", "Research", "tu-1"))

        entries = [json.loads(ln) for ln in self.queue_lines()]

        self.assertEqual(1, len(entries))
        self.assertEqual("Research", entries[0]["agent_type"])
        self.assertEqual("Alias#1", entries[0]["agent_instance_id"])

    def test_dispatch_type_comes_from_subagent_type_not_from_the_id_prefix(self):
        # An older entry of another type is queued first, so oldest-first or
        # prefix-based matching would pick the wrong entry.
        self.replay(
            pre_dispatch_payload("Planner#2", "Planner", "tu-0"),
            pre_dispatch_payload("Alias#1", "Research", "tu-1"),
            start_payload("agt-a", "Research"),
        )

        self.assertEqual("Alias#1", self.mapped_instance("agt-a"))
        self.assertEqual(["Planner#2"], self.queued_ids())

    def test_dispatch_without_subagent_type_is_matched_by_id_prefix(self):
        self.replay(
            pre_dispatch_payload("Planner#2", None, "tu-0"),
            pre_dispatch_payload("Research#1", None, "tu-1"),
            start_payload("agt-a", "Research"),
        )

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual(["Planner#2"], self.queued_ids())

    def test_type_match_ignores_case_and_separators(self):
        self.replay(
            pre_dispatch_payload("Reviewer#3", "Reviewer", "tu-0"),
            pre_dispatch_payload("test-writer-tdd#4", "test-writer-tdd", "tu-1"),
            start_payload("agt-a", "TestWriterTDD"),
        )

        self.assertEqual("test-writer-tdd#4", self.mapped_instance("agt-a"))
        self.assertEqual(["Reviewer#3"], self.queued_ids())


class TestStartWithoutMatchingDispatch(CorrelationCase):

    def test_unique_fallback_id_with_the_agent_type_as_prefix(self):
        self.replay(start_payload("agt-a", "Research"))

        self.assertRegex(self.mapped_instance("agt-a"), fallback_pattern("Research"))
        self.assertEqual([self.mapped_instance("agt-a")], self.folders())
        self.assert_no_unknown_agent_folder()

    def test_two_unmatched_starts_get_distinct_ids_and_folders(self):
        self.replay(
            start_payload("agt-a", "Research"),
            start_payload("agt-b", "Research"),
        )

        first, second = self.mapped_instance("agt-a"), self.mapped_instance("agt-b")
        self.assertNotEqual(first, second)
        self.assertEqual(sorted([first, second]), self.folders())

    def test_explicit_agent_instance_id_line_in_the_task_message_is_used(self):
        self.replay(start_payload("agt-a", "Research",
                                  prompt=dispatch_prompt("Research#4")))

        self.assertEqual("Research#4", self.mapped_instance("agt-a"))
        self.assertEqual(["Research#4"], self.folders())

    def test_explicit_line_is_used_when_only_another_type_is_pending(self):
        self.replay(
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            start_payload("agt-a", "Research", prompt=dispatch_prompt("Research#4")),
        )

        self.assertEqual("Research#4", self.mapped_instance("agt-a"))
        self.assertEqual(["TestWriter#2"], self.queued_ids())

    def test_fallback_prefix_is_a_valid_path_component_for_an_odd_type(self):
        self.replay(start_payload("agt-a", "my agent#2"))

        instance = self.mapped_instance("agt-a")

        self.assertRegex(instance, r"^[^#\s]+_\d{8}T\d{6}Z-[0-9a-f]{4}$")
        self.assertTrue(self.paths.invocation_dir(RUN_ID, instance).is_dir())

    def test_invocation_start_carries_the_fallback_id(self):
        self.replay(start_payload("agt-a", "Research"))

        instance = self.mapped_instance("agt-a")
        started = next(e for e in self.events(instance) if e["event"] == "invocation_start")

        self.assertRegex(instance, fallback_pattern("Research"))
        self.assertEqual(instance, started["agent_instance_id"])
        self.assertEqual("Research", started["agent_type"])


class TestStartWithoutAgentType(CorrelationCase):

    def test_untyped_start_consumes_no_dispatch(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-a", None),
        )

        self.assertRegex(self.mapped_instance("agt-a"), fallback_pattern("agent"))
        self.assertEqual(["Research#1"], self.queued_ids())
        self.assert_no_unknown_agent_folder()

    def test_empty_type_is_treated_like_an_absent_type(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-a", ""),
        )

        self.assertRegex(self.mapped_instance("agt-a"), fallback_pattern("agent"))
        self.assertEqual(["Research#1"], self.queued_ids())

    def test_untyped_start_uses_the_explicit_line_and_still_consumes_nothing(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-a", None, prompt=dispatch_prompt("TestWriter#9")),
        )

        self.assertEqual("TestWriter#9", self.mapped_instance("agt-a"))
        self.assertEqual(["Research#1"], self.queued_ids())

    def test_pending_dispatch_stays_available_to_a_later_typed_start(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-a", None),
            start_payload("agt-b", "Research"),
        )

        self.assertEqual("Research#1", self.mapped_instance("agt-b"))


class TestTaskMessageTrust(CorrelationCase):

    _MENTION = "You work after Research#1 has finished and read its report."

    def test_bare_mention_without_an_explicit_line_gets_the_fallback_id(self):
        self.replay(start_payload("agt-a", "TestWriter", prompt=self._MENTION))

        instance = self.mapped_instance("agt-a")

        self.assertRegex(instance, fallback_pattern("TestWriter"))
        self.assertNotIn("Research#1", self.folders())

    def test_bare_mention_never_feeds_an_existing_invocation(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-research", "Research"),
            start_payload("agt-writer", "TestWriter", prompt=self._MENTION),
        )

        self.assertEqual(1, self.names("Research#1").count("invocation_start"))
        self.assertNotEqual("Research#1", self.mapped_instance("agt-writer"))
        self.assertRegex(self.mapped_instance("agt-writer"), fallback_pattern("TestWriter"))

    def test_matching_dispatch_wins_over_a_bare_mention(self):
        self.replay(
            pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-2"),
            start_payload("agt-writer", "TestWriter", prompt=self._MENTION),
        )

        self.assertEqual("TestWriter#2", self.mapped_instance("agt-writer"))

    def test_dispatch_with_only_a_bare_mention_is_not_queued(self):
        self.replay(pre_dispatch_payload(
            "Research#9", "Research", "tu-9",
            prompt="Please hand this to Research#9 when ready."))

        self.assertEqual([], self.queue_lines())

    def test_explicit_line_beside_a_bare_mention_is_the_one_used(self):
        prompt = self._MENTION + ' {"agent_instance_id": "TestWriter#5"}'
        self.replay(start_payload("agt-a", "TestWriter", prompt=prompt))

        self.assertEqual("TestWriter#5", self.mapped_instance("agt-a"))


class TestBareMentionDoesNotBindARun(CorrelationCase):

    bind_run = False

    def test_prompt_with_a_bare_mention_and_a_run_id_does_not_bind_the_run(self):
        prompt = f"Follow Research#1. run_id: {RUN_ID}"
        self.replay(start_payload("agt-a", "TestWriter", prompt=prompt))

        self.assertFalse(self.run_root(RUN_ID).exists())
        folders = self.folders("unknown-run")
        self.assertEqual(1, len(folders))
        self.assertRegex(folders[0], fallback_pattern("TestWriter"))
