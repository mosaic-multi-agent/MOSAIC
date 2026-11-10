"""Tests for the compaction events written by the PreCompact and PostCompact handlers.

Covers:
  - PreCompact writes a compaction event with phase "pre", PostCompact with phase "post"
  - compaction events are stamped schema 1.5.0 while every other event keeps 1.3.0
  - token fields are present only when the payload carries them
  - an agent_id in the payload routes the event to that subagent's own stream,
    otherwise it goes to the orchestrator stream
  - usage-record emission on these hooks is unchanged
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_handlers_session as session_handlers
import mosaic_logger_runstate as runstate

_RUN_ID = "20260101T000000Z-ab12"
_SESSION_ID = "compaction-session"
_TS = "2026-01-01T00:00:00.000Z"
_AGENT_ID = "aid-compact-1"
_INSTANCE_ID = "Research#2"


def _read_jsonl(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class CompactionCase(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = pathlib.Path(self._tmp.name)
        self.paths = core.build_paths(self.root)

    def _ctx(self, hook_name, **payload_extra):
        payload = {"hook_event_name": hook_name, "session_id": _SESSION_ID, **payload_extra}
        ctx = core.HookContext(payload, self.root, self.paths, _TS)
        ctx.run_id = _RUN_ID
        return ctx

    def pre(self, **payload_extra):
        session_handlers.handle_pre_compact(self._ctx("PreCompact", **payload_extra))

    def post(self, **payload_extra):
        session_handlers.handle_post_compact(self._ctx("PostCompact", **payload_extra))

    def orchestrator_events(self):
        return _read_jsonl(self.paths.orchestrator_events(_RUN_ID))

    def invocation_events(self):
        return _read_jsonl(self.paths.invocation_events(_RUN_ID, _INSTANCE_ID))

    def register_subagent(self):
        runstate.put_agent_mapping(self.paths, _RUN_ID, _AGENT_ID, _INSTANCE_ID, "Research")

    def compactions(self, events):
        return [e for e in events if e["event"] == "compaction"]


class TestPhase(CompactionCase):

    def test_pre_compact_writes_phase_pre(self):
        self.pre(trigger="auto", tokens_before=9000)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual("pre", event["phase"])

    def test_post_compact_writes_phase_post(self):
        self.post(tokens_after=900)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual("post", event["phase"])

    def test_pre_then_post_write_two_events_with_distinct_phases(self):
        self.pre(trigger="manual", tokens_before=9000)
        self.post(tokens_after=900)
        phases = [e["phase"] for e in self.compactions(self.orchestrator_events())]
        self.assertEqual(["pre", "post"], phases)


class TestSchemaStamp(CompactionCase):

    def test_pre_compact_event_is_stamped_1_5_0(self):
        self.pre(trigger="auto")
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual("1.5.0", event["schema_version"])

    def test_post_compact_event_is_stamped_1_5_0(self):
        self.post(tokens_after=5)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual("1.5.0", event["schema_version"])

    def test_build_event_stamps_compaction_1_5_0(self):
        event = core.build_event("compaction", self._ctx("PreCompact"))
        self.assertEqual("1.5.0", event["schema_version"])

    def test_build_event_keeps_other_events_at_1_3_0(self):
        for name in ("session_start", "run_start", "turn", "tool_call_start",
                     "notification",
                     "invocation_start", "invocation_end"):
            with self.subTest(event=name):
                event = core.build_event(name, self._ctx("SessionStart"))
                self.assertEqual("1.3.0", event["schema_version"])

    def test_build_event_stamps_usage_record_1_7_0(self):
        event = core.build_event("usage_record", self._ctx("SessionStart"))
        self.assertEqual("1.7.0", event["schema_version"])

    def test_handler_written_non_compaction_event_keeps_1_3_0(self):
        session_handlers.handle_session_start(
            self._ctx("SessionStart", source="startup"))
        events = self.orchestrator_events()
        self.assertTrue(events)
        for event in events:
            self.assertEqual("1.3.0", event["schema_version"])


class TestTokenFields(CompactionCase):

    def test_pre_without_tokens_before_has_no_token_fields(self):
        self.pre(trigger="auto")
        (event,) = self.compactions(self.orchestrator_events())
        self.assertNotIn("tokens_before", event)
        self.assertNotIn("tokens_after", event)

    def test_post_without_tokens_after_has_no_token_fields(self):
        self.post()
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual("post", event["phase"])
        self.assertNotIn("tokens_before", event)
        self.assertNotIn("tokens_after", event)

    def test_pre_carries_tokens_before_from_payload_and_no_tokens_after(self):
        self.pre(trigger="auto", tokens_before=123456)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual(123456, event["tokens_before"])
        self.assertNotIn("tokens_after", event)

    def test_post_carries_tokens_after_from_payload_and_no_tokens_before(self):
        self.post(tokens_after=4321)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual(4321, event["tokens_after"])
        self.assertNotIn("tokens_before", event)

    def test_post_tokens_after_zero_is_kept(self):
        self.post(tokens_after=0)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertEqual(0, event["tokens_after"])

    def test_pre_drops_non_integer_tokens_before(self):
        for bad in ("123", 12.5, True, None):
            with self.subTest(value=bad):
                self.pre(trigger="auto", tokens_before=bad)
                event = self.compactions(self.orchestrator_events())[-1]
                self.assertNotIn("tokens_before", event)

    def test_post_drops_non_integer_tokens_after(self):
        for bad in ("123", 12.5, True, None):
            with self.subTest(value=bad):
                self.post(tokens_after=bad)
                event = self.compactions(self.orchestrator_events())[-1]
                self.assertNotIn("tokens_after", event)

    def test_pre_without_trigger_has_no_trigger_field(self):
        self.pre(tokens_before=10)
        (event,) = self.compactions(self.orchestrator_events())
        self.assertNotIn("trigger", event)


class TestRouting(CompactionCase):

    def test_pre_without_agent_id_goes_to_orchestrator_stream(self):
        self.register_subagent()
        self.pre(trigger="auto")
        self.assertEqual(1, len(self.compactions(self.orchestrator_events())))
        self.assertEqual([], self.compactions(self.invocation_events()))

    def test_post_without_agent_id_goes_to_orchestrator_stream(self):
        self.register_subagent()
        self.post(tokens_after=3)
        self.assertEqual(1, len(self.compactions(self.orchestrator_events())))
        self.assertEqual([], self.compactions(self.invocation_events()))

    def test_pre_with_agent_id_goes_to_that_subagents_stream(self):
        self.register_subagent()
        self.pre(agent_id=_AGENT_ID, trigger="auto", tokens_before=70)
        (event,) = self.compactions(self.invocation_events())
        self.assertEqual("pre", event["phase"])
        self.assertEqual(70, event["tokens_before"])
        self.assertEqual("1.5.0", event["schema_version"])

    def test_post_with_agent_id_goes_to_that_subagents_stream(self):
        self.register_subagent()
        self.post(agent_id=_AGENT_ID, tokens_after=7)
        (event,) = self.compactions(self.invocation_events())
        self.assertEqual("post", event["phase"])
        self.assertEqual(7, event["tokens_after"])

    def test_subagent_compaction_is_not_written_to_orchestrator_stream(self):
        self.register_subagent()
        self.pre(agent_id=_AGENT_ID, trigger="auto")
        self.post(agent_id=_AGENT_ID, tokens_after=7)
        self.assertEqual([], self.compactions(self.orchestrator_events()))

    def test_unmapped_agent_id_falls_back_to_orchestrator_stream(self):
        self.register_subagent()
        self.pre(agent_id="aid-unknown", trigger="auto")
        self.assertEqual(1, len(self.compactions(self.orchestrator_events())))
        self.assertEqual([], self.compactions(self.invocation_events()))

    def test_empty_agent_id_is_treated_as_absent(self):
        self.register_subagent()
        self.pre(agent_id="", trigger="auto")
        self.assertEqual(1, len(self.compactions(self.orchestrator_events())))
        self.assertEqual([], self.compactions(self.invocation_events()))


class TestUsageRecordEmissionUnchanged(CompactionCase):

    def _transcript(self):
        path = self.root / "transcript.jsonl"
        record = {"type": "assistant", "message": {
            "id": "msg_c1", "model": "claude-opus-4-5", "usage": {"input_tokens": 10}}}
        path.write_text(json.dumps(record) + "\n", encoding="utf-8")
        return str(path)

    def test_pre_compact_still_emits_usage_record_stamped_1_7_0(self):
        self.pre(trigger="auto", transcript_path=self._transcript())
        events = self.orchestrator_events()
        usage = [e for e in events if e["event"] == "usage_record"]
        self.assertEqual(1, len(usage))
        self.assertEqual("1.7.0", usage[0]["schema_version"])
        self.assertEqual(1, len(self.compactions(events)))

    def test_post_compact_still_emits_usage_record_stamped_1_7_0(self):
        self.post(tokens_after=5, transcript_path=self._transcript())
        events = self.orchestrator_events()
        usage = [e for e in events if e["event"] == "usage_record"]
        self.assertEqual(1, len(usage))
        self.assertEqual("1.7.0", usage[0]["schema_version"])
        self.assertEqual(1, len(self.compactions(events)))


if __name__ == "__main__":
    unittest.main()
