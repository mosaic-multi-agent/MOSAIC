"""The transcript reader reports the call time of each record's API message.

An API message spans several transcript lines that share one message.id. Every
record of the message reports the same call time: the normalised timestamp of
the first line of the message that has a usable one. A usable timestamp is a
string that parses as an ISO 8601 UTC time (a non-UTC offset, a date without a
time, or an impossible date is not usable). A message without any usable
timestamp reports None.
"""

import json
import os
import pathlib
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_transcript as transcript

FIXTURES = pathlib.Path(__file__).parent / "fixtures"


def _call_times(fixture):
    """Line uuid -> call_time for every assistant line of a fixture.

    The reader returns one record per assistant line in file order; the
    fixture's uuids name those lines (record_id is shared within a message).
    """
    path = FIXTURES / fixture
    uuids = []
    for line in path.read_text("utf-8").splitlines():
        entry = json.loads(line)
        if entry.get("type") == "assistant":
            uuids.append(entry["uuid"])
    records = transcript.read_assistant_records(str(path))
    assert len(records) == len(uuids)
    return {u: r.call_time for u, r in zip(uuids, records)}


class TestMessageCallTime(unittest.TestCase):

    def setUp(self):
        self.times = _call_times("usage_call_timestamps.jsonl")

    def test_every_line_of_a_multi_line_message_reports_the_first_line_time(self):
        for record_id in ("ts-a0", "ts-a1", "ts-a2"):
            self.assertEqual("2026-11-01T16:00:01.100Z", self.times[record_id],
                             record_id)

    def test_two_line_messages_report_their_first_line_time(self):
        self.assertEqual("2026-11-01T16:00:12.000Z", self.times["ts-c0"])
        self.assertEqual("2026-11-01T16:00:12.000Z", self.times["ts-c1"])
        self.assertEqual("2026-11-01T16:00:20.900Z", self.times["ts-d0"])
        self.assertEqual("2026-11-01T16:00:20.900Z", self.times["ts-d1"])

    def test_single_line_message_reports_its_own_time(self):
        self.assertEqual("2026-11-01T16:00:07.250Z", self.times["ts-b0"])

    def test_distinct_messages_report_distinct_times(self):
        message_times = {self.times[r] for r in
                         ("ts-a0", "ts-b0", "ts-c0", "ts-d0")}
        self.assertEqual(4, len(message_times))


class TestFirstLineWithoutUsableTimestamp(unittest.TestCase):

    def setUp(self):
        self.times = _call_times("usage_call_timestamps_gaps.jsonl")

    def test_missing_key_on_first_line_uses_first_later_line_with_one(self):
        for record_id in ("gap-f0", "gap-f1", "gap-f2"):
            self.assertEqual("2026-11-01T16:10:02.200Z", self.times[record_id],
                             record_id)

    def test_space_separated_and_non_string_values_are_skipped(self):
        for record_id in ("gap-u0", "gap-u1", "gap-u2"):
            self.assertEqual("2026-11-01T16:10:06.100Z", self.times[record_id],
                             record_id)

    def test_message_without_any_usable_timestamp_reports_none(self):
        for record_id in ("gap-n0", "gap-n1", "gap-n2"):
            self.assertIsNone(self.times[record_id], record_id)

    def test_unusable_message_does_not_affect_its_neighbours(self):
        self.assertEqual("2026-11-01T16:10:12.000Z", self.times["gap-k0"])

    def test_lines_without_message_id_are_each_their_own_message(self):
        self.assertEqual("2026-11-01T16:10:15.500Z", self.times["gap-i0"])
        self.assertEqual("2026-11-01T16:10:16.500Z", self.times["gap-i1"])


class TestTimestampNormalisation(unittest.TestCase):

    def setUp(self):
        self.times = _call_times("usage_call_timestamps_formats.jsonl")

    def test_short_fraction_is_padded_to_milliseconds(self):
        self.assertEqual("2026-11-01T16:20:01.500Z", self.times["fmt-short"])

    def test_long_fraction_is_reduced_to_milliseconds(self):
        self.assertEqual("2026-11-01T16:20:02.123Z", self.times["fmt-long"])

    def test_utc_offset_designator_becomes_z_and_missing_fraction_becomes_zeros(
            self):
        self.assertEqual("2026-11-01T16:20:03.000Z", self.times["fmt-offset"])

    def test_values_outside_the_usable_shape_report_none(self):
        for record_id in ("fmt-nonutc", "fmt-dateonly", "fmt-badday"):
            self.assertIsNone(self.times[record_id], record_id)


class TestTimestampFreeTranscript(unittest.TestCase):

    def test_records_of_a_transcript_without_timestamps_report_none(self):
        times = _call_times("requested_tools_transcript.jsonl")
        self.assertTrue(times)
        for record_id, call_time in times.items():
            self.assertIsNone(call_time, record_id)


if __name__ == "__main__":
    unittest.main()
