"""usage_record carries the tools its API call requested.

An API message spans several transcript records that share one message.id (for
example one record each for thinking, text and tool_use). The requested tools
of a call are gathered across the whole group, so whichever record is emitted
for the id carries them all. {} means the call requested no tools. A group
whose tool records arrive in a later read is emitted again with its full tools.
"""

import json
import os
import pathlib
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_core as core
import mosaic_logger_transcript as transcript
import mosaic_logger_usage as usage
from runner_mode_support import (
    INSTANCE_ID, RUN_ID as RUNNER_RUN_ID, WorkspaceTestCase, fire, read_jsonl,
    runner_env, write_transcript,
)

FIXTURE = "requested_tools_transcript.jsonl"
FIXTURES = pathlib.Path(__file__).parent / "fixtures"
RUN_ID = "20261022T100000Z-ab12"
AGENT = "Research#1"

ONE_TOOL = "msg_011CgY375awohE1WNRWg4fDE"
SAME_TOOL_TWICE = "msg_011CgY382BqKCZHwL9dKNqgV"
TWO_TOOLS = "msg_011CgY8oQXANsrN2d7ZYexrM"
NO_TOOLS = "msg_011CgYHZ2cT2mUQRm6hfftSD"

EXPECTED = {
    ONE_TOOL: {"Read": 1},
    SAME_TOOL_TWICE: {"Agent": 2},
    TWO_TOOLS: {"Edit": 1, "Grep": 1},
    NO_TOOLS: {},
}


def _assistant(message_id, *blocks, content=None):
    body = {"id": message_id, "model": "claude-opus-4-5", "role": "assistant",
            "content": list(blocks) if content is None else content,
            "usage": {"input_tokens": 5, "output_tokens": 7}}
    return {"type": "assistant", "message": body}


def _tool(name, tool_id="toolu_a"):
    block = {"type": "tool_use", "name": name, "input": {}}
    if tool_id is not None:
        block["id"] = tool_id
    return block


def _text():
    return {"type": "text", "text": "[text]"}


def _write(path, records):
    pathlib.Path(path).write_text(
        "\n".join(json.dumps(r) for r in records) + "\n", "utf-8")
    return str(path)


def _by_record_id(events):
    grouped = {}
    for event in events:
        grouped.setdefault(event["record_id"], []).append(event)
    return grouped


class NativeCase(unittest.TestCase):

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)
        paths = core.build_paths(self.root)
        payload = {"hook_event_name": "Stop", "session_id": "s-tools"}
        self.ctx = core.HookContext(payload, self.root, paths,
                                    "2026-10-22T10:00:00.000Z")
        self.ctx.run_id = RUN_ID

    def copy_fixture(self):
        dest = self.root / FIXTURE
        shutil.copyfile(FIXTURES / FIXTURE, dest)
        return str(dest)

    def orchestrator_usage(self):
        events = read_jsonl(self.ctx.paths.orchestrator_events(RUN_ID))
        return [e for e in events if e["event"] == usage.USAGE_EVENT]

    def subagent_usage(self):
        sink = core.event_sink_for(self.ctx, AGENT)
        return [e for e in read_jsonl(sink) if e["event"] == usage.USAGE_EVENT]


class TestGroupWideToolsNative(NativeCase):

    def assert_every_event_carries_group_tools(self, events):
        grouped = _by_record_id(events)
        self.assertEqual(set(EXPECTED), set(grouped))
        for record_id, expected in EXPECTED.items():
            for event in grouped[record_id]:
                self.assertEqual(expected, event.get("requested_tools"),
                                 f"{record_id} record {event['record_index']}")

    def test_orchestrator_events_carry_the_tools_of_their_whole_message(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), None, "orchestrator_transcript")
        self.assert_every_event_carries_group_tools(self.orchestrator_usage())

    def test_agent_events_carry_the_tools_of_their_whole_message(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), AGENT, "agent_transcript")
        self.assert_every_event_carries_group_tools(self.subagent_usage())

    def test_tool_requested_twice_counts_twice(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), None, "orchestrator_transcript")
        events = _by_record_id(self.orchestrator_usage())[SAME_TOOL_TWICE]
        self.assertEqual({"Agent": 2}, events[-1]["requested_tools"])

    def test_tool_less_call_writes_an_empty_object_not_an_absent_field(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), None, "orchestrator_transcript")
        for event in _by_record_id(self.orchestrator_usage())[NO_TOOLS]:
            self.assertIn("requested_tools", event)
            self.assertEqual({}, event["requested_tools"])

    def test_first_record_of_a_group_holding_no_tool_block_still_carries_tools(
            self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), None, "orchestrator_transcript")
        first = _by_record_id(self.orchestrator_usage())[TWO_TOOLS][0]
        self.assertEqual({"Edit": 1, "Grep": 1}, first["requested_tools"])


