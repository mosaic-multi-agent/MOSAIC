"""Reasoning tokens and effort read from transcript assistant records.

token_usage.reasoning_tokens is copied from
message.usage.output_tokens_details.thinking_tokens (including 0) by both
readers and is omitted when the field is absent; output_tokens is never
adjusted. RecordFacts.effort is the record's top-level `effort`, verbatim, and
None when the record has none.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_transcript as transcript

FIXTURES = pathlib.Path(__file__).parent / "fixtures"
_ABSENT = object()


def _record(thinking=_ABSENT, effort=_ABSENT, output_tokens=50,
            details=_ABSENT, message_id="msg_1"):
    usage = {"input_tokens": 5, "output_tokens": output_tokens}
    if details is not _ABSENT:
        usage["output_tokens_details"] = details
    elif thinking is not _ABSENT:
        usage["output_tokens_details"] = {"thinking_tokens": thinking}
    record = {"type": "assistant",
              "message": {"id": message_id, "model": "m", "usage": usage}}
    if effort is not _ABSENT:
        record["effort"] = effort
    return record


class ReaderCase(unittest.TestCase):

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.path = pathlib.Path(tmp.name) / "t.jsonl"

    def write(self, *records):
        self.path.write_text(
            "\n".join(json.dumps(r) for r in records) + "\n", "utf-8")

    def per_record(self):
        return transcript.read_assistant_records(str(self.path))

    def last_usage(self):
        return transcript.read_last_assistant_facts(str(self.path)).token_usage


class ReasoningRules:
    """Shared cases; subclasses bind the reader under test."""

    def usage_for(self, record):
        self.write(record)
        return self.read_usage()

    def test_thinking_tokens_become_reasoning_tokens(self):
        usage = self.usage_for(_record(thinking=30))
        self.assertEqual(30, usage["reasoning_tokens"])

    def test_zero_thinking_tokens_is_kept_as_zero(self):
        usage = self.usage_for(_record(thinking=0))
        self.assertIn("reasoning_tokens", usage)
        self.assertEqual(0, usage["reasoning_tokens"])

    def test_output_tokens_are_unchanged_by_reasoning(self):
        usage = self.usage_for(_record(thinking=30, output_tokens=50))
        self.assertEqual(50, usage["output_tokens"])
        self.assertEqual(30, usage["reasoning_tokens"])

    def test_absent_details_object_omits_reasoning_tokens(self):
        usage = self.usage_for(_record())
        self.assertEqual(50, usage["output_tokens"])
        self.assertNotIn("reasoning_tokens", usage)

    def test_details_without_thinking_tokens_omits_reasoning_tokens(self):
        usage = self.usage_for(_record(details={"other": 3}))
        self.assertNotIn("reasoning_tokens", usage)

    def test_non_numeric_thinking_tokens_omits_reasoning_tokens(self):
        usage = self.usage_for(_record(thinking="30"))
        self.assertNotIn("reasoning_tokens", usage)

    def test_boolean_thinking_tokens_omits_reasoning_tokens(self):
        usage = self.usage_for(_record(thinking=True))
        self.assertNotIn("reasoning_tokens", usage)

    def test_non_dict_details_omits_reasoning_tokens(self):
        usage = self.usage_for(_record(details="oops"))
        self.assertNotIn("reasoning_tokens", usage)

    def test_thinking_tokens_alone_yield_reasoning_only_usage(self):
        record = {"type": "assistant", "message": {
            "id": "m", "model": "m", "usage": {
                "output_tokens_details": {"thinking_tokens": 9}}}}
        usage = self.usage_for(record)
        self.assertEqual({"reasoning_tokens": 9}, usage)


class TestPerRecordReasoning(ReasoningRules, ReaderCase):

    def read_usage(self):
        return self.per_record()[-1].token_usage

    def test_each_record_carries_its_own_reasoning(self):
        self.write(_record(thinking=7, message_id="a"),
                   _record(message_id="b"),
                   _record(thinking=0, message_id="c"))
        usages = [r.token_usage for r in self.per_record()]
        self.assertEqual(7, usages[0]["reasoning_tokens"])
        self.assertNotIn("reasoning_tokens", usages[1])
        self.assertEqual(0, usages[2]["reasoning_tokens"])


class TestLastRecordReasoning(ReasoningRules, ReaderCase):

    def read_usage(self):
        return self.last_usage()

    def test_reasoning_comes_from_the_last_record_only(self):
        self.write(_record(thinking=7, message_id="a"),
                   _record(message_id="b"))
        self.assertNotIn("reasoning_tokens", self.last_usage())


class TestRecordEffort(ReaderCase):

    def effort_of(self, record):
        self.write(record)
        return self.per_record()[-1].effort

    def test_effort_is_copied_verbatim(self):
        self.assertEqual("medium", self.effort_of(_record(effort="medium")))

    def test_unknown_effort_value_is_not_normalised(self):
        self.assertEqual("XHigh", self.effort_of(_record(effort="XHigh")))

    def test_missing_effort_is_none(self):
        self.assertIsNone(self.effort_of(_record()))

    def test_empty_effort_is_none(self):
        self.assertIsNone(self.effort_of(_record(effort="")))

    def test_non_string_effort_is_none(self):
        self.assertIsNone(self.effort_of(_record(effort=3)))

    def test_effort_inside_message_is_ignored(self):
        record = _record()
        record["message"]["effort"] = "high"
        self.assertIsNone(self.effort_of(record))

    def test_effort_is_not_taken_from_environment(self):
        old = os.environ.get("CLAUDE_EFFORT")
        os.environ["CLAUDE_EFFORT"] = "max"
        try:
            self.assertIsNone(self.effort_of(_record()))
        finally:
            if old is None:
                os.environ.pop("CLAUDE_EFFORT", None)
            else:
                os.environ["CLAUDE_EFFORT"] = old

    def test_effort_is_per_record(self):
        self.write(_record(effort="low", message_id="a"),
                   _record(message_id="b"),
                   _record(effort="high", message_id="c"))
        self.assertEqual(["low", None, "high"],
                         [r.effort for r in self.per_record()])


class TestFixtureTranscripts(unittest.TestCase):

    def records(self, name):
        return transcript.read_assistant_records(str(FIXTURES / name))

    def test_orchestrator_fixture_records_are_medium_with_reasoning(self):
        recs = self.records("effort_orchestrator_transcript.jsonl")
        self.assertEqual(["medium", "medium"], [r.effort for r in recs])
        self.assertEqual([30, 0],
                         [r.token_usage["reasoning_tokens"] for r in recs])
        self.assertEqual(50, recs[0].token_usage["output_tokens"])

    def test_subagent_fixture_has_high_record_and_bare_record(self):
        recs = self.records("effort_subagent_transcript.jsonl")
        self.assertEqual(["high", None], [r.effort for r in recs])
        self.assertEqual(55, recs[0].token_usage["reasoning_tokens"])
        self.assertNotIn("reasoning_tokens", recs[1].token_usage)


if __name__ == "__main__":
    unittest.main()
