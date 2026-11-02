"""Which hook inputs bind a VS Code session to a run.

A session is bound to a run only by a MOSAIC dispatch: the PreToolUse prompt of a
subagent-launch tool that carries an agent instance id as well as a run id. User prompts
and assistant replies that merely quote a run id (bare or as `run_id: <id>`), and tool
calls that are not dispatches, never bind: they write nothing to that run's stream and
relocate nothing into it. A dispatch for another run switches the binding. The binding
dispatch also relocates the session's earlier unknown-run lines, and the dispatching
event itself lands in the bound run.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session_routing_flow as sf
import vscode_flow as flow
import mosaic_logger_session_run as session_run

_QUOTE_FORMS = {
    "bare id": lambda run_id: f"please continue {run_id} now",
    "run_id label": lambda run_id: f"run_id: {run_id} is what we resume",
    "json": lambda run_id: f'{{"run_id":"{run_id}","task_description":"work"}}',
}


def _quote_in_prompt(ws, session_id, text):
    sf.send(sf.prompt_payload(ws.path, session_id, None, text=text))


def _quote_in_reply(ws, session_id, text):
    sf.send(sf.reply_payload(ws.path, session_id, text))


_CARRIERS = {"user prompt": _quote_in_prompt, "assistant reply": _quote_in_reply}


def _new_workspace(case):
    ws = sf.RoutingWorkspace()
    case.addCleanup(ws.cleanup)
    return ws


class TestQuotedRunIdDoesNotBind(unittest.TestCase):

    def _unbound_session_quoting(self, carrier, text):
        ws = _new_workspace(self)
        ws.send_all(sf.session_start_payload(ws.path, sf.SESSION_A),
                    sf.tool_payload(ws.path, sf.SESSION_A, "pre", 1))
        carrier(ws, sf.SESSION_A, text)
        ws.send_all(sf.tool_payload(ws.path, sf.SESSION_A, "pre", 2))
        return ws

    def test_quoted_run_id_writes_nothing_to_that_runs_stream(self):
        for carrier_name, carrier in _CARRIERS.items():
            for form_name, form in _QUOTE_FORMS.items():
                with self.subTest(carrier=carrier_name, form=form_name):
                    ws = self._unbound_session_quoting(carrier, form(sf.RUN_A))
                    self.assertEqual([], ws.stream(sf.RUN_A))
                    self.assertFalse(ws.stream_path(sf.RUN_A).exists())

    def test_quoted_run_id_leaves_the_sessions_earlier_lines_in_unknown_run(self):
        for carrier_name, carrier in _CARRIERS.items():
            for form_name, form in _QUOTE_FORMS.items():
                with self.subTest(carrier=carrier_name, form=form_name):
                    ws = self._unbound_session_quoting(carrier, form(sf.RUN_A))
                    names = ws.names(sf.UNKNOWN, sf.SESSION_A)
                    self.assertIn("session_start", names)
                    self.assertIn("run_start", names)
                    self.assertEqual(2, len(ws.read_starts(sf.UNKNOWN, sf.SESSION_A)))

    def test_quoted_run_id_leaves_the_session_without_a_cached_run(self):
        for carrier_name, carrier in _CARRIERS.items():
            with self.subTest(carrier=carrier_name):
                ws = self._unbound_session_quoting(carrier, _QUOTE_FORMS["run_id label"](sf.RUN_A))
                self.assertIsNone(session_run.get_session_run_id(ws.paths, sf.SESSION_A))

    def test_a_reply_quoting_a_run_id_does_not_route_session_end_and_run_end_there(self):
        ws = self._unbound_session_quoting(_quote_in_reply, _QUOTE_FORMS["run_id label"](sf.RUN_A))
        self.assertEqual([], ws.stream(sf.RUN_A))
        names = ws.names(sf.UNKNOWN, sf.SESSION_A)
        self.assertIn("session_end", names)
        self.assertIn("run_end", names)


class TestQuotedRunIdDoesNotSwitchABoundSession(unittest.TestCase):

    def test_quoting_another_run_keeps_the_binding_and_writes_nothing_to_the_other_run(self):
        for carrier_name, carrier in _CARRIERS.items():
            for form_name, form in _QUOTE_FORMS.items():
                with self.subTest(carrier=carrier_name, form=form_name):
                    ws = _new_workspace(self)
                    ws.send_all(sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A))
                    carrier(ws, sf.SESSION_A, form(sf.RUN_B))
                    ws.send_all(sf.tool_payload(ws.path, sf.SESSION_A, "pre", 1))
                    self.assertEqual([], ws.stream(sf.RUN_B))
                    self.assertEqual(1, len(ws.read_starts(sf.RUN_A, sf.SESSION_A)))
                    self.assertEqual(sf.RUN_A,
                                     session_run.get_session_run_id(ws.paths, sf.SESSION_A))

    def test_a_reply_quoting_another_run_keeps_run_end_in_the_bound_run(self):
        ws = _new_workspace(self)
        ws.send_all(sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A),
                    sf.reply_payload(ws.path, sf.SESSION_A, f"run_id: {sf.RUN_B}"))
        self.assertIn("run_end", ws.names(sf.RUN_A, sf.SESSION_A))
        self.assertEqual([], ws.stream(sf.RUN_B))


class TestOnlyADispatchWithAnInstanceIdBinds(unittest.TestCase):

    def _assert_unbound(self, ws):
        self.assertEqual([], ws.stream(sf.RUN_A))
        self.assertIsNone(session_run.get_session_run_id(ws.paths, sf.SESSION_A))

    def test_a_launch_tool_prompt_without_an_instance_id_does_not_bind(self):
        ws = _new_workspace(self)
        data = sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A)
        data["tool_input"]["prompt"] = f"run_id: {sf.RUN_A}\ntask: look around"
        ws.send_all(sf.session_start_payload(ws.path, sf.SESSION_A), data)
        self._assert_unbound(ws)
        self.assertEqual(1, len(ws.events_of(sf.UNKNOWN, sf.SESSION_A, "tool_call_start")))

    def test_a_launch_tool_prompt_without_a_run_id_does_not_bind(self):
        ws = _new_workspace(self)
        data = sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A)
        data["tool_input"]["prompt"] = "agent_instance_id: worker#1\ntask: look around"
        ws.send_all(data)
        self._assert_unbound(ws)

    def test_a_dispatch_shaped_prompt_in_a_tool_that_is_not_a_launch_tool_does_not_bind(self):
        ws = _new_workspace(self)
        data = sf.tool_payload(ws.path, sf.SESSION_A, "pre", 1)
        data["tool_input"]["prompt"] = sf.dispatch_payload(
            ws.path, sf.SESSION_A, sf.RUN_A)["tool_input"]["prompt"]
        ws.send_all(data)
        self._assert_unbound(ws)

    def test_a_run_path_in_tool_arguments_does_not_bind(self):
        ws = _new_workspace(self)
        data = sf.tool_payload(ws.path, sf.SESSION_A, "pre", 1)
        data["tool_input"] = {"filePath": f"/ws/Orchestration-{sf.RUN_A}/Research.md"}
        ws.send_all(data)
        self._assert_unbound(ws)

    def test_a_dispatch_with_instance_id_and_run_id_binds(self):
        ws = _new_workspace(self)
        ws.send_all(sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertEqual(sf.RUN_A, session_run.get_session_run_id(ws.paths, sf.SESSION_A))


class TestDispatchBindsAndRelocates(unittest.TestCase):

    def setUp(self):
        self.ws = _new_workspace(self)
        self.ws.send_all(sf.session_start_payload(self.ws.path, sf.SESSION_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 2))

    def test_dispatch_moves_the_sessions_earlier_unknown_run_lines_into_the_run(self):
        before = [e["event"] for e in self.ws.events_of(sf.UNKNOWN, sf.SESSION_A)]
        self.assertTrue(before)
        self.ws.send_all(sf.dispatch_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))
        moved = [e["event"] for e in self.ws.events_of(sf.RUN_A, sf.SESSION_A)]
        self.assertEqual(before, moved[:len(before)])
        self.assertEqual(2, len(self.ws.read_starts(sf.RUN_A, sf.SESSION_A)))

    def test_dispatching_tool_call_start_lands_in_the_bound_run(self):
        self.ws.send_all(sf.dispatch_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        launches = [e for e in self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")
                    if e.get("tool_name") == "runSubagent"]
        self.assertEqual(1, len(launches))
        self.assertEqual(sf.RUN_A, launches[0].get("run_id"))
        unknown_launches = [e for e in self.ws.events_of(sf.UNKNOWN, sf.SESSION_A)
                            if e.get("tool_name") == "runSubagent"]
        self.assertEqual([], unknown_launches)

    def test_events_after_the_dispatch_land_in_the_bound_run(self):
        self.ws.send_all(sf.dispatch_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 3),
                         sf.stop_payload(self.ws.path, sf.SESSION_A))
        self.assertEqual(3, len(self.ws.read_starts(sf.RUN_A, sf.SESSION_A)))
        self.assertIn("run_end", self.ws.names(sf.RUN_A, sf.SESSION_A))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))

    def test_other_sessions_unknown_run_lines_are_not_relocated(self):
        self.ws.send_all(sf.session_start_payload(self.ws.path, sf.SESSION_B),
                         sf.dispatch_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertTrue(self.ws.events_of(sf.UNKNOWN, sf.SESSION_B, "session_start"))
        self.assertEqual([], self.ws.events_of(sf.RUN_A, sf.SESSION_B))


class TestDispatchForAnotherRunSwitchesTheBinding(unittest.TestCase):

    def test_dispatch_for_run_b_switches_a_session_bound_to_run_a(self):
        ws = _new_workspace(self)
        ws.send_all(sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A),
                    sf.tool_payload(ws.path, sf.SESSION_A, "pre", 1),
                    sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_B, n=2),
                    sf.tool_payload(ws.path, sf.SESSION_A, "pre", 2))
        self.assertEqual(sf.RUN_B, session_run.get_session_run_id(ws.paths, sf.SESSION_A))
        self.assertEqual(1, len(ws.read_starts(sf.RUN_A, sf.SESSION_A)))
        self.assertEqual(1, len(ws.read_starts(sf.RUN_B, sf.SESSION_A)))

    def test_the_second_dispatch_itself_lands_in_the_new_run(self):
        ws = _new_workspace(self)
        ws.send_all(sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_A),
                    sf.dispatch_payload(ws.path, sf.SESSION_A, sf.RUN_B, n=2))
        launches_b = [e for e in ws.events_of(sf.RUN_B, sf.SESSION_A, "tool_call_start")
                      if e.get("tool_name") == "runSubagent"]
        self.assertEqual(1, len(launches_b))


class TestRecordedSubagentDispatch(unittest.TestCase):
    """The captured-shape dispatch binds the session and the subagent follows it."""

    def test_dispatch_event_and_invocation_land_in_the_dispatched_run(self):
        ws = _new_workspace(self)
        ws.start_subagent()
        launches = [e for e in ws.stream(flow.RUN_ID) if e.get("event") == "tool_call_start"
                    and e.get("tool_name") == "runSubagent"]
        self.assertEqual(1, len(launches))
        self.assertIn("session_start", [e.get("event") for e in ws.stream(flow.RUN_ID)])
        self.assertIsNotNone(ws.invocation_start())
        self.assertEqual([], ws.stream(sf.UNKNOWN))


if __name__ == "__main__":
    unittest.main()
