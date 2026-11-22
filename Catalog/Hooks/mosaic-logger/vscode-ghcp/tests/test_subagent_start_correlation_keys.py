"""The correlation key of a SubagentStart.

With an agent_id the key is the normalised spawning tool-call id. Without one, every
spawn gets its own minted key {agent_name}_{timestamp}-{4 hex}, so two same-named
spawns never share a key, a mapping, a spawn index entry or an instance id. A start
without agent id never matches an existing invocation (it is a new spawn).
"""

import datetime
import json
import os
import re
import sys
import tempfile
import pathlib
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, SUFFIX, CorrelationCase, fallback_pattern,
)
import mosaic_logger_core as core  # noqa: E402
import mosaic_logger_subagent_state as subagent_state  # noqa: E402

_MINTED_RE = r"^Research_\d{8}T\d{6}Z-[0-9a-f]{4}$"


class TestMintSpawnKey(unittest.TestCase):

    def test_key_has_the_fallback_id_form(self):
        self.assertRegex(subagent_state.mint_spawn_key("Research"), _MINTED_RE)

    def test_key_carries_the_given_timestamp(self):
        now = datetime.datetime(2026, 11, 12, 22, 15, 0, tzinfo=datetime.timezone.utc)

        key = subagent_state.mint_spawn_key("Research", now)

        self.assertRegex(key, r"^Research_20261112T221500Z-[0-9a-f]{4}$")

    def test_two_keys_for_one_name_and_moment_differ(self):
        now = datetime.datetime(2026, 11, 12, 22, 15, 0, tzinfo=datetime.timezone.utc)

        keys = {subagent_state.mint_spawn_key("Research", now) for _ in range(40)}

        self.assertGreater(len(keys), 1)

    def test_name_is_sanitised_to_a_single_path_component(self):
        key = subagent_state.mint_spawn_key("my/agent:name")

        self.assertRegex(key, r"^[^/\\:]+_\d{8}T\d{6}Z-[0-9a-f]{4}$")

    def test_unusable_names_never_raise(self):
        results = []
        for name in ("", None, "   ", "\x00"):
            results.append(subagent_state.mint_spawn_key(name))

        self.assertTrue(all(isinstance(key, str) and key for key in results))


