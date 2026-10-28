"""Tests for the compaction event written by the preCompact hook.

Covers:
  - one phase-less compaction event per preCompact firing, stamped schema 1.5.0
  - no token fields are written when the payload has none. Whether the GHCP CLI
    preCompact payload carries token counts is unverified (no documented payload
    shape), so positive and non-integer token cases are deliberately untested here.
  - the event goes to a subagent's stream only when the payload names the
    compacted subagent (agentId); a subagent merely running is not enough
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

    def orchestrator_compactions(self):
        # The run folder of the orchestrator stream depends on run-id resolution,
        # so every orchestrator stream under the log root is read.
        events = []
        for f in sorted(self.paths.root.rglob("00_orchestrator_events.jsonl")):
            events.extend(_read_jsonl(f))
        return [e for e in events if e["event"] == "compaction"]

    def subagent_compactions(self):
        events = _read_jsonl(self.paths.invocation_events(_RUN_ID, _INSTANCE_ID))
        return [e for e in events if e["event"] == "compaction"]


class TestEventShape(CompactionCase):

    def test_pre_compact_writes_one_event_stamped_1_5_0(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="auto")
        (event,) = self.orchestrator_compactions()
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertEqual("auto", event["trigger"])

    def test_event_has_no_phase(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="manual")
        (event,) = self.orchestrator_compactions()
        self.assertNotIn("phase", event)

    def test_payload_without_token_values_writes_no_token_fields(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="auto")
        (event,) = self.orchestrator_compactions()
        self.assertNotIn("tokens_before", event)
        self.assertNotIn("tokens_after", event)


class TestRouting(CompactionCase):

    def test_pre_compact_without_agent_id_goes_to_orchestrator_stream(self):
        self.start_running_subagent()
        self.fire("preCompact", trigger="auto")
        self.assertEqual(1, len(self.orchestrator_compactions()))
        self.assertEqual([], self.subagent_compactions())

    def test_pre_compact_naming_the_subagent_goes_to_its_stream(self):
        self.start_running_subagent()
        self.fire("preCompact", agentId=_AGENT_ID, trigger="auto")
        (event,) = self.subagent_compactions()
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertNotIn("phase", event)
        self.assertEqual([], self.orchestrator_compactions())


if __name__ == "__main__":
    unittest.main()
