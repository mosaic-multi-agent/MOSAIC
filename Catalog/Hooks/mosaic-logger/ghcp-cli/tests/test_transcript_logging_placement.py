"""Which stream a transcript usage call lands on: hard keys only, native and Runner mode.

Native mode: an event without agentId is the orchestrator's; an event with an agentId goes to the
invocation that the agent mapping (written at subagentStart) names, and is held back until that
mapping exists. Runner mode: the session itself decides (subagent role: the invocation;
orchestrator role: the orchestrator stream). Nothing falls back to a default invocation.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, INSTANCE_ID, RUN_ID, SECOND_RUN, TranscriptLoggingCase, invocation, orchestrator,
    ts, usage_event)

OTHER_INSTANCE = "Implementation#2"


class NativePlacementTest(TranscriptLoggingCase):
    def test_call_without_agent_id_goes_to_the_orchestrator_stream(self):
        self.start_run()
        self.write([usage_event("orch-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record("orch-1"))

    def test_call_with_mapped_agent_id_goes_to_that_invocation_stream(self):
        self.start_run()
        self.spawn("agent-1")
        self.write([usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="e1")])
        self.stop()
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record("sub-1"))

    def test_invocation_call_names_its_invocation(self):
        self.start_run()
        self.spawn("agent-1")
        self.write([usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="e1")])
        self.stop()
        record = self.usage_record("sub-1")
        self.assertEqual(INSTANCE_ID, record["agent_instance_id"])
        self.assertEqual("orchestrator_transcript", record["source"])

    def test_orchestrator_call_is_not_given_an_invocation(self):
        self.start_run()
        self.write([usage_event("orch-1", ts=ts(10), event_id="e1")])
        self.stop()
        record = self.usage_record("orch-1")
        self.assertNotIn("agent_instance_id", record)
        self.assertEqual("orchestrator_transcript", record["source"])

    def test_each_agent_goes_to_its_own_invocation(self):
        self.start_run()
        self.spawn("agent-1", INSTANCE_ID)
        self.spawn("agent-2", OTHER_INSTANCE)
        self.write([
            usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="e1"),
            usage_event("sub-2", ts=ts(11), agent_id="agent-2", event_id="e2"),
            usage_event("orch-1", ts=ts(12), event_id="e3"),
        ])
        self.stop()
        self.assertEqual([invocation(FIRST_RUN, INSTANCE_ID)], self.streams_of_record("sub-1"))
        self.assertEqual([invocation(FIRST_RUN, OTHER_INSTANCE)], self.streams_of_record("sub-2"))
        self.assertEqual([orchestrator(FIRST_RUN)], self.streams_of_record("orch-1"))

    def test_agent_of_the_earlier_run_stays_on_its_own_run_after_the_session_moves_on(self):
        self.start_run(FIRST_RUN)
        self.spawn("agent-1", run_id=FIRST_RUN)
        self.start_run(SECOND_RUN)
        self.write([usage_event("sub-1", ts=ts(10), agent_id="agent-1", event_id="e1"),
                    usage_event("orch-1", ts=ts(11), event_id="e2")])
        self.stop()
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record("sub-1"))
        self.assertEqual([orchestrator(SECOND_RUN)], self.streams_of_record("orch-1"))


class NativeUnmappedAgentTest(TranscriptLoggingCase):
    def test_call_of_unmapped_agent_is_not_logged_anywhere(self):
        self.start_run()
        self.write([usage_event("late-1", ts=ts(10), agent_id="agent-late", event_id="e1")])
        self.stop()
        self.assertEqual([], self.streams_of_record("late-1"))

    def test_unmapped_call_never_creates_a_fallback_invocation_stream(self):
        self.start_run()
        self.write([usage_event("late-1", ts=ts(10), agent_id="agent-late", event_id="e1")])
        self.stop()
        self.stop("sessionEnd", reason="complete")
        names = [p.name for p in self.logs.rglob("*") if p.is_dir()]
        self.assertEqual([], [n for n in names if "unmapped" in n or "unknown-agent" in n])
        self.assertEqual([], self.usage_records())

    def test_held_back_call_does_not_block_other_calls(self):
        self.start_run()
        self.write([usage_event("late-1", ts=ts(10), agent_id="agent-late", event_id="e1"),
                    usage_event("orch-1", ts=ts(11), event_id="e2")])
        self.stop()
        self.assertEqual(["orch-1"], self.record_ids())

    def test_call_is_logged_by_a_later_read_once_the_mapping_exists(self):
        self.start_run()
        self.write([usage_event("late-1", ts=ts(10), agent_id="agent-late", event_id="e1")])
        self.stop()
        self.spawn("agent-late")
        self.stop()
        self.assertEqual([invocation(FIRST_RUN)], self.streams_of_record("late-1"))

    def test_held_back_call_keeps_its_own_timestamp_when_logged_later(self):
        self.start_run()
        self.write([usage_event("late-1", ts=ts(10), agent_id="agent-late", event_id="e1")])
        self.stop()
        self.spawn("agent-late")
        self.stop()
        self.assertEqual(ts(9), self.usage_record("late-1")["timestamp"])


class NativeUnmappedAmongMappedAgentsTest(TranscriptLoggingCase):
    """An unmapped agentId is never attributed to another, mapped (and running) invocation."""

    def stream_record_ids(self, label):
        return [event["record_id"] for stream, event in self.usage_records() if stream == label]

    def setUp(self):
        super().setUp()
        self.start_run()
        self.spawn("agent-1", INSTANCE_ID)
        self.write([usage_event("own-1", ts=ts(10), agent_id="agent-1", event_id="e1"),
                    usage_event("late-1", ts=ts(11), agent_id="agent-late", event_id="e2")])
        self.stop()

    def test_unmapped_call_is_on_no_stream_while_another_agent_is_mapped(self):
        self.assertEqual([], self.streams_of_record("late-1"))

    def test_mapped_invocation_stream_holds_only_its_own_calls(self):
        self.assertEqual(["own-1"], self.stream_record_ids(invocation(FIRST_RUN, INSTANCE_ID)))
        self.assertEqual([], self.stream_record_ids(orchestrator(FIRST_RUN)))

    def test_call_lands_on_the_instance_its_agent_is_later_mapped_to(self):
        self.spawn("agent-late", OTHER_INSTANCE)
        self.stop()
        self.assertEqual([invocation(FIRST_RUN, OTHER_INSTANCE)], self.streams_of_record("late-1"))
        self.assertEqual([invocation(FIRST_RUN, INSTANCE_ID)], self.streams_of_record("own-1"))


class RunnerSubagentPlacementTest(TranscriptLoggingCase):
    def setUp(self):
        super().setUp()
        self.subagent_env()
        self.dispatch("userPromptSubmitted", {"prompt": "Do the research task"})

    def test_call_goes_to_the_invocation_stream_of_the_role(self):
        self.write([usage_event("run-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.assertEqual([invocation(RUN_ID)], self.streams_of_record("run-1"))

    def test_call_names_the_invocation_and_the_agent_transcript_source(self):
        self.write([usage_event("run-1", ts=ts(10), event_id="e1")])
        self.stop()
        record = self.usage_record("run-1")
        self.assertEqual(INSTANCE_ID, record["agent_instance_id"])
        self.assertEqual("agent_transcript", record["source"])
        self.assertEqual(RUN_ID, record["run_id"])

    def test_every_call_of_the_session_belongs_to_the_invocation_whatever_its_agent_id(self):
        self.write([usage_event("run-1", ts=ts(10), event_id="e1"),
                    usage_event("run-2", ts=ts(11), agent_id="inner-agent", event_id="e2")])
        self.stop()
        self.assertEqual([invocation(RUN_ID)], self.streams_of_record("run-1"))
        self.assertEqual([invocation(RUN_ID)], self.streams_of_record("run-2"))

    def test_session_end_after_the_claimed_completion_still_reads_later_calls(self):
        self.write([usage_event("run-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.append([usage_event("run-2", ts=ts(20), event_id="e2")])
        self.stop("sessionEnd", reason="complete")
        self.assertEqual(["run-1", "run-2"], self.record_ids())

    def test_calls_are_logged_once_when_completion_fires_twice(self):
        self.write([usage_event("run-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.stop("sessionEnd", reason="complete")
        self.assertEqual([invocation(RUN_ID)], self.streams_of_record("run-1"))


class RunnerOrchestratorPlacementTest(TranscriptLoggingCase):
    def setUp(self):
        super().setUp()
        self.orchestrator_env()

    def test_call_goes_to_the_orchestrator_stream_of_the_run(self):
        self.write([usage_event("orch-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.assertEqual([orchestrator(RUN_ID)], self.streams_of_record("orch-1"))

    def test_call_with_an_agent_id_also_goes_to_the_orchestrator_stream(self):
        self.write([usage_event("orch-2", ts=ts(10), agent_id="any-agent", event_id="e1")])
        self.stop()
        self.assertEqual([orchestrator(RUN_ID)], self.streams_of_record("orch-2"))

    def test_call_carries_the_orchestrator_transcript_source_and_no_invocation(self):
        self.write([usage_event("orch-1", ts=ts(10), event_id="e1")])
        self.stop()
        record = self.usage_record("orch-1")
        self.assertEqual("orchestrator_transcript", record["source"])
        self.assertNotIn("agent_instance_id", record)

    def test_session_end_reads_calls_left_after_the_last_stop(self):
        self.write([usage_event("orch-1", ts=ts(10), event_id="e1")])
        self.stop()
        self.append([usage_event("orch-2", ts=ts(20), event_id="e2")])
        self.stop("sessionEnd", reason="complete")
        self.assertEqual([orchestrator(RUN_ID)], self.streams_of_record("orch-2"))


if __name__ == "__main__":
    unittest.main()
