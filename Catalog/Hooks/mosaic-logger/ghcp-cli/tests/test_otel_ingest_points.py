"""Where the ghcp-cli adapter ingests OTel usage and where it starts reading.

In native mode, with COPILOT_OTEL_FILE_EXPORTER_PATH set, agentStop and subagentStop run
one non-final ingest of the OTel file into the firing's run. No other event ingests, and
Runner mode (MOSAIC_ROLE set) never ingests from hooks. sessionStart records the file's
size for the session, and the run takes the smallest such size of its sessions as its
first cursor, so usage from before the run's first session never reaches the run.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support

RUNNER_SESSION = "cli-runner-session-0001"
RUNNER_RUN = "20261028T120000Z-cccc"


class TestIngestTriggers(support.OtelHookCase):

    def test_agent_stop_ingests_usage_into_the_sessions_run(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"), support.usage_span("s2"))
        self.agent_stop()
        self.assertEqual(["s1", "s2"], self.orchestrator_record_ids())

    def test_ingested_records_carry_the_cli_harness_session_and_run(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        record = self.orchestrator_usage()[0]
        self.assertEqual("ghcp-cli", record.get("harness"))
        self.assertEqual(support.SESSION, record.get("session_id"))
        self.assertEqual(support.RUN, record.get("run_id"))

    def test_subagent_stop_ingests_usage_into_the_run(self):
        self.start_session()
        self.subagent_start()
        self.add_spans(support.usage_span("sub-1"))
        self.assertEqual([], self.orchestrator_record_ids())
        self.subagent_stop()
        self.assertEqual(["sub-1"], self.orchestrator_record_ids())

    def test_only_agent_stop_and_subagent_stop_ingest(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.subagent_start()
        self.dispatch("preToolUse", {"toolName": "view", "toolArgs": {"path": "a.md"}})
        self.dispatch("postToolUse", {"toolName": "view", "toolArgs": {"path": "a.md"},
                                      "toolResult": {"textResultForLlm": "x"}})
        self.dispatch("preCompact", {})
        self.dispatch("notification", {"message": "hello"})
        self.session_start()
        self.prompt()
        self.dispatch("sessionEnd", {"reason": "complete"})
        self.assertEqual([], self.orchestrator_record_ids())
        self.assertFalse(self.any_ingest_state())
        self.agent_stop()
        self.assertEqual(["s1"], self.orchestrator_record_ids())

    def test_each_stop_ingests_only_what_is_new(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        self.add_spans(support.usage_span("s2"))
        self.agent_stop()
        self.agent_stop()
        self.assertEqual(["s1", "s2"], self.orchestrator_record_ids())

    def test_stop_without_a_known_run_ingests_nothing(self):
        self.session_start()
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        self.assertEqual([], self.orchestrator_record_ids())
        self.assertFalse(self.any_ingest_state())

    def test_ingest_is_non_final_and_bounded_by_a_budget(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched() as calls:
            self.agent_stop()
        self.assertEqual(1, len(calls))
        kwargs = calls[0][1]
        self.assertFalse(kwargs.get("final", False))
        self.assertFalse(kwargs.get("whole_file", False))
        budget = kwargs.get("budget_seconds")
        self.assertIsInstance(budget, (int, float))
        self.assertTrue(0 < budget <= 5.0, budget)

    def test_pending_spans_stay_pending_at_a_stop(self):
        self.start_session()
        self.add_spans(support.usage_span("orphan", parent="no-such-parent"))
        self.agent_stop()
        self.assertEqual([], self.orchestrator_record_ids())
        self.assertFalse((self.run_dir() / "00_unattributed_events.jsonl").exists())
        self.assertEqual(1, len(self.state()["pending"]))

    def test_a_hook_firing_prints_nothing(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.assertEqual("", self.agent_stop())
        self.assertEqual(["s1"], self.orchestrator_record_ids())


class TestRunnerModeNeverIngests(support.OtelHookCase):
    """Each test first proves the native path ingests, then that Runner mode does not."""

    def _native_control(self):
        self.start_session()
        self.add_spans(support.usage_span("native-1"))
        self.agent_stop()
        self.assertEqual(["native-1"], self.orchestrator_record_ids())

    def _runner_events(self, role):
        if role == "orchestrator":
            self.orchestrator_env(RUNNER_RUN)
        else:
            self.subagent_env(RUNNER_RUN, support.INSTANCE)
        self.dispatch("sessionStart", {}, RUNNER_SESSION)
        self.dispatch("userPromptSubmitted", {"prompt": "work"}, RUNNER_SESSION)
        # Written after the session started, so it lies inside any recorded cursor window.
        self.add_spans(support.usage_span("runner-1", conversation=RUNNER_SESSION))
        self.dispatch("agentStop", {"stopReason": "end_turn"}, RUNNER_SESSION)
        self.dispatch("subagentStop", {"agentId": "x", "response": "done"}, RUNNER_SESSION)
        self.dispatch("sessionEnd", {"reason": "complete"}, RUNNER_SESSION)

    def _assert_runner_run_untouched(self):
        self.assertEqual([], self.orchestrator_record_ids(RUNNER_RUN))
        self.assertEqual([], support.record_ids(
            self.paths.invocation_events(RUNNER_RUN, support.INSTANCE)))
        self.assertFalse((self.run_dir(RUNNER_RUN) / ".otel-ingest").exists())
        self.assertEqual(["native-1"], self.orchestrator_record_ids())

    def test_orchestrator_role_never_ingests(self):
        self._native_control()
        self._runner_events("orchestrator")
        self._assert_runner_run_untouched()

    def test_subagent_role_never_ingests(self):
        self._native_control()
        self._runner_events("subagent")
        self._assert_runner_run_untouched()

    def test_runner_mode_writes_no_ingest_diagnostics(self):
        self._native_control()
        os.environ.pop(support.ENV_VAR)
        self._runner_events("orchestrator")
        self.assertEqual([], self.diag_lines("variable_unset"))
        self.assertEqual([], self.diag_lines("file_missing"))


class TestStartingOffset(support.OtelHookCase):

    def test_usage_written_before_the_first_session_start_is_not_ingested(self):
        self.add_spans(support.usage_span("before"))
        self.start_session()
        self.add_spans(support.usage_span("after"))
        self.agent_stop()
        self.assertEqual(["after"], self.orchestrator_record_ids())

    def test_the_run_starts_at_the_smallest_offset_of_its_sessions(self):
        self.add_spans(support.usage_span("before"))
        self.start_session(support.SESSION)
        self.add_spans(support.usage_span("a1"))
        self.start_session(support.OTHER_SESSION)
        self.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.agent_stop(support.OTHER_SESSION)
        self.assertEqual(["a1", "b1"], self.orchestrator_record_ids())

    def test_a_repeated_session_start_does_not_move_the_recorded_offset(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.session_start()
        self.agent_stop()
        self.assertEqual(["s1"], self.orchestrator_record_ids())

    def test_each_run_takes_the_offset_of_its_own_sessions(self):
        self.start_session(support.SESSION, support.RUN)
        self.add_spans(support.usage_span("a1"))
        self.add_spans(support.usage_span("b0", conversation=support.OTHER_SESSION))
        self.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.agent_stop(support.OTHER_SESSION)
        self.assertEqual(["b1"], self.orchestrator_record_ids(support.OTHER_RUN))

    def test_a_cursor_already_in_the_run_state_wins_over_a_later_session_offset(self):
        self.start_session(support.SESSION)
        self.add_spans(support.usage_span("a1"))
        self.agent_stop(support.SESSION)
        self.add_spans(support.usage_span("a2"))
        self.start_session(support.OTHER_SESSION)
        self.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.agent_stop(support.OTHER_SESSION)
        self.assertEqual(["a1", "a2", "b1"], self.orchestrator_record_ids())

    def test_ingesting_never_modifies_the_otel_file(self):
        self.add_spans(support.usage_span("before"))
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        before = self.otel_path.read_bytes()
        self.agent_stop()
        self.assertEqual(["s1"], self.orchestrator_record_ids())
        self.assertEqual(before, self.otel_path.read_bytes())


if __name__ == "__main__":
    unittest.main()
