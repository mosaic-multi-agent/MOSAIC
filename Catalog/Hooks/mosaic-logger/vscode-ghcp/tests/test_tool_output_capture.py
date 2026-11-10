"""tool_call_end.tool_output comes from the PostToolUse `tool_response` field.

VS Code supplies the tool output as a top-level `tool_response` (a string in every
captured payload; an empty string for tools without output). The GHCP CLI shape
`tool_result.text_result_for_llm` is not read.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow


class TestToolOutputCapture(unittest.TestCase):

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

    def test_non_empty_tool_response_becomes_tool_output(self):
        end = self.post_read(tool_response="line one\nline two")
        self.assertEqual("line one\nline two", end.get("tool_output"))

    def test_tool_output_is_stored_whole(self):
        text = "x" * 20_000 + "\ntail marker"
        end = self.post_read(tool_response=text)
        self.assertEqual(text, end.get("tool_output"))

    def test_empty_tool_response_omits_tool_output(self):
        end = self.post_read()  # the captured shape: tool_response is ""
        self.assertNotIn("tool_output", end)

    def test_absent_tool_response_omits_tool_output(self):
        end = self.post_read(tool_response="<absent>")
        self.assertNotIn("tool_output", end)

    def test_non_empty_structured_tool_response_is_kept(self):
        for value in ({"files": ["marker-file.md"]}, ["marker-item"]):
            with self.subTest(value=value):
                ws = flow.Workspace()
                self.addCleanup(ws.cleanup)
                ws.send("post_read_empty", tool_response=value)
                end = next(e for e in ws.orchestrator_events() if e["event"] == "tool_call_end")
                self.assertIn("marker", str(end.get("tool_output")))

    def test_empty_structured_tool_response_omits_tool_output(self):
        for value in ({}, []):
            with self.subTest(value=value):
                ws = flow.Workspace()
                self.addCleanup(ws.cleanup)
                ws.send("post_read_empty", tool_response=value)
                end = next(e for e in ws.orchestrator_events() if e["event"] == "tool_call_end")
                self.assertNotIn("tool_output", end)

    def test_cli_shaped_tool_result_is_not_read(self):
        end = self.post_read(tool_result={"text_result_for_llm": "cli shaped output"})
        self.assertNotIn("tool_output", end)

    def test_spawning_post_tool_use_carries_the_response_as_tool_output(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        expected = flow.payload("post_dispatch", self.ws.path)["tool_response"]
        self.assertEqual(expected, self.tool_call_end(flow.TOOL_USE_ID).get("tool_output"))

    def test_tool_call_end_has_no_null_values(self):
        end = self.post_read(tool_response="some output")
        self.assertEqual("some output", end["tool_output"])
        for key, value in end.items():
            with self.subTest(key=key):
                self.assertIsNotNone(value)


if __name__ == "__main__":
    unittest.main()
