"""OTel ingest state that must survive between hook processes in the ghcp-cli adapter.

The harness starts one hook process per event, so anything held only in memory is lost
between a session start and a stop, and between two stops. Every event of these tests
therefore runs in its own process: the starting offset recorded at session start still
limits the first ingest, and the unset / missing diagnostic is still reported once per run.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support


class TestStartingOffsetAcrossProcesses(support.OtelHookCase):

    def test_usage_from_before_the_session_start_is_not_ingested_by_a_later_process(self):
        self.add_spans(support.usage_span("before"))
        self.process_session_start()
        self.process_prompt()
        self.add_spans(support.usage_span("after"))
        self.process_agent_stop()
        self.assertEqual(["after"], self.orchestrator_record_ids())


class TestVariableUnsetAcrossProcesses(support.OtelHookCase):
    otel_set = False

    def test_one_diagnostic_per_run_over_stops_in_separate_processes(self):
        self.process_session_start()
        self.process_prompt()
        for _ in range(3):
            self.process_agent_stop()
        found = self.diag_lines("variable_unset")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(support.RUN, found[0].get("run_id"))


class TestFileMissingAcrossProcesses(support.OtelHookCase):

    def test_one_diagnostic_per_run_over_stops_in_separate_processes(self):
        self.process_session_start()
        self.process_prompt()
        for _ in range(3):
            self.process_agent_stop()
        found = self.diag_lines("file_missing")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(support.RUN, found[0].get("run_id"))

    def test_a_file_that_appears_later_is_ingested_by_a_later_process(self):
        self.process_session_start()
        self.process_prompt()
        self.process_agent_stop()
        self.add_spans(support.usage_span("late-1"))
        self.process_agent_stop()
        self.assertEqual(["late-1"], self.orchestrator_record_ids())
        self.assertEqual(1, len(self.diag_lines("file_missing")))


class TestHookProcessIngest(support.OtelHookCase):

    def test_agent_stop_in_a_hook_process_ingests_with_empty_stdout(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.process_agent_stop()
        self.assertEqual(["s1"], self.orchestrator_record_ids())


if __name__ == "__main__":
    unittest.main()
