"""Usage-call reading from a GHCP CLI session transcript.

Covers: one call per session.usage_record, record key, token mapping (uncached input is
inputTokens minus cache), model/provider/effort/reasoning, call start from timestamp minus
duration, agentId presence/absence, requested tools from toolCounts, record index.
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as support  # noqa: E402
from transcript_usage_support import compaction_event, read_events, turn_end_event, usage_event  # noqa: E402


class FixtureCallsTest(unittest.TestCase):
    """Verbatim-excerpt fixture: two orchestrator calls and one native subagent call."""

    def setUp(self):
        import mosaic_logger_transcript_usage as reader
        self.result = reader.read_transcript_usage(support.USAGE_FIXTURE)
        self.calls = {c.record_id: c for c in self.result.calls}

    def test_one_call_per_usage_record_in_file_order(self):
        ids = [c.record_id for c in self.result.calls]
        self.assertEqual(ids, ["msg_011CgYVG8wdmXTDQVoia95oJ", "msg_011CgYVH9vhJ2XcMRiGduZqp",
                               "msg_011CgYVQ1GVRRTYxjNq2FJ69"])
        self.assertTrue(self.result.readable)

    def test_token_mapping_subtracts_cache_from_input(self):
        call = self.calls["msg_011CgYVG8wdmXTDQVoia95oJ"]
        self.assertEqual(call.token_usage, {
            "input_tokens": 4, "cache_read_tokens": 12492, "cache_creation_tokens": 45656,
            "output_tokens": 356, "reasoning_tokens": 167})

    def test_model_provider_effort(self):
        call = self.calls["msg_011CgYVG8wdmXTDQVoia95oJ"]
        self.assertEqual((call.model, call.provider, call.effort),
                         ("claude-sonnet-5.5", "github-copilot", "medium"))

    def test_call_start_is_event_timestamp_minus_duration(self):
        call = self.calls["msg_011CgYVG8wdmXTDQVoia95oJ"]
        self.assertEqual(call.timestamp, "2026-10-30T13:18:54.520Z")

    def test_orchestrator_call_has_no_agent_id(self):
        self.assertIsNone(self.calls["msg_011CgYVG8wdmXTDQVoia95oJ"].agent_id)

    def test_subagent_call_carries_top_level_agent_id(self):
        call = self.calls["msg_011CgYVH9vhJ2XcMRiGduZqp"]
        self.assertEqual(call.agent_id, "55d56cda-80eb-4a41-9068-b0d0a58d93cc")
        self.assertEqual(call.model, "claude-haiku-5.5")
        self.assertEqual(call.timestamp, "2026-10-30T13:19:08.492Z")
        self.assertEqual(call.token_usage["input_tokens"], 4)

    def test_requested_tools_from_tool_counts(self):
        self.assertEqual(self.calls["msg_011CgYVG8wdmXTDQVoia95oJ"].requested_tools, {"glob": 1, "powershell": 1})

    def test_empty_tool_counts_means_no_tools_requested(self):
        call = self.calls["msg_011CgYVQ1GVRRTYxjNq2FJ69"]
        self.assertEqual(call.requested_tools, {})
        self.assertEqual(call.token_usage["reasoning_tokens"], 0)

    def test_record_index_is_ordinal_among_usage_and_compaction_events(self):
        self.assertEqual([c.record_index for c in self.result.calls], [0, 1, 2])

    def test_clean_transcript_has_no_alarm_and_no_problems(self):
        self.assertFalse(self.result.alarm)
        self.assertEqual(self.result.problems, [])


class RecordIndexTest(unittest.TestCase):
    def test_compaction_events_count_toward_the_ordinal(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = read_events(tmp, [usage_event("a", event_id="e1"), compaction_event("c", event_id="e2"),
                                       turn_end_event(), usage_event("b", event_id="e3")])
        self.assertEqual([(c.record_id, c.record_index) for c in result.calls], [("a", 0), ("b", 2)])
        self.assertEqual(result.compactions[0].record_index, 1)


class TokenMappingTest(unittest.TestCase):
    def _call(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [usage_event(**overrides)])

    def test_uncached_input_is_input_minus_cache_read_and_write(self):
        result = self._call(inputTokens=1000, cacheReadTokens=300, cacheWriteTokens=200, outputTokens=50)
        self.assertEqual(result.calls[0].token_usage, {
            "input_tokens": 500, "cache_read_tokens": 300, "cache_creation_tokens": 200,
            "output_tokens": 50, "reasoning_tokens": 20})

    def test_fully_cached_input_gives_zero_uncached_input(self):
        result = self._call(inputTokens=500, cacheReadTokens=300, cacheWriteTokens=200)
        self.assertEqual(result.calls[0].token_usage["input_tokens"], 0)

    def test_input_smaller_than_cache_leaves_input_out_and_reports_it(self):
        result = self._call(inputTokens=100, cacheReadTokens=300, cacheWriteTokens=200)
        usage = result.calls[0].token_usage
        self.assertNotIn("input_tokens", usage)
        self.assertEqual((usage["cache_read_tokens"], usage["cache_creation_tokens"], usage["output_tokens"]),
                         (300, 200, 50))
        self.assertEqual(len(result.problems), 1)
        self.assertEqual(result.problems[0].record_id, "call-1")

    def test_reasoning_above_output_is_left_out_and_reported(self):
        result = self._call(outputTokens=10, reasoningTokens=25)
        usage = result.calls[0].token_usage
        self.assertNotIn("reasoning_tokens", usage)
        self.assertEqual(usage["output_tokens"], 10)
        self.assertEqual(support.problem_keys(result), [("call-1", "reasoningTokens")])

    def test_reasoning_equal_to_output_is_kept(self):
        result = self._call(outputTokens=20, reasoningTokens=20)
        self.assertEqual(result.calls[0].token_usage["reasoning_tokens"], 20)

    def test_no_resolvable_token_figure_gives_no_token_usage(self):
        result = self._call(drop=("inputTokens", "outputTokens", "cacheReadTokens", "cacheWriteTokens",
                                  "reasoningTokens"))
        self.assertEqual(len(result.calls), 1)
        self.assertIsNone(result.calls[0].token_usage)


class CallIdentityTest(unittest.TestCase):
    def _call(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [usage_event(**overrides)]).calls[0]

    def test_provider_recognised_by_kind_alone(self):
        self.assertEqual(self._call(provider={"id": "other", "kind": "copilot"}).provider, "github-copilot")

    def test_provider_recognised_by_id_alone(self):
        self.assertEqual(self._call(provider={"id": "copilot"}).provider, "github-copilot")

    def test_other_provider_is_omitted(self):
        self.assertIsNone(self._call(provider={"id": "azure", "kind": "byok"}).provider)

    def test_missing_provider_is_omitted(self):
        self.assertIsNone(self._call(drop=("provider",)).provider)

    def test_missing_effort_and_model_are_none(self):
        call = self._call(drop=("reasoningEffort", "model"))
        self.assertIsNone(call.effort)
        self.assertIsNone(call.model)

    def test_effort_is_kept_verbatim(self):
        self.assertEqual(self._call(reasoningEffort="xhigh").effort, "xhigh")


class CallStartTest(unittest.TestCase):
    def _start(self, ts, duration):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [usage_event(ts=ts, duration=duration)]).calls[0].timestamp

    def test_start_crosses_a_second_boundary(self):
        self.assertEqual(self._start("2026-10-30T13:00:00.200Z", 1500), "2026-10-30T12:59:58.700Z")

    def test_start_crosses_midnight(self):
        self.assertEqual(self._start("2026-11-01T00:00:00.100Z", 200), "2026-10-31T23:59:59.900Z")

    def test_zero_duration_start_equals_event_timestamp(self):
        self.assertEqual(self._start("2026-10-30T13:00:10.250Z", 0), "2026-10-30T13:00:10.250Z")


class RequestedToolsTest(unittest.TestCase):
    def _tools(self, **overrides):
        with tempfile.TemporaryDirectory() as tmp:
            return read_events(tmp, [usage_event(**overrides)])

    def test_counts_are_kept_per_tool(self):
        self.assertEqual(self._tools(toolCounts={"view": 2, "grep": 1}).calls[0].requested_tools,
                         {"view": 2, "grep": 1})

    def test_zero_count_entries_are_dropped(self):
        self.assertEqual(self._tools(toolCounts={"view": 2, "grep": 0}).calls[0].requested_tools, {"view": 2})

    def test_only_zero_counts_gives_empty_map(self):
        self.assertEqual(self._tools(toolCounts={"grep": 0}).calls[0].requested_tools, {})

    def test_missing_tool_counts_is_absent_and_reported(self):
        result = self._tools(drop=("toolCounts",))
        self.assertIsNone(result.calls[0].requested_tools)
        self.assertEqual(support.problem_keys(result), [("call-1", "toolCounts")])

    def test_negative_count_makes_the_field_absent_and_reported(self):
        result = self._tools(toolCounts={"view": -1})
        self.assertIsNone(result.calls[0].requested_tools)
        self.assertEqual(support.problem_keys(result), [("call-1", "toolCounts")])

    def test_non_integer_count_makes_the_field_absent_and_reported(self):
        result = self._tools(toolCounts={"view": "two"})
        self.assertIsNone(result.calls[0].requested_tools)
        self.assertEqual(support.problem_keys(result), [("call-1", "toolCounts")])


if __name__ == "__main__":
    unittest.main()
