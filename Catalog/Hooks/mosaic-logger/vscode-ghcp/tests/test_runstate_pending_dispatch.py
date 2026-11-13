"""Tests for the pending-dispatch correlation mechanism in mosaic_logger_runstate
(vscode-ghcp variant).

In vscode-ghcp, the pending-dispatch queue bridges the gap between PreToolUse
(where the Agent/Task/runSubagent tool input contains the agent_instance_id
in tool_input.prompt) and SubagentStart (where agent_name is first known).
The queue mechanism is identical in behavior to the claude-code and ghcp-cli
variants.

Covers (the runstate delegation of the queue's put side; consumption, staleness,
corruption and cross-process behaviour are covered by test_pending_dispatch_*.py):
- put_pending_dispatch: persistence, JSONL append (FIFO), parent-dir creation,
  success/failure return, never-raises, prompt field round-trip, agent_type
- pop_pending_dispatch no longer exists (untyped FIFO consumption was removed)
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


_RUN_ID = "20260101T120000Z-d7e3"
_SESSION_ID = "vscode-sess-pd-main"
_SESSION_ID_ALT = "vscode-sess-pd-alt"
_AGENT_INSTANCE_ID = "planner-tdd-soft#12"
_EXTRACTED_RUN_ID = "20260101T110000Z-a1b2"

_ADAPTER_ROOT = pathlib.Path(__file__).parent.parent
_SHARED_ROOT = _ADAPTER_ROOT.parent / "copilot-shared"

_DISPATCH_PROMPT = (
    '{"agent_instance_id": "planner-tdd-soft#12", '
    '"run_id": "20260101T110000Z-a1b2", '
    '"task_description": "Write the plan."}'
)


def _old_timestamp():
    """Return an ISO 8601 UTC timestamp that is definitely stale (year 2020)."""
    return "2020-01-01T00:00:00.000Z"


def _fresh_timestamp():
    """Return the current UTC time as ISO 8601 ms timestamp."""
    now = datetime.datetime.now(datetime.timezone.utc)
    ms = now.microsecond // 1000
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ms:03d}Z"


def _read_jsonl(path):
    """Read all non-empty lines of a JSONL file as parsed dicts."""
    return [json.loads(ln) for ln in pathlib.Path(path).read_text("utf-8").splitlines()
            if ln.strip()]


def _write_pending_dispatch_file(path, entries):
    """Write a JSONL pending-dispatch file directly (for test setup)."""
    path = pathlib.Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        for entry in entries:
            fh.write(json.dumps(entry) + "\n")


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
        """When extracted_run_id is None it must be stored as null (not omitted).
        This is adapter-internal state, not an emitted event; the omit-null rule
        does not apply here."""
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

    In vscode-ghcp, the prompt is the text from tool_input.prompt on a
    PreToolUse event with tool_name in SUBAGENT_DISPATCH_TOOL_NAMES. The full
    text is stored so that SubagentStart can recover agent_instance_id and
    run_id from it.
    """

    _SAMPLE_PROMPT = (
        '{"agent_instance_id": "test-writer-tdd#18", '
        '"run_id": "20260802T181610Z-e66f", '
        '"task_description": "Write failing tests for Stage 1."}'
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


class TestRunstateDelegatesTheQueue(unittest.TestCase):
    """runstate keeps the put side as a delegation to the queue module and no longer
    offers untyped first-in-first-out consumption."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.paths = core.build_paths(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def test_pop_pending_dispatch_no_longer_exists(self):
        self.assertFalse(hasattr(runstate, "pop_pending_dispatch"))

    def test_the_age_limit_is_the_queue_modules_limit(self):
        from pending_dispatch_support import pending
        self.assertEqual(pending.PENDING_DISPATCH_MAX_AGE_SECONDS,
                         runstate.PENDING_DISPATCH_MAX_AGE_SECONDS)
        self.assertEqual(120, runstate.PENDING_DISPATCH_MAX_AGE_SECONDS)

    def test_put_accepts_and_stores_an_agent_type(self):
        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, _AGENT_INSTANCE_ID, None, agent_type="Planner")
        path = self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID)
        self.assertEqual("Planner", _read_jsonl(path)[0]["agent_type"])

    def test_an_entry_put_through_runstate_is_taken_by_its_type(self):
        from pending_dispatch_support import pending
        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, _AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID,
            prompt=_DISPATCH_PROMPT)
        got = pending.take_pending_dispatch(self.paths, _RUN_ID, _SESSION_ID, "planner-tdd-soft")
        self.assertEqual((_AGENT_INSTANCE_ID, _EXTRACTED_RUN_ID, _DISPATCH_PROMPT), tuple(got))


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
            sys.path.insert(0, {str(_SHARED_ROOT)!r})
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
                    [sys.executable, "-c", helper_script, str(idx)],
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
