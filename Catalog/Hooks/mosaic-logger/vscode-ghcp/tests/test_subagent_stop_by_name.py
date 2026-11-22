"""SubagentStop without agent_id: a name-only stop marks exactly one minted-key spawn.

A spawn started without agent id has a minted key that no later firing can re-derive,
so a name-only SubagentStop marks stopped the oldest not-yet-stopped minted-key spawn
of that name (preferring the stop's own session). Such a spawn is completed only by
the orchestrator's Stop sweep: one invocation_end, no response, in its own folder.
"""

import json
import os
import pathlib
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from correlation_support import (  # noqa: E402
    RUN_ID, SESSION_ID, SUFFIX, CorrelationCase, ConcurrentHooksCase,
)
import mosaic_logger_core as core  # noqa: E402
import mosaic_logger_subagent_state as subagent_state  # noqa: E402

_DIAGNOSTICS = "mosaic-logger-diagnostics.jsonl"


class _NameOnlyCase(CorrelationCase):

    def start_spawn(self, name="Research", session_id=SESSION_ID, pause=0.03):
        """Start one name-only spawn; returns its spawn index entry."""
        before = {e["key"] for e in self.spawn_entries()}
        self.start(agent_name=name, session_id=session_id)
        time.sleep(pause)  # created_at has ms precision and orders the spawns
        return next(e for e in self.spawn_entries() if e["key"] not in before)

    def stopped(self, entry):
        return self.paths.subagent_index_file(entry["key"], "stopped").exists()

    def diagnostics(self):
        path = self.paths.root / _DIAGNOSTICS
        if not path.exists():
            return []
        return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]

    def stop_by_name(self, name="Research", session_id=SESSION_ID):
        self.stop_agent(agent_name=name, session_id=session_id)


class TestNameOnlyStopMarksOneSpawn(_NameOnlyCase):

    def test_one_stop_marks_only_the_oldest_of_two_same_named_spawns(self):
        first = self.start_spawn()
        second = self.start_spawn()

        self.stop_by_name()

        self.assertTrue(self.stopped(first))
        self.assertFalse(self.stopped(second))

    def test_two_stops_mark_both_spawns(self):
        first = self.start_spawn()
        second = self.start_spawn()

        self.stop_by_name()
        self.stop_by_name()

        self.assertTrue(self.stopped(first))
        self.assertTrue(self.stopped(second))

    def test_a_third_stop_has_nothing_to_mark_and_reports_an_unmapped_stop(self):
        self.start_spawn()
        self.stop_by_name()

        self.stop_by_name()

        kinds = [d.get("kind") for d in self.diagnostics()]
        self.assertIn("unmapped_subagent_stop", kinds)

    def test_a_stop_of_another_name_marks_nothing(self):
        spawn = self.start_spawn("Research")

        self.stop_by_name("Planner")

        self.assertFalse(self.stopped(spawn))
        self.assertIn("unmapped_subagent_stop", [d.get("kind") for d in self.diagnostics()])

    def test_a_spawn_with_an_agent_id_is_not_a_candidate(self):
        self.start("toolu_idd" + SUFFIX, agent_name="Research")

        self.stop_by_name()

        self.assertFalse(self.paths.subagent_index_file("toolu_idd", "stopped").exists())
        self.assertIn("unmapped_subagent_stop", [d.get("kind") for d in self.diagnostics()])

    def test_a_stop_with_neither_id_nor_name_marks_nothing(self):
        spawn = self.start_spawn()

        self.stop_agent()

        self.assertFalse(self.stopped(spawn))

    def test_a_stop_with_an_agent_id_still_uses_the_agent_id_key(self):
        self.start("toolu_idd" + SUFFIX, agent_name="Research")
        spawn = self.start_spawn()

        self.stop_agent("toolu_idd" + SUFFIX, agent_name="Research")

        self.assertTrue(self.paths.subagent_index_file("toolu_idd", "stopped").exists())
        self.assertFalse(self.stopped(spawn))


class TestSessionHandlingOfNameOnlyStops(_NameOnlyCase):

    def test_the_stops_own_session_is_preferred_over_an_older_spawn(self):
        older = self.start_spawn(session_id="session-one")
        newer = self.start_spawn(session_id="session-two")

        self.stop_by_name(session_id="session-two")

        self.assertTrue(self.stopped(newer))
        self.assertFalse(self.stopped(older))

    def test_a_session_without_a_candidate_falls_back_to_every_session(self):
        older = self.start_spawn(session_id="session-one")
        newer = self.start_spawn(session_id="session-two")

        self.stop_by_name(session_id="session-three")

        self.assertTrue(self.stopped(older))
        self.assertFalse(self.stopped(newer))

    def test_a_stop_without_a_session_id_considers_every_session(self):
        older = self.start_spawn(session_id="session-one")
        newer = self.start_spawn(session_id="session-two")

        self.stop_by_name(session_id=None)

        self.assertTrue(self.stopped(older))
        self.assertFalse(self.stopped(newer))

    def test_an_already_stopped_candidate_of_the_own_session_is_skipped(self):
        own = self.start_spawn(session_id="session-two")
        other = self.start_spawn(session_id="session-one")
        self.stop_by_name(session_id="session-two")
        self.assertTrue(self.stopped(own))

        self.stop_by_name(session_id="session-two")

        self.assertTrue(self.stopped(other))


