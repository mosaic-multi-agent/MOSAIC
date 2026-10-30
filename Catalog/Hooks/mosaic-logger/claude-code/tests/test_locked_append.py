"""The claude-code copy of the locked append module behaves as the log format requires.

Parallel writers lose, tear or blank no line; a writer that cannot get the lock in time drops
the line and reports it without raising; a stale lock is broken. The copy is standard library
only and byte-identical to the shared copy.
"""

import ast
import importlib
import os
import pathlib
import sys
import tempfile
import time
import unittest

_TESTS = os.path.dirname(os.path.abspath(__file__))
_ADAPTER_DIR = pathlib.Path(_TESTS).parent
_MODULE = "mosaic_logger_locked_append"
sys.path.insert(0, str(_ADAPTER_DIR))
sys.path.insert(0, _TESTS)

from locked_append_support import hold_lock, line_problems, lock_file_of, release_after, run_writers

LINE = b'{"event":"probe"}\n'
BOUND_SLACK = 6.0
RELEASE_AFTER = 0.5  # a lock released this long into the wait
WAITED_AT_LEAST = 0.4


class TestLockedAppendCopy(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.tmp = pathlib.Path(self._tmp.name)

    def module(self):
        return importlib.import_module(_MODULE)

    def test_parallel_writers_of_long_lines_lose_tear_or_blank_no_line(self):
        target = self.tmp / "run" / "00_orchestrator_events.jsonl"

        problems = run_writers("locked", [_ADAPTER_DIR], target)

        self.assertEqual([], problems)
        self.assertEqual([], line_problems(target))

    def test_limits_are_the_values_of_the_log_format(self):
        la = self.module()

        self.assertEqual(2.0, la.WAIT_LIMIT_SECONDS)
        self.assertEqual(10.0, la.STALE_AFTER_SECONDS)

    def test_appends_creates_the_directory_and_leaves_no_lock_file(self):
        la = self.module()
        target = self.tmp / "new" / "events.jsonl"

        written = la.locked_append(target, LINE)

        self.assertTrue(written)
        self.assertEqual(LINE, target.read_bytes())
        self.assertFalse(lock_file_of(target).exists())

    def test_a_held_lock_drops_the_line_within_the_bound_and_reports_once(self):
        la = self.module()
        target = self.tmp / "events.jsonl"
        hold_lock(target)
        reports = []

        started = time.monotonic()
        written = la.locked_append(target, LINE, lambda message, exc: reports.append((message, exc)))
        elapsed = time.monotonic() - started

        self.assertFalse(written)
        self.assertFalse(target.exists())
        self.assertLess(elapsed, BOUND_SLACK)
        self.assertEqual(1, len(reports))
        self.assertIsNone(reports[0][1])
        self.assertIn(target.name, reports[0][0])

    def test_a_writer_waits_for_a_lock_released_mid_wait_and_then_writes(self):
        la = self.module()
        target = self.tmp / "events.jsonl"
        hold_lock(target)
        reports = []
        release_after(lock_file_of(target), RELEASE_AFTER)

        started = time.monotonic()
        written = la.locked_append(target, LINE, lambda message, exc: reports.append(message))
        elapsed = time.monotonic() - started

        self.assertTrue(written)
        self.assertEqual(LINE, target.read_bytes())
        self.assertGreaterEqual(elapsed, WAITED_AT_LEAST)
        self.assertEqual([], reports)
        self.assertFalse(lock_file_of(target).exists())

    def test_holding_lock_waits_for_a_lock_released_mid_wait_and_then_yields_true(self):
        la = self.module()
        target = self.tmp / "events.jsonl"
        hold_lock(target)
        release_after(lock_file_of(target), RELEASE_AFTER)

        started = time.monotonic()
        with la.holding_lock(target) as held:
            inside = lock_file_of(target).exists()
        elapsed = time.monotonic() - started

        self.assertTrue(held)
        self.assertTrue(inside)
        self.assertGreaterEqual(elapsed, WAITED_AT_LEAST)
        self.assertFalse(lock_file_of(target).exists())

    def test_a_lock_a_few_seconds_old_is_not_stale_and_the_line_is_dropped(self):
        la = self.module()
        target = self.tmp / "events.jsonl"
        lock = hold_lock(target, age_seconds=5)

        written = la.locked_append(target, LINE)

        self.assertFalse(written)
        self.assertFalse(target.exists())
        self.assertTrue(lock.exists())

    def test_a_stale_lock_is_broken_and_the_line_written(self):
        la = self.module()
        target = self.tmp / "events.jsonl"
        hold_lock(target, age_seconds=30)

        self.assertTrue(la.locked_append(target, LINE))
        self.assertEqual(LINE, target.read_bytes())

    def test_the_copy_is_identical_to_the_shared_copy(self):
        own = (_ADAPTER_DIR / (_MODULE + ".py")).read_bytes()
        shared = (_ADAPTER_DIR.parent / "copilot-shared" / (_MODULE + ".py")).read_bytes()

        self.assertEqual(shared, own)

    def test_the_module_imports_only_the_standard_library(self):
        tree = ast.parse((_ADAPTER_DIR / (_MODULE + ".py")).read_text(encoding="utf-8"))
        names = set()
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                names.update(alias.name.split(".")[0] for alias in node.names)
            elif isinstance(node, ast.ImportFrom):
                self.assertEqual(0, node.level)
                names.add((node.module or "").split(".")[0])

        self.assertTrue(names <= set(sys.stdlib_module_names), names)


if __name__ == "__main__":
    unittest.main()
