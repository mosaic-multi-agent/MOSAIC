"""usage_record carries per-record effort and reasoning_tokens.

effort is copied from the transcript record's top-level `effort` and omitted
when the record has none. Orchestrator and subagent streams each carry the
effort of their own transcript. Superseding observations of one record_id carry
the same effort. reasoning_tokens appears inside token_usage.
"""

import json
import os
import pathlib
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_transcript as transcript
import mosaic_logger_usage as usage

FIXTURES = pathlib.Path(__file__).parent / "fixtures"
RUN_ID = "20261022T100000Z-ab12"
AGENT = "Research#1"


def _read_jsonl(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines()
            if ln.strip()]


def _rec(message_id, effort=None, output_tokens=10, thinking=None):
    u = {"input_tokens": 5, "output_tokens": output_tokens}
    if thinking is not None:
        u["output_tokens_details"] = {"thinking_tokens": thinking}
    record = {"type": "assistant", "message": {
        "id": message_id, "model": "claude-opus-4-5", "usage": u}}
    if effort is not None:
        record["effort"] = effort
    return record


class EffortCase(unittest.TestCase):

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)
        paths = core.build_paths(self.root)
        payload = {"hook_event_name": "Stop", "session_id": "s-effort"}
        self.ctx = core.HookContext(payload, self.root, paths,
                                    "2026-10-22T10:00:00.000Z")
        self.ctx.run_id = RUN_ID

    def copy_fixture(self, name):
        dest = self.root / name
        shutil.copyfile(FIXTURES / name, dest)
        return str(dest)

    def write(self, name, records):
        path = self.root / name
        path.write_text(
            "\n".join(json.dumps(r) for r in records) + "\n", "utf-8")
        return str(path)

    def orchestrator_usage(self):
        events = _read_jsonl(self.ctx.paths.orchestrator_events(RUN_ID))
        return [e for e in events if e["event"] == usage.USAGE_EVENT]

    def subagent_usage(self):
        sink = core.event_sink_for(self.ctx, AGENT)
        return [e for e in _read_jsonl(sink) if e["event"] == usage.USAGE_EVENT]


class TestEffortOnStreams(EffortCase):

    def test_orchestrator_stream_carries_medium_and_subagent_stream_high(self):
        orch = self.copy_fixture("effort_orchestrator_transcript.jsonl")
        sub = self.copy_fixture("effort_subagent_transcript.jsonl")

        usage.emit_usage_records(self.ctx, orch, None, "orchestrator_transcript")
        usage.emit_usage_records(self.ctx, sub, AGENT, "agent_transcript")

        self.assertEqual(["medium", "medium"],
                         [e.get("effort") for e in self.orchestrator_usage()])
        self.assertEqual("high", self.subagent_usage()[0].get("effort"))

    def test_record_without_effort_has_no_effort_field(self):
        sub = self.copy_fixture("effort_subagent_transcript.jsonl")

        usage.emit_usage_records(self.ctx, sub, AGENT, "agent_transcript")

        events = self.subagent_usage()
        self.assertEqual(2, len(events))
        self.assertEqual("high", events[0]["effort"])
        self.assertNotIn("effort", events[1])

    def test_effort_value_is_copied_verbatim(self):
        path = self.write("t.jsonl", [_rec("m1", effort="xhigh")])
        usage.emit_usage_records(self.ctx, path, None, "orchestrator_transcript")
        self.assertEqual("xhigh", self.orchestrator_usage()[0]["effort"])

    def test_effort_is_per_record_within_one_transcript(self):
        path = self.write("t.jsonl", [_rec("m1", effort="low"), _rec("m2"),
                                      _rec("m3", effort="high")])
        usage.emit_usage_records(self.ctx, path, None, "orchestrator_transcript")
        self.assertEqual(["low", None, "high"],
                         [e.get("effort") for e in self.orchestrator_usage()])


class TestReasoningInUsageRecord(EffortCase):

    def test_reasoning_tokens_is_inside_token_usage(self):
        orch = self.copy_fixture("effort_orchestrator_transcript.jsonl")

        usage.emit_usage_records(self.ctx, orch, None, "orchestrator_transcript")

        events = self.orchestrator_usage()
        self.assertEqual(30, events[0]["token_usage"]["reasoning_tokens"])
        self.assertEqual(50, events[0]["token_usage"]["output_tokens"])
        self.assertNotIn("reasoning_tokens", events[0])

    def test_zero_reasoning_survives_emission(self):
        orch = self.copy_fixture("effort_orchestrator_transcript.jsonl")
        usage.emit_usage_records(self.ctx, orch, None, "orchestrator_transcript")
        self.assertEqual(
            0, self.orchestrator_usage()[1]["token_usage"]["reasoning_tokens"])

    def test_absent_reasoning_is_not_written(self):
        sub = self.copy_fixture("effort_subagent_transcript.jsonl")
        usage.emit_usage_records(self.ctx, sub, AGENT, "agent_transcript")
        self.assertNotIn("reasoning_tokens",
                         self.subagent_usage()[1]["token_usage"])


class TestSupersedingObservations(EffortCase):

    def test_superseding_observation_carries_the_same_effort(self):
        path = self.write("t.jsonl", [_rec("m1", effort="high", output_tokens=10)])
        usage.emit_usage_records(self.ctx, path, None, "orchestrator_transcript")
        self.write("t.jsonl", [_rec("m1", effort="high", output_tokens=40,
                                    thinking=25)])

        written = usage.emit_usage_records(
            self.ctx, path, None, "orchestrator_transcript")

        events = self.orchestrator_usage()
        self.assertEqual(1, written)
        self.assertEqual(2, len(events))
        self.assertEqual(events[0]["record_id"], events[1]["record_id"])
        self.assertEqual(["high", "high"], [e.get("effort") for e in events])
        self.assertEqual(25, events[1]["token_usage"]["reasoning_tokens"])

    def test_unchanged_transcript_does_not_reemit_effort_records(self):
        path = self.write("t.jsonl", [_rec("m1", effort="medium")])
        usage.emit_usage_records(self.ctx, path, None, "orchestrator_transcript")
        again = usage.emit_usage_records(
            self.ctx, path, None, "orchestrator_transcript")
        self.assertEqual(0, again)


class TestFingerprintCoversEffort(unittest.TestCase):

    def facts(self, effort):
        return transcript.RecordFacts(
            "r1", 0, model="m", token_usage={"output_tokens": 3},
            effort=effort)

    def test_effort_change_alone_changes_the_fingerprint(self):
        self.assertNotEqual(
            usage.record_fingerprint(self.facts("medium")),
            usage.record_fingerprint(self.facts("high")))

    def test_presence_of_effort_changes_the_fingerprint(self):
        self.assertNotEqual(
            usage.record_fingerprint(self.facts(None)),
            usage.record_fingerprint(self.facts("high")))

    def test_same_effort_gives_same_fingerprint(self):
        self.assertEqual(
            usage.record_fingerprint(self.facts("high")),
            usage.record_fingerprint(self.facts("high")))


if __name__ == "__main__":
    unittest.main()
