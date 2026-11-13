"""Tests for the capture-only invocation of the Claude Code adapter.

The synchronous Agent|Task PreToolUse registration runs the adapter in a
capture-only mode: it records the dispatch (queues one pending dispatch and
adopts the prompt's run id) and nothing else. It writes no event to any stream,
writes no usage records and does not read the transcript.

Covers:
  - Agent and Task dispatches: exactly one queue entry with the prompt's
    instance id, run id, prompt and subagent type; the run id is adopted
  - no event of any kind, in native and in Runner mode, even with a transcript
  - prompts without an instance id, non-dispatch tools, malformed input
  - the entrypoint argument selecting capture-only vs the full dispatch
"""

import contextlib
import io
import json
import os
import pathlib
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
import mosaic_logger_handlers_tools as tools
import mosaic_logger_runstate as runstate
from runner_mode_support import (
    OTHER_RUN_ID, RUN_ID, WorkspaceTestCase, assistant_record, fire_capture,
    read_jsonl, runner_env, write_transcript,
)

_SESSION = "capture-session"
_INSTANCE = "test-writer-tdd#12"
_STATE_DIRS = (".pending-dispatch", ".session-runs")
_FAR_FUTURE = "2999-01-01T00:00:00.000Z"


def _prompt(run_id=RUN_ID, instance=_INSTANCE):
    fields = {"agent_instance_id": instance, "task_description": "do the work"}
    if run_id:
        fields["run_id"] = run_id
    return json.dumps(fields)


def _pre(tool_name="Agent", prompt=None, subagent_type="test-writer-tdd", **extra):
    tool_input = {"description": "dispatch",
                  "prompt": _prompt() if prompt is None else prompt}
    if subagent_type is not None:
        tool_input["subagent_type"] = subagent_type
    return {"tool_name": tool_name, "tool_use_id": "tu-1",
            "tool_input": tool_input, "session_id": _SESSION, **extra}


def _norm(path):
    return os.path.normcase(os.path.abspath(str(path)))


@contextlib.contextmanager
def _recorded_opens():
    """Record every file path opened (builtins.open, io.open, Path.open) while active."""
    opened = set()
    real_open = io.open
    real_path_open = pathlib.Path.open

    def recording_open(file, *args, **kwargs):
        if isinstance(file, (str, bytes, os.PathLike)):
            opened.add(_norm(os.fsdecode(file)))
        return real_open(file, *args, **kwargs)

    def recording_path_open(self, *args, **kwargs):
        opened.add(_norm(self))
        return real_path_open(self, *args, **kwargs)

    with mock.patch("builtins.open", recording_open), \
            mock.patch("io.open", recording_open), \
            mock.patch.object(pathlib.Path, "open", recording_path_open):
        yield opened


class _CaptureCase(WorkspaceTestCase):

    def queue(self, session_id=_SESSION):
        return read_jsonl(self.paths.pending_dispatch_entry(session_id))

    def binding(self, session_id=_SESSION):
        return runstate.get_session_run_binding(
            self.paths, session_id, at=_FAR_FUTURE)

    def written_files(self):
        root = self.paths.root
        if not root.exists():
            return []
        return sorted(p for p in root.rglob("*") if p.is_file())

    def assert_only_dispatch_state_written(self):
        """Nothing but the queue and the session timeline exists in the log tree."""
        for path in self.written_files():
            self.assertIn(path.parent.name, _STATE_DIRS,
                          f"capture-only wrote an unexpected file: {path}")

    def capture(self, **fields):
        with runner_env(self.workspace):
            fire_capture(self.workspace, **fields)


