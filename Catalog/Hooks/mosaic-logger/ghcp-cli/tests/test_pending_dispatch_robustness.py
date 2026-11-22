"""take_pending_dispatch under damaged, stale and contended queues.

A corrupt line never blocks correlation; stale entries are dropped wherever they
sit; a failed queue rewrite never leads to one entry being handed out twice; and
put and take share one lock so neither can write while the other holds it.
"""

import os
import stat
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core as core  # noqa: E402
from locked_append_support import hold_lock  # noqa: E402
from pending_dispatch_support import (  # noqa: E402
    RUN, SESSION, QueueCase, entry, pending, queue_path, read_queue_ids, write_queue,
)


class TestCorruptLines(QueueCase):

    def test_a_corrupt_line_does_not_block_the_valid_entries(self):
        write_queue(self.paths, [entry("Research#1"), "{broken json", entry("Research#2")])

        self.assertEqual("Research#1", self.take_id("Research"))
        self.assertEqual("Research#2", self.take_id("Research"))

    def test_a_corrupt_line_before_the_first_entry_is_skipped(self):
        write_queue(self.paths, ["not json at all", entry("Research#1")])

        self.assertEqual("Research#1", self.take_id("Research"))

    def test_lines_that_are_not_dispatch_objects_are_skipped(self):
        write_queue(self.paths, ["[1, 2, 3]", '"text"', "42", '{"run_id": null}',
                                 '{"agent_instance_id": ""}', '{"agent_instance_id": 7}',
                                 entry("Research#1")])

        self.assertEqual("Research#1", self.take_id("Research"))

    def test_corrupt_lines_are_dropped_when_the_queue_is_rewritten(self):
        write_queue(self.paths, [entry("Research#1"), "{broken json", entry("Planner#2")])

        self.take_id("Research")

        self.assertEqual(["Planner#2"], read_queue_ids(self.paths))

    def test_a_queue_of_only_corrupt_lines_yields_nothing(self):
        write_queue(self.paths, ["{broken", "also broken"])

        self.assertIsNone(self.take_id("Research"))


class TestStaleEntries(QueueCase):

    def test_a_stale_entry_behind_a_fresh_one_is_never_returned(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2", age=500)])

        self.assertEqual("Research#1", self.take_id("Research"))
        self.assertIsNone(self.take_id("Research"))

    def test_a_stale_entry_between_fresh_ones_is_skipped(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2", age=500),
                                 entry("Research#3")])

        self.assertEqual("Research#1", self.take_id("Research"))
        self.assertEqual("Research#3", self.take_id("Research"))

    def test_a_stale_entry_of_the_wanted_type_behind_a_fresh_entry_of_another_type(self):
        write_queue(self.paths, [entry("Planner#1"), entry("Research#2", age=500)])

        self.assertIsNone(self.take_id("Research"))

    def test_stale_entries_anywhere_are_removed_when_the_queue_is_rewritten(self):
        write_queue(self.paths, [entry("Research#1"), entry("Planner#2", age=500),
                                 entry("Planner#3"), entry("Planner#4", age=500)])

        self.take_id("Research")

        self.assertEqual(["Planner#3"], read_queue_ids(self.paths))

    def test_an_entry_with_an_unparseable_or_missing_timestamp_is_stale(self):
        bad = entry("Research#1")
        bad["created_at"] = "yesterday-ish"
        missing = entry("Research#2")
        del missing["created_at"]
        write_queue(self.paths, [bad, missing])

        self.assertIsNone(self.take_id("Research"))

    def test_max_age_seconds_sets_the_staleness_limit(self):
        write_queue(self.paths, [entry("Research#1", age=30)])

        self.assertIsNone(self.take_id("Research", max_age_seconds=10))
        write_queue(self.paths, [entry("Research#1", age=30)])
        self.assertEqual("Research#1", self.take_id("Research"))

    def test_the_default_age_limit_is_120_seconds(self):
        self.assertEqual(120, pending.PENDING_DISPATCH_MAX_AGE_SECONDS)
        write_queue(self.paths, [entry("Research#1", age=100)])
        self.assertEqual("Research#1", self.take_id("Research"))
        write_queue(self.paths, [entry("Research#2", age=150)])
        self.assertIsNone(self.take_id("Research"))


