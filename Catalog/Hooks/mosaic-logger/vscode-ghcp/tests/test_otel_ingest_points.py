"""Where the vscode-ghcp adapter ingests OTel usage and where it starts reading.

With COPILOT_OTEL_FILE_EXPORTER_PATH set, Stop and SubagentStop run one non-final ingest
of the OTel file into the firing's run. No other event ingests. SessionStart records the
file's size for the session, and the run takes the smallest such size of its sessions as
its first cursor, so usage from before the run's first session never reaches the run.
"""

import json
import os
import subprocess
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support
import session_routing_flow as rf
import vscode_flow as flow


class _OtelCase(unittest.TestCase):
    def setUp(self):
        self.ws = support.OtelWorkspace()
        self.addCleanup(self.ws.cleanup)
        patcher = support.clean_environment(self.ws.otel_path)
        self.addCleanup(patcher.stop)


class TestIngestTriggers(_OtelCase):

    def test_stop_ingests_usage_into_the_sessions_run(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"), support.usage_span("s2"))
        self.ws.stop()
        self.assertEqual(["s1", "s2"], self.ws.orchestrator_record_ids())

    def test_ingested_records_carry_the_vscode_harness_session_and_run(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        record = self.ws.orchestrator_usage()[0]
        self.assertEqual("vscode-ghcp", record.get("harness"))
        self.assertEqual(support.SESSION, record.get("session_id"))
        self.assertEqual(support.RUN, record.get("run_id"))

    def _subagent_flow_up_to_start(self):
        """The orchestrator's session names flow.RUN_ID; a subagent is started in it."""
        ws = self.ws
        ws.session_start()
        ws.prompt(support.SESSION, flow.RUN_ID)
        ws.send("pre_dispatch", session_id=support.SESSION)
        ws.send("start", session_id=support.SESSION)

    def test_subagent_stop_ingests_usage_into_the_run(self):
        ws = self.ws
        self._subagent_flow_up_to_start()
        ws.add_spans(support.usage_span("sub-1"))
        self.assertEqual([], ws.orchestrator_record_ids(flow.RUN_ID))
        ws.send("stop", session_id=support.SESSION)
        self.assertEqual(["sub-1"], ws.orchestrator_record_ids(flow.RUN_ID))

    def test_only_stop_and_subagent_stop_ingest(self):
        ws = self.ws
        self._subagent_flow_up_to_start()
        ws.add_spans(support.usage_span("s1"))
        ws.send("post_read_empty", session_id=support.SESSION)
        ws.send("session_start", session_id=support.SESSION)
        ws.send("pre_dispatch", session_id=support.SESSION, hook_event_name="PreCompact")
        rf.send(rf.tool_payload(ws.path, support.SESSION, "pre", 1))
        rf.send(rf.tool_payload(ws.path, support.SESSION, "post", 1))
        ws.prompt(support.SESSION, flow.RUN_ID)
        self.assertEqual([], ws.orchestrator_record_ids(flow.RUN_ID))
        self.assertFalse((ws.run_dir(flow.RUN_ID) / ".otel-ingest").exists())
        ws.send("orchestrator_stop", session_id=support.SESSION)
        self.assertEqual(["s1"], ws.orchestrator_record_ids(flow.RUN_ID))

    def test_subagent_start_and_dispatch_events_do_not_ingest(self):
        ws = self.ws
        ws.session_start()
        ws.prompt(support.SESSION, flow.RUN_ID)
        ws.add_spans(support.usage_span("s1"))
        ws.send("pre_dispatch", session_id=support.SESSION)
        ws.send("start", session_id=support.SESSION)
        ws.send("post_dispatch", session_id=support.SESSION)
        self.assertEqual([], ws.orchestrator_record_ids(flow.RUN_ID))
        self.assertFalse((ws.run_dir(flow.RUN_ID) / ".otel-ingest").exists())
        ws.send("stop", session_id=support.SESSION)
        self.assertEqual(["s1"], ws.orchestrator_record_ids(flow.RUN_ID))

    def test_each_stop_ingests_only_what_is_new(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        self.ws.add_spans(support.usage_span("s2"))
        self.ws.stop()
        self.ws.stop()
        self.assertEqual(["s1", "s2"], self.ws.orchestrator_record_ids())

    def test_stop_without_a_known_run_ingests_nothing(self):
        self.ws.session_start()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.stop()
        self.assertEqual([], self.ws.orchestrator_record_ids())
        self.assertFalse((self.ws.log_root / "unknown-run" / ".otel-ingest").exists())

    def test_ingest_is_non_final_and_bounded_by_a_budget(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        with support.ingest_patched() as calls:
            self.ws.stop()
        self.assertEqual(1, len(calls))
        kwargs = calls[0][1]
        self.assertFalse(kwargs.get("final", False))
        self.assertFalse(kwargs.get("whole_file", False))
        budget = kwargs.get("budget_seconds")
        self.assertIsInstance(budget, (int, float))
        self.assertTrue(0 < budget <= 5.0, budget)

    def test_pending_spans_stay_pending_at_a_stop(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("orphan", chat_session="no-such-tool-call"))
        self.ws.stop()
        self.assertEqual([], self.ws.orchestrator_record_ids())
        self.assertFalse((self.ws.run_dir() / "00_unattributed_events.jsonl").exists())
        self.assertEqual(1, len(self.ws.state()["pending"]))


class TestStartingOffset(_OtelCase):

    def test_usage_written_before_the_first_session_start_is_not_ingested(self):
        self.ws.add_spans(support.usage_span("before"))
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("after"))
        self.ws.stop()
        self.assertEqual(["after"], self.ws.orchestrator_record_ids())

    def test_the_run_starts_at_the_smallest_offset_of_its_sessions(self):
        self.ws.add_spans(support.usage_span("before"))
        self.ws.start_session(support.SESSION)
        self.ws.add_spans(support.usage_span("a1"))
        self.ws.start_session(support.OTHER_SESSION)
        self.ws.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.ws.stop(support.OTHER_SESSION)
        self.assertEqual(["a1", "b1"], self.ws.orchestrator_record_ids())

    def test_a_repeated_session_start_does_not_move_the_recorded_offset(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        self.ws.session_start()
        self.ws.stop()
        self.assertEqual(["s1"], self.ws.orchestrator_record_ids())

    def test_each_run_takes_the_offset_of_its_own_sessions(self):
        self.ws.start_session(support.SESSION, support.RUN)
        self.ws.add_spans(support.usage_span("a1"))
        self.ws.add_spans(support.usage_span("b0", conversation=support.OTHER_SESSION))
        self.ws.start_session(support.OTHER_SESSION, support.OTHER_RUN)
        self.ws.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.ws.stop(support.OTHER_SESSION)
        self.assertEqual(["b1"], self.ws.orchestrator_record_ids(support.OTHER_RUN))

    def test_a_cursor_already_in_the_run_state_wins_over_a_later_session_offset(self):
        self.ws.start_session(support.SESSION)
        self.ws.add_spans(support.usage_span("a1"))
        self.ws.stop(support.SESSION)
        self.ws.add_spans(support.usage_span("a2"))
        self.ws.start_session(support.OTHER_SESSION)
        self.ws.add_spans(support.usage_span("b1", conversation=support.OTHER_SESSION))
        self.ws.stop(support.OTHER_SESSION)
        self.assertEqual(["a1", "a2", "b1"], self.ws.orchestrator_record_ids())

    def test_ingesting_never_modifies_the_otel_file(self):
        self.ws.add_spans(support.usage_span("before"))
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        before = self.ws.otel_path.read_bytes()
        self.ws.stop()
        self.assertEqual(["s1"], self.ws.orchestrator_record_ids())
        self.assertEqual(before, self.ws.otel_path.read_bytes())


class TestHookProcessIngest(_OtelCase):
    """A real hook process finds the shared module beside the adapter and ingests."""

    def test_stop_in_a_hook_process_ingests_with_empty_stdout(self):
        self.ws.start_session()
        self.ws.add_spans(support.usage_span("s1"))
        adapter = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                               "mosaic_logger.py")
        env = dict(os.environ)
        env["PYTHONPATH"] = os.pathsep.join(
            [str(support.SHARED_DIR), env.get("PYTHONPATH", "")])
        result = subprocess.run(
            [sys.executable, adapter], capture_output=True, timeout=60, env=env,
            input=json.dumps(rf.stop_payload(self.ws.path, support.SESSION)).encode())
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)
        self.assertEqual(["s1"], self.ws.orchestrator_record_ids())


if __name__ == "__main__":
    unittest.main()
