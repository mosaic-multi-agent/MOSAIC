"""Tests that every adapter-built event carrying `model` carries `provider`.

The provider is derived from the adapter's own process environment. Each
scenario drives one event path (run_start, session_start, Stop-time turn and
usage_record, hand-back invocation_end and turn, SubagentStop, Runner
subagent session end) under a controlled environment and checks the `provider`
of every event that has a `model`. An environment that yields no provider
produces events without the key. Events without a model never carry it.
"""

import json
import os
import pathlib
import shutil
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger
import mosaic_logger_core as core
import mosaic_logger_handlers_session as session_handlers

from runner_mode_support import (
    INSTANCE_ID, RUN_ID, WorkspaceTestCase, assistant_record, fire,
    read_jsonl, runner_env, user_record, write_transcript,
)

_TS = "2026-01-01T00:00:00.000Z"
_MODEL = "claude-opus-4-5"
_AGENT_ID = "a24d6c04de2a3dec4"
_NATIVE_INSTANCE = "TestWriter#5"
_SESSION = "provider-session"

_PROVIDER_ENV_VARS = (
    "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX",
    "CLAUDE_CODE_USE_FOUNDRY", "CLAUDE_CODE_USE_ANTHROPIC_AWS",
    "CLAUDE_CODE_USE_MANTLE", "CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST",
    "ANTHROPIC_BASE_URL", "ANTHROPIC_BEDROCK_BASE_URL",
    "ANTHROPIC_VERTEX_BASE_URL",
)

PLAIN = {}
BEDROCK = {"CLAUDE_CODE_USE_BEDROCK": "1"}
VERTEX = {"CLAUDE_CODE_USE_VERTEX": "1"}
PROXY = {"ANTHROPIC_BASE_URL": "https://proxy.example.com"}
TWO_PROVIDERS = {"CLAUDE_CODE_USE_BEDROCK": "1", "CLAUDE_CODE_USE_VERTEX": "1"}


class ProviderEnvCase(WorkspaceTestCase):
    """Base: runs each scenario under a fully controlled provider environment."""

    def use_env(self, env):
        patcher = mock.patch.dict(os.environ, dict(env))
        patcher.start()
        self.addCleanup(patcher.stop)
        for name in _PROVIDER_ENV_VARS:
            if name not in env:
                os.environ.pop(name, None)

    def reset_logs(self):
        """Drop all log and state files so each environment starts clean."""
        shutil.rmtree(self.paths.root, ignore_errors=True)

    def model_events(self, events, names):
        """Events of the named types that carry a model; fails when none do."""
        found = [e for e in events if e["event"] in names and "model" in e]
        got = sorted({e["event"] for e in found})
        self.assertEqual(sorted(set(names)), got,
                         "scenario did not produce model-bearing %s" % (names,))
        return found

    def assert_provider(self, events, expected):
        for e in events:
            if expected is None:
                self.assertNotIn("provider", e, e["event"])
            else:
                self.assertEqual(expected, e.get("provider"), e["event"])

    def assert_all_envs(self, scenario, names):
        """Run scenario once per environment and check the provider on each."""
        cases = ((PLAIN, "anthropic"), (BEDROCK, "amazon-bedrock"),
                 (VERTEX, "google-vertex-anthropic"), (PROXY, None),
                 (TWO_PROVIDERS, None))
        for env, expected in cases:
            with self.subTest(env=sorted(env)):
                self.use_env(env)
                events = scenario()
                self.assert_provider(self.model_events(events, names), expected)


# ---------------------------------------------------------------------------
# Native orchestrator paths
# ---------------------------------------------------------------------------

def _native_ctx(workspace, payload):
    paths = core.build_paths(workspace)
    ctx = core.HookContext(payload, workspace, paths, _TS)
    ctx.run_id = RUN_ID
    return ctx


class TestNativeSessionEvents(ProviderEnvCase):

    def _orchestrator_stream(self):
        return read_jsonl(self.paths.orchestrator_events(RUN_ID))

    def _fresh(self):
        self.reset_logs()

    def test_run_start_carries_provider(self):
        def scenario():
            self._fresh()
            ctx = _native_ctx(self.workspace, {
                "hook_event_name": "SessionStart", "session_id": _SESSION,
                "model": _MODEL})
            session_handlers.emit_run_start(ctx)
            return self._orchestrator_stream()
        self.assert_all_envs(scenario, ["run_start"])

    def test_session_start_carries_provider(self):
        def scenario():
            self._fresh()
            ctx = _native_ctx(self.workspace, {
                "hook_event_name": "SessionStart", "session_id": _SESSION,
                "source": "startup", "model": _MODEL})
            session_handlers.handle_session_start(ctx)
            return self._orchestrator_stream()
        self.assert_all_envs(scenario, ["session_start"])

    def test_stop_turn_and_usage_record_carry_provider(self):
        transcript = write_transcript(
            self.workspace, [user_record("hi"), assistant_record("done")])

        def scenario():
            self._fresh()
            ctx = _native_ctx(self.workspace, {
                "hook_event_name": "Stop", "session_id": _SESSION,
                "last_assistant_message": "done",
                "transcript_path": transcript})
            session_handlers.handle_stop(ctx)
            return self._orchestrator_stream()
        self.assert_all_envs(scenario, ["turn", "usage_record"])

    def test_session_start_without_model_carries_no_provider(self):
        self.use_env(PLAIN)
        ctx = _native_ctx(self.workspace, {
            "hook_event_name": "SessionStart", "session_id": _SESSION,
            "source": "startup"})
        session_handlers.handle_session_start(ctx)
        event = self._orchestrator_stream()[0]
        self.assertNotIn("model", event)
        self.assertNotIn("provider", event)

    def test_user_turn_carries_no_provider(self):
        self.use_env(BEDROCK)
        ctx = _native_ctx(self.workspace, {
            "hook_event_name": "UserPromptSubmit", "session_id": _SESSION,
            "user_prompt": "go"})
        session_handlers.handle_user_prompt_submit(ctx)
        event = self._orchestrator_stream()[0]
        self.assertNotIn("provider", event)