class TestFailedRewrite(QueueCase):

    def drain(self, agent_type, limit=6):
        returned = []
        for _ in range(limit):
            got = self.take_id(agent_type)
            if got is None:
                break
            returned.append(got)
        return returned

    # The queue rewrite goes through core.atomic_replace (the design's pinned
    # dependency); each test asserts the injected failure was really exercised.

    def test_an_entry_is_never_handed_out_twice_when_the_rewrite_fails(self):
        write_queue(self.paths, [entry("Research#1"), entry("Planner#2")])

        with mock.patch.object(core, "atomic_replace", return_value=False) as replace:
            first = self.take_id("Research")
        later = self.drain("Research")

        self.assertTrue(replace.called, "the rewrite failure was never injected")
        returned = ([first] if first else []) + later
        self.assertLessEqual(returned.count("Research#1"), 1, returned)
        self.assertEqual("Planner#2", self.take_id("Planner"))

    def test_no_entry_is_handed_out_twice_when_one_rewrite_fails_among_several(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2"), entry("Research#3")])

        with mock.patch.object(core, "atomic_replace", return_value=False) as replace:
            first = self.take_id("Research")
        later = self.drain("Research")

        self.assertTrue(replace.called, "the rewrite failure was never injected")
        returned = ([first] if first else []) + later
        self.assertEqual(len(returned), len(set(returned)), returned)

    def test_every_rewrite_failing_still_never_returns_an_entry_twice(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2")])

        with mock.patch.object(core, "atomic_replace", return_value=False) as replace:
            returned = [r for r in (self.take_id("Research") for _ in range(4)) if r]
            self.assertTrue(replace.called, "the rewrite failure was never injected")
            self.assertTrue(pending.put_pending_dispatch(
                self.paths, RUN, SESSION, "Research#9", None))

        self.assertEqual(len(returned), len(set(returned)), returned)
        self.assertIn("Research#9", read_queue_ids(self.paths))

    # Both the atomic replace and any in-place rewrite fail. How the entry is
    # then marked consumed is the implementation's choice (removal or a consumed
    # record), and the failing take's own result is unspecified; the one rule is
    # that no take, failing or later, returns the entry twice.

    def lock_queue_file_read_only(self):
        path = queue_path(self.paths)
        os.chmod(path, stat.S_IREAD)
        self.addCleanup(os.chmod, path, stat.S_IREAD | stat.S_IWRITE)
        return path

    def test_a_take_whose_every_rewrite_fails_never_returns_the_entry_twice(self):
        write_queue(self.paths, [entry("Research#1")])
        self.lock_queue_file_read_only()

        with mock.patch.object(core, "atomic_replace", return_value=False) as replace:
            failing = self.take_id("Research")
            while_failing = self.drain("Research")

        self.assertTrue(replace.called, "the rewrite failure was never injected")
        returned = ([failing] if failing else []) + while_failing
        self.assertLessEqual(returned.count("Research#1"), 1, returned)

    def test_an_entry_is_handed_out_at_most_once_across_a_failing_and_later_takes(self):
        write_queue(self.paths, [entry("Research#1")])
        path = self.lock_queue_file_read_only()

        with mock.patch.object(core, "atomic_replace", return_value=False):
            failing = self.take_id("Research")
        os.chmod(path, stat.S_IREAD | stat.S_IWRITE)
        later = self.drain("Research")

        returned = ([failing] if failing else []) + later
        self.assertLessEqual(returned.count("Research#1"), 1, returned)

    def test_a_failed_rewrite_followed_by_a_successful_one_hands_out_each_entry_once(self):
        write_queue(self.paths, [entry("Research#1"), entry("Research#2")])
        path = self.lock_queue_file_read_only()

        with mock.patch.object(core, "atomic_replace", return_value=False):
            failing = self.take_id("Research")
        os.chmod(path, stat.S_IREAD | stat.S_IWRITE)
        later = self.drain("Research")

        returned = ([failing] if failing else []) + later
        self.assertEqual(["Research#1", "Research#2"], returned)


class TestSharedQueueLock(QueueCase):
    """Put and take hold the same sidecar lock on the queue file."""

    def test_take_returns_nothing_while_another_process_holds_the_queue_lock(self):
        write_queue(self.paths, [entry("Research#1")])
        before = queue_path(self.paths).read_bytes()
        hold_lock(queue_path(self.paths))

        self.assertIsNone(self.take_id("Research"))
        self.assertEqual(before, queue_path(self.paths).read_bytes())

    def test_put_reports_false_and_writes_nothing_while_the_queue_lock_is_held(self):
        write_queue(self.paths, [entry("Research#1")])
        before = queue_path(self.paths).read_bytes()
        hold_lock(queue_path(self.paths))

        stored = pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#2", None)

        self.assertFalse(stored)
        self.assertEqual(before, queue_path(self.paths).read_bytes())

    def test_the_queue_lock_is_released_after_put_and_take(self):
        pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#1", None)
        self.take_id("Research")

        self.assertTrue(pending.put_pending_dispatch(self.paths, RUN, SESSION, "Research#2", None))
        self.assertEqual("Research#2", self.take_id("Research"))


if __name__ == "__main__":
    unittest.main()
