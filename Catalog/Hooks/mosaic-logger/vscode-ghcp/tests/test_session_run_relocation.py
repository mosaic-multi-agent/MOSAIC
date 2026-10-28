"""Events a session wrote to unknown-run/ before its run was known follow it to the run.

Once a firing establishes a session's run, that session's lines in
unknown-run/00_orchestrator_events.jsonl are moved to the run's orchestrator stream in
their original order with the envelope run_id set to the run. Other sessions' lines
stay byte-for-byte; repeating the step moves nothing twice.
"""

import json
import os
import subprocess
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session_routing_flow as sf

_ADAPTER = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                        "mosaic_logger.py")


class _Base(unittest.TestCase):

    def setUp(self):
        self.ws = sf.RoutingWorkspace()
        self.addCleanup(self.ws.cleanup)

    def early_events_of(self, session_id, tool_calls):
        """SessionStart plus tool calls, all before the session's run is known."""
        self.ws.send_all(sf.session_start_payload(self.ws.path, session_id))
        for i in range(tool_calls):
            self.ws.send_all(sf.tool_payload(self.ws.path, session_id, "pre", i))


class TestRelocationOfEarlierEvents(_Base):

    def test_earlier_events_move_to_the_run_folder_once_the_run_is_known(self):
        self.early_events_of(sf.SESSION_A, 2)
        before = [e["event"] for e in self.ws.events_of(sf.UNKNOWN, sf.SESSION_A)]
        self.assertTrue(before)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))
        moved = [e["event"] for e in self.ws.events_of(sf.RUN_A, sf.SESSION_A)]
        for name in set(before):
            self.assertEqual(before.count(name), moved.count(name), name)

    def test_moved_events_keep_their_original_order(self):
        self.early_events_of(sf.SESSION_A, 3)
        original = [e["event"] for e in self.ws.events_of(sf.UNKNOWN, sf.SESSION_A)]
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        moved = [e["event"] for e in self.ws.events_of(sf.RUN_A, sf.SESSION_A)]
        self.assertEqual(original, moved[:len(original)])

    def test_moved_events_have_the_run_in_their_envelope_and_other_fields_unchanged(self):
        self.early_events_of(sf.SESSION_A, 1)
        original = self.ws.events_of(sf.UNKNOWN, sf.SESSION_A, "tool_call_start")[0]
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        moved = self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")[0]
        self.assertEqual(sf.RUN_A, moved.get("run_id"))
        self.assertEqual({**original, "run_id": sf.RUN_A}, moved)

    def test_run_start_written_before_the_run_was_known_ends_up_in_the_run_folder(self):
        self.early_events_of(sf.SESSION_A, 0)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        starts = self.ws.events_of(sf.RUN_A, sf.SESSION_A, "run_start")
        self.assertEqual(1, len(starts))
        self.assertEqual(sf.RUN_A, starts[0].get("run_id"))

    def test_events_after_the_prompt_follow_the_relocated_ones(self):
        self.early_events_of(sf.SESSION_A, 1)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 9))
        starts = self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")
        self.assertEqual(2, len(starts))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A, "tool_call_start"))


class TestOtherSessionsLeftUntouched(_Base):

    def test_lines_of_other_sessions_stay_byte_for_byte(self):
        self.ws.send_all(sf.session_start_payload(self.ws.path, sf.SESSION_B),
                         sf.tool_payload(self.ws.path, sf.SESSION_B, "pre", 1))
        self.early_events_of(sf.SESSION_A, 1)
        self.ws.send_all(sf.tool_payload(self.ws.path, sf.SESSION_B, "post", 2))
        b_lines = [ln for ln in self.ws.stream_text(sf.UNKNOWN).splitlines(keepends=True)
                   if json.loads(ln).get("session_id") == sf.SESSION_B]
        self.assertTrue(b_lines)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertEqual(b_lines, self.ws.stream_text(sf.UNKNOWN).splitlines(keepends=True))
        self.assertEqual([], self.ws.events_of(sf.RUN_A, sf.SESSION_B))

    def test_events_already_in_the_run_folder_from_another_session_are_kept(self):
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_B, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_B, "pre", 1))
        self.early_events_of(sf.SESSION_A, 1)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        self.assertEqual(1, len(self.ws.events_of(sf.RUN_A, sf.SESSION_B, "tool_call_start")))
        self.assertEqual(1, len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")))


class TestRelocationIsIdempotent(_Base):

    def test_repeating_the_prompt_does_not_duplicate_events(self):
        self.early_events_of(sf.SESSION_A, 2)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        first = self.ws.stream_text(sf.RUN_A)
        count = len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start"))
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 5))
        self.assertEqual(count + 1,
                         len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")))
        self.assertEqual(1, len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "run_start")))
        self.assertTrue(self.ws.stream_text(sf.RUN_A).startswith(first))

    def test_a_session_with_a_known_run_never_writes_again_to_unknown_run(self):
        self.early_events_of(sf.SESSION_A, 1)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.stop_payload(self.ws.path, sf.SESSION_A))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))


class TestRunChange(_Base):

    def test_events_written_under_an_earlier_run_stay_there_when_the_run_changes(self):
        self.early_events_of(sf.SESSION_A, 1)
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 4),
                         sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_B))
        self.assertEqual(2, len(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")))
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))


class TestHookProcess(_Base):

    def test_hook_exits_zero_and_prints_nothing_when_it_relocates(self):
        self.early_events_of(sf.SESSION_A, 1)
        data = json.dumps(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A))
        proc = subprocess.run([sys.executable, _ADAPTER], input=data, capture_output=True,
                              text=True, cwd=str(self.ws.path))
        self.assertEqual(0, proc.returncode)
        self.assertEqual("", proc.stdout)
        self.assertEqual([], self.ws.events_of(sf.UNKNOWN, sf.SESSION_A))
        self.assertTrue(self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start"))

    def test_unreadable_unknown_run_stream_never_loses_the_current_event(self):
        # A directory where the shared stream file should be makes reading it fail.
        stream = self.ws.stream_path(sf.UNKNOWN)
        stream.parent.mkdir(parents=True, exist_ok=True)
        stream.mkdir()
        self.ws.send_all(sf.prompt_payload(self.ws.path, sf.SESSION_A, sf.RUN_A),
                         sf.tool_payload(self.ws.path, sf.SESSION_A, "pre", 8))
        starts = self.ws.events_of(sf.RUN_A, sf.SESSION_A, "tool_call_start")
        self.assertEqual(1, len(starts))


if __name__ == "__main__":
    unittest.main()
