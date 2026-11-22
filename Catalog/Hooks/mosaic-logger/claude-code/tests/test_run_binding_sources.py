"""Tests for which hook content binds a native session to a run.

A session is bound to a run only by a MOSAIC dispatch (a subagent-launch prompt
carrying an agent instance id and a run id), by the Runner-declared
MOSAIC_RUN_ID, by the native MOSAIC_RUN_ID environment fallback, or by its
existing binding. User prompts, assistant replies, notification text and a
SubagentStart prompt without an agent instance id never bind, whether the run
id is bare or in structured `run_id: <id>` form.

Covers:
  - quoted run ids in never-source fields: session stays unbound and nothing
    reaches the quoted run's folder
  - a bound session is not switched by a quoted id, but is switched by a
    dispatch for another run
  - legitimate sources still bind
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
from runner_mode_support import (
    OTHER_RUN_ID, RUN_ID, WorkspaceTestCase, fire, fire_capture, read_jsonl,
    runner_env,
)

_FAR_FUTURE = "2999-01-01T00:00:00.000Z"
_INSTANCE = "TestWriter#7"


def _dispatch_prompt(run_id, instance=_INSTANCE):
    return (f'{{"agent_instance_id": "{instance}", "run_id": "{run_id}", '
            '"task_description": "do the work"}')


def _quoted_forms(run_id):
    """Ways a free-text field can mention a run id without dispatching."""
    return {
        "bare": f"Earlier we looked at run {run_id} and its logs.",
        "structured": f"The previous run_id: {run_id} finished cleanly.",
        "json": f'Summary: {{"run_id": "{run_id}", "status_code": "SUCCESS"}}',
    }


# (event, field carrying the quoted text, extra payload fields)
_NEVER_SOURCES = (
    ("Stop", "last_assistant_message", {}),
    ("SubagentStop", "last_assistant_message", {"agent_id": "agt-q1"}),
    ("UserPromptSubmit", "prompt", {}),
    ("Notification", "message", {"notification_type": "info"}),
)


class _BindingCase(WorkspaceTestCase):
    def binding(self, session_id="bind-sess"):
        return runstate.get_session_run_binding(
            self.paths, session_id, at=_FAR_FUTURE)

    def run_dir_exists(self, run_id):
        return self.paths.run_root(run_id).exists()

    def fire_native(self, event, **fields):
        with runner_env(self.workspace):
            fire(self.workspace, event, **{"session_id": "bind-sess", **fields})

    def fire_capture_native(self, **fields):
        with runner_env(self.workspace):
            fire_capture(self.workspace, **{"session_id": "bind-sess", **fields})

    def dispatch_via_tool(self, run_id, tool_name="Agent", instance=_INSTANCE):
        """A dispatch as the registration fires it: capture entry, then catch-all."""
        fields = dict(tool_name=tool_name, tool_use_id="tu-1",
                      tool_input={"prompt": _dispatch_prompt(run_id, instance)})
        self.fire_capture_native(**fields)
        self.fire_native("PreToolUse", **fields)


class TestQuotedRunIdDoesNotBind(_BindingCase):
    """Free text that mentions a run id never binds the session to it."""

    def test_quoted_run_id_in_never_source_does_not_bind_or_reach_the_run(self):
        for event, field, extra in _NEVER_SOURCES:
            for form, text in _quoted_forms(RUN_ID).items():
                with self.subTest(event=event, form=form):
                    sid = f"quote-{event}-{form}"
                    with runner_env(self.workspace):
                        fire(self.workspace, event, session_id=sid,
                             **{field: text}, **extra)
                    self.assertIsNone(self.binding(sid))
                    self.assertFalse(
                        self.run_dir_exists(RUN_ID),
                        f"{event} quoting a run id wrote into that run's folder")

    def test_later_firings_of_a_quoting_session_stay_out_of_the_quoted_run(self):
        self.fire_native("Stop", last_assistant_message=f"see run_id: {RUN_ID}")
        self.fire_native("Notification", notification_type="info", message="later")
        self.fire_native("PreToolUse", tool_name="Bash", tool_use_id="tu-2",
                         tool_input={"command": "ls"})
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_quoting_reply_with_transcript_does_not_export_into_the_run(self):
        transcript = os.path.join(str(self.workspace), "t.jsonl")
        with open(transcript, "w", encoding="utf-8") as fh:
            fh.write('{"type": "assistant"}\n')
        self.fire_native("Stop", last_assistant_message=f"quoting {RUN_ID}",
                         transcript_path=transcript)
        self.assertFalse(self.paths.orchestrator_raw(RUN_ID).exists())
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_subagent_start_prompt_without_instance_id_does_not_bind(self):
        for form, text in _quoted_forms(RUN_ID).items():
            with self.subTest(form=form):
                sid = f"nodispatch-{form}"
                with runner_env(self.workspace):
                    fire(self.workspace, "SubagentStart", session_id=sid,
                         agent_id=f"agt-{form}", agent_type="Helper",
                         agent_prompt=text)
                self.assertIsNone(self.binding(sid))
                self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_json_prompt_with_run_id_but_no_instance_id_does_not_bind(self):
        prompt = f'{{"run_id": "{RUN_ID}", "task_description": "no instance"}}'
        with runner_env(self.workspace):
            fire(self.workspace, "SubagentStart", session_id="no-inst",
                 agent_id="agt-ni", agent_type="Helper", agent_prompt=prompt)
        self.assertIsNone(self.binding("no-inst"))
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_full_dispatch_payload_in_never_source_field_does_not_bind(self):
        # A complete dispatch (instance id plus run id) pasted or quoted in a
        # field that is not a dispatch channel is still just text.
        text = "Here is the prompt I sent: " + _dispatch_prompt(RUN_ID)
        for event, field, extra in _NEVER_SOURCES:
            with self.subTest(event=event):
                sid = f"pasted-{event}"
                with runner_env(self.workspace):
                    fire(self.workspace, event, session_id=sid,
                         **{field: text}, **extra)
                self.assertIsNone(self.binding(sid))
                self.assertFalse(
                    self.run_dir_exists(RUN_ID),
                    f"{event} carrying a dispatch payload wrote into the run")

    def test_agent_tool_prompt_without_instance_id_does_not_bind(self):
        fields = dict(tool_name="Agent", tool_use_id="tu-3",
                      tool_input={"prompt": f"look at run_id: {RUN_ID}"})
        self.fire_capture_native(**fields)
        self.fire_native("PreToolUse", **fields)
        self.assertIsNone(self.binding())
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_resolve_run_identity_leaves_run_id_unset_for_quoted_text(self):
        for event, field, extra in _NEVER_SOURCES:
            for form, text in _quoted_forms(RUN_ID).items():
                with self.subTest(event=event, form=form):
                    payload = {"hook_event_name": event,
                               "session_id": f"unit-{event}-{form}",
                               field: text, **extra}
                    ctx = core.HookContext(
                        payload, self.workspace, self.paths,
                        "2026-07-27T17:00:00.000Z")
                    with runner_env(self.workspace):
                        mosaic_logger.resolve_run_identity(ctx)
                    self.assertIsNone(ctx.run_id)


class TestForeignSessionQuotingAnExistingRun(_BindingCase):
    """A second session that only quotes a run id leaves that run's files alone."""

    def _snapshot(self):
        root = self.paths.run_root(RUN_ID)
        return {str(p.relative_to(root)): p.read_bytes()
                for p in sorted(root.rglob("*")) if p.is_file()}

    def test_quoting_session_leaves_the_existing_run_files_unchanged(self):
        first = os.path.join(str(self.workspace), "a.jsonl")
        with open(first, "w", encoding="utf-8") as fh:
            fh.write('{"type": "assistant", "text": "from session a"}\n')
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "Stop", session_id="sess-a",
                 last_assistant_message="done", transcript_path=first)
        self.assertTrue(self.paths.orchestrator_events(RUN_ID).exists())
        self.assertTrue(self.paths.orchestrator_raw(RUN_ID).exists())
        before = self._snapshot()

        second = os.path.join(str(self.workspace), "b.jsonl")
        with open(second, "w", encoding="utf-8") as fh:
            fh.write('{"type": "assistant", "text": "from session b"}\n')
        quote = f"Earlier run_id: {RUN_ID} finished."
        with runner_env(self.workspace):
            for event, field, extra in _NEVER_SOURCES:
                fire(self.workspace, event, session_id="sess-b",
                     transcript_path=second, **{field: quote}, **extra)
            fire(self.workspace, "PreToolUse", session_id="sess-b",
                 tool_name="Bash", tool_use_id="tu-b",
                 tool_input={"command": "ls"})
            fire(self.workspace, "SessionEnd", session_id="sess-b",
                 reason="other", transcript_path=second)

        self.assertIsNone(self.binding("sess-b"))
        self.assertEqual(before, self._snapshot())


