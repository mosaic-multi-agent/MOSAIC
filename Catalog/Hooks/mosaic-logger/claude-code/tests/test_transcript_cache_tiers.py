"""Cache-write tier mapping from transcript records.

A record carries the tier split only when message.usage.cache_creation holds
numeric ephemeral_5m_input_tokens and ephemeral_1h_input_tokens that sum to
cache_creation_input_tokens. Otherwise neither tier key is written. Both
readers (per-record and last-record) follow the same rule.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_transcript as transcript

TIER_KEYS = ("cache_creation_5m_tokens", "cache_creation_1h_tokens")
_ABSENT = object()


def _record(total=_ABSENT, cache_creation=_ABSENT, message_id="msg_1"):
    usage = {"input_tokens": 5, "output_tokens": 2}
    if total is not _ABSENT:
        usage["cache_creation_input_tokens"] = total
    if cache_creation is not _ABSENT:
        usage["cache_creation"] = cache_creation
    return {"type": "assistant",
            "message": {"id": message_id, "model": "m", "usage": usage}}


class TierCase(unittest.TestCase):

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.path = pathlib.Path(tmp.name) / "t.jsonl"

    def write(self, *records):
        self.path.write_text(
            "\n".join(json.dumps(r) for r in records) + "\n", "utf-8")

    def per_record_usage(self):
        return transcript.read_assistant_records(str(self.path))[-1].token_usage

    def last_record_usage(self):
        return transcript.read_last_assistant_facts(str(self.path)).token_usage


class TierMappingRules:
    """Shared cases; subclasses bind the reader under test."""

    def usage_for(self, record):
        self.write(record)
        return self.read_usage()

    def assert_no_tier_fields(self, usage):
        for key in TIER_KEYS:
            self.assertNotIn(key, usage)

    def test_consistent_split_yields_both_tier_fields_summing_to_total(self):
        usage = self.usage_for(_record(10, {
            "ephemeral_5m_input_tokens": 4, "ephemeral_1h_input_tokens": 6}))
        self.assertEqual(4, usage["cache_creation_5m_tokens"])
        self.assertEqual(6, usage["cache_creation_1h_tokens"])
        self.assertEqual(10, usage["cache_creation_tokens"])

    def test_zero_valued_tier_is_kept_when_split_is_consistent(self):
        usage = self.usage_for(_record(8, {
            "ephemeral_5m_input_tokens": 8, "ephemeral_1h_input_tokens": 0}))
        self.assertEqual(8, usage["cache_creation_5m_tokens"])
        self.assertEqual(0, usage["cache_creation_1h_tokens"])

    def test_missing_cache_creation_object_yields_no_tier_fields(self):
        usage = self.usage_for(_record(10))
        self.assertEqual(10, usage["cache_creation_tokens"])
        self.assert_no_tier_fields(usage)

    def test_non_dict_cache_creation_yields_no_tier_fields(self):
        self.assert_no_tier_fields(self.usage_for(_record(10, "oops")))

    def test_only_5m_key_yields_no_tier_fields(self):
        self.assert_no_tier_fields(
            self.usage_for(_record(10, {"ephemeral_5m_input_tokens": 10})))

    def test_only_1h_key_yields_no_tier_fields(self):
        self.assert_no_tier_fields(
            self.usage_for(_record(10, {"ephemeral_1h_input_tokens": 10})))

    def test_non_numeric_tier_value_yields_no_tier_fields(self):
        self.assert_no_tier_fields(self.usage_for(_record(10, {
            "ephemeral_5m_input_tokens": "4", "ephemeral_1h_input_tokens": 6})))

    def test_boolean_tier_value_yields_no_tier_fields(self):
        self.assert_no_tier_fields(self.usage_for(_record(1, {
            "ephemeral_5m_input_tokens": True,
            "ephemeral_1h_input_tokens": 0})))

    def test_tiers_not_summing_to_total_yield_no_tier_fields(self):
        usage = self.usage_for(_record(10, {
            "ephemeral_5m_input_tokens": 4, "ephemeral_1h_input_tokens": 5}))
        self.assertEqual(10, usage["cache_creation_tokens"])
        self.assert_no_tier_fields(usage)

    def test_split_without_total_yields_no_tier_fields(self):
        self.assert_no_tier_fields(self.usage_for(_record(_ABSENT, {
            "ephemeral_5m_input_tokens": 4, "ephemeral_1h_input_tokens": 6})))

    def test_other_usage_keys_are_unchanged_by_a_split(self):
        usage = self.usage_for(_record(10, {
            "ephemeral_5m_input_tokens": 4, "ephemeral_1h_input_tokens": 6}))
        self.assertEqual(5, usage["input_tokens"])
        self.assertEqual(2, usage["output_tokens"])


class TestPerRecordReaderTiers(TierMappingRules, TierCase):

    def read_usage(self):
        return self.per_record_usage()


class TestLastRecordReaderTiers(TierMappingRules, TierCase):

    def read_usage(self):
        return self.last_record_usage()

    def test_tiers_come_from_the_last_record_only(self):
        self.write(
            _record(10, {"ephemeral_5m_input_tokens": 4,
                         "ephemeral_1h_input_tokens": 6}, message_id="a"),
            _record(7, message_id="b"))
        usage = self.last_record_usage()
        self.assertEqual(7, usage["cache_creation_tokens"])
        self.assert_no_tier_fields(usage)


if __name__ == "__main__":
    unittest.main()
