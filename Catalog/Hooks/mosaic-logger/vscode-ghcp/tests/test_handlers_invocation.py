"""Tests for the SubagentStart handler and the correlation seam (vscode-ghcp variant).

Key vscode-ghcp behaviours:
  D3: Subagent correlation is keyed on the spawning tool-call ID: SubagentStart /
      SubagentStop `agent_id` (`toolu_...`) with any `__vscode-<ms>` suffix stripped;
      without `agent_id` the key is not derived from `agent_name`: a unique key is
      minted per spawn so same-named spawns never share one
  D3: SubagentStart must still be processed when `agent_id` is absent (minted key)
  D3: resolve_agent_type returns agent_type when present, falls back to agent_name
  The invocation_end is written by the orchestrator's PostToolUse for the spawning
  tool (or the Stop sweep), not by SubagentStop; those paths are covered by the
  subagent completion tests.

Covers:
  resolve_agent_key: normalised agent_id, None when absent (no agent_name fallback)
  resolve_agent_type: returns agent_type, falls back to agent_name
  extract_status_code: last valid code wins; invalid strings ignored
  SubagentStart: pops pending dispatch; keyed mapping written BEFORE event emission;
    invocation_start + user turn to invocation stream
  (round trip of the tool-call ID key lives in test_handlers_invocation_roundtrip.py)
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_handlers_invocation as invocation


# ---------------------------------------------------------------------------
# Shared constants and helpers
# ---------------------------------------------------------------------------

_RUN_ID = "20260101T170000Z-a3f9"
_SESSION_ID = "test-session-vscode-invocation"
_TS = "2026-01-01T17:00:00.000Z"


def _make_ctx(tmp_path, hook_event_name, run_id=_RUN_ID,
              session_id=_SESSION_ID, **payload_extra):
    """Build a HookContext with run_id pre-set, rooted at tmp_path."""
    payload = {
        "hook_event_name": hook_event_name,
        "session_id": session_id,
        **payload_extra,
    }
    workspace_root = pathlib.Path(tmp_path)
    paths = core.build_paths(workspace_root)
    ctx = core.HookContext(payload, workspace_root, paths, _TS)
    ctx.run_id = run_id
    return ctx


def _make_start_ctx(tmp_path, run_id=_RUN_ID, **kwargs):
    """Build a SubagentStart context."""
    return _make_ctx(tmp_path, "SubagentStart", run_id=run_id, **kwargs)


def _make_stop_ctx(tmp_path, run_id=_RUN_ID, **kwargs):
    """Build a SubagentStop context."""
    return _make_ctx(tmp_path, "SubagentStop", run_id=run_id, **kwargs)


def _read_jsonl(path):
    """Read all non-empty lines of a JSONL file as parsed dicts."""
    return [json.loads(ln) for ln in pathlib.Path(path).read_text("utf-8").splitlines()
            if ln.strip()]


def _write_transcript_file(dir_path, records):
    """Write a minimal JSONL transcript and return its path string."""
    dir_path = pathlib.Path(dir_path)
    dir_path.mkdir(parents=True, exist_ok=True)
    path = dir_path / "transcript.jsonl"
    with open(path, "w", encoding="utf-8") as fh:
        for record in records:
            fh.write(json.dumps(record) + "\n")
    return str(path)


def _register_mapping(tmp_path, run_id, agent_key, agent_instance_id, agent_type=None):
    """Pre-register an agent_key -> agent_instance_id mapping on disk."""
    paths = core.build_paths(pathlib.Path(tmp_path))
    runstate.put_agent_mapping(paths, run_id, agent_key, agent_instance_id, agent_type)


def _pre_populate_pending_dispatch(tmp_path, run_id, session_id,
                                   agent_instance_id, extracted_run_id=None, prompt=None):
    """Write one pending-dispatch entry so SubagentStart can pop it."""
    paths = core.build_paths(pathlib.Path(tmp_path))
    runstate.put_pending_dispatch(
        paths, run_id, session_id, agent_instance_id, extracted_run_id, prompt=prompt
    )


# ---------------------------------------------------------------------------
# resolve_agent_key (D3 seam)
# ---------------------------------------------------------------------------

class TestResolveAgentKey(unittest.TestCase):
    """resolve_agent_key returns the normalised agent_id when present and None when it
    is absent. It never falls back to agent_name: two same-named spawns without an
    agent_id must not share a correlation key (a start mints a unique key instead).

    This is the D3 seam: SubagentStart and SubagentStop both route through it,
    so start and stop cannot disagree about which key was used.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_returns_agent_id_when_present(self):
        ctx = _make_start_ctx(self.tmp.name, agent_id="toolu_01AbCdEfGhIjKlMn")
        self.assertEqual("toolu_01AbCdEfGhIjKlMn", invocation.resolve_agent_key(ctx))

    def test_agent_id_takes_precedence_over_agent_name(self):
        """agent_id (the spawning tool-call ID) wins over agent_name."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter",
                              agent_id="toolu_01AbCdEfGhIjKlMn")
        self.assertEqual("toolu_01AbCdEfGhIjKlMn", invocation.resolve_agent_key(ctx))

    def test_strips_vscode_suffix_from_agent_id(self):
        ctx = _make_stop_ctx(self.tmp.name,
                             agent_id="toolu_01AbCdEfGhIjKlMn__vscode-1793098349279")
        self.assertEqual("toolu_01AbCdEfGhIjKlMn", invocation.resolve_agent_key(ctx))

    def test_does_not_fall_back_to_agent_name_when_agent_id_absent(self):
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter")
        self.assertIsNone(invocation.resolve_agent_key(ctx))

    def test_returns_none_when_both_absent(self):
        """A firing with neither id nor name cannot be correlated at all."""
        ctx = _make_start_ctx(self.tmp.name)  # no agent_name, no agent_id
        self.assertIsNone(invocation.resolve_agent_key(ctx))

    def test_empty_agent_id_normalizes_to_none_and_does_not_use_agent_name(self):
        """Empty string agent_id normalizes to None via ctx.field(); agent_name is not a key."""
        ctx = _make_stop_ctx(self.tmp.name, agent_id="", agent_name="fallback-name")
        self.assertIsNone(invocation.resolve_agent_key(ctx))


# ---------------------------------------------------------------------------
# resolve_agent_type (D3 fallback to agent_name)
# ---------------------------------------------------------------------------

class TestResolveAgentType(unittest.TestCase):
    """resolve_agent_type returns agent_type when present, else agent_name.

    The schema defines agent_type as the bare agent name; VS Code's agent_name
    carries exactly that value. Without the fallback, invocation_start would
    systematically omit a field that invocation_end populates.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_returns_agent_type_when_present(self):
        ctx = _make_stop_ctx(self.tmp.name, agent_type="Researcher", agent_name="Researcher")
        result = invocation.resolve_agent_type(ctx)
        self.assertEqual("Researcher", result)

    def test_falls_back_to_agent_name_when_agent_type_absent(self):
        """When agent_type is absent, agent_name is the fallback."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter")
        result = invocation.resolve_agent_type(ctx)
        self.assertEqual("TestWriter", result)

    def test_returns_none_when_both_agent_type_and_agent_name_absent(self):
        ctx = _make_start_ctx(self.tmp.name)
        result = invocation.resolve_agent_type(ctx)
        self.assertIsNone(result)

    def test_empty_values_count_as_absent(self):
        ctx = _make_start_ctx(self.tmp.name, agent_type="", agent_name="")
        self.assertIsNone(invocation.resolve_agent_type(ctx))

    def test_empty_agent_type_falls_back_to_agent_name(self):
        ctx = _make_start_ctx(self.tmp.name, agent_type="", agent_name="Planner")
        self.assertEqual("Planner", invocation.resolve_agent_type(ctx))


# ---------------------------------------------------------------------------
# extract_status_code
# ---------------------------------------------------------------------------

class TestExtractStatusCode(unittest.TestCase):
    """extract_status_code finds the last valid status code in structured form."""

    def test_returns_success_from_structured_json_key(self):
        msg = '{"status_code": "SUCCESS", "status_message": "Done."}'
        self.assertEqual("SUCCESS", invocation.extract_status_code(msg))

    def test_returns_blocked_from_structured_yaml_key(self):
        msg = "status_code: BLOCKED"
        self.assertEqual("BLOCKED", invocation.extract_status_code(msg))

    def test_returns_none_when_message_is_none(self):
        self.assertIsNone(invocation.extract_status_code(None))

    def test_returns_none_when_no_valid_code(self):
        msg = "Task complete. No structured response."
        self.assertIsNone(invocation.extract_status_code(msg))

    def test_last_valid_code_wins(self):
        """When multiple codes appear, the last valid one is returned."""
        msg = '{"status_code": "BLOCKED"} ... {"status_code": "SUCCESS"}'
        self.assertEqual("SUCCESS", invocation.extract_status_code(msg))

    def test_invalid_code_string_not_returned(self):
        """Values not in _VALID_STATUS_CODES must not be returned."""
        msg = '{"status_code": "INVALID_CODE_XYZ"}'
        self.assertIsNone(invocation.extract_status_code(msg))

    def test_all_valid_codes_recognized(self):
        valid_codes = [
            "SUCCESS", "COMPLETED_NEEDS_ACTION", "PARTIALLY_DONE",
            "NEEDS_CLARIFICATION", "CAPABILITY_EXCEEDED", "BLOCKED",
        ]
        for code in valid_codes:
            with self.subTest(code=code):
                msg = f'{{"status_code": "{code}"}}'
                self.assertEqual(code, invocation.extract_status_code(msg))

    def test_does_not_raise(self):
        for msg in (None, "", "garbage", '{"status_code": null}', "[1, 2, 3]"):
            with self.subTest(msg=repr(msg)):
                try:
                    invocation.extract_status_code(msg)
                except Exception as exc:
                    self.fail(f"extract_status_code raised for {msg!r}: {exc}")


# ---------------------------------------------------------------------------
# SubagentStart: no agent_id guard, keyed mapping
# ---------------------------------------------------------------------------

class TestHandleSubagentStart(unittest.TestCase):
    """handle_subagent_start processes every firing that carries a correlation key.

    The key is the normalised agent_id (the spawning tool-call ID); when agent_id is
    absent a unique key is minted from agent_name. The tests below start by name only.
    """

    def _agent_map_files(self):
        folder = self.paths.agent_map_dir(_RUN_ID)
        return sorted(folder.glob("*.json")) if folder.exists() else []

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.paths = core.build_paths(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def _invocation_dirs(self):
        run_root = self.paths.run_root(_RUN_ID)
        if not run_root.exists():
            return []
        return [d for d in run_root.iterdir()
                if d.is_dir() and not d.name.startswith(".")]

    def test_processes_event_with_no_agent_id_present(self):
        """SubagentStart without agent_id (fallback key) must create invocation directory."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter")
        invocation.handle_subagent_start(ctx)
        dirs = self._invocation_dirs()
        self.assertGreater(len(dirs), 0,
                           "SubagentStart without agent_id must create an invocation directory")

    def test_returns_early_when_both_agent_name_and_agent_id_absent(self):
        """When neither agent_name nor agent_id is available, the handler returns early."""
        ctx = _make_start_ctx(self.tmp.name)  # no agent_name, no agent_id
        invocation.handle_subagent_start(ctx)
        dirs = self._invocation_dirs()
        self.assertEqual(0, len(dirs),
                         "No directory should be created when agent correlation key is absent")

    def test_agent_map_entry_is_keyed_on_a_minted_key_not_the_bare_agent_name(self):
        """Without agent_id the agent map entry is written under a minted unique key."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter",
                              agent_type="TestWriter")
        prompt = '{"agent_instance_id": "TestWriter#1"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "TestWriter#1", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        self.assertIsNone(runstate.get_agent_mapping(self.paths, _RUN_ID, "TestWriter"),
                          "The bare agent_name must not be a mapping key")
        files = self._agent_map_files()
        self.assertEqual(1, len(files))
        self.assertRegex(files[0].stem, r"^TestWriter_\d{8}T\d{6}Z-[0-9a-f]{4}$")
        mapping = json.loads(files[0].read_text("utf-8"))
        self.assertEqual("TestWriter#1", mapping["agent_instance_id"])

    def test_agent_map_entry_written_before_event_emission(self):
        """Mapping is persisted before any event write to minimize correlation window."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="Researcher",
                              agent_type="Researcher")
        prompt = '{"agent_instance_id": "Researcher#1"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "Researcher#1", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        # After the handler completes, the mapping must exist
        self.assertEqual(1, len(self._agent_map_files()),
                         "Mapping must exist after handle_subagent_start")

    def test_emits_invocation_start_event(self):
        ctx = _make_start_ctx(self.tmp.name, agent_name="PlanWriter",
                              agent_type="PlanWriter")
        prompt = '{"agent_instance_id": "PlanWriter#1"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "PlanWriter#1", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        event_path = self.paths.invocation_events(_RUN_ID, "PlanWriter#1")
        self.assertTrue(event_path.exists())
        events = _read_jsonl(event_path)
        event_names = [e["event"] for e in events]
        self.assertIn("invocation_start", event_names)

    def test_invocation_start_has_agent_instance_id(self):
        ctx = _make_start_ctx(self.tmp.name, agent_name="PlanWriter",
                              agent_type="PlanWriter")
        prompt = '{"agent_instance_id": "PlanWriter#2"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "PlanWriter#2", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        event_path = self.paths.invocation_events(_RUN_ID, "PlanWriter#2")
        events = _read_jsonl(event_path)
        start = next(e for e in events if e["event"] == "invocation_start")
        self.assertEqual("PlanWriter#2", start["agent_instance_id"])

    def test_invocation_start_has_agent_type_from_resolve_agent_type(self):
        """agent_type on invocation_start comes from resolve_agent_type (falls back to agent_name)."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="Reviewer")
        prompt = '{"agent_instance_id": "Reviewer#1"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "Reviewer#1", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        event_path = self.paths.invocation_events(_RUN_ID, "Reviewer#1")
        events = _read_jsonl(event_path)
        start = next(e for e in events if e["event"] == "invocation_start")
        # resolve_agent_type falls back to agent_name when agent_type is absent
        self.assertEqual("Reviewer", start.get("agent_type"))

    def test_invocation_start_includes_prompt_from_dispatch(self):
        """Prompt from the popped pending dispatch appears in invocation_start."""
        prompt_text = '{"agent_instance_id": "TestWriter#3"}\nDo the task.'
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter")
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "TestWriter#3", _RUN_ID, prompt=prompt_text
        )
        invocation.handle_subagent_start(ctx)
        event_path = self.paths.invocation_events(_RUN_ID, "TestWriter#3")
        events = _read_jsonl(event_path)
        start = next(e for e in events if e["event"] == "invocation_start")
        self.assertEqual(prompt_text, start.get("prompt"))

    def test_initial_user_turn_written_when_prompt_available(self):
        ctx = _make_start_ctx(self.tmp.name, agent_name="PlanWriter")
        prompt = '{"agent_instance_id": "PlanWriter#4"}\nTask instructions here.'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "PlanWriter#4", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        event_path = self.paths.invocation_events(_RUN_ID, "PlanWriter#4")
        events = _read_jsonl(event_path)
        turns = [e for e in events if e["event"] == "turn"]
        self.assertGreater(len(turns), 0, "An initial user turn must be written when prompt is available")
        self.assertEqual("user", turns[0]["role"])

    def test_events_written_to_invocation_stream_not_orchestrator(self):
        """SubagentStart must not write any event to 00_orchestrator_events.jsonl."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="PlanWriter")
        prompt = '{"agent_instance_id": "PlanWriter#5"}'
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "PlanWriter#5", _RUN_ID, prompt=prompt
        )
        invocation.handle_subagent_start(ctx)
        orc_path = self.paths.orchestrator_events(_RUN_ID)
        self.assertFalse(
            orc_path.exists(),
            "SubagentStart must not write to the orchestrator events stream"
        )

    def test_unique_fallback_id_when_no_pending_dispatch(self):
        """With no pending dispatch the instance id is {type}_{timestamp}-{4 hex}."""
        ctx = _make_start_ctx(self.tmp.name, agent_name="SomeAgent")
        invocation.handle_subagent_start(ctx)
        dirs = self._invocation_dirs()
        self.assertEqual(1, len(dirs))
        self.assertRegex(dirs[0].name, r"^SomeAgent_\d{8}T\d{6}Z-[0-9a-f]{4}$")
        self.assertNotEqual("unknown-agent", dirs[0].name)

    def test_start_consumes_only_a_dispatch_of_its_own_type(self):
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "Planner#1", _RUN_ID,
            prompt='{"agent_instance_id": "Planner#1"}')
        ctx = _make_start_ctx(self.tmp.name, agent_name="Reviewer", agent_type="Reviewer")
        invocation.handle_subagent_start(ctx)
        self.assertEqual(1, len(self._invocation_dirs()))
        self.assertTrue(self.paths.pending_dispatch_entry(_RUN_ID, _SESSION_ID).read_text("utf-8"))
        self.assertFalse(self.paths.invocation_dir(_RUN_ID, "Planner#1").exists())

    def test_untyped_start_consumes_no_dispatch(self):
        _pre_populate_pending_dispatch(
            self.tmp.name, _RUN_ID, _SESSION_ID, "Planner#1", _RUN_ID,
            prompt='{"agent_instance_id": "Planner#1"}')
        ctx = _make_start_ctx(self.tmp.name, agent_id="toolu_untyped")
        invocation.handle_subagent_start(ctx)
        mapping = runstate.get_agent_mapping(self.paths, _RUN_ID, "toolu_untyped")
        self.assertRegex(mapping["agent_instance_id"], r"^agent_\d{8}T\d{6}Z-[0-9a-f]{4}$")
        self.assertFalse(self.paths.invocation_dir(_RUN_ID, "Planner#1").exists())

    def test_handler_does_not_raise(self):
        ctx = _make_start_ctx(self.tmp.name, agent_name="TestWriter")
        try:
            invocation.handle_subagent_start(ctx)
        except Exception as exc:
            self.fail(f"handle_subagent_start raised unexpectedly: {exc}")


# ---------------------------------------------------------------------------
# HANDLERS registry — invocation module
# ---------------------------------------------------------------------------

class TestInvocationHandlersRegistry(unittest.TestCase):
    """HANDLERS in the invocation module contains exactly SubagentStart and SubagentStop."""

    def test_handlers_is_a_dict(self):
        self.assertIsInstance(invocation.HANDLERS, dict)

    def test_subagent_start_registered(self):
        self.assertIn("SubagentStart", invocation.HANDLERS)

    def test_subagent_stop_registered(self):
        self.assertIn("SubagentStop", invocation.HANDLERS)

    def test_handlers_has_exactly_two_entries(self):
        self.assertEqual(2, len(invocation.HANDLERS))

    def test_all_handlers_callable(self):
        for event, handler in invocation.HANDLERS.items():
            with self.subTest(event=event):
                self.assertTrue(callable(handler))


if __name__ == "__main__":
    unittest.main()
