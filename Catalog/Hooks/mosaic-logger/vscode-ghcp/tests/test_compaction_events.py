"""Tests for the compaction event written by the PreCompact hook.

Covers:
  - one phase-less compaction event per PreCompact firing, stamped schema 1.5.0
  - token fields come from the payload only; custom_instructions is never written
  - the event goes to a subagent's stream only when the payload names the subagent
    or the adapter state maps the payload's session id to a subagent invocation;
    a subagent merely running is not enough
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow

_SUBAGENT_SESSION = "subagent-own-session-1"


def _compactions(events):
    return [e for e in events if e["event"] == "compaction"]


class CompactionCase(unittest.TestCase):

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def pre_compact(self, **overrides):
        """Fire PreCompact; the payload is the session-start shape without `source`."""
        fields = {"hook_event_name": "PreCompact", "source": "<absent>", **overrides}
        flow.dispatch(flow.payload("session_start", self.ws.path, **fields))

    def start_subagent_in_own_session(self):
        self.ws.send("session_start")
        self.ws.send("pre_dispatch", session_id=_SUBAGENT_SESSION)
        self.ws.send("start", session_id=_SUBAGENT_SESSION)
        self.assertTrue(self.ws.invocation_start())  # the spawn was associated

    def orchestrator_compactions(self):
        return _compactions(self.ws.orchestrator_events())

    def subagent_compactions(self):
        return _compactions(self.ws.invocation_events())


class TestEventShape(CompactionCase):

    def test_pre_compact_writes_one_event_stamped_1_5_0(self):
        self.ws.send("session_start")
        self.pre_compact(trigger="auto")
        (event,) = self.orchestrator_compactions()
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertEqual("auto", event["trigger"])

    def test_event_has_no_phase(self):
        self.ws.send("session_start")
        self.pre_compact(trigger="manual")
        (event,) = self.orchestrator_compactions()
        self.assertNotIn("phase", event)

    def test_custom_instructions_is_not_written(self):
        self.ws.send("session_start")
        self.pre_compact(trigger="auto", custom_instructions="keep the plan")
        (event,) = self.orchestrator_compactions()
        self.assertNotIn("custom_instructions", event)
        self.assertNotIn("keep the plan", str(event))


class TestTokenFields(CompactionCase):

    def test_payload_without_tokens_writes_no_token_fields(self):
        self.ws.send("session_start")
        self.pre_compact(trigger="auto")
        (event,) = self.orchestrator_compactions()
        self.assertNotIn("tokens_before", event)
        self.assertNotIn("tokens_after", event)

    def test_tokens_before_comes_from_the_payload(self):
        self.ws.send("session_start")
        self.pre_compact(trigger="auto", tokens_before=88000)
        (event,) = self.orchestrator_compactions()
        self.assertEqual(88000, event["tokens_before"])
        self.assertNotIn("tokens_after", event)

    def test_non_integer_tokens_before_is_dropped(self):
        self.ws.send("session_start")
        for bad in ("88000", 12.5, True, None):
            with self.subTest(value=bad):
                self.pre_compact(trigger="auto", tokens_before=bad)
                event = self.orchestrator_compactions()[-1]
                self.assertNotIn("tokens_before", event)


class TestRouting(CompactionCase):

    def test_running_subagent_alone_does_not_redirect_the_event(self):
        self.ws.start_subagent()
        self.pre_compact(trigger="auto")
        self.assertEqual(1, len(self.orchestrator_compactions()))
        self.assertEqual([], self.subagent_compactions())

    def test_payload_naming_the_subagent_routes_to_its_stream(self):
        self.ws.start_subagent()
        self.pre_compact(trigger="auto", agent_id=flow.CALL_ID)
        (event,) = self.subagent_compactions()
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertNotIn("phase", event)
        self.assertEqual([], self.orchestrator_compactions())

    def test_session_id_mapped_to_a_subagent_routes_to_its_stream(self):
        self.start_subagent_in_own_session()
        self.pre_compact(trigger="auto", session_id=_SUBAGENT_SESSION)
        (event,) = self.subagent_compactions()
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertEqual([], self.orchestrator_compactions())

    def test_orchestrator_session_id_stays_on_the_orchestrator_stream(self):
        self.start_subagent_in_own_session()
        self.pre_compact(trigger="auto")
        self.assertEqual(1, len(self.orchestrator_compactions()))
        self.assertEqual([], self.subagent_compactions())


if __name__ == "__main__":
    unittest.main()
