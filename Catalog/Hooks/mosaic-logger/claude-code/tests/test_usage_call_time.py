"""usage_record events carry the time of their API call from the transcript.

Every emitted copy of one API message carries the same call time (the first
transcript line of the message), so the copy LogAnalyzer keeps shows the call
start. Re-processing a transcript gives the same times. A message without a
usable transcript timestamp keeps the hook-firing time and is still emitted.
"""

import json
import os
import pathlib
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core as core
import mosaic_logger_usage as usage
from runner_mode_support import read_jsonl

FIXTURES = pathlib.Path(__file__).parent / "fixtures"
RUN_ID = "20261101T160000Z-ab12"
AGENT = "Research#1"
FIRING_TIME = "2026-11-01T17:00:00.000Z"
LATER_FIRING_TIME = "2026-11-01T17:30:00.000Z"

# Message id -> call time (record_id of an emitted event is the message id).
CALL_TIMES = {
    "msg_ts_A": "2026-11-01T16:00:01.100Z",
    "msg_ts_B": "2026-11-01T16:00:07.250Z",
    "msg_ts_C": "2026-11-01T16:00:12.000Z",
    "msg_ts_D": "2026-11-01T16:00:20.900Z",
}


class UsageCase(unittest.TestCase):

    def make_ctx(self, firing_time=FIRING_TIME):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        root = pathlib.Path(tmp.name)
        payload = {"hook_event_name": "SubagentStop", "session_id": "s-time"}
        ctx = core.HookContext(payload, root, core.build_paths(root),
                               firing_time)
        ctx.run_id = RUN_ID
        return ctx

    def emit(self, ctx, fixture, path=None):
        """Emit the fixture's records as the AGENT; return (count, events)."""
        if path is None:
            path = ctx.workspace_root / fixture
            shutil.copyfile(FIXTURES / fixture, path)
        count = usage.emit_usage_records(
            ctx, str(path), AGENT, "agent_transcript")
        return count, self.events(ctx)

    def events(self, ctx):
        sink = core.event_sink_for(ctx, AGENT)
        return [e for e in read_jsonl(sink) if e["event"] == usage.USAGE_EVENT]


class TestEmittedCallTimes(UsageCase):

    def test_each_message_is_stamped_with_its_own_transcript_time(self):
        _, events = self.emit(self.make_ctx(), "usage_call_timestamps.jsonl")
        self.assertEqual(set(CALL_TIMES), {e["record_id"] for e in events})
        for event in events:
            self.assertEqual(CALL_TIMES[event["record_id"]],
                             event["timestamp"], event["record_id"])

    def test_n_messages_give_n_distinct_times_not_the_firing_time(self):
        _, events = self.emit(self.make_ctx(), "usage_call_timestamps.jsonl")
        times = {e["timestamp"] for e in events}
        self.assertEqual(4, len(times))
        self.assertNotIn(FIRING_TIME, times)

    def test_timestamp_keeps_its_envelope_position(self):
        _, events = self.emit(self.make_ctx(), "usage_call_timestamps.jsonl")
        keys = list(events[0].keys())
        self.assertEqual(
            ["schema_version", "event", "timestamp", "harness"], keys[:4])

    def test_later_line_emitted_in_a_later_firing_keeps_the_first_line_time(
            self):
        ctx = self.make_ctx()
        lines = (FIXTURES / "usage_call_timestamps.jsonl").read_text(
            "utf-8").splitlines()
        partial = ctx.workspace_root / "growing.jsonl"
        partial.write_text("\n".join(lines[:1]) + "\n", "utf-8")
        self.emit(ctx, "growing.jsonl", partial)

        partial.write_text("\n".join(lines) + "\n", "utf-8")
        later = core.HookContext(ctx.payload, ctx.workspace_root, ctx.paths,
                                 LATER_FIRING_TIME)
        later.run_id = RUN_ID
        _, events = self.emit(later, "growing.jsonl", partial)

        message_a = [e for e in events if e["record_id"] == "msg_ts_A"]
        self.assertGreater(len(message_a), 1)
        for event in message_a:
            self.assertEqual("2026-11-01T16:00:01.100Z", event["timestamp"],
                             event["record_index"])

    def test_first_line_without_timestamp_uses_first_later_line_time(self):
        _, events = self.emit(
            self.make_ctx(), "usage_call_timestamps_gaps.jsonl")
        first = [e for e in events if e["record_id"] == "msg_gap_first"]
        self.assertTrue(first)
        for event in first:
            self.assertEqual("2026-11-01T16:10:02.200Z",
                             event["timestamp"], event["record_id"])


class TestStableCallTimes(UsageCase):

    def test_reprocessing_gives_identical_timestamps_for_timed_records(self):
        _, first = self.emit(self.make_ctx(FIRING_TIME),
                             "usage_call_timestamps.jsonl")
        _, second = self.emit(self.make_ctx(LATER_FIRING_TIME),
                              "usage_call_timestamps.jsonl")
        by_id = {e["record_id"]: e["timestamp"] for e in first}
        self.assertEqual(set(CALL_TIMES), set(by_id))
        self.assertEqual(by_id, {e["record_id"]: e["timestamp"]
                                 for e in second})


class TestFallbackToFiringTime(UsageCase):

    def test_message_without_usable_timestamp_is_emitted_with_firing_time(
            self):
        _, events = self.emit(
            self.make_ctx(), "usage_call_timestamps_gaps.jsonl")
        untimed = [e for e in events if e["record_id"] == "msg_gap_none"]
        self.assertTrue(untimed)
        for event in untimed:
            self.assertEqual(FIRING_TIME, event["timestamp"])

    def test_timed_neighbours_of_an_untimed_message_keep_their_transcript_time(
            self):
        _, events = self.emit(
            self.make_ctx(), "usage_call_timestamps_gaps.jsonl")
        by_id = {e["record_id"]: e["timestamp"] for e in events}
        self.assertEqual("2026-11-01T16:10:12.000Z", by_id["msg_gap_ok"])
        self.assertEqual(FIRING_TIME, by_id["msg_gap_none"])

    def test_transcript_without_timestamps_is_stamped_with_firing_time(self):
        count, events = self.emit(
            self.make_ctx(), "requested_tools_transcript.jsonl")
        self.assertGreater(count, 0)
        self.assertEqual(count, len(events))
        for event in events:
            self.assertEqual(FIRING_TIME, event["timestamp"])


if __name__ == "__main__":
    unittest.main()
