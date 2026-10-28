"""Subagent firings that cannot be mapped to an invocation, and concurrent firings.

An unmappable SubagentStop or spawning PostToolUse writes no invocation event to any
stream and leaves a diagnostic at OrchestrationLogs/mosaic-logger-diagnostics.jsonl,
visible without MOSAIC_LOGGER_DEBUG. Hooks write nothing to stdout and exit 0 on every
path. Two firings racing in separate hook processes still yield exactly one
invocation_end.
"""

import json
import os
import subprocess
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow

_ADAPTER = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                        "mosaic_logger.py")
_OTHER_CALL_ID = "toolu_01UnknownCallIdAbCdEfGh"


def _env_without_debug():
    env = dict(os.environ)
    env.pop("MOSAIC_LOGGER_DEBUG", None)
    return env


class _UnsetDebugCase(unittest.TestCase):
    """Runs with MOSAIC_LOGGER_DEBUG unset so only default-visible output counts."""

    def setUp(self):
        patcher = mock.patch.dict(os.environ)
        patcher.start()
        self.addCleanup(patcher.stop)
        os.environ.pop("MOSAIC_LOGGER_DEBUG", None)
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def kinds(self):
        return [d.get("kind") for d in self.ws.diagnostics()]

    def assert_no_invocation_written(self):
        self.assertEqual(0, self.ws.invocation_event_count())
        self.assertEqual([], self.ws.invocation_folders())


class TestUnmappedSubagentStop(_UnsetDebugCase):

    def test_stop_without_a_started_subagent_writes_no_invocation_event(self):
        self.ws.send("session_start")
        self.ws.send("stop", agent_id=_OTHER_CALL_ID)
        self.assert_no_invocation_written()

    def test_stop_without_a_started_subagent_leaves_a_default_visible_diagnostic(self):
        self.ws.send("session_start")
        self.ws.send("stop", agent_id=_OTHER_CALL_ID)
        matching = [d for d in self.ws.diagnostics() if d.get("kind") == "unmapped_subagent_stop"]
        self.assertEqual(1, len(matching), f"diagnostics: {self.ws.diagnostics()}")
        self.assertEqual("vscode-ghcp", matching[0].get("harness"))
        self.assertEqual([_OTHER_CALL_ID], matching[0].get("ids"))
        self.assertTrue(matching[0].get("timestamp"))
        self.assertTrue(matching[0].get("message"))

    def test_diagnostic_ids_carry_the_key_without_the_vscode_suffix(self):
        self.ws.send("stop", agent_id=_OTHER_CALL_ID + flow.SUFFIX)
        matching = [d for d in self.ws.diagnostics() if d.get("kind") == "unmapped_subagent_stop"]
        self.assertEqual([_OTHER_CALL_ID], matching[0].get("ids"))

    def test_stop_without_any_agent_key_leaves_a_diagnostic(self):
        self.ws.send("session_start")
        self.ws.send("stop", agent_id="<absent>")
        self.assert_no_invocation_written()
        self.assertIn("unmapped_subagent_stop", self.kinds())

    def test_diagnostic_is_not_an_event_stream_line(self):
        self.ws.send("stop", agent_id=_OTHER_CALL_ID)
        self.assertTrue(self.ws.diagnostics())
        self.assertFalse(any("event" in d for d in self.ws.diagnostics()))

    def test_mapped_subagent_flow_leaves_no_diagnostic(self):
        self.ws.start_subagent()
        self.ws.send("stop")
        self.ws.send("post_dispatch")
        self.ws.send("orchestrator_stop")
        self.assertEqual([], self.ws.diagnostics())


