"""Tests for the locked usage state of emit_usage_records.

Covers:
  - A firing that cannot get the stream's state lock emits nothing and leaves
    the state file untouched; with the lock free, emission is unchanged
  - The state lock is held from the state load to the state save, including
    while events are appended, and is released afterwards
  - State read outcomes: a read error on an existing state file emits nothing
    (with a debug line, state untouched); invalid content is treated as empty;
    an absent file emits everything
  - After the Runner-mode state reset, the stream's records are emitted again
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_handlers_runner as handlers_runner
import mosaic_logger_locked_append as locked_append
import mosaic_logger_usage as usage

from locked_append_support import hold_lock, lock_file_of

_RUN_ID = "20260101T170000Z-c9a1"
_TS = "2026-01-01T17:00:00.000Z"
_SOURCE = "orchestrator_transcript"


def _record(message_id, input_tokens):
    return {"type": "assistant", "message": {
        "id": message_id, "model": "claude-opus-4-5",
        "usage": {"input_tokens": input_tokens}}}


def _write_transcript(path, records):
    pathlib.Path(path).write_text(
        "\n".join(json.dumps(r) for r in records) + "\n", encoding="utf-8")


def _usage_events(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    events = [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]
    return [e for e in events if e.get("event") == usage.USAGE_EVENT]


class _StateLockCase(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = pathlib.Path(self._tmp.name)
        paths = core.build_paths(self.root)
        self.ctx = core.HookContext(
            {"hook_event_name": "Stop", "session_id": "s-lock"}, self.root, paths, _TS)
        self.ctx.run_id = _RUN_ID
        self.sink = core.event_sink_for(self.ctx, None)
        self.key = usage.stream_key_for(self.sink)
        self.state_path = self.ctx.paths.usage_state_entry(_RUN_ID, self.key)
        self.transcript = self.root / "t.jsonl"

    def emit(self):
        return usage.emit_usage_records(
            self.ctx, str(self.transcript), None, _SOURCE)

    def write_two(self):
        _write_transcript(self.transcript, [_record("msg_a", 5), _record("msg_b", 7)])

    def seed_state_from_first_emission(self):
        self.write_two()
        self.assertEqual(2, self.emit())

    def ids_emitted(self):
        return [e["record_id"] for e in _usage_events(self.sink)]


class LockedEmissionTests(_StateLockCase):
    def setUp(self):
        super().setUp()
        patcher = mock.patch.object(locked_append, "WAIT_LIMIT_SECONDS", 0.1)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_lock_held_elsewhere_emits_nothing_and_writes_no_state(self):
        self.write_two()
        hold_lock(self.state_path)

        count = self.emit()

        self.assertEqual(0, count)
        self.assertEqual([], self.ids_emitted())
        self.assertFalse(self.state_path.exists())

    def test_lock_held_elsewhere_leaves_existing_state_unchanged(self):
        self.seed_state_from_first_emission()
        before = self.state_path.read_bytes()
        _write_transcript(self.transcript, [
            _record("msg_a", 5), _record("msg_b", 70), _record("msg_c", 9)])
        hold_lock(self.state_path)

        count = self.emit()

        self.assertEqual(0, count)
        self.assertEqual(2, len(self.ids_emitted()))
        self.assertEqual(before, self.state_path.read_bytes())

    def test_firing_after_the_lock_is_free_catches_up(self):
        self.write_two()
        lock = hold_lock(self.state_path)
        self.assertEqual(0, self.emit())
        lock.unlink()

        count = self.emit()

        self.assertEqual(2, count)

    def test_lock_free_emits_new_and_changed_and_skips_unchanged(self):
        self.seed_state_from_first_emission()
        _write_transcript(self.transcript, [
            _record("msg_a", 5), _record("msg_b", 70), _record("msg_c", 9)])

        count = self.emit()

        self.assertEqual(2, count)
        self.assertEqual(["msg_a", "msg_b", "msg_b", "msg_c"], sorted(self.ids_emitted()))

    def test_lock_is_held_while_loading_appending_and_saving(self):
        self.write_two()
        lock = lock_file_of(self.state_path)
        seen = {"load": [], "append": [], "save": []}
        real_load, real_append, real_save = (
            usage.load_usage_state, core.append_event, usage.save_usage_state)

        def load(*a, **k):
            seen["load"].append(lock.exists())
            return real_load(*a, **k)

        def append(*a, **k):
            seen["append"].append(lock.exists())
            return real_append(*a, **k)

        def save(*a, **k):
            seen["save"].append(lock.exists())
            return real_save(*a, **k)

        with mock.patch.object(usage, "load_usage_state", load), \
                mock.patch.object(core, "append_event", append), \
                mock.patch.object(usage, "save_usage_state", save):
            self.assertEqual(2, self.emit())

        self.assertEqual([True], seen["load"])
        self.assertEqual([True, True], seen["append"])
        self.assertEqual([True], seen["save"])

    def test_lock_is_released_after_emission(self):
        self.write_two()

        self.emit()

        self.assertFalse(lock_file_of(self.state_path).exists())


class StateReadOutcomeTests(_StateLockCase):
    def _read_text_failing_for_state(self):
        real = pathlib.Path.read_text
        state = self.state_path

        def read_text(path_self, *a, **k):
            if pathlib.Path(path_self) == state:
                raise PermissionError("state file is being replaced")
            return real(path_self, *a, **k)

        return mock.patch.object(pathlib.Path, "read_text", read_text)

    def test_read_error_on_existing_state_emits_nothing(self):
        self.seed_state_from_first_emission()
        _write_transcript(self.transcript, [
            _record("msg_a", 5), _record("msg_b", 7), _record("msg_c", 9)])

        with self._read_text_failing_for_state():
            count = self.emit()

        self.assertEqual(0, count)
        self.assertEqual(2, len(self.ids_emitted()))

    def test_read_error_does_not_overwrite_the_state_file(self):
        self.seed_state_from_first_emission()
        before = self.state_path.read_bytes()
        _write_transcript(self.transcript, [_record("msg_a", 50), _record("msg_c", 9)])

        with self._read_text_failing_for_state():
            self.emit()

        self.assertEqual(before, self.state_path.read_bytes())

    def test_read_error_writes_a_debug_line_naming_the_state_file(self):
        self.seed_state_from_first_emission()

        with self._read_text_failing_for_state(), \
                mock.patch.object(core, "debug_log") as debug:
            self.emit()

        messages = [str(c.args[0]) for c in debug.call_args_list if c.args]
        self.assertTrue(
            any(self.state_path.name in m for m in messages),
            "no debug line names the state file: %r" % messages)

    def test_read_error_releases_the_lock(self):
        self.seed_state_from_first_emission()

        with self._read_text_failing_for_state():
            self.emit()

        self.assertFalse(lock_file_of(self.state_path).exists())

    def test_invalid_state_content_is_treated_as_empty(self):
        invalid = {
            "corrupt json": "{not json",
            "wrong version": json.dumps({"version": 99, "records": {"msg_a": "x"}}),
            "wrong shape": json.dumps({"version": 1, "records": ["msg_a"]}),
        }
        for label, content in invalid.items():
            with self.subTest(label):
                self.state_path.parent.mkdir(parents=True, exist_ok=True)
                self.state_path.write_text(content, encoding="utf-8")
                self.write_two()

                self.assertEqual(2, self.emit())

    def test_absent_state_file_emits_everything(self):
        self.write_two()
        self.assertFalse(self.state_path.exists())

        count = self.emit()

        self.assertEqual(2, count)
        self.assertTrue(self.state_path.exists())


class RunnerResetTests(_StateLockCase):
    def test_records_are_emitted_again_after_the_runner_state_reset(self):
        instance = "worker#1"
        sink = self.ctx.paths.invocation_events(_RUN_ID, instance)
        self.ctx.transcript_path = str(self.transcript)
        self.write_two()
        first = usage.emit_usage_records(
            self.ctx, str(self.transcript), instance, "agent_transcript")
        again = usage.emit_usage_records(
            self.ctx, str(self.transcript), instance, "agent_transcript")
        self.assertEqual((2, 0), (first, again))

        handlers_runner._emit_session_usage(self.ctx, _RUN_ID, instance, sink)

        self.assertEqual(4, len(_usage_events(sink)))
        self.assertEqual(
            ["msg_a", "msg_a", "msg_b", "msg_b"],
            sorted(e["record_id"] for e in _usage_events(sink)))


if __name__ == "__main__":
    unittest.main()
