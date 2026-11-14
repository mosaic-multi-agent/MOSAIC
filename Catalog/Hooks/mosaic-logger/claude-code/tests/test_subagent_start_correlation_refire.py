"""A SubagentStart/SubagentStop pair re-fired for an agent that already ran.

Claude Code fires the lifecycle again for a finished agent (for example when the
user kills the agent's orphaned process). The logger continues the existing
invocation: it keeps the mapping, consumes no pending dispatch, writes into the
same folder and stream, and never logs the agent's usage a second time. A new
agent dispatched meanwhile keeps its own instance id.

Driven through the hook entry point (mosaic_logger.dispatch) with harness-shaped
payloads.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (
    RUN_ID, CorrelationCase, assistant_record,
    pre_dispatch_payload, start_payload, stop_payload, write_agent_transcript,
)

_OLD_AGENT = "agt-old-0001"
_NEW_AGENT = "agt-new-0002"
_OLD = "Research#1"
_NEW = "TestWriter#2"


class _RefireCase(CorrelationCase):

    def first_cycle(self, records=None):
        """Dispatch, start and stop the old agent once; returns its transcript path."""
        transcript = write_agent_transcript(
            self.session_path, _OLD_AGENT, records or [assistant_record("m1")])
        self.replay(
            pre_dispatch_payload(_OLD, "Research", "tu-old"),
            start_payload(_OLD_AGENT, "Research"),
            stop_payload(_OLD_AGENT, "Research", self.session_path, transcript, "first answer"),
        )
        return transcript

    def refire(self, transcript):
        self.replay(
            start_payload(_OLD_AGENT, "Research"),
            stop_payload(_OLD_AGENT, "Research", self.session_path, transcript, "again"),
        )


class TestRefiredStartOfFinishedAgent(_RefireCase):

    def test_mapping_is_not_overwritten(self):
        transcript = self.first_cycle()
        self.replay(pre_dispatch_payload(_NEW, "TestWriter", "tu-new"))
        before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire(transcript)

        self.assertEqual(before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))

    def test_pending_dispatch_of_another_agent_is_not_consumed(self):
        transcript = self.first_cycle()
        self.replay(pre_dispatch_payload(_NEW, "TestWriter", "tu-new"))

        self.refire(transcript)

        self.assertEqual([_NEW], self.queued_ids())

    def test_pending_dispatch_of_the_same_type_is_not_consumed(self):
        transcript = self.first_cycle()
        self.replay(pre_dispatch_payload("Research#7", "Research", "tu-same"))

        self.refire(transcript)

        self.assertEqual(["Research#7"], self.queued_ids())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))

    def test_new_agent_still_gets_its_own_id_after_the_refire(self):
        transcript = self.first_cycle()
        self.replay(pre_dispatch_payload(_NEW, "TestWriter", "tu-new"))
        self.refire(transcript)

        self.replay(start_payload(_NEW_AGENT, "TestWriter"))

        self.assertEqual(_NEW, self.mapped_instance(_NEW_AGENT))
        self.assertEqual([], self.queued_ids())
        self.assertEqual(sorted([_OLD, _NEW]), self.folders())
        self.assert_no_unknown_agent_folder()

    def test_old_agent_usage_appears_exactly_once(self):
        transcript = self.first_cycle()
        self.replay(pre_dispatch_payload(_NEW, "TestWriter", "tu-new"))

        self.refire(transcript)
        self.replay(start_payload(_NEW_AGENT, "TestWriter"))

        self.assertEqual(["m1"], self.usage_ids(_OLD))
        self.assertNotIn("m1", self.usage_ids(_NEW))

    def test_refired_stop_logs_only_usage_that_is_new(self):
        self.first_cycle()
        grown = write_agent_transcript(
            self.session_path, _OLD_AGENT,
            [assistant_record("m1"), assistant_record("m2")])

        self.refire(grown)

        self.assertEqual(["m1", "m2"], self.usage_ids(_OLD))

    def test_refire_writes_into_the_same_folder_without_a_second_start(self):
        transcript = self.first_cycle()
        input_path = self.paths.invocation_input(RUN_ID, _OLD)
        input_before = input_path.read_bytes()

        self.refire(transcript)

        names = self.names(_OLD)
        self.assertEqual(1, names.count("invocation_start"))
        user_turns = [e for e in self.events(_OLD)
                      if e["event"] == "turn" and e.get("role") == "user"]
        self.assertEqual(1, len(user_turns))
        self.assertEqual(input_before, input_path.read_bytes())
        self.assertEqual([_OLD], self.folders())

    def test_refire_writes_no_second_invocation_end(self):
        transcript = self.first_cycle()

        self.refire(transcript)

        # The re-fired start continues the old invocation (not a remapped agent).
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(_OLD, self.mapped_instance(_OLD_AGENT))
        self.assert_no_unknown_agent_folder()
        ends = [e for e in self.events(_OLD) if e["event"] == "invocation_end"]
        self.assertEqual(1, len(ends))
        self.assertEqual("first answer", ends[0]["response"])
        self.assertIn("first answer", self.paths.invocation_output(RUN_ID, _OLD).read_text("utf-8"))
        self.assertNotIn("again", self.paths.invocation_output(RUN_ID, _OLD).read_text("utf-8"))

    def test_second_refire_changes_nothing_further(self):
        transcript = self.first_cycle()
        self.refire(transcript)
        events_before = self.events(_OLD)

        self.refire(transcript)

        self.assertEqual(events_before, self.events(_OLD))
        self.assertEqual([_OLD], self.folders())


class TestRefireWhenRunWasAdoptedFromTheDispatch(_RefireCase):
    """The first start learns its run from the queued dispatch; the re-fire must
    still find the mapping written under that run."""

    bind_run = False

    def test_refire_finds_the_mapping_under_the_adopted_run(self):
        transcript = write_agent_transcript(
            self.session_path, _OLD_AGENT, [assistant_record("m1")])
        self.replay(
            pre_dispatch_payload(_OLD, "Research", "tu-old"),
            start_payload(_OLD_AGENT, "Research"),
            stop_payload(_OLD_AGENT, "Research", self.session_path, transcript, "first answer"),
            pre_dispatch_payload(_NEW, "TestWriter", "tu-new"),
        )
        before = self.mapping_path(_OLD_AGENT).read_bytes()

        self.refire(transcript)

        self.assertEqual(before, self.mapping_path(_OLD_AGENT).read_bytes())
        self.assertEqual([_NEW], self.queued_ids())
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(["m1"], self.usage_ids(_OLD))
        self.assertEqual([], self.folders("unknown-run"))
