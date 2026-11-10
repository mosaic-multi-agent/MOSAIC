"""The usage_record a transcript call becomes: wire fields, per-item stamps and schema version."""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, SECOND_RUN, SESSION_ID, TranscriptLoggingCase, call_start, ts, usage_event)


class UsageRecordFieldsTest(TranscriptLoggingCase):
    def logged(self, **overrides):
        self.start_run()
        self.write([usage_event("call-1", ts=ts(30), event_id="e1", **overrides)])
        self.stop()
        return self.usage_record("call-1")

    def test_record_key_is_the_api_call_id(self):
        self.assertEqual("call-1", self.logged()["record_id"])

    def test_timestamp_is_the_call_start_not_the_event_time_or_the_hook_firing(self):
        self.assertEqual(call_start(30), self.logged()["timestamp"])

    def test_token_fields_follow_the_transcript_mapping(self):
        usage = self.logged()["token_usage"]
        self.assertEqual({"input_tokens": 500, "cache_read_tokens": 300,
                          "cache_creation_tokens": 200, "output_tokens": 50,
                          "reasoning_tokens": 20}, usage)

    def test_model_provider_and_effort_are_carried(self):
        record = self.logged()
        self.assertEqual("claude-sonnet-5.5", record["model"])
        self.assertEqual("github-copilot", record["provider"])
        self.assertEqual("medium", record["effort"])

    def test_event_is_stamped_with_the_item_run_and_session(self):
        record = self.logged()
        self.assertEqual(FIRST_RUN, record["run_id"])
        self.assertEqual(SESSION_ID, record["session_id"])
        self.assertEqual("ghcp-cli", record["harness"])

    def test_usage_record_is_stamped_schema_1_7_0(self):
        self.assertEqual("1.7.0", self.logged()["schema_version"])

    def test_a_main_call_has_no_call_kind(self):
        self.assertNotIn("call_kind", self.logged())

    def test_requested_tools_come_from_the_tool_counts(self):
        record = self.logged(toolCounts={"view": 2, "grep": 1})
        self.assertEqual({"view": 2, "grep": 1}, record["requested_tools"])

    def test_zero_count_tools_are_dropped(self):
        record = self.logged(toolCounts={"view": 1, "grep": 0})
        self.assertEqual({"view": 1}, record["requested_tools"])

    def test_a_call_that_requested_no_tools_writes_an_empty_object(self):
        record = self.logged(toolCounts={})
        self.assertIn("requested_tools", record)
        self.assertEqual({}, record["requested_tools"])

    def test_provider_is_left_out_when_the_transcript_does_not_name_copilot(self):
        record = self.logged(provider={"id": "byok", "kind": "custom"})
        self.assertNotIn("provider", record)


class PerItemStampTest(TranscriptLoggingCase):
    def test_each_call_keeps_its_own_timestamp(self):
        self.start_run()
        self.write([usage_event("c1", ts=ts(10), event_id="e1"),
                    usage_event("c2", ts=ts(40), event_id="e2")])
        self.stop()
        self.assertEqual(call_start(10), self.usage_record("c1")["timestamp"])
        self.assertEqual(call_start(40), self.usage_record("c2")["timestamp"])

    def test_call_first_placed_after_the_session_moved_on_carries_the_new_run(self):
        self.start_run(FIRST_RUN)
        self.write([usage_event("c1", ts=ts(10), event_id="e1")])
        self.stop()
        self.start_run(SECOND_RUN)
        self.append([usage_event("c2", ts=ts(20), event_id="e2")])
        self.stop()
        self.assertEqual(FIRST_RUN, self.usage_record("c1")["run_id"])
        self.assertEqual(SECOND_RUN, self.usage_record("c2")["run_id"])


if __name__ == "__main__":
    unittest.main()
