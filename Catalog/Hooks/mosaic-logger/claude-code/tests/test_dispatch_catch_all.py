"""Tests for the async catch-all PreToolUse on Agent and Task dispatches.

The catch-all entry logs every tool call. For an Agent or Task dispatch it still
writes tool_call_start (and usage records where enabled) but no longer queues the
dispatch or adopts its run id: only the capture-only entry does that.

Covers:
  - tool_call_start and usage records for Agent and Task, as for any tool
  - nothing queued and no run adopted from the prompt
  - where the dispatch's own tool_call_start lands, in both firing orders
"""

import json
import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
import mosaic_logger_handlers_tools as tools
import mosaic_logger_runstate as runstate
import mosaic_logger_usage as usage
from runner_mode_support import (
    OTHER_RUN_ID, RUN_ID, WorkspaceTestCase, assistant_record, fire, fire_capture,
    read_jsonl, runner_env, write_transcript,
)

_SESSION = "catch-all-session"
_INSTANCE = "test-writer-tdd#12"
_FAR_FUTURE = "2999-01-01T00:00:00.000Z"


def _dispatch_fields(tool_name, run_id=OTHER_RUN_ID, **extra):
    prompt = json.dumps({"agent_instance_id": _INSTANCE, "run_id": run_id,
                         "task_description": "do the work"})
    return {"session_id": _SESSION, "tool_name": tool_name,
            "tool_use_id": f"tu-{tool_name}",
            "tool_input": {"prompt": prompt, "subagent_type": "test-writer-tdd"},
            **extra}


class _CatchAllCase(WorkspaceTestCase):

    def catch_all(self, tool_name, run_id=OTHER_RUN_ID, **extra):
        with runner_env(self.workspace):
            fire(self.workspace, "PreToolUse",
                 **_dispatch_fields(tool_name, run_id, **extra))

    def capture(self, tool_name, run_id=OTHER_RUN_ID):
        with runner_env(self.workspace):
            fire_capture(self.workspace, **_dispatch_fields(tool_name, run_id))

    def queue(self):
        return read_jsonl(self.paths.pending_dispatch_entry(_SESSION))

    def binding(self):
        return runstate.get_session_run_binding(
            self.paths, _SESSION, at=_FAR_FUTURE)

    def names(self, run_id):
        return [e["event"] for e in self.orchestrator_events(run_id)]


class TestCatchAllDoesNotQueue(_CatchAllCase):

    def test_agent_dispatch_queues_nothing(self):
        self.catch_all("Agent")
        self.assertEqual([], self.queue())

    def test_task_dispatch_queues_nothing(self):
        self.catch_all("Task")
        self.assertEqual([], self.queue())

    def test_dispatch_prompt_run_id_is_not_adopted_into_the_session(self):
        for tool_name in ("Agent", "Task"):
            with self.subTest(tool_name=tool_name):
                self.catch_all(tool_name)
        self.assertIsNone(self.binding())
        self.assertFalse(self.paths.run_root(OTHER_RUN_ID).exists())

    def test_handler_called_directly_queues_nothing_and_adopts_nothing(self):
        for tool_name in ("Agent", "Task"):
            with self.subTest(tool_name=tool_name):
                payload = {"hook_event_name": "PreToolUse",
                           **_dispatch_fields(tool_name)}
                ctx = core.HookContext(payload, self.workspace, self.paths,
                                       "2026-11-13T10:00:00.000Z")
                tools.handle_pre_tool_use(ctx)
                self.assertIsNone(ctx.run_id)
        self.assertEqual([], self.queue())
        self.assertIsNone(self.binding())

    def test_handler_never_raises_for_dispatch_tools(self):
        for tool_name in ("Agent", "Task"):
            with self.subTest(tool_name=tool_name):
                payload = {"hook_event_name": "PreToolUse",
                           **_dispatch_fields(tool_name)}
                ctx = core.HookContext(payload, self.workspace, self.paths,
                                       "2026-11-13T10:00:00.000Z")
                try:
                    tools.handle_pre_tool_use(ctx)
                except Exception as exc:
                    self.fail(f"handle_pre_tool_use raised {exc!r}")


