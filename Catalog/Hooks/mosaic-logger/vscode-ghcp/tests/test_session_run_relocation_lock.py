"""The unknown-run relocation takes the same append lock as every other writer of the streams.

It moves the unknown-run stream aside only while holding that stream's lock, and appends to
the run's stream under that stream's lock. When a lock cannot be had in time nothing is lost:
the lines are either written or still in the unknown-run stream, and the relocation does not raise.
"""

import json
import os
import pathlib
import sys
import tempfile
import time
import unittest
import unittest.mock

_TESTS = os.path.dirname(os.path.abspath(__file__))
_ADAPTER_DIR = pathlib.Path(_TESTS).parent
sys.path.insert(0, str(_ADAPTER_DIR.parent / "copilot-shared"))
sys.path.insert(0, str(_ADAPTER_DIR))
sys.path.insert(0, _TESTS)

import mosaic_logger_core as core
import mosaic_logger_session_run as session_run
from locked_append_support import hold_lock, lock_file_of

SESSION_A = "session-a"
SESSION_B = "session-b"
RUN = "20261029T000000Z-aaaa"
BOUND_SLACK = 15.0  # two bounded waits at most, each 2 s, plus margin


def _line(session, n):
    return json.dumps({"event": "probe", "session_id": session, "run_id": "unknown-run", "n": n}) + "\n"


class TestRelocationTakesTheAppendLock(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.paths = core.LogPaths(pathlib.Path(self._tmp.name))
        self.unknown = self.paths.orchestrator_events(session_run.UNKNOWN_RUN)
        self.run_stream = self.paths.orchestrator_events(RUN)
        self.unknown.parent.mkdir(parents=True)
        self.original = "".join([_line(SESSION_A, 1), _line(SESSION_B, 2), _line(SESSION_A, 3)]).encode()
        self.unknown.write_bytes(self.original)

    def relocate(self):
        started = time.monotonic()
        session_run.relocate_unknown_run_events(self.paths, SESSION_A, RUN)
        self.assertLess(time.monotonic() - started, BOUND_SLACK)

    def snapshots(self):
        return list(self.unknown.parent.glob("*.relocating-*"))

    def test_moves_the_session_lines_and_leaves_no_append_lock_file_behind(self):
        self.relocate()

        moved = [json.loads(x)["n"] for x in self.run_stream.read_text(encoding="utf-8").splitlines()]
        kept = [json.loads(x)["n"] for x in self.unknown.read_text(encoding="utf-8").splitlines()]
        self.assertEqual([1, 3], moved)
        self.assertEqual([2], kept)
        self.assertEqual([], self.snapshots())
        self.assertFalse(lock_file_of(self.unknown).exists())
        self.assertFalse(lock_file_of(self.run_stream).exists())

    def test_when_the_unknown_run_stream_lock_is_held_nothing_is_renamed_or_moved(self):
        lock = hold_lock(self.unknown)

        self.relocate()

        self.assertEqual(self.original, self.unknown.read_bytes())
        self.assertFalse(self.run_stream.exists())
        self.assertEqual([], self.snapshots())
        self.assertTrue(lock.exists())

    def test_when_the_run_stream_lock_is_held_the_whole_content_stays_in_the_unknown_run_stream(self):
        lock = hold_lock(self.run_stream)

        self.relocate()

        self.assertFalse(self.run_stream.exists())
        self.assertEqual(self.original, self.unknown.read_bytes())
        self.assertEqual([], self.snapshots())
        self.assertTrue(lock.exists())

    def test_when_appending_the_kept_lines_back_fails_the_snapshot_is_kept_and_the_moved_lines_are_written(self):
        real = session_run.locked_append
        unknown = self.unknown

        def refuse_the_unknown_run_stream(path, line, *args, **kwargs):
            if pathlib.Path(path) == unknown:
                return False
            return real(path, line, *args, **kwargs)

        with unittest.mock.patch.object(session_run, "locked_append", refuse_the_unknown_run_stream):
            self.relocate()

        moved = [json.loads(x)["n"] for x in self.run_stream.read_text(encoding="utf-8").splitlines()]
        self.assertEqual([1, 3], moved)
        snapshots = self.snapshots()
        self.assertEqual(1, len(snapshots))
        self.assertEqual(self.original, snapshots[0].read_bytes())

    def test_a_later_relocation_moves_the_lines_once_the_run_stream_lock_is_free(self):
        lock = hold_lock(self.run_stream)
        self.relocate()
        lock.unlink()

        self.relocate()

        moved = [json.loads(x)["n"] for x in self.run_stream.read_text(encoding="utf-8").splitlines()]
        self.assertEqual([1, 3], moved)


if __name__ == "__main__":
    unittest.main()
