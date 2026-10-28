"""Relocation never loses lines that other hook processes append to unknown-run/ meanwhile."""

import json
import os
import sys
import threading
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session_routing_flow as sf
import mosaic_logger_session_run as session_run


def _line(session_id, n):
    return json.dumps({"event": "tool_call_start", "session_id": session_id,
                       "run_id": "unknown-run", "n": n}) + "\n"


class TestConcurrentRelocation(unittest.TestCase):

    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)
        self.source = self.ws.stream_path(sf.UNKNOWN)
        self.source.parent.mkdir(parents=True, exist_ok=True)
        self.source.write_text(_line(sf.SESSION_A, 1) + _line(sf.SESSION_B, 2), "utf-8")

    def _append(self, text):
        with open(self.source, "ab") as fh:
            fh.write(text.encode("utf-8"))

    def test_a_line_appended_while_the_snapshot_is_processed_survives(self):
        real = session_run._split_session_lines

        def split_with_append(raw, session_id, run_id):
            self._append(_line(sf.SESSION_B, 3))
            self._append(_line(sf.SESSION_A, 4))
            return real(raw, session_id, run_id)

        with mock.patch.object(session_run, "_split_session_lines", split_with_append):
            session_run.relocate_unknown_run_events(self.ws.paths, sf.SESSION_A, sf.RUN_A)
        remaining = sorted(e["n"] for e in self.ws.stream(sf.UNKNOWN))
        moved = [e["n"] for e in self.ws.stream(sf.RUN_A)]
        self.assertEqual([1], moved)
        # Session A's concurrent line stays in the live stream (picked up by a later pass).
        self.assertEqual([2, 3, 4], remaining)

    def test_two_concurrent_relocations_move_each_line_exactly_once(self):
        threads = [threading.Thread(target=session_run.relocate_unknown_run_events,
                                    args=(self.ws.paths, sf.SESSION_A, sf.RUN_A))
                   for _ in range(2)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        self.assertEqual([1], [e["n"] for e in self.ws.stream(sf.RUN_A)])
        self.assertEqual([2], [e["n"] for e in self.ws.stream(sf.UNKNOWN)])

    def test_no_snapshot_file_is_left_behind(self):
        session_run.relocate_unknown_run_events(self.ws.paths, sf.SESSION_A, sf.RUN_A)
        self.assertEqual([self.source.name], os.listdir(self.source.parent))

    def test_a_later_prompt_retries_a_relocation_that_did_not_happen(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.source.write_text(_line(sf.SESSION_A, 9), "utf-8")
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertIn(9, [e.get("n") for e in self.ws.stream(sf.RUN_A)])


if __name__ == "__main__":
    unittest.main()