class TestBoundSessionSwitching(_BindingCase):
    """A bound session follows dispatches, not quotes."""

    def test_quoted_id_does_not_switch_a_bound_session(self):
        self.dispatch_via_tool(RUN_ID)
        self.fire_native("Stop",
                         last_assistant_message=f"also run_id: {OTHER_RUN_ID}")
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_quoting_prompt_and_notification_do_not_switch_a_bound_session(self):
        self.dispatch_via_tool(RUN_ID)
        self.fire_native("UserPromptSubmit", prompt=f"check {OTHER_RUN_ID}")
        self.fire_native("Notification", notification_type="info",
                         message=f"run_id: {OTHER_RUN_ID}")
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_quoted_id_turn_still_lands_in_the_bound_run(self):
        self.dispatch_via_tool(RUN_ID)
        self.fire_native("Stop", last_assistant_message=f"mentions {OTHER_RUN_ID}")
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("turn", [e["event"] for e in events])

    def test_quoting_subagent_start_prompt_does_not_switch_a_bound_session(self):
        self.dispatch_via_tool(RUN_ID)
        self.fire_native("SubagentStart", agent_id="agt-q2", agent_type="Helper",
                         agent_prompt=f"look at run_id: {OTHER_RUN_ID}")
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_dispatch_for_another_run_switches_the_binding(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch_via_tool(OTHER_RUN_ID, instance="TestWriter#8")
        self.assertEqual(OTHER_RUN_ID, self.binding())

    def test_firings_after_a_switching_dispatch_land_in_the_new_run(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch_via_tool(OTHER_RUN_ID, instance="TestWriter#8")
        self.fire_native("Notification", notification_type="info",
                         message="after switch")
        events = read_jsonl(self.paths.orchestrator_events(OTHER_RUN_ID))
        self.assertIn("notification", [e["event"] for e in events])


class TestLegitimateBindingSources(_BindingCase):
    """Dispatch content, Runner MOSAIC_RUN_ID and the env fallback still bind."""

    def test_agent_tool_dispatch_with_instance_id_binds(self):
        self.dispatch_via_tool(RUN_ID, tool_name="Agent")
        self.assertEqual(RUN_ID, self.binding())

    def test_task_tool_dispatch_with_instance_id_binds(self):
        self.dispatch_via_tool(RUN_ID, tool_name="Task")
        self.assertEqual(RUN_ID, self.binding())

    def test_later_firings_after_a_dispatch_land_in_the_dispatched_run(self):
        self.dispatch_via_tool(RUN_ID)
        self.fire_native("Notification", notification_type="info", message="n")
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("notification", [e["event"] for e in events])

    def test_subagent_start_dispatch_prompt_binds_and_routes_the_invocation(self):
        self.fire_native("SubagentStart", agent_id="agt-1", agent_type="TestWriter",
                         agent_prompt=_dispatch_prompt(RUN_ID))
        self.assertEqual(RUN_ID, self.binding())
        events = read_jsonl(self.paths.invocation_events(RUN_ID, _INSTANCE))
        self.assertIn("invocation_start", [e["event"] for e in events])

    def test_subagent_start_pending_dispatch_run_id_binds(self):
        runstate.put_pending_dispatch(
            self.paths, "bind-sess", _INSTANCE, RUN_ID,
            prompt=_dispatch_prompt(RUN_ID))
        self.fire_native("SubagentStart", agent_id="agt-2", agent_type="TestWriter")
        self.assertEqual(RUN_ID, self.binding())
        events = read_jsonl(self.paths.invocation_events(RUN_ID, _INSTANCE))
        self.assertIn("invocation_start", [e["event"] for e in events])

    def test_runner_run_id_binds_and_ignores_quoted_ids(self):
        with runner_env(self.workspace, role="orchestrator", run_id=RUN_ID):
            fire(self.workspace, "Stop", session_id="bind-sess",
                 last_assistant_message=f"also run_id: {OTHER_RUN_ID}")
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("turn", [e["event"] for e in events])
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_runner_run_id_is_not_overridden_by_an_agent_dispatch(self):
        with runner_env(self.workspace, role="orchestrator", run_id=RUN_ID):
            fire_capture(self.workspace, session_id="bind-sess",
                         tool_name="Agent", tool_use_id="tu-9",
                         tool_input={"prompt": _dispatch_prompt(OTHER_RUN_ID)})
            fire(self.workspace, "PreToolUse", session_id="bind-sess",
                 tool_name="Agent", tool_use_id="tu-9",
                 tool_input={"prompt": _dispatch_prompt(OTHER_RUN_ID)})
            fire(self.workspace, "Notification", session_id="bind-sess",
                 notification_type="info", message="after dispatch")
        self.assertEqual(RUN_ID, self.binding())
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("notification", [e["event"] for e in events])
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_runner_run_id_is_not_overridden_by_a_subagent_start_dispatch(self):
        with runner_env(self.workspace, role="orchestrator", run_id=RUN_ID):
            fire(self.workspace, "SubagentStart", session_id="bind-sess",
                 agent_id="agt-9", agent_type="TestWriter",
                 agent_prompt=_dispatch_prompt(OTHER_RUN_ID))
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_native_env_fallback_binds(self):
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "SessionStart", session_id="bind-sess")
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("session_start", [e["event"] for e in events])

    def test_env_fallback_session_ignores_quoted_ids(self):
        with runner_env(self.workspace, run_id=RUN_ID):
            fire(self.workspace, "Stop", session_id="bind-sess",
                 last_assistant_message=f"also run_id: {OTHER_RUN_ID}")
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))
        events = read_jsonl(self.paths.orchestrator_events(RUN_ID))
        self.assertIn("turn", [e["event"] for e in events])


if __name__ == "__main__":
    unittest.main()
