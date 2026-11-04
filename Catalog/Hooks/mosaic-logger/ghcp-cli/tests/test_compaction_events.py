"""Tests for the preCompact hook, which no longer writes a compaction event.

GHCP CLI compactions are logged from the session transcript (a native compaction fires no
preCompact at all), so a preCompact firing adds nothing to any stream, whatever its payload names.
Covers:
  - no compaction event on the orchestrator stream
  - no compaction event on a subagent's stream when the payload names the subagent (agentId)
  - no other event type is written on behalf of preCompact either
"""

import contextlib
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

_RUN_ID = "20261028T100000Z-b7c4"
_INSTANCE_ID = "Research#3"
_SESSION = "ghcp-compaction-session"
_AGENT_ID = "agent-c1"


def _read_jsonl(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class CompactionCase(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.ws = pathlib.Path(self._tmp.name)
        self.paths = core.build_paths(self.ws)

    def fire(self, event, **payload):
        body = {"cwd": str(self.ws), "sessionId": _SESSION, **payload}
        with contextlib.redirect_stdout(io.StringIO()):
            mosaic_logger.dispatch(event, json.dumps(body))

    def start_running_subagent(self):
        prompt = json.dumps({"agent_instance_id": _INSTANCE_ID, "run_id": _RUN_ID,
                             "task_description": "Do it."})
        self.fire("sessionStart")
        self.fire("preToolUse", toolName="task", toolUseId="toolu_C1",
                  toolArgs={"prompt": prompt})
        self.fire("subagentStart", agentId=_AGENT_ID, agentType="Research")

    def all_events(self):
        events = []
        for f in sorted(self.paths.root.rglob("*_events.jsonl")):
            events.extend(_read_jsonl(f))
        return events

    def compactions(self):
        return [e for e in self.all_events() if e["event"] == "compaction"]


class TestPreCompactWritesNothing(CompactionCase):

    def test_pre_compact_without_agent_id_writes_no_compaction_event(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="auto")
        self.assertEqual([], self.compactions())

    def test_pre_compact_naming_the_subagent_writes_no_compaction_event(self):
        self.start_running_subagent()
        self.fire("preCompact", agentId=_AGENT_ID, trigger="auto")
        self.assertEqual([], self.compactions())

    def test_manual_trigger_writes_no_compaction_event(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="manual")
        self.assertEqual([], self.compactions())

    def test_pre_compact_adds_no_event_at_all(self):
        self.start_running_subagent()
        before = len(self.all_events())
        self.fire("preCompact", agentId=_AGENT_ID, trigger="auto")
        self.fire("preCompact", trigger="auto")
        self.assertEqual(before, len(self.all_events()))


if __name__ == "__main__":
    unittest.main()