class TestUnmappedSpawningPostToolUse(_UnsetDebugCase):

    def test_completion_without_a_started_subagent_writes_no_invocation_event(self):
        self.ws.send("session_start")
        self.ws.send("post_dispatch", tool_use_id=_OTHER_CALL_ID + flow.SUFFIX)
        self.assert_no_invocation_written()

    def test_completion_without_a_started_subagent_leaves_a_default_visible_diagnostic(self):
        self.ws.send("session_start")
        self.ws.send("post_dispatch", tool_use_id=_OTHER_CALL_ID + flow.SUFFIX)
        matching = [d for d in self.ws.diagnostics()
                    if d.get("kind") == "unmapped_subagent_completion"]
        self.assertEqual(1, len(matching), f"diagnostics: {self.ws.diagnostics()}")
        self.assertEqual("vscode-ghcp", matching[0].get("harness"))
        self.assertEqual([_OTHER_CALL_ID], matching[0].get("ids"))

    def test_unmapped_completion_still_records_its_tool_call_end(self):
        self.ws.send("post_dispatch", tool_use_id=_OTHER_CALL_ID + flow.SUFFIX)
        ends = [e for e in self.ws.orchestrator_events() if e.get("event") == "tool_call_end"]
        self.assertEqual(1, len(ends))

    def test_other_subagents_completion_does_not_end_this_subagent(self):
        self.ws.start_subagent()
        self.ws.send("post_dispatch", tool_use_id=_OTHER_CALL_ID + flow.SUFFIX)
        self.assertEqual([], self.ws.invocation_ends())
        self.assertIn("unmapped_subagent_completion", self.kinds())


class TestHookProcessContract(unittest.TestCase):
    """Real hook processes: exit 0 and empty stdout on mapped and unmapped paths."""

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)

    def run_hook(self, data):
        return subprocess.run([sys.executable, _ADAPTER], input=json.dumps(data).encode(),
                              capture_output=True, timeout=30, env=_env_without_debug())

    def test_unmapped_stop_exits_0_with_empty_stdout_and_a_diagnostic(self):
        result = self.run_hook(flow.payload("stop", self.ws.path, agent_id=_OTHER_CALL_ID))
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)
        self.assertIn("unmapped_subagent_stop", [d.get("kind") for d in self.ws.diagnostics()])

    def test_unmapped_completion_exits_0_with_empty_stdout(self):
        result = self.run_hook(flow.payload(
            "post_dispatch", self.ws.path, tool_use_id=_OTHER_CALL_ID + flow.SUFFIX))
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)

    def test_mapped_completion_exits_0_with_empty_stdout(self):
        for kind in ("session_start", "pre_dispatch", "start", "stop"):
            self.assertEqual(0, self.run_hook(flow.payload(kind, self.ws.path)).returncode)
        result = self.run_hook(flow.payload("post_dispatch", self.ws.path))
        self.assertEqual(0, result.returncode)
        self.assertEqual(b"", result.stdout)
        self.assertEqual(1, len(self.ws.invocation_ends()))


class TestConcurrentFirings(unittest.TestCase):
    """The stop and the spawning PostToolUse race in separate processes."""

    ROUNDS = 3

    def spawn(self):
        return subprocess.Popen([sys.executable, _ADAPTER], stdin=subprocess.PIPE,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                env=_env_without_debug())

    def race(self, first, second):
        procs = [self.spawn(), self.spawn()]
        for proc, data in zip(procs, (first, second)):
            proc.stdin.write(json.dumps(data).encode())
            proc.stdin.close()
        for proc in procs:
            proc.wait(timeout=60)
            self.assertEqual(0, proc.returncode)
            self.assertEqual(b"", proc.stdout.read())
            proc.stdout.close()
            proc.stderr.close()

    def test_racing_stop_and_spawning_post_tool_use_yield_one_end(self):
        for round_no in range(self.ROUNDS):
            with self.subTest(round=round_no):
                ws = flow.Workspace()
                self.addCleanup(ws.cleanup)
                ws.start_subagent()
                self.race(flow.payload("stop", ws.path),
                          flow.payload("post_dispatch", ws.path))
                ends = ws.invocation_ends()
                self.assertEqual(1, len(ends))
                self.assertEqual("SUCCESS", ends[0].get("status_code"))

    def test_racing_duplicate_spawning_post_tool_use_yield_one_end(self):
        for round_no in range(self.ROUNDS):
            with self.subTest(round=round_no):
                ws = flow.Workspace()
                self.addCleanup(ws.cleanup)
                ws.start_subagent()
                self.race(flow.payload("post_dispatch", ws.path),
                          flow.payload("post_dispatch", ws.path))
                self.assertEqual(1, len(ws.invocation_ends()))


if __name__ == "__main__":
    unittest.main()
