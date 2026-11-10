"""Tests for which hook content binds a native GHCP CLI session to a run.

A session is bound to a run only by a MOSAIC dispatch (a subagent-launch prompt
carrying an agent instance id and a run id), by the Runner-declared
MOSAIC_RUN_ID, or by its existing binding. User prompts, subagent replies,
notification text, tool-argument file paths and a dispatch prompt without an
agent instance id never bind, whether the run id is bare or in structured
`run_id: <id>` form.

Covers:
  - quoted run ids in never-source fields: session stays unbound and nothing
    reaches the quoted run's folder
  - a bound session is not switched by a quoted id, but is switched by a
    dispatch for another run
  - the preToolUse subagent-launch dispatch binds before the dispatching
    tool_call_start and the pending dispatch are written
  - subagentStart dispatch prompts and Runner MOSAIC_RUN_ID still bind
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
from runner_mode_support import (
    OTHER_RUN_ID, RUN_ID, RunnerModeTestCase, event_types, read_jsonl,
)

_INSTANCE = "TestWriter#7"
_SESSION = "bind-sess"


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
    ("subagentStop", "response", {"agentId": "agt-q1"}),
    ("userPromptSubmitted", "prompt", {}),
    ("notification", "message", {"notificationType": "info"}),
)


class _BindingCase(RunnerModeTestCase):
    def dispatch(self, event, payload=None, session_id=_SESSION):
        """Fire a hook; unless told otherwise it comes from the session the binding is read for."""
        return super().dispatch(event, payload, session_id=session_id)

    def binding(self, session_id=_SESSION):
        return runstate.get_session_run_id(self.paths, session_id)

    def run_dir_exists(self, run_id):
        return self.paths.run_root(run_id).exists()

    def bind_existing(self, run_id, session_id=_SESSION):
        """Give a session an existing binding without going through a dispatch."""
        runstate.put_session_run_id(self.paths, session_id, run_id)

    def dispatch_via_tool(self, run_id, tool_name="agent", instance=_INSTANCE,
                          session_id=_SESSION, call_id="tu-1"):
        self.dispatch("preToolUse", {
            "toolName": tool_name, "toolUseId": call_id,
            "toolArgs": {"prompt": _dispatch_prompt(run_id, instance)},
        }, session_id=session_id)


class TestQuotedRunIdDoesNotBind(_BindingCase):
    """Free text that mentions a run id never binds the session to it."""

    def test_quoted_run_id_in_never_source_does_not_bind_or_reach_the_run(self):
        for event, field, extra in _NEVER_SOURCES:
            for form, text in _quoted_forms(RUN_ID).items():
                with self.subTest(event=event, form=form):
                    sid = f"quote-{event}-{form}"
                    self.dispatch(event, {field: text, **extra}, session_id=sid)
                    self.assertIsNone(self.binding(sid))
                    self.assertFalse(
                        self.run_dir_exists(RUN_ID),
                        f"{event} quoting a run id wrote into that run's folder")

    def test_later_firings_of_a_quoting_session_stay_out_of_the_quoted_run(self):
        self.dispatch("userPromptSubmitted", {"prompt": f"see run_id: {RUN_ID}"})
        self.dispatch("notification", {"notificationType": "info", "message": "later"})
        self.dispatch("preToolUse", {"toolName": "bash", "toolUseId": "tu-2",
                                     "toolArgs": {"command": "ls"}})
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_tool_argument_file_paths_do_not_bind(self):
        path = f"C:/AI/MOSAIC/MOSAIC/Orchestration-{RUN_ID}/Stage-1/Plan.md"
        for tool in ("view", "create", "bash"):
            with self.subTest(tool=tool):
                sid = f"path-{tool}"
                self.dispatch("preToolUse", {
                    "toolName": tool, "toolUseId": f"tu-{tool}",
                    "toolArgs": {"filePath": path, "path": path}}, session_id=sid)
                self.assertIsNone(self.binding(sid))
                self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_later_firings_after_a_path_argument_stay_out_of_the_run(self):
        path = f"C:/AI/MOSAIC/MOSAIC/Orchestration-{RUN_ID}/Plan.md"
        self.dispatch("preToolUse", {"toolName": "view", "toolUseId": "tu-p",
                                     "toolArgs": {"filePath": path}})
        self.dispatch("notification", {"notificationType": "info", "message": "n"})
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_subagent_start_prompt_without_instance_id_does_not_bind(self):
        for form, text in _quoted_forms(RUN_ID).items():
            with self.subTest(form=form):
                sid = f"nodispatch-{form}"
                self.dispatch("subagentStart", {
                    "agentId": f"agt-{form}", "agentType": "Helper",
                    "agentPrompt": text}, session_id=sid)
                self.assertIsNone(self.binding(sid))
                self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_json_prompt_with_run_id_but_no_instance_id_does_not_bind(self):
        prompt = f'{{"run_id": "{RUN_ID}", "task_description": "no instance"}}'
        self.dispatch("subagentStart", {
            "agentId": "agt-ni", "agentType": "Helper", "agentPrompt": prompt},
            session_id="no-inst")
        self.assertIsNone(self.binding("no-inst"))
        self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_agent_tool_prompt_without_instance_id_does_not_bind(self):
        for tool in ("agent", "task"):
            with self.subTest(tool=tool):
                self.dispatch("preToolUse", {
                    "toolName": tool, "toolUseId": f"tu-{tool}",
                    "toolArgs": {"prompt": f"look at run_id: {RUN_ID}"}},
                    session_id=f"noinst-{tool}")
                self.assertIsNone(self.binding(f"noinst-{tool}"))
                self.assertFalse(self.run_dir_exists(RUN_ID))

    def test_full_dispatch_payload_in_never_source_field_does_not_bind(self):
        # A complete dispatch (instance id plus run id) pasted or quoted in a
        # field that is not a dispatch channel is still just text.
        text = "Here is the prompt I sent: " + _dispatch_prompt(RUN_ID)
        for event, field, extra in _NEVER_SOURCES:
            with self.subTest(event=event):
                sid = f"pasted-{event}"
                self.dispatch(event, {field: text, **extra}, session_id=sid)
                self.assertIsNone(self.binding(sid))
                self.assertFalse(
                    self.run_dir_exists(RUN_ID),
                    f"{event} carrying a dispatch payload wrote into the run")

    def test_resolve_run_identity_leaves_run_id_unset_for_quoted_text(self):
        for event, field, extra in _NEVER_SOURCES:
            for form, text in _quoted_forms(RUN_ID).items():
                with self.subTest(event=event, form=form):
                    payload = {"sessionId": f"unit-{event}-{form}",
                               field: text, **extra}
                    ctx = core.HookContext(
                        event, payload, self.workspace, self.paths,
                        "2026-07-27T17:00:00.000Z")
                    mosaic_logger.resolve_run_identity(ctx)
                    self.assertIsNone(ctx.run_id)


class TestForeignSessionQuotingAnExistingRun(_BindingCase):
    """A second session that only quotes a run id leaves that run's files alone."""

    def _snapshot(self):
        root = self.paths.run_root(RUN_ID)
        return {str(p.relative_to(root)): p.read_bytes()
                for p in sorted(root.rglob("*")) if p.is_file()}

    def _transcript(self, name, text):
        path = self.workspace / name
        path.write_text('{"type": "assistant", "text": "%s"}\n' % text,
                        encoding="utf-8")
        return str(path)

    def test_quoting_session_leaves_the_existing_run_files_unchanged(self):
        self.bind_existing(RUN_ID, "sess-a")
        self.dispatch("agentStop", {
            "transcriptPath": self._transcript("a.jsonl", "from session a"),
            "stopReason": "end_turn"}, session_id="sess-a")
        self.assertTrue(self.paths.orchestrator_events(RUN_ID).exists())
        self.assertTrue(self.paths.orchestrator_raw(RUN_ID).exists())
        before = self._snapshot()

        second = self._transcript("b.jsonl", "from session b")
        quote = f"Earlier run_id: {RUN_ID} finished."
        for event, field, extra in _NEVER_SOURCES:
            self.dispatch(event, {"transcriptPath": second, field: quote, **extra},
                          session_id="sess-b")
        self.dispatch("preToolUse", {"toolName": "bash", "toolUseId": "tu-b",
                                     "toolArgs": {"command": "ls"}},
                      session_id="sess-b")
        self.dispatch("agentStop", {"transcriptPath": second,
                                    "stopReason": "end_turn"}, session_id="sess-b")
        self.dispatch("sessionEnd", {"reason": "other", "transcriptPath": second},
                      session_id="sess-b")

        self.assertIsNone(self.binding("sess-b"))
        self.assertEqual(before, self._snapshot())