class TestCatchAllStillLogsTheToolCall(_CatchAllCase):

    def test_agent_and_task_write_tool_call_start(self):
        for tool_name in ("Agent", "Task"):
            with self.subTest(tool_name=tool_name):
                self.catch_all(tool_name)
        events = [e for e in self.orchestrator_events("unknown-run")
                  if e["event"] == "tool_call_start"]
        self.assertEqual({"Agent", "Task"}, {e["tool_name"] for e in events})
        self.assertEqual({"tu-Agent", "tu-Task"}, {e["call_id"] for e in events})

    def test_tool_call_start_carries_the_dispatch_input(self):
        self.catch_all("Agent")
        (event,) = [e for e in self.orchestrator_events("unknown-run")
                    if e["event"] == "tool_call_start"]
        self.assertIn(_INSTANCE, json.dumps(event["tool_input"]))

    def test_usage_records_are_written_for_a_dispatch_when_enabled(self):
        for tool_name in ("Agent", "Task"):
            with self.subTest(tool_name=tool_name):
                transcript = write_transcript(
                    self.workspace,
                    [assistant_record("dispatching", message_id=f"msg_{tool_name}")],
                    name=f"{tool_name}.jsonl")
                with mock.patch.dict(os.environ, {}, clear=False):
                    os.environ.pop(usage.CAPTURE_MODE_ENV, None)
                    self.catch_all(tool_name, transcript_path=transcript)
        records = [e for e in self.orchestrator_events("unknown-run")
                   if e["event"] == "usage_record"]
        self.assertEqual({"msg_Agent", "msg_Task"},
                         {r["record_id"] for r in records})

    def test_no_usage_records_when_only_boundaries_are_captured(self):
        transcript = write_transcript(
            self.workspace, [assistant_record("dispatching", message_id="msg_b")])
        with mock.patch.dict(os.environ, {usage.CAPTURE_MODE_ENV: "boundaries"}):
            self.catch_all("Agent", transcript_path=transcript)
        self.assertIn("tool_call_start", self.names("unknown-run"))
        self.assertNotIn("usage_record", self.names("unknown-run"))

    def test_other_tools_are_logged_as_before(self):
        with runner_env(self.workspace):
            fire(self.workspace, "PreToolUse", session_id=_SESSION,
                 tool_name="Bash", tool_use_id="tu-bash",
                 tool_input={"command": "ls"})
        events = [e for e in self.orchestrator_events("unknown-run")
                  if e["event"] == "tool_call_start"]
        self.assertEqual(["Bash"], [e["tool_name"] for e in events])
        self.assertEqual([], self.queue())


class TestOwnToolCallStartDestination(_CatchAllCase):
    """The two firings of one dispatch are not ordered by the registration; the
    dispatch's own tool_call_start lands in the dispatch's run only when the
    capture finished first."""

    def test_capture_first_lands_the_event_in_the_dispatched_run(self):
        self.capture("Agent")
        self.catch_all("Agent")
        self.assertIn("tool_call_start", self.names(OTHER_RUN_ID))
        self.assertNotIn("tool_call_start", self.names("unknown-run"))

    def test_catch_all_first_lands_the_event_where_any_tool_would(self):
        self.catch_all("Agent")
        self.capture("Agent")
        self.assertIn("tool_call_start", self.names("unknown-run"))
        self.assertNotIn("tool_call_start", self.names(OTHER_RUN_ID))

    def test_capture_first_queues_the_dispatch_exactly_once(self):
        self.capture("Agent")
        self.catch_all("Agent")
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in self.queue()])

    def test_catch_all_first_then_capture_queues_the_dispatch_exactly_once(self):
        self.catch_all("Task")
        self.capture("Task")
        self.assertEqual([_INSTANCE], [e["agent_instance_id"] for e in self.queue()])

    def test_later_calls_follow_the_binding_set_by_the_capture(self):
        self.capture("Agent", OTHER_RUN_ID)
        with runner_env(self.workspace):
            fire(self.workspace, "PreToolUse", session_id=_SESSION,
                 tool_name="Bash", tool_use_id="tu-later",
                 tool_input={"command": "ls"})
        later = [e for e in self.orchestrator_events(OTHER_RUN_ID)
                 if e["event"] == "tool_call_start"]
        self.assertEqual(["Bash"], [e["tool_name"] for e in later])


if __name__ == "__main__":
    unittest.main()
