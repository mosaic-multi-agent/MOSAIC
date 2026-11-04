"""Per-event schema_version stamping.

Each event carries the highest schema_version whose fields the adapter implements for it:
invocation_start (spawn_call_id) is 1.4.0, compaction is 1.5.0, tool_call_end is 1.6.0,
usage_record is 1.7.0 (requested_tools, transcript-sourced usage), every other event is 1.0.0.
Applies in native mode and in Runner mode.
"""

import contextlib
import io
import json
import os
import pathlib
import sys
import tempfile
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
from tests.runner_mode_support import RunnerModeTestCase

_NEWER = {"invocation_start": "1.4.0", "usage_record": "1.7.0", "compaction": "1.5.0",
          "tool_call_end": "1.6.0"}
_RUN_ID = "20261028T100000Z-b7c4"
_INSTANCE_ID = "Research#3"
_SESSION = "ghcp-versions-session"


def _expected(event_name):
    return _NEWER.get(event_name, "1.0.0")


def _ctx():
    return types.SimpleNamespace(session_id="s1", run_id=_RUN_ID,
                                 timestamp="2026-01-01T00:00:00.000Z")


def _read_all_events(workspace):
    root = workspace / "OrchestrationLogs"
    events = []
    for f in sorted(root.rglob("*_events.jsonl")):
        events.extend(json.loads(ln) for ln in f.read_text("utf-8").splitlines() if ln.strip())
    return events


class TestSchemaVersionFor(unittest.TestCase):

    def test_invocation_start_is_1_4_0(self):
        self.assertEqual("1.4.0", core.schema_version_for("invocation_start"))

    def test_usage_record_is_1_7_0(self):
        self.assertEqual("1.7.0", core.schema_version_for("usage_record"))

    def test_compaction_is_1_5_0(self):
        self.assertEqual("1.5.0", core.schema_version_for("compaction"))

    def test_tool_call_end_is_1_6_0(self):
        self.assertEqual("1.6.0", core.schema_version_for("tool_call_end"))

    def test_other_events_are_1_0_0(self):
        for name in ("run_start", "run_end", "session_start", "session_end", "turn",
                     "tool_call_start", "invocation_end",
                     "context_compaction"):
            with self.subTest(event=name):
                self.assertEqual("1.0.0", core.schema_version_for(name))


class TestBuildEventStampsPerEventVersion(unittest.TestCase):

    def test_invocation_start_is_stamped_1_4_0(self):
        event = core.build_event("invocation_start", _ctx())
        self.assertEqual("1.4.0", event["schema_version"])

    def test_usage_record_is_stamped_1_7_0(self):
        event = core.build_event("usage_record", _ctx())
        self.assertEqual("1.7.0", event["schema_version"])

    def test_tool_call_start_is_stamped_1_0_0(self):
        event = core.build_event("tool_call_start", _ctx())
        self.assertEqual("1.0.0", event["schema_version"])

    def test_tool_call_end_is_stamped_1_6_0(self):
        event = core.build_event("tool_call_end", _ctx())
        self.assertEqual("1.6.0", event["schema_version"])

    def test_compaction_is_stamped_1_5_0(self):
        event = core.build_event("compaction", _ctx())
        self.assertEqual("1.5.0", event["schema_version"])


class TestNativeSessionEventVersions(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.ws = pathlib.Path(self._tmp.name)
        prompt = json.dumps({"agent_instance_id": _INSTANCE_ID, "run_id": _RUN_ID,
                             "task_description": "Do it."})
        self._fire("sessionStart")
        self._fire("preToolUse", toolName="task", toolUseId="toolu_V1",
                   toolArgs={"prompt": prompt})
        self._fire("subagentStart", agentId="agent-v1", agentType="Research")
        self._fire("preToolUse", agentId="agent-v1", toolName="bash", toolUseId="tu-1",
                   toolArgs={"command": "ls"})
        self._fire("postToolUse", agentId="agent-v1", toolName="bash", toolUseId="tu-1",
                   toolResult={"textResultForLlm": "ok"})
        self._fire("subagentStop", agentId="agent-v1",
                   response='{"status_code": "SUCCESS", "run_id": "%s"}' % _RUN_ID)
        self._fire("preCompact", trigger="auto")
        transcript = self.ws / "events.jsonl"
        transcript.write_text("".join(json.dumps(e) + "\n" for e in (
            {"type": "session.usage_record", "id": "u1", "timestamp": "2026-10-30T13:00:10.000Z",
             "data": {"usage": {"apiCallId": "call-1", "model": "m", "inputTokens": 10,
                                "outputTokens": 5, "cacheReadTokens": 0, "cacheWriteTokens": 0,
                                "duration": 100, "toolCounts": {}}}},
            {"type": "session.compaction_complete", "id": "c1", "timestamp": "2026-10-30T13:00:20.000Z",
             "data": {"success": True, "outcome": "applied", "compactionAttemptId": "a1",
                      "trigger": "threshold", "preCompactionTokens": 100, "postCompactionTokens": 10}},
        )), encoding="utf-8")
        self._fire("agentStop", transcriptPath=str(transcript))
        self._fire("sessionEnd", reason="complete")
        self.events = _read_all_events(self.ws)

    def _fire(self, event, **payload):
        body = {"cwd": str(self.ws), "sessionId": _SESSION, **payload}
        with contextlib.redirect_stdout(io.StringIO()):
            mosaic_logger.dispatch(event, json.dumps(body))

    def test_session_exercises_the_versioned_event_types(self):
        names = {e["event"] for e in self.events}
        self.assertTrue({"run_start", "invocation_start", "invocation_end",
                         "tool_call_start", "tool_call_end", "compaction", "usage_record",
                         "run_end"} <= names, names)

    def test_every_emitted_event_carries_the_version_of_its_type(self):
        for event in self.events:
            with self.subTest(event=event["event"]):
                self.assertEqual(_expected(event["event"]), event["schema_version"])


class TestRunnerModeEventVersions(RunnerModeTestCase):

    def test_runner_subagent_events_carry_the_version_of_their_type(self):
        self.subagent_env()
        self.dispatch("userPromptSubmitted", {"prompt": "Do the research task"})
        self.dispatch("sessionStart", {"source": "new"})
        self.dispatch("preToolUse", {"toolName": "bash", "toolArgs": {"command": "ls"},
                                     "toolUseId": "tu-1"})
        self.dispatch("agentStop", {"stopReason": "end_turn"})
        self.dispatch("sessionEnd", {"reason": "complete"})
        events = _read_all_events(self.workspace)
        names = {e["event"] for e in events}
        self.assertIn("invocation_start", names)
        for event in events:
            with self.subTest(event=event["event"]):
                self.assertEqual(_expected(event["event"]), event["schema_version"])

    def test_runner_orchestrator_events_carry_the_version_of_their_type(self):
        self.orchestrator_env()
        self.dispatch("sessionStart", {"source": "new"})
        self.dispatch("preToolUse", {"toolName": "bash", "toolArgs": {"command": "ls"},
                                     "toolUseId": "tu-1"})
        events = _read_all_events(self.workspace)
        self.assertTrue(events)
        for event in events:
            with self.subTest(event=event["event"]):
                self.assertEqual(_expected(event["event"]), event["schema_version"])


if __name__ == "__main__":
    unittest.main()