class TestPutSpawnEntryFields(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.paths = core.build_paths(pathlib.Path(self._tmp.name))

    def put(self, key="k1", **kwargs):
        self.assertTrue(subagent_state.put_spawn_entry(
            self.paths, key, RUN_ID, "Research#1", "sess", "Research", **kwargs))
        return json.loads(self.paths.subagent_index_file(key, "json").read_text("utf-8"))

    def test_agent_name_minted_flag_and_creation_time_are_stored(self):
        entry = self.put(agent_name="Research", minted_key=True)

        self.assertEqual("Research", entry["agent_name"])
        self.assertIs(True, entry["minted_key"])
        self.assertRegex(entry["created_at"], r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$")

    def test_minted_flag_is_absent_unless_true(self):
        self.assertNotIn("minted_key", self.put("k2", agent_name="Research"))
        self.assertNotIn("minted_key", self.put("k3", agent_name="Research", minted_key=False))

    def test_agent_name_is_absent_when_not_given(self):
        self.assertNotIn("agent_name", self.put("k4"))

    def test_existing_fields_are_unchanged(self):
        entry = self.put("k5")

        self.assertEqual({"key": "k5", "run_id": RUN_ID, "agent_instance_id": "Research#1",
                          "session_id": "sess", "agent_type": "Research"},
                         {k: entry[k] for k in ("key", "run_id", "agent_instance_id",
                                                "session_id", "agent_type")})
        self.assertIn("created_at", entry)


class TestKeyOfAStartWithAgentId(CorrelationCase):

    def test_the_key_is_the_normalised_agent_id(self):
        self.start("toolu_abc" + SUFFIX, "Research")

        entry = self.spawn_entry("toolu_abc")

        self.assertEqual("toolu_abc", entry["key"])
        self.assertNotIn("minted_key", entry)
        self.assertIsNotNone(self.mapped_instance("toolu_abc"))

    def test_invocation_start_records_the_spawn_call_id(self):
        self.start("toolu_abc" + SUFFIX, "Research")

        started = next(e for e in self.events(self.instance_of("toolu_abc"))
                       if e["event"] == "invocation_start")

        self.assertEqual("toolu_abc", started["spawn_call_id"])


class TestKeyOfAStartWithoutAgentId(CorrelationCase):

    def only_entry(self):
        entries = self.spawn_entries()
        self.assertEqual(1, len(entries))
        return entries[0]

    def test_the_key_is_minted_from_the_agent_name(self):
        self.start(agent_name="Research")

        entry = self.only_entry()

        self.assertRegex(entry["key"], _MINTED_RE)
        self.assertIs(True, entry["minted_key"])
        self.assertEqual("Research", entry["agent_name"])

    def test_the_agent_map_entry_is_written_under_the_minted_key_not_the_bare_name(self):
        self.start(agent_name="Research")

        entry = self.only_entry()

        self.assertEqual(entry["agent_instance_id"], self.mapped_instance(entry["key"]))
        self.assertFalse(self.mapping_path("Research").exists())

    def test_the_spawn_entry_names_the_invocation_folder_and_session(self):
        self.start(agent_name="Research")

        entry = self.only_entry()

        self.assertRegex(entry["agent_instance_id"], fallback_pattern("Research"))
        self.assertEqual([entry["agent_instance_id"]], self.folders())
        self.assertEqual(RUN_ID, entry["run_id"])

    def test_invocation_start_has_no_spawn_call_id(self):
        self.start(agent_name="Research")

        started = next(e for e in self.events(self.only_entry()["agent_instance_id"])
                       if e["event"] == "invocation_start")

        self.assertNotIn("spawn_call_id", started)

    def test_two_same_named_starts_get_distinct_keys_mappings_and_instance_ids(self):
        self.start(agent_name="Research")
        self.start(agent_name="Research")

        entries = self.spawn_entries()

        self.assertEqual(2, len(entries))
        self.assertEqual(2, len({e["key"] for e in entries}))
        self.assertEqual(2, len({e["agent_instance_id"] for e in entries}))
        self.assertEqual(sorted(e["agent_instance_id"] for e in entries), self.folders())
        for e in entries:
            self.assertEqual(e["agent_instance_id"], self.mapped_instance(e["key"]))
        self.assert_no_unknown_agent_folder()

    def test_a_repeated_name_only_start_is_a_new_spawn_not_a_continuation(self):
        self.dispatch_agent("Research#1")
        self.start(agent_name="Research")
        self.dispatch_agent("Research#2")

        self.start(agent_name="Research")

        self.assertEqual(["Research#1", "Research#2"], self.folders())
        self.assertEqual([], self.queued_ids())

    def test_each_same_named_start_consumes_its_own_dispatch(self):
        self.dispatch_agent("Research#1")
        self.dispatch_agent("Research#2")

        self.start(agent_name="Research")
        self.start(agent_name="Research")

        instances = sorted(e["agent_instance_id"] for e in self.spawn_entries())
        self.assertEqual(["Research#1", "Research#2"], instances)

    def test_a_different_named_start_gets_its_own_prefix(self):
        self.start(agent_name="Research")
        self.start(agent_name="Planner")

        prefixes = sorted(re.match(r"^([A-Za-z]+)_", e["key"]).group(1)
                          for e in self.spawn_entries())
        self.assertEqual(["Planner", "Research"], prefixes)

    def test_a_start_with_neither_id_nor_name_writes_nothing(self):
        self.start()

        self.assertEqual([], self.spawn_entries())
        self.assertEqual([], self.folders())


if __name__ == "__main__":
    unittest.main()
