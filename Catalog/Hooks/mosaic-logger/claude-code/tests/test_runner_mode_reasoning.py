"""Reasoning tokens in the runner-mode session-end usage.

The session-end invocation_end sums usage over every assistant record.
reasoning_tokens is the sum over the records that report it; a record without
the field counts as no reasoning; the key is omitted only when no record
reports it.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from runner_mode_support import (
    INSTANCE_ID, RUN_ID, WorkspaceTestCase, fire, runner_env, write_transcript,
)


def _record(message_id, thinking=None, output_tokens=10):
    usage = {"input_tokens": 10, "output_tokens": output_tokens}
    if thinking is not None:
        usage["output_tokens_details"] = {"thinking_tokens": thinking}
    return {"type": "assistant", "message": {
        "id": message_id, "model": "claude-opus-4-5", "role": "assistant",
        "content": [{"type": "text", "text": "x"}], "usage": usage}}


class SummedReasoningCase(WorkspaceTestCase):

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


class TestSummedSessionReasoning(SummedReasoningCase):

    def test_reasoning_is_summed_over_all_reporting_records(self):
        usage = self.session_end_usage(
            _record("m1", thinking=30), _record("m2", thinking=12))
        self.assertEqual(42, usage["reasoning_tokens"])

    def test_record_without_the_field_counts_as_no_reasoning(self):
        usage = self.session_end_usage(
            _record("m1", thinking=30), _record("m2"), _record("m3", thinking=5))
        self.assertEqual(35, usage["reasoning_tokens"])

    def test_all_zero_reports_sum_to_present_zero(self):
        usage = self.session_end_usage(
            _record("m1", thinking=0), _record("m2", thinking=0))
        self.assertIn("reasoning_tokens", usage)
        self.assertEqual(0, usage["reasoning_tokens"])

    def test_zero_report_among_records_without_field_is_present_zero(self):
        usage = self.session_end_usage(_record("m1", thinking=0), _record("m2"))
        self.assertEqual(0, usage["reasoning_tokens"])

    def test_no_reporting_record_omits_reasoning_tokens(self):
        usage = self.session_end_usage(_record("m1"), _record("m2"))
        self.assertNotIn("reasoning_tokens", usage)

    def test_output_tokens_stay_the_plain_sum(self):
        usage = self.session_end_usage(
            _record("m1", thinking=30, output_tokens=50),
            _record("m2", output_tokens=7))
        self.assertEqual(57, usage["output_tokens"])
        self.assertEqual(30, usage["reasoning_tokens"])


if __name__ == "__main__":
    unittest.main()