class TestCaptureOnlyQueuesDispatch(_CaptureCase):

    def test_agent_dispatch_queues_exactly_one_entry_with_prompt_fields(self):
        prompt = _prompt()
        self.capture(**_pre("Agent", prompt))
        entries = self.queue()
        self.assertEqual(1, len(entries))
        self.assertEqual(_INSTANCE, entries[0]["agent_instance_id"])
        self.assertEqual(RUN_ID, entries[0]["run_id"])
        self.assertEqual(prompt, entries[0]["prompt"])
        self.assertEqual("test-writer-tdd", entries[0]["agent_type"])

    def test_task_dispatch_queues_exactly_one_entry_with_prompt_fields(self):
        prompt = _prompt()
        self.capture(**_pre("Task", prompt))
        entries = self.queue()
        self.assertEqual(1, len(entries))
        self.assertEqual(_INSTANCE, entries[0]["agent_instance_id"])
        self.assertEqual(RUN_ID, entries[0]["run_id"])
        self.assertEqual(prompt, entries[0]["prompt"])
        self.assertEqual("test-writer-tdd", entries[0]["agent_type"])

    def test_dispatch_adopts_the_prompts_run_id_into_the_session_binding(self):
        self.capture(**_pre("Agent", _prompt(OTHER_RUN_ID)))
        self.assertEqual(OTHER_RUN_ID, self.binding())

    def test_dispatch_without_run_id_queues_with_null_run_and_binds_nothing(self):
        self.capture(**_pre("Agent", _prompt(run_id=None)))
        entries = self.queue()
        self.assertEqual(1, len(entries))
        self.assertIsNone(entries[0]["run_id"])
        self.assertIsNone(self.binding())

    def test_each_dispatch_is_queued_once_in_arrival_order(self):
        self.capture(**_pre("Agent", _prompt(instance="test-writer-tdd#12")))
        self.capture(**_pre("Agent", _prompt(instance="test-writer-tdd#13")))
        self.assertEqual(["test-writer-tdd#12", "test-writer-tdd#13"],
                         [e["agent_instance_id"] for e in self.queue()])

    def test_subagent_type_is_left_out_when_empty_or_missing(self):
        for index, subagent_type in enumerate(("", None)):
            with self.subTest(subagent_type=subagent_type):
                self.capture(**_pre("Agent", _prompt(instance=f"helper#{index}"),
                                    subagent_type=subagent_type))
        self.assertEqual(2, len(self.queue()))
        for entry in self.queue():
            self.assertFalse(entry.get("agent_type"))

    def test_session_without_id_queues_under_the_sessionless_queue(self):
        payload = {"hook_event_name": "PreToolUse", "cwd": str(self.workspace),
                   "tool_name": "Agent", "tool_use_id": "tu-1",
                   "tool_input": {"prompt": _prompt()}}
        with runner_env(self.workspace):
            mosaic_logger.dispatch_capture(json.dumps(payload))
        entries = read_jsonl(self.paths.pending_dispatch_entry(None))
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in entries])
        self.assertIsNone(self.binding(None))

    def test_session_without_id_still_adopts_the_run_id_into_the_context(self):
        payload = {"hook_event_name": "PreToolUse", "cwd": str(self.workspace),
                   "tool_name": "Agent", "tool_use_id": "tu-1",
                   "tool_input": {"prompt": _prompt()}}
        ctx = core.HookContext(payload, self.workspace, self.paths,
                               "2026-11-13T10:00:00.000Z")
        tools.capture_dispatch(ctx)
        self.assertEqual(RUN_ID, ctx.run_id)
        self.assertIsNone(self.binding(None))

    def test_capture_dispatch_with_a_built_context_queues_one_entry(self):
        payload = {"hook_event_name": "PreToolUse", "session_id": _SESSION,
                   **_pre("Agent")}
        ctx = core.HookContext(payload, self.workspace, self.paths,
                               "2026-11-13T10:00:00.000Z")
        tools.capture_dispatch(ctx)
        self.assertEqual(RUN_ID, ctx.run_id)
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in self.queue()])


class TestCaptureSwallowsFailedWrites(_CaptureCase):

    def _ctx(self):
        payload = {"hook_event_name": "PreToolUse", "session_id": _SESSION,
                   **_pre("Agent")}
        return core.HookContext(payload, self.workspace, self.paths,
                                "2026-11-13T10:00:00.000Z")

    def test_failed_binding_write_does_not_prevent_the_queue_write(self):
        with mock.patch.object(runstate, "put_session_run_binding",
                               side_effect=OSError("binding disk full")):
            try:
                tools.capture_dispatch(self._ctx())
            except Exception as exc:
                self.fail(f"capture_dispatch raised {exc!r}")
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in self.queue()])

    def test_failed_queue_write_never_raises(self):
        with mock.patch.object(runstate, "put_pending_dispatch",
                               side_effect=OSError("queue disk full")):
            try:
                tools.capture_dispatch(self._ctx())
            except Exception as exc:
                self.fail(f"capture_dispatch raised {exc!r}")
        self.assertEqual([], self.queue())


