"""Round-trip test: SubagentStart writes the mapping under the tool-call ID key (vscode-ghcp)."""

import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_handlers_invocation as invocation

_RUN_ID = "20260101T170000Z-a3f9"
_SESSION_ID = "test-session-vscode-invocation"
_TS = "2026-01-01T17:00:00.000Z"


def _make_start_ctx(tmp_path, **payload_extra):
    """Build a SubagentStart context with run_id pre-set, rooted at tmp_path."""
    payload = {"hook_event_name": "SubagentStart", "session_id": _SESSION_ID,
               **payload_extra}
    workspace_root = pathlib.Path(tmp_path)
    ctx = core.HookContext(payload, workspace_root, core.build_paths(workspace_root), _TS)
    ctx.run_id = _RUN_ID
    return ctx


class TestSubagentCorrelationRoundTrip(unittest.TestCase):
    """SubagentStart writes the tool-call-ID-keyed mapping that later firings resolve.

    The handlers run in separate processes but must agree on the key; the mapping
    on disk is the handoff.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.paths = core.build_paths(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def test_start_writes_tool_call_id_keyed_mapping(self):
        """The mapping written by SubagentStart must be readable via the normalised agent_id."""
        call_id = "toolu_01MappingTestAbCdEfGh"
        prompt = '{"agent_instance_id": "MappingTestAgent#1"}'

        runstate.put_pending_dispatch(
            self.paths, _RUN_ID, _SESSION_ID, "MappingTestAgent#1", _RUN_ID, prompt=prompt
        )
        start_ctx = _make_start_ctx(self.tmp.name,
                                    agent_id=call_id + "__vscode-1793098349279",
                                    agent_type="MappingTestAgent")
        invocation.handle_subagent_start(start_ctx)

        mapping = runstate.get_agent_mapping(self.paths, _RUN_ID, call_id)
        self.assertIsNotNone(mapping,
                             "Mapping written by SubagentStart must be readable via the tool-call ID")
        self.assertEqual("MappingTestAgent#1", mapping.get("agent_instance_id"))
