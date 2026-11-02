"""Each transcript call is logged exactly once: across reads, read points, runs and processes.

Covers repeated and overlapping reads, a transcript that grows between reads, a read cut short
by a half-written line, one session holding two orchestrations, an append dropped on a held
lock, and several hook processes reading the same transcript at once.
"""

import json
import os
import subprocess
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core  # noqa: E402,F401  (puts the shared append module on sys.path)
import mosaic_logger_locked_append as locked  # noqa: E402
from locked_append_support import hold_lock  # noqa: E402
from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, INSTANCE_ID, SECOND_RUN, SESSION_ID, TranscriptLoggingCase, compaction_event,
    invocation, orchestrator, ts, usage_event)

HOOK_SCRIPT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "mosaic_logger.py")


def _calls(first, last, **kwargs):
    return [usage_event("call-%d" % n, ts=ts(n), event_id="e%d" % n, **kwargs)
            for n in range(first, last + 1)]


def _marker(kind, event_id, second):
    return {"type": kind, "id": event_id, "timestamp": ts(second), "data": {}}


class RepeatedReadsTest(TranscriptLoggingCase):
    def test_reading_the_same_transcript_again_logs_nothing_new(self):
        self.start_run()
        self.write(_calls(10, 14))
        for _ in range(3):
            self.stop()
        self.assertEqual(["call-%d" % n for n in range(10, 15)], self.record_ids())

    def test_call_present_twice_in_one_transcript_is_logged_once(self):
        self.start_run()
        self.write([usage_event("call-10", ts=ts(10), event_id="e1"),
                    usage_event("call-10", ts=ts(10), event_id="e2")])
        self.stop()
        self.assertEqual(["call-10"], self.record_ids())

    def test_overlapping_read_points_log_each_call_once(self):
        self.start_run()
        self.spawn("agent-1")
        self.write(_calls(10, 13) + [usage_event("sub-1", ts=ts(14), agent_id="agent-1", event_id="s1")])
        self.stop()
        self.subagent_stop("agent-1")
        self.stop("sessionEnd", reason="complete")
        self.append(_calls(20, 22))
        self.spawn("agent-2", "Implementation#2")
        self.stop()
        self.stop("sessionEnd", reason="complete")
        expected = ["call-%d" % n for n in (10, 11, 12, 13, 20, 21, 22)] + ["sub-1"]
        self.assertEqual(sorted(expected), self.record_ids())

    def test_calls_added_between_reads_are_logged_by_the_next_read(self):
        self.start_run()
        self.write(_calls(10, 11))
        self.stop()
        self.append(_calls(12, 13))
        self.stop()
        self.assertEqual(["call-%d" % n for n in range(10, 14)], self.record_ids())

    def test_session_end_picks_up_calls_left_after_the_last_stop(self):
        self.start_run()
        self.write(_calls(10, 11))
        self.stop()
        self.append(_calls(12, 13))
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(["call-%d" % n for n in range(10, 14)], self.record_ids())

    def test_subagent_start_and_subagent_stop_are_read_points(self):
        self.start_run()
        self.write(_calls(10, 10))
        self.spawn("agent-1")
        self.assertEqual(["call-10"], self.record_ids())
        self.append(_calls(11, 11))
        self.subagent_stop("agent-1")
        self.assertEqual(["call-10", "call-11"], self.record_ids())


class InterruptedReadTest(TranscriptLoggingCase):
    def test_half_written_last_line_is_logged_by_a_later_read_once_complete(self):
        self.start_run()
        third = json.dumps(_calls(12, 12)[0])
        self.write(_calls(10, 11), trailing=third[:60])
        self.stop()
        self.assertEqual(["call-10", "call-11"], self.record_ids())
        self.write(_calls(10, 12))
        self.stop()
        self.assertEqual(["call-10", "call-11", "call-12"], self.record_ids())

    def test_read_that_dies_after_some_appends_loses_and_repeats_nothing(self):
        self.start_run()
        self.write(_calls(10, 14))
        with self.failing_appends(lambda e: e.get("record_id") == "call-12", how="raise"):
            self.stop_surviving_a_crash()
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(["call-%d" % n for n in range(10, 15)], self.record_ids())

    def test_read_that_dies_on_the_first_append_loses_nothing(self):
        self.start_run()
        self.write(_calls(10, 12))
        with self.failing_appends(lambda e: e.get("event") == "usage_record", how="raise"):
            self.stop_surviving_a_crash()
        self.stop()
        self.assertEqual(["call-10", "call-11", "call-12"], self.record_ids())

    def test_run_that_stopped_early_loses_only_calls_after_its_last_read(self):
        self.start_run()
        self.write(_calls(10, 11))
        self.stop()
        self.append(_calls(12, 14))
        # the run stopped here without another stop; a later session end picks the rest up
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(["call-%d" % n for n in range(10, 15)], self.record_ids())


