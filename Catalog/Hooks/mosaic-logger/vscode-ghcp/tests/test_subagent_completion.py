"""A finished VS Code subagent yields exactly one invocation_end.

Payloads are shaped like the captured VS Code hook inputs (tests/fixtures/):
SubagentStart / SubagentStop carry agent_id = the spawning tool-call ID and no
response text; the orchestrator's PostToolUse for runSubagent carries the subagent's
full protocol response in tool_response and a tool_use_id with a `__vscode-<ms>`
suffix. The invocation_end must hold the response and status code and land with a
02_output.md in the invocation folder, whichever of the two firings comes first.

Transcript-derived model / token_usage on invocation_end is intentionally not covered
here: the end is now written from the spawning PostToolUse (or the Stop sweep), whose
payloads carry the orchestrator's session transcript, not the subagent's, so the
contract does not promise those fields on invocation_end.
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow

_RESPONSE = flow.payload("post_dispatch", ".")["tool_response"]


class _WorkspaceCase(unittest.TestCase):
    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def assert_single_complete_end(self):
        ends = self.ws.invocation_ends()
        self.assertEqual(1, len(ends), f"expected exactly one invocation_end, got {ends}")
        end = ends[0]
        self.assertEqual(flow.INSTANCE_ID, end.get("agent_instance_id"))
        self.assertEqual(_RESPONSE, end.get("response"))
        self.assertEqual("SUCCESS", end.get("status_code"))
        artifact = self.ws.output_artifact()
        self.assertTrue(artifact.exists(), "02_output.md must be written")
        self.assertIn("Rewrote Requirements.md", artifact.read_text("utf-8"))
        return end


class TestCompletionEitherOrder(_WorkspaceCase):
    """Exactly one invocation_end with response and status_code in every firing order."""

    def test_stop_then_spawning_post_tool_use(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        self.assert_single_complete_end()

    def test_spawning_post_tool_use_then_stop(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        self.ws.send("stop")
        self.assert_single_complete_end()

    def test_spawning_post_tool_use_without_any_stop(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        self.assert_single_complete_end()

    def test_end_follows_start_in_the_invocation_stream(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        kinds = [e["event"] for e in self.ws.invocation_events()]
        self.assertLess(kinds.index("invocation_start"), kinds.index("invocation_end"))

    def test_repeated_delivery_of_the_spawning_post_tool_use_adds_no_second_end(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        self.ws.send("post_dispatch")
        self.ws.send("stop")
        self.assert_single_complete_end()

    def test_no_invocation_end_in_an_orchestrator_stream(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        leaked = [e for e in self.ws.orchestrator_events()
                  if e.get("event") in ("invocation_start", "invocation_end")]
        self.assertEqual([], leaked)

    def test_spawning_post_tool_use_still_records_its_tool_call_end(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        ends = [e for e in self.ws.orchestrator_events()
                if e.get("event") == "tool_call_end" and e.get("call_id") == flow.TOOL_USE_ID]
        self.assertEqual(1, len(ends))


class TestResponseForms(_WorkspaceCase):
    """What the response and status_code of the invocation_end are made from."""

    def test_structured_tool_response_is_stored_as_json_text(self):
        structured = {"agent_instance_id": flow.INSTANCE_ID, "run_id": flow.RUN_ID,
                      "status_code": "BLOCKED", "error_code": "E101"}
        self.ws.start_subagent()
        self.ws.send("post_dispatch", tool_response=structured)
        end = self.ws.invocation_ends()[0]
        self.assertIsInstance(end["response"], str)
        self.assertEqual(structured, json.loads(end["response"]))
        self.assertEqual("BLOCKED", end.get("status_code"))

    def test_response_without_status_code_omits_status_code(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch", tool_response="Plain prose result, nothing structured.")
        end = self.ws.invocation_ends()[0]
        self.assertEqual("Plain prose result, nothing structured.", end["response"])
        self.assertNotIn("status_code", end)

    def test_large_response_is_stored_whole(self):
        big = '{"status_code": "SUCCESS"}\n' + "y" * 50_000
        self.ws.start_subagent()
        self.ws.send("post_dispatch", tool_response=big)
        self.assertEqual(big, self.ws.invocation_ends()[0]["response"])

    def test_empty_tool_response_still_ends_once_without_response_or_status(self):
        for empty in ("", "<absent>", {}):
            with self.subTest(tool_response=empty):
                ws = flow.Workspace()
                self.addCleanup(ws.cleanup)
                ws.start_subagent()
                ws.send("post_dispatch", tool_response=empty)
                ends = ws.invocation_ends()
                self.assertEqual(1, len(ends))
                self.assertNotIn("response", ends[0])
                self.assertNotIn("status_code", ends[0])

    def test_no_null_values_in_invocation_end(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch")
        self.assertEqual(_RESPONSE, self.ws.invocation_ends()[0]["response"])
        for key, value in self.ws.invocation_ends()[0].items():
            with self.subTest(key=key):
                self.assertIsNotNone(value)


class TestNoSubagentUsageOnEnd(_WorkspaceCase):
    """The orchestrator's transcript figures never become the subagent's usage."""

    def assert_no_usage(self, end):
        for key in ("model", "provider", "token_usage"):
            self.assertNotIn(key, end)
        text = self.ws.output_artifact().read_text("utf-8").lower()
        self.assertNotIn("token", text)

    def test_end_from_the_spawning_post_tool_use_carries_no_usage(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        self.assert_no_usage(self.ws.invocation_ends()[0])

    def test_end_from_the_stop_sweep_carries_no_usage(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("orchestrator_stop")
        self.assert_no_usage(self.ws.invocation_ends()[0])


class TestStopSweep(_WorkspaceCase):
    """A subagent that stopped but never got its spawning PostToolUse is ended at Stop."""

    def assert_swept_end(self):
        ends = self.ws.invocation_ends()
        self.assertEqual(1, len(ends))
        self.assertEqual(flow.INSTANCE_ID, ends[0].get("agent_instance_id"))
        self.assertNotIn("response", ends[0])
        self.assertNotIn("status_code", ends[0])
        self.assertTrue(self.ws.output_artifact().exists(), "02_output.md must be written")

    def test_stopped_subagent_without_post_tool_use_is_ended_at_stop(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("orchestrator_stop")
        self.assert_swept_end()

    def test_sweep_does_not_end_a_subagent_that_never_stopped(self):
        self.ws.start_subagent()
        self.ws.send("orchestrator_stop")
        self.assertEqual([], self.ws.invocation_ends())

    def test_sweep_adds_nothing_when_the_spawning_post_tool_use_already_ended_it(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        self.ws.send("orchestrator_stop")
        self.assert_single_complete_end_after_sweep()

    def test_repeated_stop_sweeps_add_no_second_end(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("orchestrator_stop")
        self.ws.send("orchestrator_stop")
        self.assert_swept_end()

    def test_late_spawning_post_tool_use_after_sweep_adds_no_second_end(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("orchestrator_stop")
        self.ws.send("post_dispatch")
        self.assertEqual(1, len(self.ws.invocation_ends()))

    def assert_single_complete_end_after_sweep(self):
        ends = self.ws.invocation_ends()
        self.assertEqual(1, len(ends))
        self.assertEqual(_RESPONSE, ends[0].get("response"))
        self.assertEqual("SUCCESS", ends[0].get("status_code"))


if __name__ == "__main__":
    unittest.main()
