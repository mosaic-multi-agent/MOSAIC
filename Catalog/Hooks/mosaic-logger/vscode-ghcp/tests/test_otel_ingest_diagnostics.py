"""OTel ingest diagnostics and hook safety in the vscode-ghcp adapter.

Every diagnostic of the ingest reaches OrchestrationLogs/mosaic-logger-diagnostics.jsonl
with MOSAIC_LOGGER_DEBUG unset. An unset variable and a missing file each give exactly
one diagnostic per run; a file that appears later is ingested at the next stop. An ingest
that raises, or runs out of budget, never changes the hook's exit code, stdout or the
events of the firing.
"""

import contextlib
import io
import json
import os
import subprocess
import sys
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support
import session_routing_flow as rf
import vscode_flow as flow

_ADAPTER = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                        "mosaic_logger.py")


def _kinds(lines):
    return [d.get("kind") for d in lines]


class _OtelCase(unittest.TestCase):
    otel_set = True

    def setUp(self):
        self.ws = support.OtelWorkspace()
        self.addCleanup(self.ws.cleanup)
        patcher = support.clean_environment(self.ws.otel_path if self.otel_set else None)
        self.addCleanup(patcher.stop)


class TestVariableUnset(_OtelCase):
    otel_set = False

    def test_one_variable_unset_diagnostic_per_run_however_many_stops(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        self.ws.stop()
        self.ws.stop()
        found = self.ws.diag_lines("variable_unset")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual("vscode-ghcp", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertTrue(found[0].get("message"))

    def test_no_usage_is_logged_and_no_other_diagnostic_is_written(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        self.assertEqual([], self.ws.orchestrator_record_ids())
        self.assertEqual(["variable_unset"], _kinds(self.ws.diag_lines()))

    def test_a_second_run_gets_its_own_diagnostic(self):
        self.ws.start_session(support.SESSION, support.RUN)
        self.ws.stop(support.SESSION)
        self.ws.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.ws.stop(support.OTHER_SESSION)
        runs = sorted(d.get("run_id") for d in self.ws.diag_lines("variable_unset"))
        self.assertEqual([support.RUN, support.OTHER_RUN], runs)

    def test_a_subagent_stop_reports_it_once_with_the_stop(self):
        self.ws.session_start()
        self.ws.prompt(support.SESSION, flow.RUN_ID)
        self.ws.send("pre_dispatch", session_id=support.SESSION)
        self.ws.send("start", session_id=support.SESSION)
        self.ws.send("stop", session_id=support.SESSION)
        self.ws.send("orchestrator_stop", session_id=support.SESSION)
        self.assertEqual(1, len(self.ws.diag_lines("variable_unset")))


class TestVariableEmpty(_OtelCase):
    otel_set = False

    def test_an_empty_variable_is_treated_as_unset(self):
        os.environ[support.ENV_VAR] = ""
        self.ws.start_session()
        self.ws.stop()
        self.ws.stop()
        self.assertEqual(1, len(self.ws.diag_lines("variable_unset")), self.ws.diag_lines())


class TestFileMissing(_OtelCase):

    def test_one_file_missing_diagnostic_per_run_and_no_unset_diagnostic(self):
        self.ws.start_session()
        self.ws.stop()
        self.ws.stop()
        found = self.ws.diag_lines("file_missing")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual("vscode-ghcp", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertEqual([], self.ws.diag_lines("variable_unset"))
        self.assertEqual([], self.ws.orchestrator_record_ids())

    def test_a_missing_file_does_not_create_ingest_state(self):
        self.ws.start_session()
        self.ws.stop()
        self.assertIsNone(self.ws.state())

    def test_a_file_that_appears_later_is_ingested_at_the_next_stop(self):
        self.ws.start_session()
        self.ws.stop()
        self.ws.add_spans(support.usage_span("late-1"), support.usage_span("late-2"))
        self.ws.stop()
        self.assertEqual(["late-1", "late-2"], self.ws.orchestrator_record_ids())
        self.assertEqual(1, len(self.ws.diag_lines("file_missing")))

    def test_a_second_run_gets_its_own_diagnostic(self):
        self.ws.start_session(support.SESSION, support.RUN)
        self.ws.stop(support.SESSION)
        self.ws.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.ws.stop(support.OTHER_SESSION)
        runs = sorted(d.get("run_id") for d in self.ws.diag_lines("file_missing"))
        self.assertEqual([support.RUN, support.OTHER_RUN], runs)


class TestIngestDiagnosticsAreDefaultVisible(_OtelCase):

    def test_skipped_lines_are_counted_and_the_valid_usage_still_ingested(self):
        self.ws.start_session()
        self.ws.add_spans(support.corrupt_line(), support.usage_span("ok-1"),
                          support.corrupt_line(), support.usage_span("ok-2"))
        self.ws.stop()
        self.assertEqual(["ok-1", "ok-2"], self.ws.orchestrator_record_ids())
        found = self.ws.diag_lines("lines_skipped")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual(2, found[0]["counts"]["malformed"])
        self.assertEqual("vscode-ghcp", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertEqual(support.SESSION, found[0].get("session_id"))

    def test_a_clean_file_leaves_no_diagnostic(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("ok-1"))
        self.ws.stop()
        self.assertEqual([], self.ws.diag_lines())

    def test_a_cli_shaped_line_is_ignored_silently_while_vscode_usage_is_ingested(self):
        self.ws.start_session()
        self.ws.add_spans(support.cli_usage_span("x1", support.SESSION),
                          support.usage_span("ok-1"))
        self.ws.stop()
        self.assertEqual(["ok-1"], self.ws.orchestrator_record_ids())
        self.assertEqual([], self.ws.diag_lines(), f"diagnostics: {self.ws.diag_lines()}")

    def test_an_unknown_state_version_is_reported_and_nothing_is_ingested(self):
        self.ws.start_session()
        state_dir = self.ws.run_dir() / ".otel-ingest"
        state_dir.mkdir(parents=True)
        (state_dir / "state.json").write_text(json.dumps({"state_version": 99}), "utf-8")
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        found = self.ws.diag_lines("unknown_state_version")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual(["99"], found[0].get("ids"))
        self.assertEqual([], self.ws.orchestrator_record_ids())

    def test_a_swallowed_internal_failure_is_reported(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise):
            self.ws.stop()
        self.assertEqual(1, len(self.ws.diag_lines("internal_error")),
                         f"diagnostics: {self.ws.diag_lines()}")

    def test_a_budget_overrun_is_reported(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_budget_exhausted):
            self.ws.stop()
        found = self.ws.diag_lines("budget_exhausted")
        self.assertEqual(1, len(found), f"diagnostics: {self.ws.diag_lines()}")
        self.assertEqual(4096, found[0]["counts"]["remaining_bytes"])

    def test_diagnostics_do_not_depend_on_the_debug_switch(self):
        self.assertNotIn("MOSAIC_LOGGER_DEBUG", os.environ)
        self.ws.start_session()
        self.ws.add_spans(support.corrupt_line())
        self.ws.stop()
        self.assertEqual(["lines_skipped"], _kinds(self.ws.diag_lines()))


def _raise(original, *args, **kwargs):
    raise RuntimeError("simulated ingest failure")


def _budget_exhausted(original, *args, **kwargs):
    diagnostic = types.SimpleNamespace(
        kind="budget_exhausted", message="time budget used up",
        counts={"remaining_bytes": 4096}, ids=[])
    return types.SimpleNamespace(
        status="ingested", records_written=0, finalized=0, pending=0,
        lines_read=0, lines_malformed=0, diagnostics=[diagnostic])


class TestHookSafetyOnIngestTrouble(_OtelCase):

    def _stop_capturing_stdout(self, session_id=support.SESSION):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.ws.stop(session_id)
        return buf.getvalue()

    def _events_of_a_stop(self, replacement):
        """Event names of the run after Stop, with ingest patched by `replacement`."""
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched(replacement) as calls:
            out = self._stop_capturing_stdout()
        self.assertEqual(1, len(calls), "the stop must have attempted an ingest")
        self.assertEqual("", out)
        return self.ws.event_names()

    def test_a_raising_ingest_leaves_the_events_of_the_stop_unchanged(self):
        baseline = support.OtelWorkspace()
        self.addCleanup(baseline.cleanup)
        os.environ.pop(support.ENV_VAR)
        baseline.start_session()
        baseline.stop()
        expected = baseline.event_names()
        os.environ[support.ENV_VAR] = str(self.ws.otel_path)
        self.assertIn("turn", expected)
        self.assertEqual(expected, self._events_of_a_stop(_raise))

    def test_a_raising_ingest_does_not_propagate_out_of_dispatch(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise) as calls:
            self.ws.stop()
            self.ws.stop()
        self.assertEqual(2, len(calls))
        self.assertIn("turn", self.ws.event_names())

    def test_a_budget_overrun_leaves_the_events_and_stdout_unchanged(self):
        events = self._events_of_a_stop(_budget_exhausted)
        self.assertIn("turn", events)

    def test_a_failing_ingest_in_a_hook_process_exits_0_with_empty_stdout(self):
        self.ws.start_session()
        # An OTel path that is a directory cannot be read as a file.
        self.ws.otel_path.mkdir(parents=True)
        env = dict(os.environ)
        env["PYTHONPATH"] = os.pathsep.join(
            [str(support.SHARED_DIR), env.get("PYTHONPATH", "")])
        result = subprocess.run(
            [sys.executable, _ADAPTER], capture_output=True, timeout=60, env=env,
            input=json.dumps(rf.stop_payload(self.ws.path, support.SESSION)).encode())
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)
        self.assertEqual(b"", result.stderr)
        # The real ingest never raises; it reports the unreadable file, proving it ran.
        self.assertEqual(1, len(self.ws.diag_lines("internal_error")), self.ws.diag_lines())

    def test_ingest_trouble_on_subagent_stop_keeps_the_subagent_completion(self):
        ws = self.ws
        ws.session_start()
        ws.prompt(support.SESSION, flow.RUN_ID)
        ws.send("pre_dispatch", session_id=support.SESSION)
        ws.send("start", session_id=support.SESSION)
        ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise):
            ws.send("stop", session_id=support.SESSION)
            ws.send("post_dispatch", session_id=support.SESSION)
        self.assertEqual(1, len(ws.invocation_ends()))
        self.assertEqual(1, len(ws.diag_lines("internal_error")))


if __name__ == "__main__":
    unittest.main()
