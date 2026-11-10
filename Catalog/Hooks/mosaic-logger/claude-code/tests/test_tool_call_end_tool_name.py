"""tool_call_end carries the tool name and the 1.6.0 schema version.

PostToolUse and PostToolUseFailure write the payload's tool_name on tool_call_end
when it is non-empty and omit the field otherwise. tool_call_end events are stamped
1.6.0; every other event keeps its version.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_handlers_tools as tools

_RUN_ID = "20260101T170000Z-b5c8"
_SESSION_ID = "tool-name-session"
_TS = "2026-01-01T17:00:00.000Z"


def _ctx(tmp, hook_event_name, **payload_extra):
    payload = {"hook_event_name": hook_event_name, "session_id": _SESSION_ID, **payload_extra}
    root = pathlib.Path(tmp)
    ctx = core.HookContext(payload, root, core.build_paths(root), _TS)
    ctx.run_id = _RUN_ID
    return ctx


def _orchestrator_events(tmp):
    path = core.build_paths(pathlib.Path(tmp)).orchestrator_events(_RUN_ID)
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class _ToolEndCase(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.tmp = self._tmp.name

    def end_event(self, handler, hook_event_name, **payload):
        handler(_ctx(self.tmp, hook_event_name, **payload))
        (event,) = _orchestrator_events(self.tmp)
        self.assertEqual("tool_call_end", event["event"])
        return event


class TestPostToolUseToolName(_ToolEndCase):

    def _post(self, **payload):
        return self.end_event(tools.handle_post_tool_use, "PostToolUse", **payload)

    def test_carries_tool_name_from_payload(self):
        event = self._post(tool_name="Read", tool_use_id="tu-1", tool_response="x")
        self.assertEqual("Read", event.get("tool_name"))

    def test_carries_mcp_tool_name_verbatim(self):
        event = self._post(tool_name="mcp__github__list_issues", tool_use_id="tu-2",
                           tool_response="[]")
        self.assertEqual("mcp__github__list_issues", event.get("tool_name"))

    def test_omits_tool_name_when_payload_has_none(self):
        event = self._post(tool_use_id="tu-3", tool_response="x")
        self.assertNotIn("tool_name", event)

    def test_omits_tool_name_when_payload_name_is_empty(self):
        event = self._post(tool_name="", tool_use_id="tu-4", tool_response="x")
        self.assertNotIn("tool_name", event)

    def test_is_stamped_1_6_0(self):
        event = self._post(tool_name="Read", tool_use_id="tu-5", tool_response="x")
        self.assertEqual("1.6.0", event["schema_version"])

    def test_nameless_end_is_still_stamped_1_6_0(self):
        event = self._post(tool_use_id="tu-6", tool_response="x")
        self.assertEqual("1.6.0", event["schema_version"])


class TestPostToolUseFailureToolName(_ToolEndCase):

    def _fail(self, **payload):
        return self.end_event(tools.handle_post_tool_use_failure, "PostToolUseFailure",
                              **payload)

    def test_carries_tool_name_from_payload(self):
        event = self._fail(tool_name="Bash", tool_use_id="tu-10", error="exit 1")
        self.assertEqual("error", event["status"])
        self.assertEqual("Bash", event.get("tool_name"))

    def test_omits_tool_name_when_payload_has_none(self):
        event = self._fail(tool_use_id="tu-11", error="exit 1")
        self.assertNotIn("tool_name", event)

    def test_is_stamped_1_6_0(self):
        event = self._fail(tool_name="Bash", tool_use_id="tu-12", error="exit 1")
        self.assertEqual("1.6.0", event["schema_version"])


class TestSubagentRoutedToolName(unittest.TestCase):

    def test_tool_name_is_written_on_an_invocation_stream_end(self):
        with tempfile.TemporaryDirectory() as tmp:
            import mosaic_logger_runstate as runstate
            paths = core.build_paths(pathlib.Path(tmp))
            runstate.put_agent_mapping(paths, _RUN_ID, "aid-1", "Research#2", "Research")
            tools.handle_post_tool_use(_ctx(tmp, "PostToolUse", agent_id="aid-1",
                                            tool_name="Grep", tool_use_id="tu-20",
                                            tool_response="x"))
            path = paths.invocation_events(_RUN_ID, "Research#2")
            events = [json.loads(ln) for ln in path.read_text("utf-8").splitlines()
                      if ln.strip()]
            ends = [e for e in events if e["event"] == "tool_call_end"]
            self.assertEqual(1, len(ends))
            self.assertEqual("Grep", ends[0].get("tool_name"))
            self.assertEqual("1.6.0", ends[0]["schema_version"])


class TestSchemaVersionTable(unittest.TestCase):

    def _ctx(self):
        return _ctx(tempfile.gettempdir(), "SessionStart")

    def test_tool_call_end_is_1_6_0(self):
        self.assertEqual("1.6.0", core.build_event("tool_call_end", self._ctx())["schema_version"])

    def test_other_events_keep_their_versions(self):
        expected = {"session_start": "1.3.0", "tool_call_start": "1.3.0", "turn": "1.3.0",
                    "usage_record": "1.7.0", "invocation_start": "1.3.0",
                    "invocation_end": "1.3.0", "compaction": "1.5.0"}
        for name, version in expected.items():
            with self.subTest(event=name):
                self.assertEqual(version, core.build_event(name, self._ctx())["schema_version"])


if __name__ == "__main__":
    unittest.main()
