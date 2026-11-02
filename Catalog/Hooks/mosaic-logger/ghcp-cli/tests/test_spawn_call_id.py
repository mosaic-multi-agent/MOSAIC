"""invocation_start.spawn_call_id for native-mode GHCP CLI subagents.

The ID of the tool call that spawned the subagent is taken from the `task`/`agent`
preToolUse payload (remembered with the pending dispatch) or from the subagentStart
payload itself. The source fields live in mosaic_logger_spawn_key.spawn_call_id_from.
When no payload supplies an ID the field is omitted. Runner mode never writes it.
No normalisation is applied: CLI tool-call IDs are already clean `toolu_...` values
and are logged verbatim (unlike the VS Code adapter, which strips `__vscode-<ms>`).
"""

import contextlib
import importlib
import io
import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
from tests.runner_mode_support import RunnerModeTestCase

_RUN_ID = "20261028T100000Z-b7c4"
_SESSION = "ghcp-spawn-session"
_TS = "2026-10-28T10:00:00.000Z"
_CALL_ID = "toolu_017pWWTUCtf17bR6MDKoNcMw"
_OTHER_CALL_ID = "toolu_01SecondSpawnAbCdEfGhIj"


def _spawn_key():
    return importlib.import_module("mosaic_logger_spawn_key")


def _prompt(instance_id):
    return json.dumps({"agent_instance_id": instance_id, "run_id": _RUN_ID,
                       "task_description": "Do it."})


def _ctx(tmp, **payload):
    paths = core.build_paths(tmp)
    return core.HookContext("subagentStart", {"sessionId": _SESSION, **payload},
                            tmp, paths, _TS)


