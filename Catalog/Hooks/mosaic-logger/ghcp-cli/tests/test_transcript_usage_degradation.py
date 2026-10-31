"""Degradation, missing record keys and never-raising behaviour of the transcript usage reader."""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as support  # noqa: E402
from transcript_usage_support import read_events, usage_event  # noqa: E402


class UnreadableFieldFixtureTest(unittest.TestCase):
    def setUp(self):
        import mosaic_logger_transcript_usage as reader
        self.result = reader.read_transcript_usage(support.UNREADABLE_FIXTURE)
        self.calls = {c.record_id: c for c in self.result.calls}

    def test_every_record_is_still_read(self):
        self.assertEqual(sorted(self.calls), ["call-cache-bad", "call-duration-bad", "call-tools-bad"])

    def test_each_unreadable_field_is_reported_once_against_its_record(self):
        self.assertEqual(sorted(support.problem_keys(self.result)), [
            ("call-cache-bad", "cacheReadTokens"), ("call-duration-bad", "duration"),
            ("call-tools-bad", "toolCounts")])

    def test_bad_tool_counts_degrades_only_the_tools(self):
        call = self.calls["call-tools-bad"]
        self.assertIsNone(call.requested_tools)
        self.assertEqual(call.token_usage["input_tokens"], 500)
        self.assertEqual(call.timestamp, "2026-10-30T13:00:09.000Z")

    def test_bad_duration_uses_the_event_timestamp_as_start(self):
        call = self.calls["call-duration-bad"]
        self.assertEqual(call.timestamp, "2026-10-30T13:00:20.000Z")
        self.assertEqual(call.requested_tools, {"view": 1})
        self.assertEqual(call.token_usage["output_tokens"], 50)

    def test_bad_cache_figure_leaves_out_the_uncached_input_only(self):
        call = self.calls["call-cache-bad"]
        self.assertNotIn("input_tokens", call.token_usage)
        self.assertNotIn("cache_read_tokens", call.token_usage)
        self.assertEqual(call.token_usage["output_tokens"], 50)
        self.assertEqual(call.token_usage["cache_creation_tokens"], 200)

    def test_degraded_records_are_no_alarm(self):
        self.assertFalse(self.result.alarm)


class MissingKeyTest(unittest.TestCase):
    def test_missing_api_call_id_uses_the_envelope_id_and_reports(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [usage_event(api_call_id=None, event_id="env-5")])
        self.assertEqual([c.record_id for c in result.calls], ["event:env-5"])
        self.assertEqual(support.problem_keys(result), [("event:env-5", "apiCallId")])

    def test_no_api_call_id_and_no_envelope_id_leaves_the_call_out(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [usage_event(api_call_id=None, event_id=None),
                                       usage_event("kept", event_id="e2")])
        self.assertEqual([c.record_id for c in result.calls], ["kept"])
        self.assertEqual(support.problem_keys(result), [(None, "apiCallId")])

    def test_left_out_call_does_not_make_an_alarm(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [usage_event(api_call_id=None, event_id=None),
                                       support.turn_end_event()])
        self.assertFalse(result.alarm)


class UnusableTranscriptTest(unittest.TestCase):
    def setUp(self):
        import mosaic_logger_transcript_usage as reader
        self.reader = reader

    def _assert_empty(self, result, readable):
        self.assertEqual(result.readable, readable)
        self.assertEqual((result.calls, result.compactions, result.problems), ([], [], []))
        self.assertFalse(result.alarm)

    def test_missing_file_is_unreadable_and_empty(self):
        with tempfile.TemporaryDirectory() as tmp:
            self._assert_empty(self.reader.read_transcript_usage(os.path.join(tmp, "nope.jsonl")), False)

    def test_directory_instead_of_file_does_not_raise(self):
        with tempfile.TemporaryDirectory() as tmp:
            self._assert_empty(self.reader.read_transcript_usage(tmp), False)

    def test_none_and_non_path_values_do_not_raise(self):
        for bad in (None, 42, b"", object()):
            with self.subTest(value=bad):
                self._assert_empty(self.reader.read_transcript_usage(bad), False)

    def test_empty_file_is_readable_and_empty(self):
        with tempfile.TemporaryDirectory() as tmp:
            self._assert_empty(read_events(tmp, []), True)


class PartialAndMalformedLinesTest(unittest.TestCase):
    def test_trailing_partial_line_is_skipped_without_a_problem(self):
        import mosaic_logger_transcript_usage as reader
        result = reader.read_transcript_usage(support.PARTIAL_FIXTURE)
        self.assertEqual([c.record_id for c in result.calls],
                         ["msg_011CgYVG8wdmXTDQVoia95oJ", "msg_011CgYVH9vhJ2XcMRiGduZqp"])
        self.assertEqual(result.problems, [])
        self.assertTrue(result.readable)

    def test_partial_line_without_newline_after_complete_events(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [usage_event("a")], trailing='{"type":"session.usage_record","da')
        self.assertEqual([c.record_id for c in result.calls], ["a"])
        self.assertEqual(result.problems, [])

    def test_malformed_and_non_object_lines_between_events_are_skipped(self):
        events = [usage_event("a", event_id="e1"), "{not json", "[1,2]", "42", "null", "", "   ",
                  usage_event("b", event_id="e2")]
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, events)
        self.assertEqual([c.record_id for c in result.calls], ["a", "b"])
        self.assertEqual(result.problems, [])

    def test_invalid_utf8_line_is_skipped_and_others_are_read(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = support.write_transcript(tmp, [usage_event("a", event_id="e1")])
            with open(path, "ab") as handle:
                handle.write(b'{"type":"x","data":"\xff\xfe"}\n')
            import json
            with open(path, "a", encoding="utf-8") as handle:
                handle.write(json.dumps(usage_event("b", event_id="e2")) + "\n")
            import mosaic_logger_transcript_usage as reader
            result = reader.read_transcript_usage(path)
        self.assertEqual([c.record_id for c in result.calls], ["a", "b"])

    def test_events_with_unexpected_shapes_do_not_raise(self):
        odd = [
            {"type": "session.usage_record"},
            {"type": "session.usage_record", "data": "text", "id": "e1"},
            {"type": "session.usage_record", "data": {"usage": {"apiCallId": 5}}, "id": "e2"},
            {"type": "session.compaction_complete", "data": None, "id": "e3"},
            {"type": "session.compaction_complete", "data": {"compactionAttemptId": ["x"]}, "id": "e4"},
            {"type": 7, "data": {}},
            {"agentId": {"a": 1}, "type": "session.usage_record", "id": "e5",
             "data": {"usage": {"apiCallId": "ok", "inputTokens": 1, "outputTokens": 1}}},
            usage_event("fine", event_id="e6"),
        ]
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, odd)
        self.assertTrue(result.readable)
        self.assertIn("fine", [c.record_id for c in result.calls])

    def test_non_string_timestamp_does_not_raise(self):
        event = usage_event("a")
        event["timestamp"] = 12345
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [event, usage_event("b", event_id="e2")])
        self.assertIn("b", [c.record_id for c in result.calls])


if __name__ == "__main__":
    unittest.main()