class TwoOrchestrationsInOneSessionTest(TranscriptLoggingCase):
    def test_each_call_is_logged_in_one_run_only(self):
        self.start_run(FIRST_RUN)
        self.write([_marker("session.resume", "r1", 5)] + _calls(10, 11) + [_marker("session.shutdown", "x1", 12)])
        self.stop()
        self.start_run(SECOND_RUN)
        self.append([_marker("session.resume", "r2", 19)] + _calls(20, 21) + [_marker("session.shutdown", "x2", 22)])
        self.stop()
        self.stop("sessionEnd", reason="complete")
        for record_id in ("call-10", "call-11"):
            self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record(record_id))
        for record_id in ("call-20", "call-21"):
            self.assertEqual([orchestrator(SECOND_RUN)], self.streams_of_record(record_id))

    def test_unread_call_is_logged_in_the_run_current_at_the_first_read_that_places_it(self):
        self.start_run(FIRST_RUN)
        self.write(_calls(10, 10))
        self.stop()
        self.append(_calls(11, 11))
        self.start_run(SECOND_RUN)
        self.stop()
        self.stop()
        self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record("call-10"))
        self.assertEqual([orchestrator(SECOND_RUN)], self.streams_of_record("call-11"))


class DroppedAppendTest(TranscriptLoggingCase):
    def setUp(self):
        super().setUp()
        patcher = mock.patch.object(locked, "WAIT_LIMIT_SECONDS", 0.05)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_call_dropped_on_a_held_lock_is_written_by_a_later_read(self):
        self.start_run()
        self.spawn("agent-1")
        self.write([usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="s1"),
                    usage_event("orch-1", ts=ts(11), event_id="o1")])
        lock = hold_lock(self.paths.invocation_events(FIRST_RUN, INSTANCE_ID))
        self.stop()
        self.assertEqual(["orch-1"], self.record_ids())
        lock.unlink()
        self.stop()
        self.assertEqual(["orch-1", "sub-1"], self.record_ids())

    def test_late_write_carries_the_call_own_timestamp_and_run(self):
        self.start_run(FIRST_RUN)
        self.spawn("agent-1")
        self.write([usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="s1")])
        lock = hold_lock(self.paths.invocation_events(FIRST_RUN, INSTANCE_ID))
        self.stop()
        self.start_run(SECOND_RUN)
        lock.unlink()
        self.stop()
        record = self.usage_record("sub-1")
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record("sub-1"))
        self.assertEqual(FIRST_RUN, record["run_id"])
        self.assertEqual(ts(9), record["timestamp"])

    def test_orchestrator_call_dropped_on_a_held_lock_is_written_later_once(self):
        self.start_run()
        self.write([usage_event("orch-1", ts=ts(10), event_id="o1")])
        lock = hold_lock(self.paths.orchestrator_events(FIRST_RUN))
        self.stop()
        self.assertEqual([], self.record_ids())
        lock.unlink()
        self.stop()
        self.stop()
        self.assertEqual(["orch-1"], self.record_ids())


class ConcurrentProcessesTest(TranscriptLoggingCase):
    PROCESSES = 4

    def hook_process(self):
        payload = {"cwd": str(self.workspace), "sessionId": SESSION_ID,
                   "transcriptPath": str(self.transcript)}
        child = subprocess.Popen([sys.executable, HOOK_SCRIPT, "agentStop"], stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=dict(os.environ))
        child.stdin.write(json.dumps(payload).encode("utf-8"))
        child.stdin.close()
        return child

    CALLS = 400

    def bulk_transcript(self):
        """CALLS usage calls (one per second, from 13:01:00) followed by one applied compaction."""
        events = [usage_event("call-%d" % n, ts="2026-10-30T13:%02d:%02d.000Z" % (1 + n // 60, n % 60),
                              event_id="e%d" % n) for n in range(self.CALLS)]
        events.append(compaction_event("attempt-1", ts="2026-10-30T13:30:00.000Z", event_id="c1"))
        return events

    def test_simultaneous_stop_hooks_log_each_call_once(self):
        self.start_run()
        self.write(self.bulk_transcript())
        children = [self.hook_process() for _ in range(self.PROCESSES)]
        for child in children:
            out, _ = child.communicate(timeout=120)
            self.assertEqual(b"", out)
        self.stop()
        expected = sorted("call-%d" % n for n in range(self.CALLS)) + ["compaction:attempt-1"]
        self.assertEqual(sorted(expected), self.record_ids())
        self.assertEqual(1, len(self.events_of("compaction")))


if __name__ == "__main__":
    unittest.main()
