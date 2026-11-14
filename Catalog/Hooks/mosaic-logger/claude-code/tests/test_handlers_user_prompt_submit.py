"""Tests for UserPromptSubmit handling, using Claude Code's real payload shape.

Claude Code sends the user's text in the `prompt` field.  Covers:
  handle_user_prompt_submit -> user turn (full untruncated content)
  Runner orchestrator role  -> user turn in the orchestrator events stream
  Runner subagent role      -> prompt in 01_input.md, invocation_start, user turn
"""

import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core as core
import mosaic_logger_handlers_session as session_handlers
from runner_mode_support import (
    INSTANCE_ID, RUN_ID, WorkspaceTestCase, fire, read_jsonl, runner_env,
)

TEST_RUN_ID = "20260101T000000Z-ab12"
TEST_SESSION_ID = "test-session-abc123"
TEST_TIMESTAMP = "2026-01-01T00:00:00.000Z"


def _make_ctx(payload: dict, tmp_path: pathlib.Path) -> core.HookContext:
    paths = core.build_paths(tmp_path)
    ctx = core.HookContext(payload, tmp_path, paths, TEST_TIMESTAMP)
    ctx.run_id = TEST_RUN_ID
    return ctx


def _read_events(ctx: core.HookContext) -> list:
    return read_jsonl(ctx.paths.orchestrator_events(ctx.run_id))


def _payload(**fields) -> dict:
    payload = {"hook_event_name": "UserPromptSubmit",
               "session_id": TEST_SESSION_ID}
    payload.update(fields)
    return payload


class TestHandleUserPromptSubmit(unittest.TestCase):
    """handle_user_prompt_submit emits a turn with role 'user' and full content."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self.tmp.name)

    def tearDown(self):
        self.tmp.cleanup()

    def _handle(self, **fields):
        ctx = _make_ctx(_payload(**fields), self.tmp_path)
        session_handlers.handle_user_prompt_submit(ctx)
        return _read_events(ctx)

    def test_emits_exactly_one_turn_event(self):
        events = self._handle(prompt="Hello, Claude!")
        self.assertEqual(1, len(events))
        self.assertEqual("turn", events[0]["event"])

    def test_role_is_user(self):
        self.assertEqual("user", self._handle(prompt="Hello!")[0]["role"])

    def test_content_matches_prompt_exactly(self):
        prompt = "What is the meaning of life?"
        self.assertEqual(prompt, self._handle(prompt=prompt)[0]["content"])

    def test_content_is_full_and_untruncated(self):
        long_prompt = "A" * 50_000
        self.assertEqual(long_prompt, self._handle(prompt=long_prompt)[0]["content"])

    def test_no_event_when_prompt_absent(self):
        self.assertEqual([], self._handle())

    def test_no_event_when_prompt_is_empty_string(self):
        self.assertEqual([], self._handle(prompt=""))

    def test_no_event_when_prompt_is_null(self):
        self.assertEqual([], self._handle(prompt=None))

    def test_legacy_user_prompt_field_is_not_read(self):
        self.assertEqual([], self._handle(user_prompt="Hello!"))

    def test_user_turn_has_no_token_usage(self):
        self.assertNotIn("token_usage", self._handle(prompt="Hello!")[0])

    def test_user_turn_has_no_model(self):
        self.assertNotIn("model", self._handle(prompt="Hello!")[0])


class TestOrchestratorSessionUserTurn(WorkspaceTestCase):
    """Native and Runner orchestrator roles record user turns from `prompt`."""

    def user_turns(self):
        return [e for e in self.orchestrator_events()
                if e["event"] == "turn" and e.get("role") == "user"]

    def submit(self, role, **fields):
        with runner_env(self.workspace, role=role, run_id=RUN_ID):
            fire(self.workspace, "SessionStart", source="startup")
            fire(self.workspace, "UserPromptSubmit", **fields)

    def test_runner_orchestrator_role_records_a_user_turn(self):
        self.submit("orchestrator", prompt="decide the next step")
        turns = self.user_turns()
        self.assertEqual(1, len(turns))
        self.assertEqual("decide the next step", turns[0]["content"])

    def test_native_orchestrator_records_a_user_turn(self):
        self.submit(None, prompt="start the workflow")
        turns = self.user_turns()
        self.assertEqual(1, len(turns))
        self.assertEqual("start the workflow", turns[0]["content"])

    def test_runner_orchestrator_role_without_prompt_writes_no_turn(self):
        self.submit("orchestrator")
        self.assertEqual([], self.user_turns())

    def test_runner_orchestrator_role_with_empty_prompt_writes_no_turn(self):
        self.submit("orchestrator", prompt="")
        self.assertEqual([], self.user_turns())

    def test_native_orchestrator_with_empty_prompt_writes_no_turn(self):
        self.submit(None, prompt="")
        self.assertEqual([], self.user_turns())


class TestSubagentRoleRealPayloadPrompt(WorkspaceTestCase):
    """A Runner subagent session logs its prompt like a native invocation."""

    PROMPT = "Review the retry policy and report back."

    def play(self, **prompt_fields):
        with runner_env(self.workspace, role="subagent", run_id=RUN_ID,
                        instance_id=INSTANCE_ID):
            fire(self.workspace, "SessionStart", source="startup")
            fire(self.workspace, "UserPromptSubmit", **prompt_fields)
            fire(self.workspace, "Stop", last_assistant_message="done")
            fire(self.workspace, "SessionEnd", reason="other")

    def test_input_file_has_a_prompt_section(self):
        self.play(prompt=self.PROMPT)
        text = self.paths.invocation_input(RUN_ID, INSTANCE_ID).read_text("utf-8")
        self.assertIn("## Prompt", text)
        self.assertIn(self.PROMPT, text)

    def test_invocation_start_carries_the_prompt(self):
        self.play(prompt=self.PROMPT)
        starts = [e for e in self.invocation_events()
                  if e["event"] == "invocation_start"]
        self.assertEqual(1, len(starts))
        self.assertEqual(self.PROMPT, starts[0]["prompt"])

    def test_a_user_turn_is_written(self):
        self.play(prompt=self.PROMPT)
        turns = [e for e in self.invocation_events()
                 if e["event"] == "turn" and e.get("role") == "user"]
        self.assertEqual([self.PROMPT], [t["content"] for t in turns])

    def test_without_prompt_there_is_no_prompt_section_and_no_user_turn(self):
        self.play()
        events = self.invocation_events()
        self.assertEqual([], [e for e in events
                              if e["event"] == "turn" and e.get("role") == "user"])
        for e in events:
            if e["event"] == "invocation_start":
                self.assertNotIn("prompt", e)
        path = self.paths.invocation_input(RUN_ID, INSTANCE_ID)
        if path.exists():
            self.assertNotIn("## Prompt", path.read_text("utf-8"))


if __name__ == "__main__":
    unittest.main()
