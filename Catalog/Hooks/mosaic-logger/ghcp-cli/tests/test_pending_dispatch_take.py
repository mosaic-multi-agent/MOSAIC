"""take_pending_dispatch: type-matched, oldest-first consumption of the queue.

The queue is a per-(run bucket, session) JSONL file. A start takes the oldest
fresh entry of its own agent type; entries of other types stay untouched and in
order; an untyped start takes nothing; consumed entries are gone from the file.
The copilot adapters store no dispatch type, so an entry's type is the prefix of
its instance id (a stored type, when present, wins). Also the type helpers.
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from pending_dispatch_support import (  # noqa: E402
    RUN, SESSION, QueueCase, entry, pending, queue_path, read_queue_ids, write_queue,
)


class TestTypeMatchedConsumption(QueueCase):

    def test_takes_the_oldest_entry_of_the_requested_type(self):
        write_queue(self.paths, [entry("Research#1"), entry("Planner#2"), entry("Research#3")])

        self.assertEqual("Research#1", self.take_id("Research"))
        self.assertEqual("Research#3", self.take_id("Research"))
        self.assertIsNone(self.take_id("Research"))

    def test_an_entry_of_another_type_is_never_returned(self):
        write_queue(self.paths, [entry("Planner#2")])

        self.assertIsNone(self.take_id("Research"))

    def test_entries_of_other_types_stay_in_their_original_order(self):
        write_queue(self.paths, [entry("Research#1"), entry("Planner#2"),
                                 entry("Research#3"), entry("Planner#4")])

        self.take("Research")

        self.assertEqual(["Planner#2", "Research#3", "Planner#4"], read_queue_ids(self.paths))

    def test_a_miss_leaves_the_queue_file_unchanged(self):
        write_queue(self.paths, [entry("Planner#2")])
        before = queue_path(self.paths).read_bytes()

        self.take("Research")

        self.assertEqual(before, queue_path(self.paths).read_bytes())

    def test_consumed_entry_is_removed_from_the_queue_file(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2")])

        self.take("Research")

        self.assertEqual(["Research#2"], read_queue_ids(self.paths))

    def test_the_type_is_the_instance_id_prefix_when_none_is_stored(self):
        write_queue(self.paths, [entry("Research#1")])

        self.assertIsNone(self.take_id("Planner"))
        self.assertEqual("Research#1", self.take_id("Research"))

    def test_stored_agent_type_wins_over_the_instance_id_prefix(self):
        write_queue(self.paths, [entry("Alias#1", "Research")])

        self.assertIsNone(self.take_id("Alias"))
        self.assertEqual("Alias#1", self.take_id("Research"))

    def test_types_match_ignoring_case_and_separators(self):
        write_queue(self.paths, [entry("test-writer-tdd#4")])

        self.assertEqual("test-writer-tdd#4", self.take_id("TestWriterTDD"))

    def test_untyped_take_returns_nothing_and_modifies_nothing(self):
        write_queue(self.paths, [entry("Research#1")])
        before = queue_path(self.paths).read_bytes()

        for untyped in (None, ""):
            self.assertIsNone(self.take_id(untyped))

        self.assertEqual(before, queue_path(self.paths).read_bytes())

    def test_result_carries_run_id_and_prompt(self):
        write_queue(self.paths, [entry("Research#1", run_id="20261001T100000Z-aa11",
                                       prompt="the task")])

        got = self.take("Research")

        self.assertEqual("20261001T100000Z-aa11", got.run_id)
        self.assertEqual("the task", got.prompt)

    def test_result_has_none_for_a_missing_run_id_and_prompt(self):
        write_queue(self.paths, [entry("Research#1")])

        got = self.take("Research")

        self.assertIsNone(got.run_id)
        self.assertIsNone(got.prompt)

    def test_sessions_have_separate_queues(self):
        write_queue(self.paths, [entry("Research#1")], session="alpha")
        write_queue(self.paths, [entry("Research#2")], session="beta")

        self.assertEqual("Research#1", self.take_id("Research", "alpha"))
        self.assertEqual("Research#2", self.take_id("Research", "beta"))

    def test_run_buckets_have_separate_queues(self):
        write_queue(self.paths, [entry("Research#1")], run="bucket-a")
        write_queue(self.paths, [entry("Research#2")], run="bucket-b")

        self.assertEqual("Research#2", self.take_id("Research", run="bucket-b"))
        self.assertEqual("Research#1", self.take_id("Research", run="bucket-a"))

    def test_absent_and_empty_queue_return_nothing(self):
        self.assertIsNone(self.take_id("Research"))
        write_queue(self.paths, [])
        self.assertIsNone(self.take_id("Research"))

    def test_never_raises_on_a_binary_queue_file(self):
        path = queue_path(self.paths)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b"\xff\xfe\x00{{{\n\x80\n")

        self.assertIsNone(self.take_id("Research"))


class TestPutStoresTheType(QueueCase):

    def lines(self):
        return [json.loads(ln) for ln in queue_path(self.paths).read_text("utf-8").splitlines()
                if ln.strip()]

    def test_agent_type_is_stored_when_given(self):
        self.assertTrue(pending.put_pending_dispatch(
            self.paths, RUN, SESSION, "Alias#1", None, agent_type="Research"))

        self.assertEqual("Research", self.lines()[0]["agent_type"])

    def test_agent_type_is_not_stored_when_empty_or_absent(self):
        pending.put_pending_dispatch(self.paths, RUN, SESSION, "A#1", None, agent_type="")
        pending.put_pending_dispatch(self.paths, RUN, SESSION, "B#2", None)

        self.assertTrue(all("agent_type" not in line for line in self.lines()))

    def test_put_then_take_round_trips_with_the_prompt(self):
        pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#1", "20261001T100000Z-aa11",
                                     prompt="the task")

        got = self.take("Research")

        self.assertEqual(("Research#1", "20261001T100000Z-aa11", "the task"), tuple(got))

    def test_put_after_take_is_not_lost(self):
        pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#1", None)
        self.take("Research")

        pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#2", None)

        self.assertEqual("Research#2", self.take_id("Research"))


class TestEntryWithoutAnyType(QueueCase):

    def test_an_entry_with_no_hash_and_no_stored_type_is_never_returned(self):
        write_queue(self.paths, [entry("plainid")])

        self.assertIsNone(self.take_id("Research"))
        self.assertIsNone(self.take_id("Planner"))
        self.assertEqual(["plainid"], read_queue_ids(self.paths))

    def test_an_entry_with_no_type_is_kept_and_does_not_block_later_entries(self):
        write_queue(self.paths, [entry("plainid"), entry("Research#2")])

        self.assertEqual("Research#2", self.take_id("Research"))
        self.assertEqual(["plainid"], read_queue_ids(self.paths))


class TestTypeHelpers(unittest.TestCase):

    def test_dispatch_type_of_is_the_part_before_the_first_hash(self):
        self.assertEqual("test-writer-tdd", pending.dispatch_type_of("test-writer-tdd#83"))
        self.assertEqual("a", pending.dispatch_type_of("a#b#c"))

    def test_dispatch_type_of_is_none_without_a_usable_prefix(self):
        for value in (None, "", "no-hash", "#5", "Research_20261001T100000Z-aa11"):
            self.assertIsNone(pending.dispatch_type_of(value), repr(value))

    def test_types_match_when_equal_ignoring_case_and_non_alphanumerics(self):
        self.assertTrue(pending.agent_types_match("test-writer-tdd", "TestWriterTDD"))
        self.assertTrue(pending.agent_types_match("Research", "research"))

    def test_types_do_not_match_when_different_or_empty(self):
        self.assertFalse(pending.agent_types_match("Research", "Planner"))
        self.assertFalse(pending.agent_types_match("", ""))
        self.assertFalse(pending.agent_types_match(None, "Research"))
        self.assertFalse(pending.agent_types_match("Research", None))


if __name__ == "__main__":
    unittest.main()
