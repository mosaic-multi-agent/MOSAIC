"""tool_call_end.tool_name comes from the postToolUse / postToolUseFailure `toolName`.

The pre-tool handler reads the same payload field. The name is written when the
payload carries a non-empty one and omitted otherwise; tool_call_end events are
stamped 1.6.0.
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

_RUN_ID = "20260101T120000Z-cd34"
_SESSION_ID = "test-ghcp-tool-name-session"
_TS = "2026-01-01T12:00:00.000Z"


class _ToolEndCase(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = pathlib.Path(self._tmp.name)

    def end_event(self, handler, event_name, **payload):
        paths = core.build_paths(self.root)
        ctx = core.HookContext(event_name, {"sessionId": _SESSION_ID, **payload},
                               self.root, paths, _TS)
        ctx.run_id = _RUN_ID
        handler(ctx)
        path = paths.orchestrator_events(_RUN_ID)
        (event,) = [json.loads(ln) for ln in path.read_text("utf-8").splitlines()
                    if ln.strip()]
        self.assertEqual("tool_call_end", event["event"])
        return event


class TestPostToolUseToolName(_ToolEndCase):

    def _post(self, **payload):
        return self.end_event(tools.handle_post_tool_use, "postToolUse", **payload)

    def test_carries_tool_name_from_payload(self):
        event = self._post(toolName="bash", toolUseId="tu-1",
                           toolResult={"textResultForLlm": "ok"})
        self.assertEqual("bash", event.get("tool_name"))

    def test_carries_another_tool_name_verbatim(self):
        event = self._post(toolName="github-mcp-server-list_issues", toolUseId="tu-2")
        self.assertEqual("github-mcp-server-list_issues", event.get("tool_name"))

    def test_omits_tool_name_when_payload_has_none(self):
        event = self._post(toolUseId="tu-3", toolResult={"textResultForLlm": "ok"})
        self.assertNotIn("tool_name", event)

    def test_omits_tool_name_when_payload_name_is_empty(self):
        event = self._post(toolName="", toolUseId="tu-4")
        self.assertNotIn("tool_name", event)

    def test_is_stamped_1_6_0(self):
        self.assertEqual("1.6.0", self._post(toolName="bash", toolUseId="tu-5")["schema_version"])

    def test_nameless_end_is_still_stamped_1_6_0(self):
        self.assertEqual("1.6.0", self._post(toolUseId="tu-6")["schema_version"])


class TestPostToolUseFailureToolName(_ToolEndCase):

    def _fail(self, **payload):
        return self.end_event(tools.handle_post_tool_use_failure, "postToolUseFailure",
                              **payload)

    def test_carries_tool_name_from_payload(self):
        event = self._fail(toolName="bash", toolUseId="tu-10", error="Command not found")
        self.assertEqual("error", event["status"])
        self.assertEqual("bash", event.get("tool_name"))

    def test_omits_tool_name_when_payload_has_none(self):
        event = self._fail(toolUseId="tu-11", error="Command not found")
        self.assertNotIn("tool_name", event)

    def test_is_stamped_1_6_0(self):
        event = self._fail(toolName="bash", toolUseId="tu-12", error="boom")
        self.assertEqual("1.6.0", event["schema_version"])


if __name__ == "__main__":
    unittest.main()
