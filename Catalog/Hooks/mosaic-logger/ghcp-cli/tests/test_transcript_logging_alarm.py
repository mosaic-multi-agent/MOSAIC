"""The format-change alarm and per-field degradation diagnostics written by transcript logging.

Both go to OrchestrationLogs/mosaic-logger-diagnostics.jsonl. The alarm is one run-tagged line
per transcript session and run; an unreadable field is one line per session and field. A hook
never fails, and the records themselves are still logged.
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as tus  # noqa: E402
from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, RUN_ID, SECOND_RUN, SESSION_ID, TranscriptLoggingCase, ts, turn_end_event, usage_event)

ALARM = "transcript_format_changed"
FIELD = "transcript_field_unreadable"


def _turns(count, first=1):
    return [turn_end_event("t%d" % n, ts=ts(n)) for n in range(first, first + count)]


class AlarmTest(TranscriptLoggingCase):
    def test_turns_without_usage_write_one_run_tagged_alarm(self):
        self.start_run()
        self.write(_turns(2))
        self.stop()
        (line,) = self.diagnostics_of(ALARM)
        self.assertEqual(FIRST_RUN, line["run_id"])
        self.assertEqual("ghcp-cli", line["harness"])
        self.assertEqual(SESSION_ID, line["session_id"])
        self.assertEqual({"assistant_turns": 2, "usage_records": 0}, line["counts"])

    def test_alarm_is_never_repeated_by_later_reads(self):
        self.start_run()
        self.write(_turns(2))
        self.stop()
        self.append(_turns(2, first=3))
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(1, len(self.diagnostics_of(ALARM)))

    def test_no_alarm_while_the_session_has_no_run(self):
        self.write(_turns(2))
        self.stop()
        self.assertEqual([], self.diagnostics_of(ALARM))

    def test_alarm_stays_eligible_and_is_written_once_the_session_has_a_run(self):
        self.write(_turns(2))
        self.stop()
        self.start_run()
        self.stop()
        self.stop()
        (line,) = self.diagnostics_of(ALARM)
        self.assertEqual(FIRST_RUN, line["run_id"])

    def test_session_holding_two_orchestrations_alarms_once_per_run(self):
        self.start_run(FIRST_RUN)
        self.write(_turns(2))
        self.stop()
        self.start_run(SECOND_RUN)
        self.stop()
        self.stop()
        self.assertEqual([FIRST_RUN, SECOND_RUN], [d["run_id"] for d in self.diagnostics_of(ALARM)])

    def test_readable_usage_is_no_alarm(self):
        self.start_run()
        self.write([usage_event("call-1", ts=ts(5), event_id="e1")] + _turns(2))
        self.stop()
        self.assertEqual([], self.diagnostics_of(ALARM))

    def test_renamed_usage_fields_alarm_and_log_no_usage(self):
        self.start_run()
        self.transcript.write_bytes(tus.ALARM_FIXTURE.read_bytes())
        self.stop()
        self.assertEqual(1, len(self.diagnostics_of(ALARM)))
        self.assertEqual([], self.usage_records())

    def test_transcript_without_assistant_turns_is_no_alarm(self):
        self.start_run()
        self.write([{"type": "session.resume", "id": "r1", "timestamp": ts(1), "data": {}},
                    {"type": "session.shutdown", "id": "x1", "timestamp": ts(2), "data": {}}])
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual([], self.diagnostics_of(ALARM))
        self.assertEqual([], self.usage_records())

    def test_empty_transcript_is_no_alarm_and_logs_nothing(self):
        self.start_run()
        self.write([])
        self.stop()
        self.assertEqual([], self.diagnostics_of(ALARM))
        self.assertEqual([], self.usage_records())

    def test_runner_subagent_alarm_carries_the_runner_run(self):
        self.subagent_env()
        self.dispatch("userPromptSubmitted", {"prompt": "Do the research task"})
        self.write(_turns(1))
        self.stop()
        (line,) = self.diagnostics_of(ALARM)
        self.assertEqual(RUN_ID, line["run_id"])

    def test_runner_orchestrator_alarm_carries_the_runner_run(self):
        self.orchestrator_env()
        self.write(_turns(1))
        self.stop()
        (line,) = self.diagnostics_of(ALARM)
        self.assertEqual(RUN_ID, line["run_id"])

    def test_alarm_write_never_fails_the_hook_or_writes_stdout(self):
        self.start_run()
        self.write(_turns(1))
        (self.logs / "mosaic-logger-diagnostics.jsonl").mkdir(parents=True)  # a directory in the way
        self.assertEqual("", self.dispatch("agentStop", {"transcriptPath": str(self.transcript)}))


class FieldDegradationTest(TranscriptLoggingCase):
    def setUp(self):
        super().setUp()
        self.start_run()
        self.transcript.write_bytes(tus.UNREADABLE_FIXTURE.read_bytes())
        self.stop()

    def test_each_unreadable_field_is_reported_once_with_its_first_record(self):
        lines = self.diagnostics_of(FIELD)
        self.assertEqual(
            [("cacheReadTokens", "call-cache-bad"), ("duration", "call-duration-bad"),
             ("toolCounts", "call-tools-bad")],
            sorted((d["ids"][0], d["ids"][1]) for d in lines))

    def test_field_line_is_tagged_with_harness_run_and_session(self):
        lines = self.diagnostics_of(FIELD)
        self.assertEqual(3, len(lines))
        for line in lines:
            self.assertEqual("ghcp-cli", line["harness"])
            self.assertEqual(FIRST_RUN, line["run_id"])
            self.assertEqual(SESSION_ID, line["session_id"])

    def test_repeated_reads_do_not_repeat_the_field_lines(self):
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(3, len(self.diagnostics_of(FIELD)))

    def test_same_field_in_a_later_record_is_not_reported_again(self):
        self.append([usage_event("call-tools-bad-2", ts=ts(50), event_id="e9", toolCounts="oops")])
        self.stop()
        fields = [d["ids"][0] for d in self.diagnostics_of(FIELD)]
        self.assertEqual(1, fields.count("toolCounts"))
        self.assertIn("call-tools-bad-2", self.record_ids())

    def test_degraded_records_are_still_logged_without_the_unreadable_field(self):
        self.assertEqual(["call-cache-bad", "call-duration-bad", "call-tools-bad"], self.record_ids())
        self.assertNotIn("requested_tools", self.usage_record("call-tools-bad"))
        self.assertNotIn("input_tokens", self.usage_record("call-cache-bad")["token_usage"])
        self.assertEqual("2026-10-30T13:00:20.000Z", self.usage_record("call-duration-bad")["timestamp"])

    def test_degradation_is_no_alarm(self):
        self.assertEqual([], self.diagnostics_of(ALARM))


class DiagnosticsWriterTest(unittest.TestCase):
    def setUp(self):
        import mosaic_logger_diagnostics as diagnostics
        self.diagnostics = diagnostics
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = self._tmp.name

    def lines(self):
        import json
        path = os.path.join(self.root, self.diagnostics.DIAGNOSTICS_FILENAME)
        with open(path, encoding="utf-8") as handle:
            return [json.loads(ln) for ln in handle.read().splitlines() if ln.strip()]

    def test_file_name_is_the_shared_diagnostics_file(self):
        self.assertEqual("mosaic-logger-diagnostics.jsonl", self.diagnostics.DIAGNOSTICS_FILENAME)

    def test_report_writes_one_line_with_the_given_fields(self):
        self.diagnostics.report(self.root, ALARM, "short text", harness="ghcp-cli", run_id=FIRST_RUN,
                                session_id="s1", counts={"assistant_turns": 3, "usage_records": 0},
                                ids=["a", "b"])
        (line,) = self.lines()
        self.assertEqual(ALARM, line["kind"])
        self.assertEqual("short text", line["message"])
        self.assertEqual("ghcp-cli", line["harness"])
        self.assertEqual(FIRST_RUN, line["run_id"])
        self.assertEqual("s1", line["session_id"])
        self.assertEqual({"assistant_turns": 3, "usage_records": 0}, line["counts"])
        self.assertEqual(["a", "b"], line["ids"])
        self.assertTrue(line["timestamp"].endswith("Z"))

    def test_empty_optional_fields_are_left_out(self):
        self.diagnostics.report(self.root, FIELD, "m", harness="ghcp-cli")
        (line,) = self.lines()
        for key in ("run_id", "session_id", "counts", "ids"):
            self.assertNotIn(key, line)

    def test_every_call_writes_a_line_the_writer_does_not_deduplicate(self):
        for _ in range(2):
            self.diagnostics.report(self.root, ALARM, "m", harness="ghcp-cli", run_id=FIRST_RUN)
        self.assertEqual(2, len(self.lines()))

    def test_report_never_raises_when_the_root_is_unusable(self):
        blocker = os.path.join(self.root, "a-file")
        with open(blocker, "w", encoding="utf-8") as handle:
            handle.write("x")
        self.diagnostics.report(os.path.join(blocker, "sub"), ALARM, "m", harness="ghcp-cli")


class KindConstantsTest(unittest.TestCase):
    def test_kinds_are_the_stable_names(self):
        import mosaic_logger_transcript_logging as logging_module
        self.assertEqual(ALARM, logging_module.ALARM_KIND)
        self.assertEqual(FIELD, logging_module.FIELD_KIND)


if __name__ == "__main__":
    unittest.main()
