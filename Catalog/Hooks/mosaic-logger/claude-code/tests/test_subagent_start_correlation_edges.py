"""Edge cases of SubagentStart identity resolution.

- A re-fired start of a finished agent whose payload has no or an empty
  agent_type still continues the old invocation (the mapping check comes first).
- An explicit agent_instance_id line in the task message loses to a pending
  dispatch of the same type.
- Dispatch capture tolerates an empty or non-string subagent_type.
- The adapter source no longer contains the 'unknown-agent' literal.

Driven through the hook entry point (mosaic_logger.dispatch).
"""

import os
import pathlib
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (
    CorrelationCase, assistant_record, dispatch_prompt,
    pre_dispatch_payload, start_payload, stop_payload, write_agent_transcript,
)

_AGENT = "agt-old-0001"
_OLD = "Research#1"


class TestRefireWithoutAgentType(CorrelationCase):

    def _first_cycle(self):
        transcript = write_agent_transcript(self.session_path, _AGENT, [assistant_record("m1")])
        self.replay(
            pre_dispatch_payload(_OLD, "Research", "tu-old"),
            start_payload(_AGENT, "Research"),
            stop_payload(_AGENT, "Research", self.session_path, transcript, "first answer"),
        )

    def _assert_old_invocation_continued(self):
        self.assertEqual(_OLD, self.mapped_instance(_AGENT))
        self.assertEqual([_OLD], self.folders())
        self.assertEqual(1, self.names(_OLD).count("invocation_start"))
        self.assert_no_unknown_agent_folder()

    def test_refire_with_absent_agent_type_continues_the_old_invocation(self):
        self._first_cycle()
        self.replay(pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-new"))

        self.replay(start_payload(_AGENT, None))

        self._assert_old_invocation_continued()
        self.assertEqual(["TestWriter#2"], self.queued_ids())

    def test_refire_with_empty_agent_type_continues_the_old_invocation(self):
        self._first_cycle()
        self.replay(pre_dispatch_payload("TestWriter#2", "TestWriter", "tu-new"))

        self.replay(start_payload(_AGENT, ""))

        self._assert_old_invocation_continued()
        self.assertEqual(["TestWriter#2"], self.queued_ids())


class TestExplicitLineAgainstPendingDispatch(CorrelationCase):

    def test_pending_dispatch_of_the_same_type_wins_over_an_explicit_line(self):
        self.replay(
            pre_dispatch_payload("Research#1", "Research", "tu-1"),
            start_payload("agt-a", "Research", prompt=dispatch_prompt("Research#9")),
        )

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual([], self.queued_ids())
        self.assertNotIn("Research#9", self.folders())


class TestDispatchCaptureWithOddSubagentType(CorrelationCase):

    def _dispatch_with_type(self, instance, subagent_type, use_id):
        payload = pre_dispatch_payload(instance, None, use_id)
        payload["tool_input"]["subagent_type"] = subagent_type
        self.replay(payload)

    def test_empty_subagent_type_is_queued_and_matched_by_id_prefix(self):
        self._dispatch_with_type("Planner#2", "", "tu-0")
        self._dispatch_with_type("Research#1", "", "tu-1")

        self.replay(start_payload("agt-a", "Research"))

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual(["Planner#2"], self.queued_ids())

    def test_non_string_subagent_type_is_queued_and_matched_by_id_prefix(self):
        self._dispatch_with_type("Planner#2", 7, "tu-0")
        self._dispatch_with_type("Research#1", ["Research"], "tu-1")

        self.replay(start_payload("agt-a", "Research"))

        self.assertEqual("Research#1", self.mapped_instance("agt-a"))
        self.assertEqual(["Planner#2"], self.queued_ids())


class TestAdapterSourceHasNoUnknownAgentLiteral(CorrelationCase):

    def test_no_adapter_module_contains_the_literal(self):
        adapter_dir = pathlib.Path(__file__).resolve().parent.parent
        offenders = [p.name for p in adapter_dir.glob("mosaic_logger*.py")
                     if "unknown-agent" in p.read_text("utf-8")]

        self.assertEqual([], offenders)