class TestCaptureOnlyWritesNoEvent(_CaptureCase):

    def test_native_capture_writes_only_the_queue_and_the_session_timeline(self):
        self.capture(**_pre("Agent"))
        self.assertTrue(self.written_files())
        self.assert_only_dispatch_state_written()
        self.assertEqual([], self.run_folders())

    def test_task_capture_writes_only_the_queue_and_the_session_timeline(self):
        self.capture(**_pre("Task"))
        self.assert_only_dispatch_state_written()
        self.assertEqual([], self.run_folders())

    def test_capture_does_not_read_the_transcript_or_emit_usage(self):
        transcript = write_transcript(
            self.workspace, [assistant_record("hello", message_id="msg_cap")])
        with _recorded_opens() as opened:
            self.capture(**_pre("Agent", transcript_path=transcript))
        self.assertNotIn(_norm(transcript), opened)
        self.assert_only_dispatch_state_written()
        self.assertEqual([], self.run_folders())

    def test_runner_orchestrator_capture_writes_no_event(self):
        transcript = write_transcript(
            self.workspace, [assistant_record("hello", message_id="msg_run")])
        with runner_env(self.workspace, role="orchestrator", run_id=RUN_ID), \
                _recorded_opens() as opened:
            fire_capture(self.workspace, **_pre("Agent", transcript_path=transcript))
        self.assertNotIn(_norm(transcript), opened)
        self.assert_only_dispatch_state_written()
        self.assertEqual([], self.run_folders())

    def test_the_open_recorder_sees_a_read_of_the_transcript(self):
        transcript = write_transcript(self.workspace, [assistant_record("x")])
        with _recorded_opens() as opened:
            with open(transcript, encoding="utf-8") as handle:
                handle.read()
            pathlib.Path(transcript).read_text("utf-8")
        self.assertIn(_norm(transcript), opened)


class TestCaptureOnlyIgnoresNonDispatches(_CaptureCase):

    def test_prompt_without_instance_id_queues_nothing_and_binds_nothing(self):
        prompt = f"please look at run_id: {RUN_ID}"
        self.capture(**_pre("Agent", prompt))
        self.assertEqual([], self.queue())
        self.assertIsNone(self.binding())

    def test_non_dispatch_tool_queues_nothing_and_binds_nothing(self):
        for tool_name in ("Bash", "Read", "Write", "agent", "Tasks"):
            with self.subTest(tool_name=tool_name):
                self.capture(**_pre(tool_name))
        self.assertEqual([], self.queue())
        self.assertIsNone(self.binding())

    def test_missing_or_empty_prompt_queues_nothing(self):
        for tool_input in ({}, {"prompt": ""}, {"prompt": None},
                           {"description": "no prompt"}):
            with self.subTest(tool_input=tool_input):
                self.capture(tool_name="Agent", tool_use_id="tu-1",
                             tool_input=tool_input, session_id=_SESSION)
        self.assertEqual([], self.queue())

    def test_non_object_tool_input_or_non_string_prompt_queues_nothing(self):
        for tool_input in ("a string", ["list"], 7, {"prompt": 42},
                           {"prompt": ["x"]}):
            with self.subTest(tool_input=tool_input):
                self.capture(tool_name="Agent", tool_use_id="tu-1",
                             tool_input=tool_input, session_id=_SESSION)
        self.assertEqual([], self.queue())

    def test_other_hook_events_are_not_captured(self):
        payload = {"hook_event_name": "SubagentStart", "cwd": str(self.workspace),
                   "agent_id": "a1", "agent_type": "test-writer-tdd",
                   "agent_prompt": _prompt(), **_pre("Agent")}
        with runner_env(self.workspace):
            mosaic_logger.dispatch_capture(json.dumps(payload))
        self.assertEqual([], self.queue())
        self.assertEqual([], self.written_files())

    def test_malformed_input_never_raises_and_writes_nothing(self):
        for raw in ("", "not json {{{", "[1, 2, 3]", "null", "42", '"text"', "{}",
                    '{"hook_event_name": "PreToolUse"}'):
            with self.subTest(raw=raw):
                with runner_env(self.workspace):
                    try:
                        mosaic_logger.dispatch_capture(raw)
                    except Exception as exc:
                        self.fail(f"dispatch_capture raised {exc!r} for {raw!r}")
        self.assertEqual([], self.written_files())

    def test_non_string_input_never_raises(self):
        with runner_env(self.workspace):
            for raw in (None, 5, b"bytes"):
                with self.subTest(raw=raw):
                    try:
                        mosaic_logger.dispatch_capture(raw)
                    except Exception as exc:
                        self.fail(f"dispatch_capture raised {exc!r}")


