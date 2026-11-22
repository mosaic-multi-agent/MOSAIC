"""Tests for the pending-dispatch capture side in mosaic_logger_runstate
(ghcp-cli variant).

In GHCP CLI, the pending-dispatch queue bridges the gap between preToolUse
(where agent/task tool input contains the agent_instance_id in 'toolArgs.prompt')
and subagentStart (where the agentId is first known). Consumption (type-matched
take, stale and corrupt entries, atomicity) is covered in
test_pending_dispatch_take / _robustness / _concurrency.

Covers:
- put_pending_dispatch: persistence, JSONL append (order), parent-dir creation,
  success/failure return, never-raises, prompt field round-trip through take
- Concurrent write safety
"""

import datetime
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import textwrap
import threading
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
from runner_mode_support import LazyModule

pending = LazyModule("mosaic_logger_pending_dispatch")

_RUN_ID = "20260101T120000Z-d7e3"
_SESSION_ID = "ghcp-sess-pd-main"
_SESSION_ID_ALT = "ghcp-sess-pd-alt"
_AGENT_INSTANCE_ID = "planner-tdd-soft#6"
_AGENT_TYPE = "planner-tdd-soft"
_EXTRACTED_RUN_ID = "20260101T110000Z-a1b2"

_ADAPTER_ROOT = pathlib.Path(__file__).parent.parent

_DISPATCH_PROMPT = (
    '{"agent_instance_id": "planner-tdd-soft#6", '
    '"run_id": "20260101T110000Z-a1b2", '
    '"task_description": "Write the plan."}'
)


def _fresh_timestamp():
    """Return the current UTC time as ISO 8601 ms timestamp."""
    now = datetime.datetime.now(datetime.timezone.utc)
    ms = now.microsecond // 1000
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ms:03d}Z"


def _read_jsonl(path):
    """Read all non-empty lines of a JSONL file as parsed dicts."""
    return [json.loads(ln) for ln in pathlib.Path(path).read_text("utf-8").splitlines()
            if ln.strip()]