class TestSpawnCallIdFrom(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.tmp = pathlib.Path(self._tmp.name)

    def _from(self, **payload):
        return _spawn_key().spawn_call_id_from(_ctx(self.tmp, **payload))

    def test_reads_tool_call_id(self):
        self.assertEqual(_CALL_ID, self._from(toolCallId=_CALL_ID))

    def test_reads_snake_case_tool_call_id(self):
        self.assertEqual(_CALL_ID, self._from(tool_call_id=_CALL_ID))

    def test_reads_tool_use_id(self):
        self.assertEqual(_CALL_ID, self._from(toolUseId=_CALL_ID))

    def test_reads_first_entry_of_tool_calls(self):
        self.assertEqual(_CALL_ID, self._from(toolCalls=[{"id": _CALL_ID},
                                                         {"id": _OTHER_CALL_ID}]))

    def test_first_present_source_wins_in_documented_order(self):
        payload = {"toolCallId": "a", "tool_call_id": "b", "toolUseId": "c",
                   "toolCalls": [{"id": "d"}]}
        self.assertEqual("a", self._from(**payload))
        del payload["toolCallId"]
        self.assertEqual("b", self._from(**payload))
        del payload["tool_call_id"]
        self.assertEqual("c", self._from(**payload))
        del payload["toolUseId"]
        self.assertEqual("d", self._from(**payload))

    def test_id_is_returned_verbatim(self):
        self.assertEqual("toolu_AbC_123", self._from(toolCallId="toolu_AbC_123"))

    def test_no_source_yields_none(self):
        self.assertIsNone(self._from())

    def test_empty_and_malformed_sources_yield_none(self):
        self.assertIsNone(self._from(toolCallId="", tool_call_id=None, toolCalls=[]))
        self.assertIsNone(self._from(toolCalls="not-a-list"))
        self.assertIsNone(self._from(toolCalls=[{"name": "task"}]))


class TestRememberAndTakeSpawnCall(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.paths = core.build_paths(pathlib.Path(self._tmp.name))

    def test_remembered_id_is_taken_for_the_same_agent_instance(self):
        sk = _spawn_key()
        self.assertTrue(sk.remember_spawn_call(self.paths, _RUN_ID, _SESSION,
                                               "Research#3", _CALL_ID))
        self.assertEqual(_CALL_ID, sk.take_spawn_call(self.paths, _RUN_ID, _SESSION,
                                                      "Research#3"))

    def test_take_without_remember_yields_none(self):
        self.assertIsNone(_spawn_key().take_spawn_call(self.paths, _RUN_ID, _SESSION,
                                                       "Research#3"))

    def test_ids_are_kept_per_agent_instance(self):
        sk = _spawn_key()
        sk.remember_spawn_call(self.paths, _RUN_ID, _SESSION, "Research#3", _CALL_ID)
        sk.remember_spawn_call(self.paths, _RUN_ID, _SESSION, "Review#4", _OTHER_CALL_ID)
        self.assertEqual(_OTHER_CALL_ID,
                         sk.take_spawn_call(self.paths, _RUN_ID, _SESSION, "Review#4"))
        self.assertEqual(_CALL_ID,
                         sk.take_spawn_call(self.paths, _RUN_ID, _SESSION, "Research#3"))

    def test_ids_are_kept_per_session(self):
        sk = _spawn_key()
        sk.remember_spawn_call(self.paths, _RUN_ID, _SESSION, "Research#3", _CALL_ID)
        self.assertIsNone(sk.take_spawn_call(self.paths, _RUN_ID, "other-session",
                                             "Research#3"))


class _NativeDispatchCase(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.ws = pathlib.Path(self._tmp.name)
        self.paths = core.build_paths(self.ws)
        self._saved = {k: os.environ.pop(k, None)
                       for k in ("MOSAIC_ROLE", "MOSAIC_RUN_ID", "MOSAIC_AGENT_INSTANCE_ID")}
        self.addCleanup(self._restore_env)

    def _restore_env(self):
        for key, value in self._saved.items():
            if value is not None:
                os.environ[key] = value

    def fire(self, event, **payload):
        body = {"cwd": str(self.ws), "sessionId": _SESSION, **payload}
        with contextlib.redirect_stdout(io.StringIO()):
            mosaic_logger.dispatch(event, json.dumps(body))

    def dispatch_task(self, instance_id, **ids):
        self.fire("preToolUse", toolName="task", toolArgs={"prompt": _prompt(instance_id)},
                  **ids)

    def invocation_start(self, instance_id):
        path = self.paths.invocation_events(_RUN_ID, instance_id)
        for line in path.read_text("utf-8").splitlines():
            event = json.loads(line)
            if event["event"] == "invocation_start":
                return event
        self.fail("no invocation_start written")


class TestNativeInvocationStartSpawnCallId(_NativeDispatchCase):

    def test_id_from_the_dispatch_pre_tool_use_tool_call_id(self):
        self.dispatch_task("Research#3", toolCallId=_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_id_from_the_dispatch_pre_tool_use_tool_use_id(self):
        self.dispatch_task("Research#3", toolUseId=_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_id_from_the_subagent_start_payload_when_the_dispatch_has_none(self):
        self.dispatch_task("Research#3")
        self.fire("subagentStart", agentId="agent-1", agentType="Research",
                  toolCallId=_CALL_ID)
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_agent_tool_dispatch_also_carries_the_id(self):
        self.fire("preToolUse", toolName="agent", toolCallId=_CALL_ID,
                  toolArgs={"prompt": _prompt("Research#3")})
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_spawn_call_id_is_omitted_when_no_payload_supplies_one(self):
        self.dispatch_task("Research#3")
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        start = self.invocation_start("Research#3")
        self.assertNotIn("spawn_call_id", start)
        self.assertEqual("Research#3", start["agent_instance_id"])

    def test_each_parallel_subagent_gets_the_id_of_its_own_dispatch(self):
        self.dispatch_task("Research#3", toolCallId=_CALL_ID)
        self.dispatch_task("Review#4", toolCallId=_OTHER_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.fire("subagentStart", agentId="agent-2", agentType="Review")
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))
        self.assertEqual(_OTHER_CALL_ID,
                         self.invocation_start("Review#4").get("spawn_call_id"))

    def test_subagent_start_payload_id_wins_over_the_dispatch_id(self):
        self.dispatch_task("Research#3", toolCallId=_OTHER_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research",
                  toolCallId=_CALL_ID)
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_ordinary_tool_call_id_does_not_replace_the_dispatch_id(self):
        self.dispatch_task("Research#3", toolCallId=_CALL_ID)
        self.fire("preToolUse", toolName="bash", toolArgs={"command": "ls"},
                  toolUseId=_OTHER_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertEqual(_CALL_ID, self.invocation_start("Research#3").get("spawn_call_id"))

    def test_ordinary_tool_call_id_is_not_taken_as_the_spawn_id(self):
        self.dispatch_task("Research#3")
        self.fire("preToolUse", toolName="bash", toolArgs={"command": "ls"},
                  toolUseId=_OTHER_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertNotIn("spawn_call_id", self.invocation_start("Research#3"))

    def test_invocation_start_with_spawn_call_id_is_stamped_1_4_0(self):
        self.dispatch_task("Research#3", toolCallId=_CALL_ID)
        self.fire("subagentStart", agentId="agent-1", agentType="Research")
        self.assertEqual("1.4.0", self.invocation_start("Research#3")["schema_version"])


class TestRunnerModeNeverWritesSpawnCallId(RunnerModeTestCase):

    def test_runner_subagent_invocation_start_has_no_spawn_call_id(self):
        self.subagent_env()
        self.dispatch("userPromptSubmitted", {"prompt": "Do the research task",
                                              "toolCallId": _CALL_ID})
        self.dispatch("sessionStart", {"source": "new", "toolCallId": _CALL_ID})
        self.dispatch("preToolUse", {"toolName": "bash", "toolArgs": {"command": "ls"},
                                     "toolUseId": "tu-1", "toolCallId": _CALL_ID})
        starts = [e for e in self.invocation_events() if e["event"] == "invocation_start"]
        self.assertEqual(1, len(starts))
        self.assertNotIn("spawn_call_id", starts[0])


if __name__ == "__main__":
    unittest.main()
