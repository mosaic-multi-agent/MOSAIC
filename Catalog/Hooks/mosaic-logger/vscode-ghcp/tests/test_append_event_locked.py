"""append_event appends each event as one whole line under the file lock.

Parallel writers on one file lose, tear or blank no line. A writer that cannot get the lock
in time drops the event, writes a debug message and returns normally.
"""

import os
import pathlib
import sys
import tempfile
import time
import unittest
import unittest.mock

_TESTS = os.path.dirname(os.path.abspath(__file__))
_ADAPTER_DIR = pathlib.Path(_TESTS).parent
# The Copilot adapters get the shared module folder beside them when deployed; claude-code does not.
_DIRS = [_ADAPTER_DIR]
if _ADAPTER_DIR.name != "claude-code":
    _DIRS.insert(0, _ADAPTER_DIR.parent / "copilot-shared")
for _d in reversed(_DIRS):
    sys.path.insert(0, str(_d))
sys.path.insert(0, _TESTS)

import mosaic_logger_core as core
from locked_append_support import hold_lock, line_problems, lock_file_of, release_after, run_writers

BOUND_SLACK = 6.0


class TestParallelAppendEvent(unittest.TestCase):

    def test_parallel_writers_of_long_lines_lose_tear_or_blank_no_line(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = pathlib.Path(tmp) / "run" / "00_orchestrator_events.jsonl"

            problems = run_writers("event", _DIRS, target)

            self.assertEqual([], problems)
            self.assertEqual([], line_problems(target))


class TestAppendEventLock(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.tmp = pathlib.Path(self._tmp.name)
        self.debug_file = self.tmp / "debug.log"
        patcher = unittest.mock.patch.dict("os.environ", {core.DEBUG_ENV_VAR: str(self.debug_file)})
        patcher.start()
        self.addCleanup(patcher.stop)

    def debug_text(self):
        return self.debug_file.read_text(encoding="utf-8") if self.debug_file.exists() else ""

    def test_appends_one_line_and_leaves_no_lock_file(self):
        target = self.tmp / "run" / "events.jsonl"

        core.append_event(target, {"event": "probe", "n": 1})
        core.append_event(target, {"event": "probe", "n": 2})

        self.assertEqual(['{"event": "probe", "n": 1}', '{"event": "probe", "n": 2}'],
                         target.read_text(encoding="utf-8").splitlines())
        self.assertFalse(lock_file_of(target).exists())

    def test_a_held_lock_drops_the_event_within_the_bound_and_logs_a_debug_message(self):
        target = self.tmp / "events.jsonl"
        lock = hold_lock(target)

        started = time.monotonic()
        result = core.append_event(target, {"event": "probe"})
        elapsed = time.monotonic() - started

        self.assertIsNone(result)
        self.assertFalse(target.exists())
        self.assertTrue(lock.exists())
        self.assertLess(elapsed, BOUND_SLACK)
        self.assertIn("append_event dropped (lock wait limit)", self.debug_text())
        self.assertIn(str(target), self.debug_text())

    def test_a_writer_waits_for_a_lock_released_mid_wait_and_then_writes_the_event(self):
        target = self.tmp / "events.jsonl"
        hold_lock(target)
        release_after(lock_file_of(target), 0.5)

        started = time.monotonic()
        core.append_event(target, {"event": "probe"})
        elapsed = time.monotonic() - started

        self.assertEqual(['{"event": "probe"}'], target.read_text(encoding="utf-8").splitlines())
        self.assertGreaterEqual(elapsed, 0.4)
        self.assertNotIn("append_event dropped", self.debug_text())

    def test_a_stale_lock_is_broken_and_the_event_written(self):
        # regression/support: the module-level tests carry the stale-break evidence
        target = self.tmp / "events.jsonl"
        hold_lock(target, age_seconds=30)

        core.append_event(target, {"event": "probe"})

        self.assertEqual(['{"event": "probe"}'], target.read_text(encoding="utf-8").splitlines())

    def test_an_unwritable_target_does_not_raise_and_leaves_no_lock_file(self):
        # regression/support: already true before the lock existed
        target = self.tmp / "is_a_directory"
        target.mkdir()

        core.append_event(target, {"event": "probe"})

        self.assertFalse(lock_file_of(target).exists())


if __name__ == "__main__":
    unittest.main()
