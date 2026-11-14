"""Cross-process consumption of the pending-dispatch queue.

Real processes, released together by a start barrier, take from and put to one
queue file. Every take must get its own entry, no entry may go to two takes, no
consumed entry may remain, and no appended entry may be lost.
"""

import json
import os
import sys
import unittest

_TESTS = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(_TESTS))
sys.path.insert(0, _TESTS)

from pending_dispatch_support import QueueCase, SESSION, entry, queue_path, read_queue_ids, run_actors, write_queue


class _ProcessCase(QueueCase):

    def raw_queue_lines(self):
        path = queue_path(self.paths)
        if not path.exists():
            return []
        return [ln for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class TestConcurrentTakes(_ProcessCase):

    def test_parallel_takes_each_get_their_own_entry_and_leave_the_queue_empty(self):
        ids = [f"Research#{n}" for n in range(1, 7)]
        write_queue(self.paths, [entry(i, "Research") for i in ids])

        results = run_actors(self.workspace, [("take", "Research", None)] * 6)

        self.assertEqual(sorted(ids), sorted(results))
        self.assertEqual([], self.raw_queue_lines())

    def test_more_takes_than_entries_hand_each_entry_out_exactly_once(self):
        write_queue(self.paths, [entry("Research#1", "Research"), entry("Research#2", "Research")])

        results = run_actors(self.workspace, [("take", "Research", None)] * 5)

        handed_out = [r for r in results if r is not None]
        self.assertEqual(["Research#1", "Research#2"], sorted(handed_out))
        self.assertEqual(3, results.count(None))
        self.assertEqual([], self.raw_queue_lines())

    def test_parallel_takes_of_different_types_each_get_an_entry_of_their_type(self):
        write_queue(self.paths, [entry("Research#1", "Research"), entry("Planner#2", "Planner"),
                                 entry("Research#3", "Research"), entry("Planner#4", "Planner")])

        results = run_actors(self.workspace, [
            ("take", "Planner", None), ("take", "Research", None),
            ("take", "Planner", None), ("take", "Research", None)])

        self.assertEqual({"Planner#2", "Planner#4"}, {results[0], results[2]})
        self.assertEqual({"Research#1", "Research#3"}, {results[1], results[3]})
        self.assertEqual([], self.raw_queue_lines())

    def test_a_take_of_an_unwanted_type_removes_nothing_under_contention(self):
        write_queue(self.paths, [entry("Planner#1", "Planner"), entry("Planner#2", "Planner")])

        results = run_actors(self.workspace, [("take", "Research", None)] * 4)

        self.assertEqual([None] * 4, results)
        self.assertEqual(["Planner#1", "Planner#2"], read_queue_ids(self.paths))


class TestConcurrentTakesAndPuts(_ProcessCase):

    def test_appends_during_takes_are_not_lost_and_taken_entries_do_not_return(self):
        seeded = [f"Research#{n}" for n in range(1, 5)]
        write_queue(self.paths, [entry(i, "Research") for i in seeded])
        added = [f"Planner#{n}" for n in range(1, 5)]
        specs = [("take", "Research", None)] * 4 + [("put", i, "Planner") for i in added]

        results = run_actors(self.workspace, specs)

        self.assertEqual(sorted(seeded), sorted(results[:4]))
        self.assertEqual([True] * 4, results[4:])
        remaining = [json.loads(ln) for ln in self.raw_queue_lines()]
        self.assertEqual(sorted(added), sorted(e["agent_instance_id"] for e in remaining))
        self.assertTrue(all(e.get("agent_type") == "Planner" for e in remaining))

    def test_parallel_puts_lose_no_entry_and_tear_no_line(self):
        specs = [("put", f"Research#{n}", "Research") for n in range(1, 9)]

        results = run_actors(self.workspace, specs)

        self.assertEqual([True] * 8, results)
        self.assertEqual(sorted(f"Research#{n}" for n in range(1, 9)),
                         sorted(read_queue_ids(self.paths)))


if __name__ == "__main__":
    unittest.main()
