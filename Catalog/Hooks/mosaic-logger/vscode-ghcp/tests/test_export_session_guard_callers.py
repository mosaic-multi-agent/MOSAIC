"""Every orchestrator-transcript export of the VS Code adapter acts for its own session.

Two sessions dispatching into the same run share `00_orchestrator_session.raw`. The first
session to export owns it; the other session's Stop and SubagentStart exports leave the raw
transcript and its sidecar untouched, while the owner's later exports refresh them.
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session_routing_flow as sf
import vscode_flow as flow

SESSION_A = "sess-owner-0001"
SESSION_B = "sess-other-0002"


class _Base(unittest.TestCase):
    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)
        self.raw = self.ws.paths.orchestrator_raw(flow.RUN_ID)
        self.sidecar = self.raw.with_name("00_orchestrator_session.meta.json")

    def transcript(self, name, text):
        path = self.ws.path / name
        path.write_text(json.dumps({"type": "assistant", "text": text}) + "\n",
                        encoding="utf-8")
        return str(path)

    def dispatch(self, session, transcript):
        """The runSubagent PreToolUse of `session`, binding it to the fixture run."""
        self.ws.send("pre_dispatch", session_id=session, transcript_path=transcript)

    def stop(self, session, transcript):
        self.dispatch(session, transcript)
        self.ws.send("orchestrator_stop", session_id=session, transcript_path=transcript)

    def subagent_start(self, session, transcript):
        self.dispatch(session, transcript)
        self.ws.send("start", session_id=session, transcript_path=transcript)

    def sidecar_session(self):
        return json.loads(self.sidecar.read_text(encoding="utf-8")).get("session_id")


class TestStopExport(_Base):
    def test_stop_records_the_exporting_session_in_the_sidecar(self):
        self.stop(SESSION_A, self.transcript("a.jsonl", "from a"))
        self.assertEqual(SESSION_A, self.sidecar_session())

    def test_stop_of_a_second_session_leaves_raw_and_sidecar_unchanged(self):
        self.stop(SESSION_A, self.transcript("a.jsonl", "from a"))
        raw_before, sidecar_before = self.raw.read_bytes(), self.sidecar.read_bytes()

        self.stop(SESSION_B, self.transcript("b.jsonl", "from b"))

        self.assertEqual(raw_before, self.raw.read_bytes())
        self.assertEqual(sidecar_before, self.sidecar.read_bytes())

    def test_stop_of_the_owner_refreshes_the_transcript(self):
        self.stop(SESSION_A, self.transcript("a.jsonl", "first"))
        grown = self.transcript("a2.jsonl", "first and more text")

        self.stop(SESSION_A, grown)

        with open(grown, "rb") as fh:
            self.assertEqual(fh.read(), self.raw.read_bytes())

    def test_stop_replaces_a_legacy_raw_without_sidecar_and_records_the_session(self):
        self.raw.parent.mkdir(parents=True, exist_ok=True)
        self.raw.write_bytes(b"legacy raw\n")
        new = self.transcript("b.jsonl", "from b")

        self.stop(SESSION_B, new)

        with open(new, "rb") as fh:
            self.assertEqual(fh.read(), self.raw.read_bytes())
        self.assertEqual(SESSION_B, self.sidecar_session())


class TestSubagentStartExport(_Base):
    def test_subagent_start_of_a_second_session_leaves_raw_and_sidecar_unchanged(self):
        self.stop(SESSION_A, self.transcript("a.jsonl", "from a"))
        raw_before, sidecar_before = self.raw.read_bytes(), self.sidecar.read_bytes()

        self.subagent_start(SESSION_B, self.transcript("b.jsonl", "from b"))

        self.assertEqual(raw_before, self.raw.read_bytes())
        self.assertEqual(sidecar_before, self.sidecar.read_bytes())

    def test_subagent_start_of_the_owner_refreshes_the_transcript(self):
        self.stop(SESSION_A, self.transcript("a.jsonl", "first"))
        grown = self.transcript("a2.jsonl", "first and more text")

        self.subagent_start(SESSION_A, grown)

        with open(grown, "rb") as fh:
            self.assertEqual(fh.read(), self.raw.read_bytes())
        self.assertEqual(SESSION_A, self.sidecar_session())


if __name__ == "__main__":
    unittest.main()