class _StdinCase(_CaptureCase):

    def run_main(self, argv, payload):
        """Run main(argv) with the payload on stdin; return (exit code, stdout text)."""
        out = io.StringIO()
        with runner_env(self.workspace), \
                mock.patch.object(sys, "stdin", io.StringIO(json.dumps(payload))), \
                contextlib.redirect_stdout(out):
            code = mosaic_logger.main(argv)
        return code, out.getvalue()

    def payload(self):
        return {"hook_event_name": "PreToolUse", "cwd": str(self.workspace),
                **_pre("Agent")}


class TestEntrypointArgument(_StdinCase):

    def test_capture_argument_queues_the_dispatch_and_writes_no_event(self):
        argv = ["mosaic_logger.py", mosaic_logger.CAPTURE_DISPATCH_ARG]
        code, stdout = self.run_main(argv, self.payload())
        self.assertEqual(0, code)
        self.assertEqual("", stdout)
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in self.queue()])
        self.assert_only_dispatch_state_written()
        self.assertEqual([], self.run_folders())

    def test_no_argument_runs_the_full_dispatch_which_queues_nothing(self):
        code, stdout = self.run_main(["mosaic_logger.py"], self.payload())
        self.assertEqual(0, code)
        self.assertEqual("", stdout)
        self.assertEqual([], self.queue())
        events = self.orchestrator_events("unknown-run")
        self.assertIn("tool_call_start", [e["event"] for e in events])

    def test_another_argument_selects_the_full_dispatch(self):
        code, _ = self.run_main(["mosaic_logger.py", "something-else"],
                                self.payload())
        self.assertEqual(0, code)
        self.assertEqual([], self.queue())
        events = self.orchestrator_events("unknown-run")
        self.assertIn("tool_call_start", [e["event"] for e in events])

    def test_argument_is_compared_exactly(self):
        argv = ["mosaic_logger.py", mosaic_logger.CAPTURE_DISPATCH_ARG.upper()]
        self.run_main(argv, self.payload())
        self.assertEqual([], self.queue())

    def test_arguments_after_the_first_are_ignored(self):
        argv = ["mosaic_logger.py", mosaic_logger.CAPTURE_DISPATCH_ARG, "extra"]
        self.run_main(argv, self.payload())
        self.assertEqual(1, len(self.queue()))

    def test_capture_argument_exits_zero_on_unreadable_stdin_content(self):
        argv = ["mosaic_logger.py", mosaic_logger.CAPTURE_DISPATCH_ARG]
        out = io.StringIO()
        with runner_env(self.workspace), \
                mock.patch.object(sys, "stdin", io.StringIO("not json {{{")), \
                contextlib.redirect_stdout(out):
            code = mosaic_logger.main(argv)
        self.assertEqual(0, code)
        self.assertEqual("", out.getvalue())
        self.assertEqual([], self.written_files())

    def test_capture_argument_exits_zero_when_stdin_cannot_be_read(self):
        class _BrokenStdin:
            def read(self, *args):
                raise OSError("stdin closed")

            def __iter__(self):
                raise OSError("stdin closed")

        argv = ["mosaic_logger.py", mosaic_logger.CAPTURE_DISPATCH_ARG]
        out = io.StringIO()
        with runner_env(self.workspace), \
                mock.patch.object(sys, "stdin", _BrokenStdin()), \
                contextlib.redirect_stdout(out):
            code = mosaic_logger.main(argv)
        self.assertEqual(0, code)
        self.assertEqual("", out.getvalue())


if __name__ == "__main__":
    unittest.main()
