"""Per-event schema_version stamping.

Each event carries the highest schema_version whose fields the adapter implements for it:
invocation_start (spawn_call_id) and usage_record are 1.4.0, every other event is 1.0.0.
"""

import os
import sys
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow
import mosaic_logger_core as core

_NEWER = {"invocation_start": "1.4.0", "usage_record": "1.4.0"}


def _expected(event_name):
    return _NEWER.get(event_name, "1.0.0")


def _ctx():
    return types.SimpleNamespace(session_id="s1", run_id="20260101T000000Z-ab12",
                                 timestamp="2026-01-01T00:00:00.000Z")


class TestSchemaVersionFor(unittest.TestCase):

    def test_invocation_start_is_1_4_0(self):
        self.assertEqual("1.4.0", core.schema_version_for("invocation_start"))

    def test_usage_record_is_1_4_0(self):
        self.assertEqual("1.4.0", core.schema_version_for("usage_record"))

    def test_other_events_are_1_0_0(self):
        for name in ("run_start", "run_end", "session_start", "session_end", "turn",
                     "tool_call_start", "tool_call_end", "invocation_end",
                     "context_compaction"):
            with self.subTest(event=name):
                self.assertEqual("1.0.0", core.schema_version_for(name))


class TestBuildEventStampsPerEventVersion(unittest.TestCase):

    def test_invocation_start_is_stamped_1_4_0(self):
        event = core.build_event("invocation_start", _ctx())
        self.assertEqual("1.4.0", event["schema_version"])

    def test_tool_call_start_is_stamped_1_0_0(self):
        event = core.build_event("tool_call_start", _ctx())
        self.assertEqual("1.0.0", event["schema_version"])


class TestEmittedEventVersions(unittest.TestCase):

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)
        for kind in ("session_start", "pre_dispatch", "start", "post_read_empty",
                     "stop", "post_dispatch", "orchestrator_stop"):
            self.ws.send(kind)
        self.events = self.ws.all_events()

    def test_session_exercises_the_versioned_event_types(self):
        names = {e["event"] for e in self.events}
        self.assertTrue({"invocation_start", "invocation_end", "tool_call_start",
                         "tool_call_end"} <= names, names)

    def test_every_emitted_event_carries_the_version_of_its_type(self):
        for event in self.events:
            with self.subTest(event=event["event"]):
                self.assertEqual(_expected(event["event"]), event["schema_version"])

    def test_invocation_start_with_spawn_call_id_is_1_4_0(self):
        start = self.ws.invocation_start()
        self.assertIsNotNone(start.get("spawn_call_id"))
        self.assertEqual("1.4.0", start["schema_version"])


if __name__ == "__main__":
    unittest.main()
