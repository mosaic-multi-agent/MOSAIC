"""Format-change alarm condition of the transcript usage reader.

The alarm is raised when a transcript has at least one assistant.turn_end event and no readable
session.usage_record event. A usage event whose usage block is not an object, or that has no
readable token figure, is not readable.
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as support  # noqa: E402
from transcript_usage_support import compaction_event, read_events, turn_end_event, usage_event  # noqa: E402

_NO_TOKENS = ("inputTokens", "outputTokens", "cacheReadTokens", "cacheWriteTokens", "reasoningTokens")


def _alarm(events):
    with tempfile.TemporaryDirectory() as tmp:
        return read_events(tmp, events).alarm


class AlarmTest(unittest.TestCase):
    def test_turn_ends_without_any_usage_record_raise_the_alarm(self):
        self.assertTrue(_alarm([turn_end_event("t1"), turn_end_event("t2")]))

    def test_renamed_usage_fields_raise_the_alarm(self):
        import mosaic_logger_transcript_usage as reader
        self.assertTrue(reader.read_transcript_usage(support.ALARM_FIXTURE).alarm)

    def test_readable_usage_record_with_turn_ends_is_no_alarm(self):
        self.assertFalse(_alarm([usage_event(), turn_end_event()]))

    def test_no_turn_end_is_no_alarm_even_without_usage(self):
        self.assertFalse(_alarm([compaction_event()]))

    def test_empty_transcript_is_no_alarm(self):
        self.assertFalse(_alarm([]))

    def test_usage_block_that_is_not_an_object_does_not_count(self):
        for bad in ("text", None, [1, 2], 7):
            with self.subTest(usage=bad):
                event = {"type": "session.usage_record", "id": "e1", "timestamp": support.DEFAULT_TS,
                         "data": {"usage": bad}}
                self.assertTrue(_alarm([event, turn_end_event()]))

    def test_usage_record_without_data_does_not_count(self):
        event = {"type": "session.usage_record", "id": "e1", "timestamp": support.DEFAULT_TS}
        self.assertTrue(_alarm([event, turn_end_event()]))

    def test_usage_without_any_token_figure_does_not_count(self):
        self.assertTrue(_alarm([usage_event(drop=_NO_TOKENS), turn_end_event()]))

    def test_usage_with_only_non_numeric_token_figures_does_not_count(self):
        event = usage_event(drop=_NO_TOKENS, inputTokens="many", outputTokens=None)
        self.assertTrue(_alarm([event, turn_end_event()]))

    def test_one_readable_record_among_unreadable_ones_is_no_alarm(self):
        events = [usage_event("bad", drop=_NO_TOKENS, event_id="e1"), turn_end_event(),
                  usage_event("good", event_id="e2")]
        self.assertFalse(_alarm(events))

    def test_a_single_numeric_token_figure_is_readable(self):
        self.assertFalse(_alarm([usage_event(drop=("inputTokens", "cacheReadTokens", "cacheWriteTokens",
                                                   "reasoningTokens")), turn_end_event()]))

    def test_compaction_cost_alone_is_not_a_readable_usage_record(self):
        self.assertTrue(_alarm([compaction_event(), turn_end_event()]))

    def test_alarm_is_judged_over_the_whole_file_not_per_order(self):
        self.assertFalse(_alarm([turn_end_event("t1"), usage_event(), turn_end_event("t2")]))

    def test_alarm_condition_does_not_stop_the_reader_from_returning_a_result(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [turn_end_event()])
        self.assertTrue(result.readable)
        self.assertEqual((result.calls, result.compactions), ([], []))

    def test_missing_transcript_is_not_an_alarm(self):
        import mosaic_logger_transcript_usage as reader
        with tempfile.TemporaryDirectory() as tmp:
            self.assertFalse(reader.read_transcript_usage(os.path.join(tmp, "absent.jsonl")).alarm)


if __name__ == "__main__":
    unittest.main()
