"""Every caller of the shared orchestrator transcript export guards by session.

A first session owns the run's shared `00_orchestrator_session.raw`. Hook
firings of a second session that resolves to the same run - SubagentStart,
SubagentStop, and an orchestrator-role SessionEnd - leave the raw transcript
and its sidecar byte-for-byte unchanged.
"""

import json
import os
import pathlib
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from runner_mode_support import RUN_ID, WorkspaceTestCase, fire, runner_env

_INSTANCE = "TestWriter#5"


def _dispatch_prompt():
    return (f'{{"agent_instance_id": "{_INSTANCE}", "run_id": "{RUN_ID}", '
            '"task_description": "do the work"}')


class _GuardCallerCase(WorkspaceTestCase):
    def setUp(self):
        super().setUp()
        first = self._transcript("a.jsonl", "from session a")
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "Stop", session_id="sess-a",
                 last_assistant_message="done", transcript_path=first)
        self.raw = self.paths.orchestrator_raw(RUN_ID)
        self.sidecar = self.raw.with_name("00_orchestrator_session.meta.json")
        self.assertTrue(self.raw.exists(), "first session did not export")
        self.raw_before = self.raw.read_bytes()
        self.sidecar_before = self.sidecar.read_bytes()

    def _transcript(self, name, text):
        path = self.workspace / name
        path.write_text(json.dumps({"type": "assistant", "text": text}) + "\n",
                        encoding="utf-8")
        return str(path)

    def assert_first_session_still_owns_the_export(self):
        self.assertEqual(self.raw_before, self.raw.read_bytes())
        self.assertEqual(self.sidecar_before, self.sidecar.read_bytes())


class TestSubagentCallersAreGuarded(_GuardCallerCase):
    def test_subagent_start_from_a_second_session_keeps_the_export(self):
        second = self._transcript("b.jsonl", "from session b")
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "SubagentStart", session_id="sess-b",
                 agent_id="agt-b", agent_type="TestWriter",
                 agent_prompt=_dispatch_prompt(), transcript_path=second)
        self.assert_first_session_still_owns_the_export()

    def test_subagent_stop_from_a_second_session_keeps_the_export(self):
        second = self._transcript("b.jsonl", "from session b")
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "SubagentStart", session_id="sess-b",
                 agent_id="agt-b", agent_type="TestWriter",
                 agent_prompt=_dispatch_prompt(), transcript_path=second)
            fire(self.workspace, "SubagentStop", session_id="sess-b",
                 agent_id="agt-b", last_assistant_message="finished",
                 transcript_path=second)
        self.assert_first_session_still_owns_the_export()


class TestOrchestratorSessionEndIsGuarded(_GuardCallerCase):
    def test_runner_orchestrator_session_end_keeps_the_first_sessions_export(self):
        second = self._transcript("b.jsonl", "from session b")
        with runner_env(self.workspace, role="orchestrator", run_id=RUN_ID):
            fire(self.workspace, "SessionStart", source="startup",
                 session_id="sess-b", transcript_path=second)
            fire(self.workspace, "Stop", session_id="sess-b",
                 last_assistant_message="done", transcript_path=second)
            fire(self.workspace, "SessionEnd", reason="other",
                 session_id="sess-b", transcript_path=second)
        self.assert_first_session_still_owns_the_export()


if __name__ == "__main__":
    unittest.main()