# ---------------------------------------------------------------------------
# Native subagent paths (hand-back and SubagentStop)
# ---------------------------------------------------------------------------

_REPLY = json.dumps({"agent_instance_id": _NATIVE_INSTANCE, "run_id": RUN_ID,
                     "status_code": "SUCCESS", "status_message": "Done."})


class TestNativeSubagentEvents(ProviderEnvCase):

    def setUp(self):
        super().setUp()
        self.session_path = self.workspace / "claude" / "sess.jsonl"
        self.session_path.parent.mkdir(parents=True)
        self.session_path.write_text("", encoding="utf-8")
        base = str(self.session_path)[:-len(".jsonl")]
        self.agent_transcript = pathlib.Path(base) / "subagents" / (
            "agent-%s.jsonl" % _AGENT_ID)
        self.agent_transcript.parent.mkdir(parents=True)
        self.agent_transcript.write_text(
            json.dumps(assistant_record("work", message_id="m1")) + "\n",
            encoding="utf-8")

    def _payload(self, event, **fields):
        p = {"hook_event_name": event, "session_id": _SESSION,
             "agent_id": _AGENT_ID, "agent_type": "TestWriter"}
        p.update(fields)
        return p

    def _start(self):
        return self._payload("SubagentStart", agent_prompt=json.dumps({
            "agent_instance_id": _NATIVE_INSTANCE, "run_id": RUN_ID}))

    def _handback(self):
        common = {"permission_mode": "auto", "tool_name": "SubagentHandback",
                  "tool_use_id": "tu-1", "tool_input": {"message": _REPLY},
                  "transcript_path": str(self.session_path)}
        pre = self._payload("PreToolUse", **common)
        post = self._payload("PostToolUse", **common, duration_ms=5,
                             tool_response={"success": True})
        return pre, post

    def _stop(self):
        return self._payload("SubagentStop", last_assistant_message="bye",
                             transcript_path=str(self.session_path),
                             agent_transcript_path=str(self.agent_transcript))

    def _run(self, *payloads):
        self.reset_logs()
        path = self.paths.invocation_events(RUN_ID, _NATIVE_INSTANCE)
        with runner_env(self.workspace):
            for p in payloads:
                mosaic_logger.dispatch(json.dumps(p))
        return read_jsonl(path)

    def test_handback_invocation_end_and_turn_carry_provider(self):
        pre, post = self._handback()

        def scenario():
            return self._run(self._start(), pre, post)
        self.assert_all_envs(scenario, ["invocation_end", "turn"])

    def test_subagent_stop_events_carry_provider(self):
        def scenario():
            return self._run(self._start(), self._stop())
        self.assert_all_envs(scenario, ["invocation_end", "turn", "usage_record"])

    def test_handback_then_stop_keeps_provider_on_every_model_event(self):
        pre, post = self._handback()
        self.use_env(BEDROCK)
        events = self._run(self._start(), pre, post, self._stop())
        modelled = self.model_events(events, ["invocation_end", "usage_record"])
        self.assert_provider(modelled, "amazon-bedrock")

    def test_tool_events_carry_no_provider(self):
        pre, post = self._handback()
        self.use_env(BEDROCK)
        events = self._run(self._start(), pre, post)
        tool_events = [e for e in events if e["event"].startswith("tool_call")]
        self.assertTrue(tool_events)
        for e in tool_events:
            self.assertNotIn("provider", e)


# ---------------------------------------------------------------------------
# Runner subagent path
# ---------------------------------------------------------------------------

class TestRunnerSubagentEvents(ProviderEnvCase):

    def _session(self):
        self.reset_logs()
        path = self.paths.invocation_events(RUN_ID, INSTANCE_ID)
        transcript = write_transcript(
            self.workspace, [user_record("go"), assistant_record("ok")])
        with runner_env(self.workspace, role="subagent", run_id=RUN_ID,
                        instance_id=INSTANCE_ID):
            fire(self.workspace, "UserPromptSubmit", user_prompt="go",
                 transcript_path=transcript)
            fire(self.workspace, "SessionEnd", reason="other",
                 last_assistant_message="ok",
                 transcript_path=transcript)
        return read_jsonl(path)

    def test_runner_invocation_end_and_turn_carry_provider(self):
        self.assert_all_envs(self._session, ["invocation_end", "turn"])

    def test_runner_two_provider_variables_omit_provider(self):
        self.use_env(TWO_PROVIDERS)
        events = self._session()
        modelled = self.model_events(events, ["invocation_end", "turn"])
        self.assert_provider(modelled, None)


if __name__ == "__main__":
    unittest.main()
