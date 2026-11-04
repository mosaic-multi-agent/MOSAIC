"""Compaction reading from a GHCP CLI session transcript.

Covers: applied and not-applied compactions, agentId, trigger and sizes, cost figures from
compactionTokensUsed, and the compaction record key.
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as support  # noqa: E402
from transcript_usage_support import compaction_event, read_events  # noqa: E402


class AppliedFixtureTest(unittest.TestCase):
    def setUp(self):
        import mosaic_logger_transcript_usage as reader
        self.result = reader.read_transcript_usage(support.COMPACTION_FIXTURE)

    def test_one_compaction_and_its_call_are_read_separately(self):
        self.assertEqual(len(self.result.compactions), 1)
        self.assertEqual([c.record_id for c in self.result.calls], ["msg_011CgYVG8wdmXTDQVoia95oJ"])

    def test_record_key_is_prefixed_attempt_id(self):
        comp = self.result.compactions[0]
        self.assertEqual(comp.record_id, "compaction:8ef2cc3d-ae5c-46e0-912a-6d2a861596a0")
        self.assertEqual(comp.record_index, 1)

    def test_actor_trigger_sizes_and_completion(self):
        comp = self.result.compactions[0]
        self.assertEqual(comp.agent_id, "8521450f-b50a-4988-b96d-173e93ecf947")
        self.assertTrue(comp.applied)
        self.assertEqual(comp.trigger, "auto")
        self.assertEqual((comp.tokens_before, comp.tokens_after), (119103, 15945))
        self.assertEqual(comp.completed_at, "2026-10-30T13:40:32.230Z")

    def test_cost_figures(self):
        usage = self.result.compactions[0].usage
        self.assertEqual(usage.token_usage, {
            "input_tokens": 4, "cache_read_tokens": 159918, "cache_creation_tokens": 1878,
            "output_tokens": 4375})
        self.assertEqual((usage.model, usage.provider), ("claude-sonnet-5.5", "github-copilot"))
        self.assertEqual(usage.timestamp, "2026-10-30T13:40:06.830Z")


class NotAppliedFixtureTest(unittest.TestCase):
    def setUp(self):
        import mosaic_logger_transcript_usage as reader
        self.result = reader.read_transcript_usage(support.NOT_APPLIED_FIXTURE)

    def test_failed_and_unchanged_compactions_are_not_applied(self):
        self.assertEqual([(c.record_id, c.applied) for c in self.result.compactions],
                         [("compaction:attempt-failed-1", False), ("compaction:attempt-nochange-2", False)])

    def test_not_applied_compaction_still_carries_cost(self):
        failed = self.result.compactions[0]
        self.assertEqual(failed.usage.token_usage["input_tokens"], 1000)
        self.assertEqual(failed.usage.timestamp, "2026-10-30T13:40:30.730Z")

    def test_trigger_and_actor_of_each(self):
        failed, unchanged = self.result.compactions
        self.assertEqual((failed.trigger, failed.agent_id), ("auto", None))
        self.assertEqual((unchanged.trigger, unchanged.agent_id), ("manual", "8521450f-b50a-4988-b96d-173e93ecf947"))
        self.assertEqual(unchanged.usage.model, "claude-haiku-5.5")


class AppliedRuleTest(unittest.TestCase):
    def _one(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [compaction_event(**overrides)]).compactions[0]

    def test_success_true_and_outcome_applied_is_applied(self):
        self.assertTrue(self._one().applied)

    def test_success_false_is_not_applied(self):
        self.assertFalse(self._one(success=False).applied)

    def test_other_outcome_is_not_applied(self):
        self.assertFalse(self._one(outcome="skipped").applied)

    def test_missing_outcome_is_not_applied(self):
        self.assertFalse(self._one(drop=("outcome",)).applied)

    def test_non_boolean_success_is_not_applied(self):
        self.assertFalse(self._one(success="true").applied)


class TriggerAndSizesTest(unittest.TestCase):
    def _one(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [compaction_event(**overrides)]).compactions[0]

    def test_threshold_is_auto(self):
        self.assertEqual(self._one(trigger="threshold").trigger, "auto")

    def test_manual_is_manual(self):
        self.assertEqual(self._one(trigger="manual").trigger, "manual")

    def test_unknown_trigger_is_omitted(self):
        self.assertIsNone(self._one(trigger="overflow").trigger)

    def test_missing_trigger_is_omitted(self):
        self.assertIsNone(self._one(drop=("trigger",)).trigger)

    def test_missing_sizes_are_none(self):
        comp = self._one(drop=("preCompactionTokens", "postCompactionTokens"))
        self.assertEqual((comp.tokens_before, comp.tokens_after), (None, None))

    def test_orchestrator_compaction_has_no_agent_id(self):
        self.assertIsNone(self._one().agent_id)

    def test_agent_id_is_the_top_level_member(self):
        self.assertEqual(self._one(agent_id="agent-9").agent_id, "agent-9")


class CostTest(unittest.TestCase):
    def _read(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [compaction_event(**overrides)])

    def test_input_excludes_cache_and_no_reasoning_is_written(self):
        usage = self._read().compactions[0].usage
        self.assertEqual(usage.token_usage, {
            "input_tokens": 600, "cache_read_tokens": 300, "cache_creation_tokens": 100, "output_tokens": 40})

    def test_start_is_completion_minus_duration(self):
        usage = self._read(ts="2026-10-30T13:00:10.000Z").compactions[0].usage
        self.assertEqual(usage.timestamp, "2026-10-30T13:00:08.000Z")

    def test_absent_cost_block_gives_no_usage(self):
        comp = self._read(drop=("compactionTokensUsed",)).compactions[0]
        self.assertIsNone(comp.usage)
        self.assertEqual(comp.record_id, "compaction:attempt-1")

    def test_non_object_cost_block_gives_no_usage(self):
        self.assertIsNone(self._read(compactionTokensUsed="n/a").compactions[0].usage)

    def test_unreadable_duration_falls_back_to_completion_and_is_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            event = compaction_event(ts="2026-10-30T13:00:10.000Z")
            event["data"]["compactionTokensUsed"]["duration"] = "slow"
            bad = read_events(tmp, [event])
        self.assertEqual(bad.compactions[0].usage.timestamp, "2026-10-30T13:00:10.000Z")
        self.assertEqual(support.problem_keys(bad), [("compaction:attempt-1", "duration")])


class CompactionKeyTest(unittest.TestCase):
    def test_missing_attempt_id_uses_envelope_id_and_reports(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [compaction_event(attempt_id=None, event_id="env-77")])
        self.assertEqual([c.record_id for c in result.compactions], ["compaction:event:env-77"])
        self.assertEqual(support.problem_keys(result), [("compaction:event:env-77", "compactionAttemptId")])

    def test_no_attempt_id_and_no_envelope_id_leaves_the_compaction_out(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [compaction_event(attempt_id=None, event_id=None),
                                       compaction_event(attempt_id="kept", event_id="e2")])
        self.assertEqual([c.record_id for c in result.compactions], ["compaction:kept"])
        self.assertEqual(support.problem_keys(result), [(None, "compactionAttemptId")])


if __name__ == "__main__":
    unittest.main()