class TestBoundSessionSwitching(_BindingCase):
    """A bound session follows dispatches, not quotes."""

    def test_quoted_id_does_not_switch_a_bound_session(self):
        self.bind_existing(RUN_ID)
        self.dispatch("userPromptSubmitted",
                      {"prompt": f"also run_id: {OTHER_RUN_ID}"})
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_quoting_notification_and_response_do_not_switch_a_bound_session(self):
        self.bind_existing(RUN_ID)
        self.dispatch("notification", {"notificationType": "info",
                                       "message": f"run_id: {OTHER_RUN_ID}"})
        self.dispatch("subagentStop", {"agentId": "agt-s",
                                       "response": f"check {OTHER_RUN_ID}"})
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_path_argument_does_not_switch_a_bound_session(self):
        self.bind_existing(RUN_ID)
        path = f"/workspace/Orchestration-{OTHER_RUN_ID}/Plan.md"
        self.dispatch("preToolUse", {"toolName": "view", "toolUseId": "tu-v",
                                     "toolArgs": {"filePath": path}})
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_quoted_id_turn_still_lands_in_the_bound_run(self):
        self.bind_existing(RUN_ID)
        self.dispatch("userPromptSubmitted",
                      {"prompt": f"mentions {OTHER_RUN_ID}"})
        self.assertIn("turn", event_types(self.orchestrator_events(RUN_ID)))

    def test_quoting_subagent_start_prompt_does_not_switch_a_bound_session(self):
        self.bind_existing(RUN_ID)
        self.dispatch("subagentStart", {
            "agentId": "agt-q2", "agentType": "Helper",
            "agentPrompt": f"look at run_id: {OTHER_RUN_ID}"})
        self.assertEqual(RUN_ID, self.binding())
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_dispatch_for_another_run_switches_the_binding(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch_via_tool(OTHER_RUN_ID, instance="TestWriter#8", call_id="tu-2")
        self.assertEqual(OTHER_RUN_ID, self.binding())

    def test_firings_after_a_switching_dispatch_land_in_the_new_run(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch_via_tool(OTHER_RUN_ID, instance="TestWriter#8", call_id="tu-2")
        self.dispatch("notification", {"notificationType": "info",
                                       "message": "after switch"})
        self.assertIn("notification",
                      event_types(self.orchestrator_events(OTHER_RUN_ID)))


class TestPreToolUseDispatchBinds(_BindingCase):
    """The subagent-launch dispatch binds the session before anything is written."""

    def test_agent_and_task_dispatch_with_instance_id_bind(self):
        for tool in ("agent", "task"):
            with self.subTest(tool=tool):
                sid = f"tool-{tool}"
                self.dispatch_via_tool(RUN_ID, tool_name=tool, session_id=sid)
                self.assertEqual(RUN_ID, self.binding(sid))

    def test_dispatching_tool_call_start_lands_in_the_dispatched_run(self):
        self.dispatch_via_tool(RUN_ID, call_id="tu-first")
        events = self.orchestrator_events(RUN_ID)
        starts = [e for e in events if e["event"] == "tool_call_start"]
        self.assertEqual(["tu-first"], [e["call_id"] for e in starts])
        self.assertFalse(self.run_dir_exists("unknown-run"))

    def test_later_firings_after_a_dispatch_land_in_the_dispatched_run(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch("notification", {"notificationType": "info", "message": "n"})
        self.assertIn("notification", event_types(self.orchestrator_events(RUN_ID)))

    def test_pending_dispatch_is_stored_under_the_dispatched_run(self):
        self.dispatch_via_tool(RUN_ID)
        pending = runstate.pop_pending_dispatch(self.paths, RUN_ID, _SESSION)
        self.assertIsNotNone(pending)
        self.assertEqual(_INSTANCE, pending["agent_instance_id"])

    def test_subagent_start_after_the_dispatch_routes_the_invocation(self):
        self.dispatch_via_tool(RUN_ID)
        self.dispatch("subagentStart", {"agentId": "agt-1", "agentType": "TestWriter"})
        self.assertIn("invocation_start",
                      event_types(self.invocation_events(RUN_ID, _INSTANCE)))


class TestLegitimateBindingSources(_BindingCase):
    """Subagent-start dispatch prompts and Runner MOSAIC_RUN_ID still bind."""

    def test_subagent_start_dispatch_prompt_binds_and_routes_the_invocation(self):
        self.dispatch("subagentStart", {
            "agentId": "agt-1", "agentType": "TestWriter",
            "agentPrompt": _dispatch_prompt(RUN_ID)})
        self.assertEqual(RUN_ID, self.binding())
        self.assertIn("invocation_start",
                      event_types(self.invocation_events(RUN_ID, _INSTANCE)))

    def test_runner_run_id_binds_and_ignores_quoted_ids(self):
        self.orchestrator_env(RUN_ID)
        self.dispatch("userPromptSubmitted",
                      {"prompt": f"also run_id: {OTHER_RUN_ID}"})
        self.assertIn("turn", event_types(self.orchestrator_events(RUN_ID)))
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_runner_run_id_is_not_overridden_by_an_agent_dispatch(self):
        self.orchestrator_env(RUN_ID)
        self.dispatch_via_tool(OTHER_RUN_ID)
        self.dispatch("notification", {"notificationType": "info",
                                       "message": "after dispatch"})
        self.assertIn("notification", event_types(self.orchestrator_events(RUN_ID)))
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))

    def test_runner_run_id_is_not_overridden_by_a_subagent_start_dispatch(self):
        self.orchestrator_env(RUN_ID)
        self.dispatch("subagentStart", {
            "agentId": "agt-9", "agentType": "TestWriter",
            "agentPrompt": _dispatch_prompt(OTHER_RUN_ID)})
        self.assertFalse(self.run_dir_exists(OTHER_RUN_ID))


if __name__ == "__main__":
    unittest.main()