class TestGroupWideToolsRunnerMode(WorkspaceTestCase):

    def fixture_records(self):
        return [json.loads(ln) for ln in
                (FIXTURES / FIXTURE).read_text("utf-8").splitlines()
                if ln.strip()]

    def usage_events(self, events):
        return [e for e in events if e["event"] == "usage_record"]

    def assert_group_tools(self, events):
        grouped = _by_record_id(events)
        self.assertEqual(set(EXPECTED), set(grouped))
        for record_id, expected in EXPECTED.items():
            for event in grouped[record_id]:
                self.assertEqual(expected, event.get("requested_tools"))

    def test_subagent_role_usage_records_carry_group_tools(self):
        path = write_transcript(self.workspace, self.fixture_records())
        with runner_env(self.workspace, role="subagent", run_id=RUNNER_RUN_ID,
                        instance_id=INSTANCE_ID):
            fire(self.workspace, "UserPromptSubmit", user_prompt="go",
                 transcript_path=path)
            fire(self.workspace, "SessionEnd", reason="other",
                 transcript_path=path)
        self.assert_group_tools(self.usage_events(self.invocation_events()))

    def test_orchestrator_role_usage_records_carry_group_tools(self):
        path = write_transcript(self.workspace, self.fixture_records())
        with runner_env(self.workspace, role="orchestrator",
                        run_id=RUNNER_RUN_ID):
            fire(self.workspace, "SessionStart", source="startup",
                 transcript_path=path)
            fire(self.workspace, "Stop", last_assistant_message="done",
                 transcript_path=path)
            fire(self.workspace, "SessionEnd", reason="other",
                 transcript_path=path)
        self.assert_group_tools(self.usage_events(self.orchestrator_events()))


class TestReadAssistantRecordsGrouping(unittest.TestCase):

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.dir = pathlib.Path(tmp.name)

    def read(self, records):
        return transcript.read_assistant_records(
            _write(self.dir / "t.jsonl", records))

    def test_every_record_of_a_group_carries_the_group_tools(self):
        facts = self.read([_assistant("m1", _text()),
                           _assistant("m1", _tool("Read", "t1")),
                           _assistant("m1", _tool("Grep", "t2"))])
        self.assertEqual([{"Read": 1, "Grep": 1}] * 3,
                         [f.requested_tools for f in facts])

    def test_repeated_block_id_counts_once(self):
        facts = self.read([_assistant("m1", _tool("Read", "t1")),
                           _assistant("m1", _tool("Read", "t1"))])
        self.assertEqual({"Read": 1}, facts[-1].requested_tools)

    def test_blocks_without_an_id_count_once_each(self):
        facts = self.read([_assistant("m1", _tool("Read", None),
                                      _tool("Read", None))])
        self.assertEqual({"Read": 2}, facts[0].requested_tools)

    def test_group_with_content_lists_but_no_tool_block_gives_empty(self):
        facts = self.read([_assistant("m1", _text())])
        self.assertEqual({}, facts[0].requested_tools)

    def test_group_without_any_list_content_gives_none(self):
        facts = self.read([_assistant("m1", content="plain text")])
        self.assertIsNone(facts[0].requested_tools)

    def test_one_list_content_record_makes_the_group_known(self):
        facts = self.read([_assistant("m1", content="plain text"),
                           _assistant("m1", _text())])
        self.assertEqual([{}, {}], [f.requested_tools for f in facts])

    def test_record_without_message_id_is_a_group_of_its_own(self):
        keyless = _assistant("x", _tool("Read", "t1"))
        del keyless["message"]["id"]
        other = _assistant("x", _tool("Grep", "t2"))
        del other["message"]["id"]
        facts = self.read([keyless, other])
        self.assertEqual([{"Read": 1}, {"Grep": 1}],
                         [f.requested_tools for f in facts])

    def test_distinct_messages_do_not_share_tools(self):
        facts = self.read([_assistant("m1", _tool("Read", "t1")),
                           _assistant("m2", _text())])
        self.assertEqual([{"Read": 1}, {}],
                         [f.requested_tools for f in facts])


