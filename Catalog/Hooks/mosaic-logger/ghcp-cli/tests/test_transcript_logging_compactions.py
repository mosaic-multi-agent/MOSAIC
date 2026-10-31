"""Transcript compactions: a compaction event on the compacting actor's stream plus a cost record.

An applied compaction writes its cost as a usage_record with call_kind compaction (record key
"compaction:<attempt id>") and then a compaction event. A compaction that did not succeed writes
the cost record only, so no consumer shows it as an applied compaction. The preCompact hook
writes nothing.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, INSTANCE_ID, RUN_ID, TranscriptLoggingCase, compaction_event, invocation,
    orchestrator, ts)

KEY = "compaction:attempt-1"


class CompactionLoggingCase(TranscriptLoggingCase):
    def compactions(self):
        return self.events_of("compaction")

    def only_compaction(self):
        """The single compaction event as (stream label, event); fails clearly when it is not one."""
        found = self.compactions()
        self.assertEqual(1, len(found), "expected one compaction event, found %d" % len(found))
        return found[0]

    def lines_of(self, label):
        """Event names of the compaction-related lines on one stream, in append order."""
        return [event["event"] for stream, event in self.all_events()
                if stream == label and event["event"] in ("usage_record", "compaction")
                and (event["event"] == "compaction" or event.get("call_kind") == "compaction")]


class AppliedCompactionTest(CompactionLoggingCase):
    def setUp(self):
        super().setUp()
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1")])
        self.stop()

    def test_compaction_event_is_on_the_orchestrator_stream(self):
        label, _ = self.only_compaction()
        self.assertEqual(orchestrator(FIRST_RUN), label)

    def test_compaction_event_carries_trigger_tokens_and_completion_time(self):
        _, event = self.only_compaction()
        self.assertEqual("auto", event["trigger"])
        self.assertEqual(9000, event["tokens_before"])
        self.assertEqual(1500, event["tokens_after"])
        self.assertEqual(ts(30), event["timestamp"])
        self.assertEqual(FIRST_RUN, event["run_id"])
        self.assertEqual("1.5.0", event["schema_version"])
        self.assertNotIn("phase", event)

    def test_compaction_cost_is_a_usage_record_with_call_kind_compaction(self):
        record = self.usage_record(KEY)
        self.assertEqual("compaction", record["call_kind"])
        self.assertEqual(orchestrator(FIRST_RUN), self.streams_of_record(KEY)[0])

    def test_compaction_cost_record_has_the_call_start_and_token_mapping(self):
        record = self.usage_record(KEY)
        self.assertEqual(ts(28), record["timestamp"])
        self.assertEqual({"input_tokens": 600, "cache_read_tokens": 300,
                          "cache_creation_tokens": 100, "output_tokens": 40}, record["token_usage"])
        self.assertEqual("claude-sonnet-5.5", record["model"])
        self.assertEqual("1.7.0", record["schema_version"])
        self.assertNotIn("requested_tools", record)

    def test_cost_record_is_appended_before_the_compaction_event(self):
        self.assertEqual(["usage_record", "compaction"], self.lines_of(orchestrator(FIRST_RUN)))

    def test_manual_trigger_is_mapped(self):
        self.write([compaction_event("attempt-m", ts=ts(40), event_id="c2", trigger="manual")])
        self.stop()
        events = [e for _, e in self.compactions() if e["timestamp"] == ts(40)]
        self.assertEqual(["manual"], [e["trigger"] for e in events])

    def test_repeated_reads_log_the_compaction_once(self):
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(1, len(self.compactions()))
        self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record(KEY))


class CompactionPlacementTest(CompactionLoggingCase):
    def test_subagent_compaction_goes_to_the_mapped_invocation_stream(self):
        self.start_run()
        self.spawn("agent-1")
        self.write([compaction_event("attempt-1", ts=ts(30), agent_id="agent-1", event_id="c1")])
        self.stop()
        label, _ = self.only_compaction()
        self.assertEqual(invocation(FIRST_RUN), label)
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record(KEY))

    def test_compaction_of_an_unmapped_agent_waits_for_the_mapping(self):
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), agent_id="agent-late", event_id="c1")])
        self.stop()
        self.assertEqual([], self.compactions())
        self.assertEqual([], self.streams_of_record(KEY))
        self.spawn("agent-late")
        self.stop()
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record(KEY))
        self.assertEqual(1, len(self.compactions()))

    def test_unmapped_compaction_is_not_attributed_to_another_mapped_agent(self):
        self.start_run()
        self.spawn("agent-1")
        self.write([compaction_event("attempt-1", ts=ts(30), agent_id="agent-late", event_id="c1")])
        self.stop()
        self.assertEqual([], self.compactions())
        self.assertEqual([], self.streams_of_record(KEY))
        self.spawn("agent-late", "Implementation#2")
        self.stop()
        label, _ = self.only_compaction()
        self.assertEqual(invocation(FIRST_RUN, "Implementation#2"), label)
        self.assertEqual([label], self.streams_of_record(KEY))

    def test_runner_subagent_compaction_goes_to_the_invocation_stream(self):
        self.subagent_env()
        self.dispatch("userPromptSubmitted", {"prompt": "Do the research task"})
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1")])
        self.stop()
        label, event = self.only_compaction()
        self.assertEqual(invocation(RUN_ID, INSTANCE_ID), label)
        self.assertEqual([invocation(RUN_ID, INSTANCE_ID)], self.streams_of_record(KEY))
        self.assertEqual("agent_transcript", self.usage_record(KEY)["source"])

    def test_runner_orchestrator_compaction_goes_to_the_orchestrator_stream(self):
        self.orchestrator_env()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1")])
        self.stop()
        label, _ = self.only_compaction()
        self.assertEqual(orchestrator(RUN_ID), label)
        self.assertEqual([orchestrator(RUN_ID)], self.streams_of_record(KEY))


class NotAppliedCompactionTest(CompactionLoggingCase):
    def log(self, **overrides):
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1", **overrides)])
        self.stop()

    def test_failed_compaction_logs_its_cost_without_a_compaction_event(self):
        self.log(success=False, outcome="failed")
        self.assertEqual("compaction", self.usage_record(KEY)["call_kind"])
        self.assertEqual([], self.compactions())

    def test_successful_compaction_that_changed_nothing_is_not_applied(self):
        self.log(success=True, outcome="no_change")
        self.assertEqual("compaction", self.usage_record(KEY)["call_kind"])
        self.assertEqual([], self.compactions())

    def test_outcome_applied_without_success_is_not_applied(self):
        self.log(success=False, outcome="applied")
        self.assertEqual([], self.compactions())
        self.assertIn(KEY, self.record_ids())

    def test_not_applied_cost_is_logged_once_across_reads(self):
        self.log(success=False, outcome="failed")
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record(KEY))


class CompactionWithoutCostOrKeyTest(CompactionLoggingCase):
    def test_compaction_without_token_usage_writes_the_event_only(self):
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1", compactionTokensUsed=None)])
        self.stop()
        self.assertEqual(1, len(self.compactions()))
        self.assertEqual([], self.streams_of_record(KEY))

    def test_compaction_with_neither_attempt_id_nor_event_id_is_never_logged(self):
        self.start_run()
        self.write([compaction_event(None, ts=ts(30), event_id=None)])
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual([], self.compactions())
        self.assertEqual([], self.usage_records())

    def test_compaction_without_attempt_id_is_keyed_by_the_event_id(self):
        self.start_run()
        self.write([compaction_event(None, ts=ts(30), event_id="ev-77")])
        self.stop()
        self.assertEqual(1, len(self.compactions()))
        self.assertEqual("compaction", self.usage_record("compaction:event:ev-77")["call_kind"])


class PreCompactHookTest(CompactionLoggingCase):
    def test_pre_compact_writes_no_compaction_event_in_native_mode(self):
        self.start_run()
        self.dispatch("preCompact", {"trigger": "auto"})
        self.assertEqual([], self.compactions())

    def test_pre_compact_naming_a_subagent_writes_no_compaction_event(self):
        self.start_run()
        self.spawn("agent-1")
        self.dispatch("preCompact", {"agentId": "agent-1", "trigger": "auto"})
        self.assertEqual([], self.compactions())

    def test_pre_compact_does_not_duplicate_the_transcript_compaction(self):
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1")])
        self.dispatch("preCompact", {"trigger": "auto"})
        self.stop()
        _, event = self.only_compaction()
        self.assertEqual(ts(30), event["timestamp"])
        self.assertEqual(9000, event["tokens_before"])


class DroppedCompactionAppendTest(CompactionLoggingCase):
    """A compaction counts as logged only when both its cost record and its event were appended."""

    def setUp(self):
        super().setUp()
        self.start_run()
        self.write([compaction_event("attempt-1", ts=ts(30), event_id="c1")])

    def assert_logged_once_with_own_time(self):
        self.assertEqual(1, len(self.streams_of_record(KEY)))
        _, event = self.only_compaction()
        self.assertEqual(ts(30), event["timestamp"])
        self.assertEqual(FIRST_RUN, event["run_id"])
        record = self.usage_record(KEY)
        self.assertEqual(ts(28), record["timestamp"])
        self.assertEqual(FIRST_RUN, record["run_id"])

    def test_dropped_cost_record_keeps_the_event_from_being_written_alone(self):
        with self.failing_appends(lambda e: e.get("record_id") == KEY):
            self.stop()
        self.assertEqual([], self.compactions())

    def test_cost_record_dropped_once_is_written_with_the_event_by_a_later_read(self):
        with self.failing_appends(lambda e: e.get("record_id") == KEY):
            self.stop()
        self.stop()
        self.assert_logged_once_with_own_time()

    def test_dropped_event_is_written_later_without_repeating_the_cost_record(self):
        with self.failing_appends(lambda e: e.get("event") == "compaction"):
            self.stop()
        self.assertEqual(1, len(self.streams_of_record(KEY)))
        self.assertEqual([], self.compactions())
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assert_logged_once_with_own_time()


if __name__ == "__main__":
    unittest.main()
