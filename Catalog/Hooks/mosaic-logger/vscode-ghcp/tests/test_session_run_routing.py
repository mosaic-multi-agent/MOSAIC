"""Run-level events of a session whose run is known are written to that run's folder.

Tool-call, session_start, run_start, session_end and run_end events carry no prompt
text, but every VS Code hook payload carries a session_id; once a session's run is known
(from its first prompt) its later events follow it. A session with no known run still
logs to unknown-run/, and sessions with different runs do not mix.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session_routing_flow as sf

_RUN_LEVEL = ("tool_call_start", "tool_call_end", "session_start", "run_start",
              "session_end", "run_end")


class TestKnownSessionRouting(unittest.TestCase):

    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))

    def test_tool_calls_after_the_prompt_land_in_the_run_folder(self):
        self.ws.send_all(sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "post", 1))
        names = self.ws.names(sf.RUN_A, sf.SESSION_A)
        self.assertIn("tool_call_start", names)
        self.assertIn("tool_call_end", names)
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))

    def test_stop_writes_session_end_and_run_end_to_the_run_folder(self):
        self.ws.send_all(sf.stop_payload(self.ws.path, sf.SESSION_A))
        names = self.ws.names(sf.RUN_A, sf.SESSION_A)
        self.assertIn("session_end", names)
        self.assertIn("run_end", names)
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))

    def test_session_start_and_run_start_of_a_session_with_cached_run_land_in_run_folder(self):
        self.ws.send_all(sf.session_start_payload(self.ws.path, sf.SESSION_A))
        names = self.ws.names(sf.RUN_A, sf.SESSION_A)
        self.assertIn("session_start", names)
        self.assertIn("run_start", names)
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))

    def test_routed_events_carry_the_run_id_in_their_envelope(self):
        self.ws.send_all(sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1))
        starts = self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")
        self.assertEqual(1, len(starts))
        self.assertEqual(sf.RUN_A, starts[0].get("run_id"))

    def test_a_new_prompt_naming_another_run_moves_later_events_to_that_run(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_B),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 2))
        self.assertEqual(1, len(self.ws.events_of(sf.RUN_B, sf.SESSION_A, "tool_call_start")))
        self.assertEqual([], self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start"))

    def test_a_prompt_without_a_run_id_keeps_the_cached_run(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, None, text="no run here"),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 3))
        self.assertEqual(1, len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))


class TestSessionsDoNotMix(unittest.TestCase):

    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)
        self.ws.send_all(
            sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
            sf.prompt_payload(self.ws.path, sf.SESSION_B, sf.RUN_B),
            sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1),
            sf.tool_payload(self.ws.path, sf.SESSION_B, "pre", 2),
            sf.stop_payload(self.ws.path, sf.SESSION_A),
            sf.stop_payload(self.ws.path, sf.SESSION_B),
        )

    def test_each_session_writes_only_to_its_own_run(self):
        for run_id, own, other in ((sf.RUN_A, sf.SESSION_A, sf.SESSION_B),
                                   (sf.RUN_B, sf.SESSION_B, sf.SESSION_A)):
            self.assertIn("tool_call_start", self.ws.names(run_id, own))
            self.assertIn("run_end", self.ws.names(run_id, own))
            self.assertEqual([], self.ws.events_of(run_id, other, "tool_call_start"))
            self.assertEqual([], self.ws.events_of(run_id, other, "run_end"))

    def test_no_run_level_event_of_either_session_is_in_unknown_run(self):
        leftovers = [e for e in self.ws.stream(sf.UNKNOWN) if e.get("event") in _RUN_LEVEL]
        self.assertEqual([], leftovers)


class TestSessionWithoutKnownRun(unittest.TestCase):

    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)

    def test_session_with_no_known_run_keeps_logging_to_unknown_run(self):
        self.ws.send_all(sf.session_start_payload(self.ws.path, sf.SESSION_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1),
                         sf.stop_payload(self.ws.path, sf.SESSION_A))
        names = self.ws.names(sf.UNKNOWN, sf.SESSION_A)
        for expected in ("session_start", "run_start", "tool_call_start", "session_end", "run_end"):
            self.assertIn(expected, names)

    def test_unknown_session_is_not_affected_by_another_sessions_run(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_B, "pre", 1))
        self.assertEqual(1, len(self.ws.events_of(sf.UNKNOWN, sf.SESSION_B, "tool_call_start")))
        self.assertEqual([], self.ws.events_of(sf.RUN_A, sf.SESSION_B))

    def test_prompt_text_without_a_run_id_does_not_establish_a_run(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, None, text="hello"),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 1))
        self.assertEqual(1, len(self.ws.events_of(sf.UNKNOWN, sf.SESSION_A, "tool_call_start")))


if __name__ == "__main__":
    unittest.main()
