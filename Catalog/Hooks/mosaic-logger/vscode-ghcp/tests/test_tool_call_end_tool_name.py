"""tool_call_end.tool_name comes from the PostToolUse `tool_name` field.

The name is written when the payload carries a non-empty one and omitted otherwise.
tool_call_end events are stamped 1.6.0. VS Code writes no post-tool-failure end, so
only the PostToolUse path exists.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow


class TestToolCallEndToolName(unittest.TestCase):

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def tool_call_end(self, call_id):
        ends = [e for e in self.ws.orchestrator_events()
                if e.get("event") == "tool_call_end" and e.get("call_id") == call_id]
        self.assertEqual(1, len(ends), f"expected one tool_call_end for {call_id}")
        return ends[0]

    def post_read(self, **overrides):
        self.ws.send("post_read_empty", **overrides)
        return self.tool_call_end(flow.payload("post_read_empty", self.ws.path)["tool_use_id"])

    def test_carries_the_payload_tool_name(self):
        end = self.post_read()  # captured shape: tool_name is "read_file"
        self.assertEqual("read_file", end.get("tool_name"))

    def test_carries_a_different_payload_tool_name_verbatim(self):
        end = self.post_read(tool_name="mcp_github_list_issues")
        self.assertEqual("mcp_github_list_issues", end.get("tool_name"))

    def test_spawning_post_tool_use_carries_the_tool_name(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        self.assertEqual("runSubagent", self.tool_call_end(flow.TOOL_USE_ID).get("tool_name"))

    def test_omits_tool_name_when_the_payload_has_none(self):
        end = self.post_read(tool_name="<absent>")
        self.assertNotIn("tool_name", end)

    def test_omits_tool_name_when_the_payload_name_is_empty(self):
        end = self.post_read(tool_name="")
        self.assertNotIn("tool_name", end)

    def test_is_stamped_1_6_0(self):
        self.assertEqual("1.6.0", self.post_read()["schema_version"])

    def test_nameless_end_is_still_stamped_1_6_0(self):
        self.assertEqual("1.6.0", self.post_read(tool_name="<absent>")["schema_version"])

    def test_other_events_keep_their_versions(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        for event in self.ws.all_events():
            if event["event"] == "tool_call_end":
                continue
            with self.subTest(event=event["event"]):
                expected = {"invocation_start": "1.4.0", "usage_record": "1.4.0",
                            "compaction": "1.5.0"}.get(event["event"], "1.0.0")
                self.assertEqual(expected, event["schema_version"])


class TestNoPostToolFailureHandler(unittest.TestCase):

    def test_adapter_registers_no_post_tool_failure_handler(self):
        import mosaic_logger_handlers_tools as tools
        self.assertFalse(hasattr(tools, "handle_post_tool_use_failure"))
        for name in getattr(tools, "HANDLERS", {}):
            self.assertNotIn("failure", str(name).lower())


if __name__ == "__main__":
    unittest.main()
