"""OTel ingest state that must survive between hook processes in the vscode-ghcp adapter.

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


class _ProcessCase(unittest.TestCase):
    otel_set = True

    def setUp(self):
        self.ws = support.OtelWorkspace()
        self.addCleanup(self.ws.cleanup)
        patcher = support.clean_environment(self.ws.otel_path if self.otel_set else None)
        self.addCleanup(patcher.stop)


class TestStartingOffsetAcrossProcesses(_ProcessCase):

    def test_usage_from_before_the_session_start_is_not_ingested_by_a_later_process(self):
        self.ws.add_spans(support.usage_span("before"))
        self.ws.process_session_start()
        self.ws.process_prompt()
        self.ws.add_spans(support.usage_span("after"))
        self.ws.process_stop()
        self.assertEqual(["after"], self.ws.orchestrator_record_ids())


class TestVariableUnsetAcrossProcesses(_ProcessCase):
    otel_set = False

    def test_one_diagnostic_per_run_over_stops_in_separate_processes(self):
        self.ws.process_session_start()
        self.ws.process_prompt()
        for _ in range(3):
            self.ws.process_stop()
        found = self.ws.diag_lines("variable_unset")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual(support.RUN, found[0].get("run_id"))


class TestFileMissingAcrossProcesses(_ProcessCase):

    def test_one_diagnostic_per_run_over_stops_in_separate_processes(self):
        self.ws.process_session_start()
        self.ws.process_prompt()
        for _ in range(3):
            self.ws.process_stop()
        found = self.ws.diag_lines("file_missing")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual(support.RUN, found[0].get("run_id"))

    def test_a_file_that_appears_later_is_ingested_by_a_later_process(self):
        self.ws.process_session_start()
        self.ws.process_prompt()
        self.ws.process_stop()
        self.ws.add_spans(support.usage_span("late-1"))
        self.ws.process_stop()
        self.assertEqual(["late-1"], self.ws.orchestrator_record_ids())
        self.assertEqual(1, len(self.ws.diag_lines("file_missing")))


if __name__ == "__main__":
    unittest.main()