class TestReemissionOnLaterToolRecords(NativeCase):

    def setUp(self):
        super().setUp()
        self.path = str(self.root / "t.jsonl")

    def emit(self):
        return usage.emit_usage_records(
            self.ctx, self.path, None, "orchestrator_transcript")

    def test_group_completed_in_a_later_read_is_emitted_again_with_its_tools(
            self):
        _write(self.path, [_assistant("m1", _text())])
        self.emit()
        _write(self.path, [_assistant("m1", _text()),
                           _assistant("m1", _tool("Read", "t1"))])

        written = self.emit()

        events = self.orchestrator_usage()
        self.assertGreater(written, 0)
        self.assertEqual({}, events[0]["requested_tools"])
        self.assertEqual({"Read": 1}, events[-1]["requested_tools"])
        self.assertEqual({"m1"}, {e["record_id"] for e in events})

    def test_further_tool_record_of_the_same_group_is_emitted_again(self):
        _write(self.path, [_assistant("m1", _tool("Read", "t1"))])
        self.emit()
        _write(self.path, [_assistant("m1", _tool("Read", "t1")),
                           _assistant("m1", _tool("Grep", "t2"))])

        self.emit()

        self.assertEqual({"Read": 1, "Grep": 1},
                         self.orchestrator_usage()[-1]["requested_tools"])

    def test_unchanged_transcript_is_not_emitted_again(self):
        _write(self.path, [_assistant("m1", _text()),
                           _assistant("m1", _tool("Read", "t1"))])
        self.emit()
        self.assertEqual(0, self.emit())


class TestFingerprintCoversRequestedTools(unittest.TestCase):

    def facts(self, tools):
        return transcript.RecordFacts(
            "r1", 0, model="m", token_usage={"output_tokens": 3},
            requested_tools=tools)

    def test_different_tools_give_different_fingerprints(self):
        self.assertNotEqual(
            usage.record_fingerprint(self.facts({"Read": 1})),
            usage.record_fingerprint(self.facts({"Grep": 1})))

    def test_different_counts_give_different_fingerprints(self):
        self.assertNotEqual(
            usage.record_fingerprint(self.facts({"Read": 1})),
            usage.record_fingerprint(self.facts({"Read": 2})))

    def test_empty_and_absent_give_different_fingerprints(self):
        self.assertNotEqual(
            usage.record_fingerprint(self.facts({})),
            usage.record_fingerprint(self.facts(None)))

    def test_same_tools_give_the_same_fingerprint(self):
        self.assertEqual(
            usage.record_fingerprint(self.facts({"Read": 1, "Grep": 2})),
            usage.record_fingerprint(self.facts({"Grep": 2, "Read": 1})))


class TestUsageRecordSchemaVersion(NativeCase):

    def test_usage_record_is_stamped_1_7_0(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), None, "orchestrator_transcript")
        versions = {e["schema_version"] for e in self.orchestrator_usage()}
        self.assertEqual({"1.7.0"}, versions)

    def test_agent_stream_usage_record_is_stamped_1_7_0(self):
        usage.emit_usage_records(
            self.ctx, self.copy_fixture(), AGENT, "agent_transcript")
        versions = {e["schema_version"] for e in self.subagent_usage()}
        self.assertEqual({"1.7.0"}, versions)


if __name__ == "__main__":
    unittest.main()
