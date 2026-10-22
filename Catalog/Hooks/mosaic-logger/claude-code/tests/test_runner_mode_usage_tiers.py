"""Cache-write tiers in the runner-mode session-end usage.

The session-end invocation_end sums usage over every assistant record. Tier
fields appear on the summed block only when every summed record carried a valid
split; otherwise neither appears, so the sum never has a one-sided or
non-summing split.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from runner_mode_support import (
    INSTANCE_ID, RUN_ID, WorkspaceTestCase, fire, runner_env, write_transcript,
)

TIER_KEYS = ("cache_creation_5m_tokens", "cache_creation_1h_tokens")


def _record(message_id, total=None, tiers=None):
    usage = {"input_tokens": 10, "output_tokens": 3}
    if total is not None:
        usage["cache_creation_input_tokens"] = total
    if tiers is not None:
        usage["cache_creation"] = {
            "ephemeral_5m_input_tokens": tiers[0],
            "ephemeral_1h_input_tokens": tiers[1]}
    return {"type": "assistant", "message": {
        "id": message_id, "model": "claude-opus-4-5", "role": "assistant",
        "content": [{"type": "text", "text": "x"}], "usage": usage}}


class SummedUsageCase(WorkspaceTestCase):

    def session_end_usage(self, *records):
        transcript = write_transcript(self.workspace, list(records))
        with runner_env(self.workspace, role="subagent", run_id=RUN_ID,
                        instance_id=INSTANCE_ID):
            fire(self.workspace, "UserPromptSubmit", user_prompt="go",
                 transcript_path=transcript)
            fire(self.workspace, "SessionEnd", reason="other",
                 transcript_path=transcript)
        ends = [e for e in self.invocation_events()
                if e["event"] == "invocation_end"]
        self.assertEqual(1, len(ends))
        return ends[0]["token_usage"]

    def assert_no_tier_fields(self, usage):
        for key in TIER_KEYS:
            self.assertNotIn(key, usage)


class TestSummedSessionUsageTiers(SummedUsageCase):

    def test_all_records_with_valid_split_yield_summed_tier_fields(self):
        usage = self.session_end_usage(
            _record("m1", 10, (4, 6)), _record("m2", 20, (15, 5)))
        self.assertEqual(30, usage["cache_creation_tokens"])
        self.assertEqual(19, usage["cache_creation_5m_tokens"])
        self.assertEqual(11, usage["cache_creation_1h_tokens"])

    def test_summed_tiers_equal_summed_total(self):
        usage = self.session_end_usage(
            _record("m1", 10, (4, 6)), _record("m2", 20, (15, 5)))
        self.assertEqual(
            usage["cache_creation_tokens"],
            usage["cache_creation_5m_tokens"]
            + usage["cache_creation_1h_tokens"])

    def test_mix_of_split_and_total_only_records_yields_no_tier_fields(self):
        usage = self.session_end_usage(
            _record("m1", 10, (4, 6)), _record("m2", 20))
        self.assertEqual(30, usage["cache_creation_tokens"])
        self.assert_no_tier_fields(usage)

    def test_mix_of_valid_and_non_summing_records_yields_no_tier_fields(self):
        usage = self.session_end_usage(
            _record("m1", 10, (4, 6)), _record("m2", 20, (1, 1)))
        self.assertEqual(30, usage["cache_creation_tokens"])
        self.assert_no_tier_fields(usage)

    def test_record_without_cache_write_among_split_records_yields_no_tiers(self):
        usage = self.session_end_usage(
            _record("m1", 10, (4, 6)), _record("m2"))
        self.assert_no_tier_fields(usage)


if __name__ == "__main__":
    unittest.main()