class TestMarkStoppedByName(unittest.TestCase):
    """The state function behind a name-only stop."""

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.paths = core.build_paths(pathlib.Path(self._tmp.name))

    def spawn(self, key, session="s1", name="Research", minted=True):
        subagent_state.put_spawn_entry(
            self.paths, key, RUN_ID, key + "-instance", session, name,
            agent_name=name, minted_key=minted)
        time.sleep(0.01)

    def mark(self, session="s1", name="Research"):
        return subagent_state.mark_stopped_by_name(self.paths, session, name)

    def test_returns_the_key_of_the_oldest_candidate(self):
        self.spawn("k-old")
        self.spawn("k-new")

        self.assertEqual("k-old", self.mark())
        self.assertEqual("k-new", self.mark())

    def test_returns_none_when_no_candidate_exists(self):
        self.assertIsNone(self.mark())
        self.spawn("k-1", name="Planner")
        self.assertIsNone(self.mark(name="Research"))

    def test_returns_none_once_every_candidate_is_stopped(self):
        self.spawn("k-1")
        self.mark()

        self.assertIsNone(self.mark())

    def test_an_entry_that_is_not_minted_is_never_a_candidate(self):
        self.spawn("k-1", minted=False)

        self.assertIsNone(self.mark())

    def test_an_entry_without_a_session_is_a_candidate_for_a_cross_session_stop(self):
        self.spawn("k-1", session=None)

        self.assertEqual("k-1", self.mark(session="s-other"))

    def test_a_stop_without_a_session_considers_every_session(self):
        self.spawn("k-1", session="s1")

        self.assertEqual("k-1", self.mark(session=None))

    def test_marking_creates_the_stopped_marker_of_that_key_only(self):
        self.spawn("k-1")
        self.spawn("k-2")

        self.mark()

        self.assertTrue(self.paths.subagent_index_file("k-1", "stopped").exists())
        self.assertFalse(self.paths.subagent_index_file("k-2", "stopped").exists())

    def test_never_raises_on_an_unreadable_index(self):
        folder = self.paths.subagent_index_dir()
        folder.mkdir(parents=True, exist_ok=True)
        (folder / "broken.json").write_text("{not json", "utf-8")

        self.assertIsNone(self.mark())


class TestMintedKeySpawnsCompleteAtTheOrchestratorStop(_NameOnlyCase):

    def folders_ended(self):
        return {name: len(self.ends(name)) for name in self.folders()}

    def test_a_stopped_spawn_ends_once_without_a_response_in_its_own_folder(self):
        spawn = self.start_spawn()
        self.stop_by_name()

        self.orchestrator_stop()

        instance = spawn["agent_instance_id"]
        self.assertEqual({instance: 1}, self.folders_ended())
        end = self.ends(instance)[0]
        self.assertNotIn("response", end)
        self.assertEqual(1, self.names(instance).count("invocation_start"))
        self.assertTrue(self.paths.invocation_output(RUN_ID, instance).exists())

    def test_two_same_named_spawns_each_end_once_in_their_own_folder(self):
        first = self.start_spawn()
        second = self.start_spawn()
        self.stop_by_name()
        self.stop_by_name()

        self.orchestrator_stop()

        expected = {first["agent_instance_id"]: 1, second["agent_instance_id"]: 1}
        self.assertEqual(expected, self.folders_ended())
        self.assertNotEqual(first["agent_instance_id"], second["agent_instance_id"])

    def test_only_the_stopped_spawn_is_completed(self):
        first = self.start_spawn()
        second = self.start_spawn()
        self.stop_by_name()

        self.orchestrator_stop()

        self.assertEqual({first["agent_instance_id"]: 1, second["agent_instance_id"]: 0},
                         self.folders_ended())

    def test_a_spawn_that_never_stopped_is_not_completed_by_the_sweep(self):
        spawn = self.start_spawn()

        self.orchestrator_stop()

        self.assertEqual({spawn["agent_instance_id"]: 0}, self.folders_ended())

    def test_repeated_sweeps_add_no_second_end(self):
        spawn = self.start_spawn()
        self.stop_by_name()

        self.orchestrator_stop()
        self.orchestrator_stop()

        self.assertEqual({spawn["agent_instance_id"]: 1}, self.folders_ended())

    def test_a_dispatch_result_never_completes_a_minted_key_spawn(self):
        spawn = self.start_spawn()
        self.stop_by_name()

        self.post_dispatch("toolu_unrelated", "a result")

        self.assertEqual({spawn["agent_instance_id"]: 0}, self.folders_ended())
        self.assertNotIn("a result", json.dumps(self.events(spawn["agent_instance_id"])))


class TestConcurrentNameOnlyStops(ConcurrentHooksCase):

    def test_two_concurrent_stops_mark_two_distinct_spawns(self):
        self.start(agent_name="Research")
        time.sleep(0.03)
        self.start(agent_name="Research")
        keys = [e["key"] for e in self.spawn_entries()]
        self.assertEqual(2, len(keys))

        stop = self.payload("SubagentStop", SESSION_ID, agent_name="Research",
                            stop_hook_active=False)
        self.run_concurrently([stop, dict(stop)])

        for key in keys:
            self.assertTrue(self.paths.subagent_index_file(key, "stopped").exists(), key)


if __name__ == "__main__":
    unittest.main()
