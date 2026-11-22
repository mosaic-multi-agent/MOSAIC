"""Concurrent SubagentStart hooks, each a separate process.

Parallel dispatches are queued, then the starts run at the same time through the
real hook entry point (mosaic_logger.py as a subprocess). Every start must get
its own dispatch, no dispatch may go to two starts, and the queue must be empty
afterwards.
"""

import json
import os
import pathlib
import subprocess
import sys
import threading

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (
    RUN_ID, SESSION_ID, CorrelationCase, pre_dispatch_payload, start_payload,
)

_ENTRY_POINT = str(pathlib.Path(__file__).resolve().parent.parent / "mosaic_logger.py")
_PROCESS_TIMEOUT_SECONDS = 60


class TestConcurrentStartsThroughTheHookEntryPoint(CorrelationCase):

    def run_starts_concurrently(self, payloads):
        """Launch one hook process per payload at (nearly) the same moment."""
        env = {**os.environ, "CLAUDE_PROJECT_DIR": str(self.workspace),
               "MOSAIC_RUN_ID": RUN_ID}
        env.pop("MOSAIC_ROLE", None)
        barrier = threading.Barrier(len(payloads))
        results = [None] * len(payloads)

        def run(index, payload):
            try:
                barrier.wait(timeout=30)
                results[index] = subprocess.run(
                    [sys.executable, _ENTRY_POINT], input=json.dumps(payload).encode(),
                    capture_output=True, env=env, timeout=_PROCESS_TIMEOUT_SECONDS)
            except Exception as exc:  # reported by the assertion below
                results[index] = exc

        threads = [threading.Thread(target=run, args=(i, p), daemon=True)
                   for i, p in enumerate(payloads)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=_PROCESS_TIMEOUT_SECONDS + 30)
        for index, result in enumerate(results):
            self.assertIsInstance(result, subprocess.CompletedProcess, f"process {index}: {result!r}")
            self.assertEqual(0, result.returncode, f"process {index}: {result.stderr!r}")

    def dispatch_all(self, specs):
        self.replay(*[pre_dispatch_payload(instance, agent_type, f"tu-{i}")
                      for i, (instance, agent_type) in enumerate(specs)])

    def test_parallel_dispatches_of_one_type_each_get_their_own_id(self):
        specs = [(f"Research#{n}", "Research") for n in range(1, 5)]
        self.dispatch_all(specs)

        self.run_starts_concurrently(
            [start_payload(f"agt-{n}", "Research") for n in range(1, 5)])

        ids = [self.mapped_instance(f"agt-{n}") for n in range(1, 5)]
        self.assertEqual(sorted(i for i, _ in specs), sorted(ids))
        self.assertEqual([], self.queue_lines())
        self.assertEqual(sorted(ids), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_repeated_rounds_never_hand_one_dispatch_to_two_starts(self):
        for round_no in range(3):
            session = f"{SESSION_ID}-round-{round_no}"
            instances = [f"Research#{round_no}{n}" for n in range(3)]
            self.replay(*[pre_dispatch_payload(inst, "Research", f"tu-{round_no}-{i}",
                                               session_id=session)
                          for i, inst in enumerate(instances)])

            self.run_starts_concurrently(
                [start_payload(f"agt-{round_no}-{n}", "Research", session_id=session)
                 for n in range(3)])

            ids = [self.mapped_instance(f"agt-{round_no}-{n}") for n in range(3)]
            self.assertEqual(sorted(instances), sorted(ids), f"round {round_no}")
            self.assertEqual([], self.queue_lines(session), f"round {round_no}")

    def test_parallel_dispatches_of_two_types_land_in_matching_folders(self):
        specs = [("Research#1", "Research"), ("TestWriter#2", "TestWriter"),
                 ("Research#3", "Research"), ("TestWriter#4", "TestWriter")]
        self.dispatch_all(specs)

        self.run_starts_concurrently([
            start_payload("agt-w1", "TestWriter"), start_payload("agt-r1", "Research"),
            start_payload("agt-w2", "TestWriter"), start_payload("agt-r2", "Research")])

        writers = {self.mapped_instance("agt-w1"), self.mapped_instance("agt-w2")}
        researchers = {self.mapped_instance("agt-r1"), self.mapped_instance("agt-r2")}
        self.assertEqual({"TestWriter#2", "TestWriter#4"}, writers)
        self.assertEqual({"Research#1", "Research#3"}, researchers)
        self.assertEqual([], self.queue_lines())

    def test_more_starts_than_dispatches_leave_the_surplus_on_unique_fallbacks(self):
        self.dispatch_all([("Research#1", "Research"), ("Research#2", "Research")])

        self.run_starts_concurrently(
            [start_payload(f"agt-{n}", "Research") for n in range(1, 5)])

        ids = [self.mapped_instance(f"agt-{n}") for n in range(1, 5)]
        self.assertEqual(4, len(set(ids)))
        self.assertEqual({"Research#1", "Research#2"}, {i for i in ids if "#" in i})
        self.assertEqual([], self.queue_lines())
        self.assert_no_unknown_agent_folder()
