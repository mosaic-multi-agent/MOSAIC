"""OTel ingest diagnostics and hook safety in the ghcp-cli adapter.

Every diagnostic of the ingest reaches OrchestrationLogs/mosaic-logger-diagnostics.jsonl
with MOSAIC_LOGGER_DEBUG unset. An unset variable and a missing file each give exactly
one diagnostic per run; a file that appears later is ingested at the next stop. An ingest
that raises, or runs out of budget, never changes the hook's exit code, stdout or the
events of the firing. Runner mode reports nothing.
"""

import json
import os
import subprocess
import sys
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support

_ADAPTER = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                        "mosaic_logger.py")


def _kinds(lines):
    return [d.get("kind") for d in lines]


_OtelCase = support.OtelHookCase


class TestVariableUnset(_OtelCase):
    otel_set = False

    def test_one_variable_unset_diagnostic_per_run_however_many_stops(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        self.agent_stop()
        self.agent_stop()
        found = self.diag_lines("variable_unset")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual("ghcp-cli", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertTrue(found[0].get("message"))

    def test_no_usage_is_logged_and_no_other_diagnostic_is_written(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        self.assertEqual([], self.orchestrator_record_ids())
        self.assertEqual(["variable_unset"], _kinds(self.diag_lines()))

    def test_a_second_run_gets_its_own_diagnostic(self):
        self.start_session(support.SESSION, support.RUN)
        self.agent_stop(support.SESSION)
        self.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.agent_stop(support.OTHER_SESSION)
        runs = sorted(d.get("run_id") for d in self.diag_lines("variable_unset"))
        self.assertEqual([support.RUN, support.OTHER_RUN], runs)

    def test_a_subagent_stop_reports_it_once_with_the_stop(self):
        self.start_session()
        self.subagent_start()
        self.subagent_stop()
        self.agent_stop()
        self.assertEqual(1, len(self.diag_lines("variable_unset")))


class TestVariableEmpty(_OtelCase):
    otel_set = False

    def test_an_empty_variable_is_treated_as_unset(self):
        os.environ[support.ENV_VAR] = ""
        self.start_session()
        self.agent_stop()
        self.agent_stop()
        self.assertEqual(1, len(self.diag_lines("variable_unset")), self.diag_lines())


class TestFileMissing(_OtelCase):

    def test_one_file_missing_diagnostic_per_run_and_no_unset_diagnostic(self):
        self.start_session()
        self.agent_stop()
        self.agent_stop()
        found = self.diag_lines("file_missing")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual("ghcp-cli", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertEqual([], self.diag_lines("variable_unset"))
        self.assertEqual([], self.orchestrator_record_ids())

    def test_a_missing_file_does_not_create_ingest_state(self):
        self.start_session()
        self.agent_stop()
        self.assertIsNone(self.state())

    def test_a_file_that_appears_later_is_ingested_at_the_next_stop(self):
        self.start_session()
        self.agent_stop()
        self.add_spans(support.usage_span("late-1"), support.usage_span("late-2"))
        self.agent_stop()
        self.assertEqual(["late-1", "late-2"], self.orchestrator_record_ids())
        self.assertEqual(1, len(self.diag_lines("file_missing")))

    def test_a_second_run_gets_its_own_diagnostic(self):
        self.start_session(support.SESSION, support.RUN)
        self.agent_stop(support.SESSION)
        self.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.agent_stop(support.OTHER_SESSION)
        runs = sorted(d.get("run_id") for d in self.diag_lines("file_missing"))
        self.assertEqual([support.RUN, support.OTHER_RUN], runs)


class TestIngestDiagnosticsAreDefaultVisible(_OtelCase):

    def test_skipped_lines_are_counted_and_the_valid_usage_still_ingested(self):
        self.start_session()
        self.add_spans(support.corrupt_line(), support.usage_span("ok-1"),
                          support.corrupt_line(), support.usage_span("ok-2"))
        self.agent_stop()
        self.assertEqual(["ok-1", "ok-2"], self.orchestrator_record_ids())
        found = self.diag_lines("lines_skipped")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(2, found[0]["counts"]["malformed"])
        self.assertEqual("ghcp-cli", found[0].get("harness"))
        self.assertEqual(support.RUN, found[0].get("run_id"))
        self.assertEqual(support.SESSION, found[0].get("session_id"))

    def test_a_clean_file_leaves_no_diagnostic(self):
        self.start_session()
        self.add_spans(support.usage_span("ok-1"))
        self.agent_stop()
        self.assertEqual([], self.diag_lines())

    def test_a_conversation_that_cannot_be_tied_to_the_run_is_reported(self):
        self.start_session()
        self.add_spans(support.usage_span("x1", conversation="some-other-session"))
        self.agent_stop()
        found = self.diag_lines("conversation_not_tied")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(1, found[0]["counts"]["usage_spans"])
        self.assertEqual([], self.orchestrator_record_ids())

    def test_an_unknown_state_version_is_reported_and_nothing_is_ingested(self):
        self.start_session()
        state_dir = self.run_dir() / ".otel-ingest"
        state_dir.mkdir(parents=True)
        (state_dir / "state.json").write_text(json.dumps({"state_version": 99}), "utf-8")
        self.add_spans(support.usage_span("s1"))
        self.agent_stop()
        found = self.diag_lines("unknown_state_version")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(["99"], found[0].get("ids"))
        self.assertEqual([], self.orchestrator_record_ids())

    def test_a_swallowed_internal_failure_is_reported(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise):
            self.agent_stop()
        self.assertEqual(1, len(self.diag_lines("internal_error")),
                         f"diagnostics: {self.diag_lines()}")

    def test_a_budget_overrun_is_reported(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_budget_exhausted):
            self.agent_stop()
        found = self.diag_lines("budget_exhausted")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(4096, found[0]["counts"]["remaining_bytes"])

    def test_diagnostics_do_not_depend_on_the_debug_switch(self):
        self.assertNotIn("MOSAIC_LOGGER_DEBUG", os.environ)
        self.start_session()
        self.add_spans(support.corrupt_line())
        self.agent_stop()
        self.assertEqual(["lines_skipped"], _kinds(self.diag_lines()))


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

    def _events_of_a_stop(self, replacement):
        """Event names of the run after Stop, with ingest patched by `replacement`."""
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(replacement) as calls:
            out = self.agent_stop()
        self.assertEqual(1, len(calls), "the stop must have attempted an ingest")
        self.assertEqual("", out)
        return self.event_names()

    def test_a_raising_ingest_leaves_the_events_of_the_stop_unchanged(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        before = len(self.event_names())
        os.environ.pop(support.ENV_VAR)
        self.agent_stop()
        plain = self.event_names()[before:]
        os.environ[support.ENV_VAR] = str(self.otel_path)
        with support.ingest_patched(_raise) as calls:
            out = self.agent_stop()
        failing = self.event_names()[before + len(plain):]
        self.assertEqual(1, len(calls), "the stop must have attempted an ingest")
        self.assertEqual("", out)
        self.assertIn("turn", plain)
        self.assertEqual(plain, failing)

    def test_a_raising_ingest_does_not_propagate_out_of_dispatch(self):
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise) as calls:
            self.agent_stop()
            self.agent_stop()
        self.assertEqual(2, len(calls))
        self.assertIn("turn", self.event_names())

    def test_a_budget_overrun_leaves_the_events_and_stdout_unchanged(self):
        events = self._events_of_a_stop(_budget_exhausted)
        self.assertIn("turn", events)

    def test_a_failing_ingest_in_a_hook_process_exits_0_with_empty_stdout(self):
        self.start_session()
        # An OTel path that is a directory cannot be read as a file.
        self.otel_path.mkdir(parents=True)
        env = dict(os.environ)
        env["PYTHONPATH"] = os.pathsep.join(
            [str(support.SHARED_DIR), env.get("PYTHONPATH", "")])
        result = subprocess.run(
            [sys.executable, _ADAPTER, "agentStop"], capture_output=True, timeout=60, env=env,
            input=json.dumps({"cwd": str(self.workspace), "sessionId": support.SESSION}).encode())
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)
        self.assertEqual(b"", result.stderr)
        # The real ingest never raises; it reports the unreadable file, proving it ran.
        self.assertEqual(1, len(self.diag_lines("internal_error")), self.diag_lines())

    def test_ingest_trouble_on_subagent_stop_keeps_the_subagent_events(self):
        self.start_session()
        self.subagent_start()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_raise) as calls:
            out = self.subagent_stop()
        self.assertEqual(1, len(calls))
        self.assertEqual("", out)
        self.assertEqual(1, len(self.diag_lines("internal_error")))
        names = [e.get("event") for e in self.invocation_events(support.RUN, support.INSTANCE)]
        self.assertIn("invocation_start", names)
        self.assertIn("invocation_end", names)


class TestRunnerModeReportsNothing(_OtelCase):
    otel_set = False

    def test_runner_mode_unset_variable_gives_no_diagnostic(self):
        self.orchestrator_env(support.RUN)
        self.dispatch("sessionStart", {})
        self.dispatch("userPromptSubmitted", {"prompt": "work"})
        self.dispatch("agentStop", {"stopReason": "end_turn"})
        self.assertEqual([], self.diag_lines())


class TestRunnerModeMissingFileReportsNothing(_OtelCase):
    otel_set = True

    def test_runner_mode_missing_file_gives_no_diagnostic(self):
        self.assertFalse(self.otel_path.exists())
        self.orchestrator_env(support.RUN)
        self.dispatch("sessionStart", {})
        self.dispatch("userPromptSubmitted", {"prompt": "work"})
        self.dispatch("agentStop", {"stopReason": "end_turn"})
        self.assertEqual([], self.diag_lines())


if __name__ == "__main__":
    unittest.main()
