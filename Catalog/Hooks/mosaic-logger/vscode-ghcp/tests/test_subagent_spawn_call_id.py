"""Subagent start correlation and invocation_start.spawn_call_id.

SubagentStart carries agent_id = the spawning tool-call ID (`toolu_...`), the same ID the
orchestrator's runSubagent tool events carry with a `__vscode-<ms>` suffix. The adapter
correlates the start with its dispatch and its stop on that ID, and writes it on
invocation_start as spawn_call_id with the suffix stripped.
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow

_SECOND_CALL_ID = "toolu_01SecondSubagentAbCdEfG"
_SECOND_INSTANCE = "requirements-review#5"


def _second_dispatch_prompt():
    return json.dumps({"agent_instance_id": _SECOND_INSTANCE, "run_id": flow.RUN_ID,
                       "task_description": "Review the requirements."})


class TestSpawnCallId(unittest.TestCase):

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def test_invocation_start_carries_the_spawning_tool_call_id(self):
        self.ws.start_subagent()
        start = self.ws.invocation_start()
        self.assertIsNotNone(start, "invocation_start must be written in the run folder")
        self.assertEqual(flow.CALL_ID, start.get("spawn_call_id"))

    def test_spawn_call_id_has_the_vscode_suffix_stripped(self):
        self.ws.send("session_start")
        self.ws.send("pre_dispatch")
        self.ws.send("start", agent_id=flow.TOOL_USE_ID)
        self.assertEqual(flow.CALL_ID, self.ws.invocation_start().get("spawn_call_id"))

    def test_spawn_call_id_matches_the_dispatch_tool_use_id_without_suffix(self):
        self.ws.start_subagent()
        dispatch_id = flow.payload("pre_dispatch", self.ws.path)["tool_use_id"]
        self.assertEqual(dispatch_id.split("__vscode-")[0],
                         self.ws.invocation_start().get("spawn_call_id"))

    def test_start_is_correlated_with_its_dispatch(self):
        self.ws.start_subagent()
        start = self.ws.invocation_start()
        self.assertEqual(flow.INSTANCE_ID, start.get("agent_instance_id"))
        self.assertEqual("requirements-refinement", start.get("agent_type"))
        self.assertTrue(self.ws.paths.invocation_dir(flow.RUN_ID, flow.INSTANCE_ID).is_dir())

    def test_agent_map_entry_is_keyed_by_the_normalised_call_id(self):
        self.ws.send("session_start")
        self.ws.send("pre_dispatch")
        self.ws.send("start", agent_id=flow.TOOL_USE_ID)
        entry = self.ws.paths.agent_map_entry(flow.RUN_ID, flow.CALL_ID)
        self.assertTrue(entry.exists(), f"expected agent-map entry {entry}")
        self.assertEqual(flow.INSTANCE_ID,
                         json.loads(entry.read_text("utf-8")).get("agent_instance_id"))

    def test_start_without_agent_id_omits_spawn_call_id(self):
        self.ws.send("session_start")
        self.ws.send("pre_dispatch")
        self.ws.send("start", agent_id="<absent>", agent_name="requirements-refinement")
        start = self.ws.invocation_start()
        self.assertIsNotNone(start, "agent_name remains a fallback correlation key")
        self.assertNotIn("spawn_call_id", start)

    def test_each_subagent_gets_its_own_spawn_call_id(self):
        self.ws.start_subagent()
        self.ws.send("pre_dispatch", tool_use_id=_SECOND_CALL_ID + "__vscode-1793098349300",
                     tool_input={"agentName": "requirements-review",
                                 "prompt": _second_dispatch_prompt()})
        self.ws.send("start", agent_id=_SECOND_CALL_ID, agent_type="requirements-review")
        second = flow.read_jsonl(self.ws.paths.invocation_events(flow.RUN_ID, _SECOND_INSTANCE))
        second_start = next(e for e in second if e["event"] == "invocation_start")
        self.assertEqual(_SECOND_CALL_ID, second_start.get("spawn_call_id"))
        self.assertEqual(flow.CALL_ID, self.ws.invocation_start().get("spawn_call_id"))


class TestStopCorrelation(unittest.TestCase):
    """The stop and the completion find the invocation the start created."""

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def test_stop_carrying_the_suffix_correlates_with_a_start_without_it(self):
        self.ws.start_subagent()
        self.ws.send("stop", agent_id=flow.TOOL_USE_ID)
        self.ws.send("orchestrator_stop")
        ends = self.ws.invocation_ends()
        self.assertEqual(1, len(ends))
        self.assertEqual(flow.INSTANCE_ID, ends[0].get("agent_instance_id"))

    def test_second_subagent_completion_ends_only_its_own_invocation(self):
        self.ws.start_subagent()
        self.ws.send("pre_dispatch", tool_use_id=_SECOND_CALL_ID + "__vscode-1793098349300",
                     tool_input={"agentName": "requirements-review",
                                 "prompt": _second_dispatch_prompt()})
        self.ws.send("start", agent_id=_SECOND_CALL_ID, agent_type="requirements-review")
        self.ws.send("post_dispatch", tool_use_id=_SECOND_CALL_ID + "__vscode-1793098349300")
        self.assertEqual([], self.ws.invocation_ends(), "first subagent must stay open")
        second = flow.read_jsonl(self.ws.paths.invocation_events(flow.RUN_ID, _SECOND_INSTANCE))
        self.assertEqual(1, [e["event"] for e in second].count("invocation_end"))


if __name__ == "__main__":
    unittest.main()
