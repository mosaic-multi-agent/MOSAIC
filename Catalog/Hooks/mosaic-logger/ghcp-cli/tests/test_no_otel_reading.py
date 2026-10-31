"""GHCP CLI hooks never read an OTel file and never write an OTel diagnostic.

Whether or not COPILOT_OTEL_FILE_EXPORTER_PATH is set (and whether it points at a file
holding usage spans, at a missing file, or at nothing), session start and every stop point
log no OTel-sourced usage, keep no OTel ingest state, open no OTel file and write no
diagnostics line.
"""

import builtins
import json
import os
import pathlib
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import runner_mode_support as rms

ENV_VAR = "COPILOT_OTEL_FILE_EXPORTER_PATH"
DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"
RUN = "20261028T100000Z-aaaa"
SESSION = "cli-session-aaaa-0001"
INSTANCE = "TestWriter#7"
AGENT_ID = "cli-agent-0001"

SET_WITH_SPANS = "set_with_spans"
SET_MISSING_FILE = "set_missing_file"
UNSET = "unset"


def _usage_span(span_id):
    """A chat span line carrying usage, as the Copilot exporter writes it."""
    return json.dumps({
        "type": "span", "traceId": "0123456789abcdef0123456789abcdef", "spanId": span_id,
        "name": "chat", "kind": 2,
        "attributes": {
            "gen_ai.operation.name": "chat",
            "gen_ai.conversation.id": SESSION,
            "gen_ai.response.model": "claude-sonnet-5.5",
            "gen_ai.usage.input_tokens": 100,
            "gen_ai.usage.output_tokens": 10,
        },
    }, separators=(",", ":"))


class _OpenRecorder:
    """Records every path opened through builtins.open or os.open while active."""

    def __init__(self):
        self.paths = []
        self._patches = []

    def __enter__(self):
        real_open = builtins.open
        real_os_open = os.open

        def open_spy(file, *args, **kwargs):
            self.paths.append(os.fspath(file) if not isinstance(file, int) else "")
            return real_open(file, *args, **kwargs)

        def os_open_spy(path, *args, **kwargs):
            self.paths.append(os.fspath(path))
            return real_os_open(path, *args, **kwargs)

        self._patches = [mock.patch.object(builtins, "open", open_spy),
                         mock.patch.object(os, "open", os_open_spy)]
        for patch in self._patches:
            patch.start()
        return self

    def __exit__(self, *exc):
        for patch in self._patches:
            patch.stop()

    def touched(self, path):
        target = os.path.normcase(os.path.abspath(path))
        return any(p and os.path.normcase(os.path.abspath(p)) == target for p in self.paths)


class _NoOtelCase(rms.RunnerModeTestCase):

    def configure_otel(self, mode):
        self.otel_path = self.workspace / "otel-export" / "copilot-otel.jsonl"
        os.environ.pop("MOSAIC_LOGGER_DEBUG", None)
        os.environ.pop(ENV_VAR, None)
        if mode == SET_WITH_SPANS:
            self.otel_path.parent.mkdir(parents=True, exist_ok=True)
            self.otel_path.write_text(_usage_span("s1") + "\n" + _usage_span("s2") + "\n", "utf-8")
            os.environ[ENV_VAR] = str(self.otel_path)
        elif mode == SET_MISSING_FILE:
            os.environ[ENV_VAR] = str(self.otel_path)

    def fire_native_session(self):
        """Every native event that used to start or run an OTel step, in one session."""
        self.dispatch("sessionStart", {}, SESSION)
        self.dispatch("userPromptSubmitted", {"prompt": f'run_id: "{RUN}"\nDo the work'}, SESSION)
        self.dispatch("agentStop", {"stopReason": "end_turn"}, SESSION)
        prompt = f'agent_instance_id: "{INSTANCE}"\nrun_id: "{RUN}"\nDo the task'
        self.dispatch("subagentStart", {"agentId": AGENT_ID, "agentName": "TestWriter",
                                        "agentPrompt": prompt}, SESSION)
        self.dispatch("subagentStop", {"agentId": AGENT_ID,
                                       "response": f'run_id: "{RUN}"\ndone'}, SESSION)
        self.dispatch("agentStop", {"stopReason": "end_turn"}, SESSION)
        self.dispatch("sessionEnd", {"reason": "complete"}, SESSION)

    def all_events(self):
        events = []
        for path in sorted(self.paths.root.rglob("*.jsonl")):
            if path.name == DIAGNOSTICS_FILENAME:
                continue
            events.extend(rms.read_jsonl(path))
        return events

    def assert_no_otel_effects(self, recorder):
        usage = [e for e in self.all_events() if e.get("event") == "usage_record"]
        self.assertEqual([], usage, "no usage_record is logged from OTel")
        self.assertEqual([], list(self.paths.root.glob("*/.otel-ingest")))
        diagnostics = rms.read_jsonl(self.paths.root / DIAGNOSTICS_FILENAME)
        self.assertEqual([], diagnostics, "no diagnostics line is written")
        self.assertFalse(recorder.touched(self.otel_path), "the OTel file is never opened")


class TestNativeHooksIgnoreOtel(_NoOtelCase):

    def _run(self, mode):
        self.configure_otel(mode)
        with _OpenRecorder() as recorder:
            self.fire_native_session()
        self.assert_no_otel_effects(recorder)

    def test_variable_set_to_a_file_with_usage_spans_logs_nothing_from_it(self):
        self._run(SET_WITH_SPANS)

    def test_variable_set_to_a_missing_file_writes_no_diagnostic(self):
        self._run(SET_MISSING_FILE)

    def test_variable_unset_writes_no_diagnostic(self):
        self._run(UNSET)

    def test_the_other_events_still_log_as_before(self):
        self.configure_otel(SET_WITH_SPANS)
        self.fire_native_session()
        names = [e.get("event") for e in self.orchestrator_events(RUN)]
        self.assertIn("run_end", names)


class TestRunnerRolesIgnoreOtel(_NoOtelCase):

    def _events(self):
        session = "cli-runner-session-0001"
        self.dispatch("sessionStart", {}, session)
        self.dispatch("userPromptSubmitted", {"prompt": "work"}, session)
        self.dispatch("agentStop", {"stopReason": "end_turn"}, session)
        self.dispatch("sessionEnd", {"reason": "complete"}, session)

    def _run(self, mode, env):
        self.configure_otel(mode)
        env()
        with _OpenRecorder() as recorder:
            self._events()
        self.assert_no_otel_effects(recorder)

    def test_orchestrator_role_with_the_variable_set(self):
        self._run(SET_WITH_SPANS, lambda: self.orchestrator_env(RUN))

    def test_orchestrator_role_with_the_variable_unset(self):
        self._run(UNSET, lambda: self.orchestrator_env(RUN))

    def test_subagent_role_with_the_variable_set(self):
        self._run(SET_WITH_SPANS, lambda: self.subagent_env(RUN, INSTANCE))

    def test_subagent_role_with_the_variable_unset(self):
        self._run(UNSET, lambda: self.subagent_env(RUN, INSTANCE))


if __name__ == "__main__":
    unittest.main()