class TestPutPendingDispatch(unittest.TestCase):
    """put_pending_dispatch persists a pending-dispatch entry for the given session."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.paths = core.build_paths(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def _put(self, session_id=_SESSION_ID, agent_instance_id=_AGENT_INSTANCE_ID,
             extracted_run_id=_EXTRACTED_RUN_ID, path_run_id=_RUN_ID):
        return runstate.put_pending_dispatch(
            self.paths, path_run_id, session_id, agent_instance_id, extracted_run_id
        )

    def _read_queue(self, session_id=_SESSION_ID, path_run_id=_RUN_ID):
        entry_path = self.paths.pending_dispatch_entry(path_run_id, session_id)
        return _read_jsonl(entry_path)

    def test_returns_true_on_success(self):
        result = self._put()
        self.assertTrue(result)

    def test_creates_entry_file(self):
        self._put()
        entry_path = self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        self.assertTrue(entry_path.exists())

    def test_entry_has_agent_instance_id(self):
        self._put(agent_instance_id="contracts-designer#4")
        entries = self._read_queue()
        self.assertEqual(1, len(entries))
        self.assertEqual("contracts-designer#4", entries[0]["agent_instance_id"])

    def test_entry_has_run_id(self):
        self._put(extracted_run_id="20260101T090000Z-cc11")
        entries = self._read_queue()
        self.assertEqual("20260101T090000Z-cc11", entries[0]["run_id"])

    def test_entry_has_null_run_id_when_none(self):
        """When extracted_run_id is None it must be stored as null (not omitted)."""
        self._put(extracted_run_id=None)
        entries = self._read_queue()
        self.assertIn("run_id", entries[0])
        self.assertIsNone(entries[0]["run_id"])

    def test_entry_has_created_at_field(self):
        self._put()
        entries = self._read_queue()
        self.assertIn("created_at", entries[0])

    def test_created_at_is_iso8601_utc_ms(self):
        self._put()
        entries = self._read_queue()
        ts = entries[0]["created_at"]
        self.assertRegex(ts, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$")

    def test_creates_parent_directory(self):
        pending_dir = self.paths.pending_dispatch_dir(_RUN_ID)
        self.assertFalse(pending_dir.exists())
        self._put()
        self.assertTrue(pending_dir.exists())

    def test_multiple_calls_append_in_order(self):
        """Multiple calls for the same session must append, preserving insertion order."""
        self._put(agent_instance_id="agent-alpha#1")
        self._put(agent_instance_id="agent-beta#2")
        self._put(agent_instance_id="agent-gamma#3")
        entries = self._read_queue()
        self.assertEqual(3, len(entries))
        self.assertEqual("agent-alpha#1", entries[0]["agent_instance_id"])
        self.assertEqual("agent-beta#2", entries[1]["agent_instance_id"])
        self.assertEqual("agent-gamma#3", entries[2]["agent_instance_id"])

    def test_different_sessions_write_to_separate_files(self):
        self._put(session_id="session-x", agent_instance_id="agent-x#1")
        self._put(session_id="session-y", agent_instance_id="agent-y#2")
        entries_x = self._read_queue(session_id="session-x")
        entries_y = self._read_queue(session_id="session-y")
        self.assertEqual(1, len(entries_x))
        self.assertEqual(1, len(entries_y))
        self.assertEqual("agent-x#1", entries_x[0]["agent_instance_id"])
        self.assertEqual("agent-y#2", entries_y[0]["agent_instance_id"])

    def test_returns_false_on_io_failure(self):
        pending_dir = self.paths.pending_dispatch_dir(_RUN_ID)
        pending_dir.parent.mkdir(parents=True, exist_ok=True)
        pending_dir.write_bytes(b"blocker")
        result = self._put()
        self.assertFalse(result)

    def test_never_raises(self):
        pending_dir = self.paths.pending_dispatch_dir(_RUN_ID)
        pending_dir.parent.mkdir(parents=True, exist_ok=True)
        pending_dir.write_bytes(b"blocker")
        try:
            runstate.put_pending_dispatch(
                self.paths, _RUN_ID, _SESSION_ID, _AGENT_INSTANCE_ID, None
            )
        except Exception as exc:
            self.fail(f"put_pending_dispatch raised unexpectedly: {exc}")


class TestPutPendingDispatchPromptField(unittest.TestCase):
    """put_pending_dispatch accepts and persists a 'prompt' keyword argument.
    take_pending_dispatch returns the stored prompt value.

    In GHCP CLI, this prompt is the raw text from toolArgs.prompt on a preToolUse
    event with toolName 'agent' or 'task'. The full text is stored so that
    subagentStart can re-read it if needed.
    """

    _SAMPLE_PROMPT = (
        '{"agent_instance_id": "test-writer-tdd#18", '
        '"run_id": "20260731T132637Z-58f7", '
        '"task_description": "Write tests for ghcp-cli adapter."}'
    )

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.paths = core.build_paths(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def test_put_accepts_prompt_keyword_argument(self):
        try:
            result = runstate.put_pending_dispatch(
                self.paths, _RUN_ID, _SESSION_ID,
                _AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID,
                prompt=self._SAMPLE_PROMPT,
            )
        except TypeError as exc:
            self.fail(
                "put_pending_dispatch does not accept 'prompt' keyword argument: "
                f"{exc}"
            )
        self.assertTrue(result)

    def test_put_persists_prompt_text_in_queue_entry(self):
        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID,
            _AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID,
            prompt=self._SAMPLE_PROMPT,
        )
        entry_path = self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        self.assertTrue(entry_path.exists())
        line = entry_path.read_text("utf-8").strip().splitlines()[0]
        entry = json.loads(line)
        self.assertIn("prompt", entry)
        self.assertEqual(self._SAMPLE_PROMPT, entry["prompt"])

    def test_prompt_key_absent_from_entry_when_not_given(self):
        """When no prompt is given, the queue entry must not contain a 'prompt' key."""
        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, _AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID
        )
        entry_path = self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        line = entry_path.read_text("utf-8").strip().splitlines()[0]
        entry = json.loads(line)
        self.assertNotIn("prompt", entry)

    def test_take_returns_correct_prompt_value(self):
        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID,
            _AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID,
            prompt=self._SAMPLE_PROMPT,
        )
        result = pending.take_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, _AGENT_TYPE)
        self.assertIsNotNone(result)
        self.assertEqual(self._SAMPLE_PROMPT, result.prompt)

    def test_take_returns_none_for_prompt_when_entry_lacks_prompt_field(self):
        """Backward-compatible: old-format entry without 'prompt' yields None."""
        entry_path = self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        entry_path.parent.mkdir(parents=True, exist_ok=True)
        entry_path.write_text(
            json.dumps({
                "agent_instance_id": _AGENT_INSTANCE_ID,
                "run_id": _EXTRACTED_RUN_ID,
                "created_at": _fresh_timestamp(),
            }) + "\n",
            encoding="utf-8",
        )
        result = pending.take_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, _AGENT_TYPE)
        self.assertIsNotNone(result)
        self.assertIsNone(result.prompt)


class TestConcurrentPutPendingDispatch(unittest.TestCase):
    """Multiple concurrent put_pending_dispatch calls for the same session must not
    lose entries. Uses real subprocesses to exercise OS-level concurrent writes."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_concurrent_writes_produce_no_torn_lines(self):
        """All concurrent put_pending_dispatch subprocesses must produce intact
        JSONL lines (no torn writes)."""
        n_concurrent = 6
        workspace = self.tmp.name

        helper_script = textwrap.dedent(f"""\
            import sys, os, pathlib
            sys.path.insert(0, {str(_ADAPTER_ROOT)!r})
            import mosaic_logger_core as core
            import mosaic_logger_runstate as runstate
            idx = sys.argv[1]
            workspace_root = pathlib.Path({workspace!r})
            paths = core.build_paths(workspace_root)
            runstate.put_pending_dispatch(
                paths, {_RUN_ID!r}, {_SESSION_ID!r},
                f"concurrent-agent#{{idx}}", None
            )
        """)

        results = [None] * n_concurrent
        threads = []

        def run_subprocess(idx):
            try:
                r = subprocess.run(
                    ["py", "-c", helper_script, str(idx)],
                    capture_output=True,
                    timeout=15,
                )
                results[idx] = r
            except Exception as exc:
                results[idx] = exc

        for i in range(n_concurrent):
            t = threading.Thread(target=run_subprocess, args=(i,), daemon=True)
            threads.append(t)
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=20)

        for i, r in enumerate(results):
            if isinstance(r, Exception):
                self.fail(f"Subprocess {i} raised: {r}")
            self.assertEqual(0, r.returncode,
                             f"Subprocess {i} failed: {r.stderr.decode()[:300]}")

        paths = core.build_paths(pathlib.Path(workspace))
        entry_path = paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        self.assertTrue(entry_path.exists())

        lines = entry_path.read_text("utf-8").splitlines()
        non_empty = [ln for ln in lines if ln.strip()]
        self.assertEqual(n_concurrent, len(non_empty))

        for i, ln in enumerate(non_empty):
            try:
                obj = json.loads(ln)
            except json.JSONDecodeError as exc:
                self.fail(f"Line {i+1} is not valid JSON (torn write?): {exc!r}")
            self.assertIn("agent_instance_id", obj)


if __name__ == "__main__":
    unittest.main()
