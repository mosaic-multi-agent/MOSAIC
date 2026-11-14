"""Shared helpers for the SubagentStart correlation tests.

Builds harness-shaped payloads (PreToolUse Agent dispatch, SubagentStart,
SubagentStop), replays them through the hook entry point, and reads back the
resulting log tree. The same file sits next to the tests that need it (the
tests folder is not an importable package for helpers).
"""

import json
import os
import pathlib
import re
import sys
import tempfile
import unittest

_ADAPTER_DIR = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(_ADAPTER_DIR))

import mosaic_logger
import mosaic_logger_core as core

RUN_ID = "20261001T100000Z-aa11"
SESSION_ID = "correlation-session"
FALLBACK_RE = r"^{prefix}_\d{{8}}T\d{{6}}Z-[0-9a-f]{{4}}$"


def fallback_pattern(prefix):
    """Regex of the unique fallback id for a given prefix."""
    return FALLBACK_RE.format(prefix=re.escape(prefix))


def dispatch_prompt(instance, run_id=RUN_ID):
    """A dispatch prompt carrying an explicit agent_instance_id line."""
    return json.dumps({"agent_instance_id": instance, "run_id": run_id,
                       "task_description": "do the work"})


def pre_dispatch_payload(instance, subagent_type, use_id, prompt=None,
                         session_id=SESSION_ID):
    """PreToolUse payload of the Agent tool, shaped like the real harness."""
    tool_input = {"description": "dispatch", "prompt": prompt or dispatch_prompt(instance)}
    if subagent_type is not None:
        tool_input["subagent_type"] = subagent_type
    return {"hook_event_name": "PreToolUse", "session_id": session_id,
            "tool_name": "Agent", "tool_use_id": use_id, "tool_input": tool_input}


def start_payload(agent_id, agent_type, prompt=None, session_id=SESSION_ID):
    """SubagentStart payload; agent_type None leaves the key out."""
    payload = {"hook_event_name": "SubagentStart", "session_id": session_id,
               "agent_id": agent_id}
    if agent_type is not None:
        payload["agent_type"] = agent_type
    if prompt is not None:
        payload["agent_prompt"] = prompt
    return payload


def stop_payload(agent_id, agent_type, session_path, agent_transcript=None,
                 message="done", session_id=SESSION_ID):
    """SubagentStop payload."""
    payload = {"hook_event_name": "SubagentStop", "session_id": session_id,
               "agent_id": agent_id, "agent_type": agent_type,
               "last_assistant_message": message,
               "transcript_path": str(session_path)}
    if agent_transcript:
        payload["agent_transcript_path"] = str(agent_transcript)
    return payload


def assistant_record(msg_id, model="claude-opus-4-5"):
    """One assistant transcript record carrying usage."""
    return {"type": "assistant", "message": {
        "id": msg_id, "model": model,
        "usage": {"input_tokens": 120, "output_tokens": 40}}}


def write_agent_transcript(session_path, agent_id, records):
    """Write (replace) the subagent transcript next to the session transcript."""
    base = str(session_path)[:-len(".jsonl")]
    target = pathlib.Path(base) / "subagents" / f"agent-{agent_id}.jsonl"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("".join(json.dumps(r) + "\n" for r in records), encoding="utf-8")
    return target


def read_jsonl(path):
    """Parsed lines of a JSONL file, or [] when absent."""
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class CorrelationCase(unittest.TestCase):
    """A temporary workspace with the run fixed through MOSAIC_RUN_ID (native mode)."""

    bind_run = True

    def setUp(self):
        self._td = tempfile.TemporaryDirectory()
        self.addCleanup(self._td.cleanup)
        self.workspace = pathlib.Path(self._td.name) / "ws"
        self.workspace.mkdir()
        self.paths = core.build_paths(self.workspace)
        self.session_path = pathlib.Path(self._td.name) / "claude" / "sess.jsonl"
        self.session_path.parent.mkdir(parents=True)
        self.session_path.write_text("", encoding="utf-8")
        self._saved = {k: os.environ.get(k)
                       for k in ("CLAUDE_PROJECT_DIR", "MOSAIC_RUN_ID", "MOSAIC_ROLE")}
        os.environ["CLAUDE_PROJECT_DIR"] = str(self.workspace)
        os.environ.pop("MOSAIC_ROLE", None)
        if self.bind_run:
            os.environ["MOSAIC_RUN_ID"] = RUN_ID
        else:
            os.environ.pop("MOSAIC_RUN_ID", None)
        self.addCleanup(self._restore_env)

    def _restore_env(self):
        for key, value in self._saved.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value

    def replay(self, *payloads):
        """Fire each payload; a dispatch PreToolUse fires the synchronous capture
        entry (which queues the dispatch) before the catch-all entry."""
        for payload in payloads:
            if (payload.get("hook_event_name") == "PreToolUse"
                    and payload.get("tool_name") in ("Agent", "Task")):
                mosaic_logger.dispatch_capture(json.dumps(payload))
            mosaic_logger.dispatch(json.dumps(payload))

    def run_root(self, run_id=RUN_ID):
        return self.paths.run_root(run_id)

    def folders(self, run_id=RUN_ID):
        """Names of the invocation folders of a run (sidecar dot-folders excluded)."""
        root = self.run_root(run_id)
        if not root.exists():
            return []
        return sorted(d.name for d in root.iterdir()
                      if d.is_dir() and not d.name.startswith("."))

    def events(self, instance, run_id=RUN_ID):
        return read_jsonl(self.paths.invocation_events(run_id, instance))

    def names(self, instance, run_id=RUN_ID):
        return [e["event"] for e in self.events(instance, run_id)]

    def mapping_path(self, agent_id, run_id=RUN_ID):
        return self.paths.agent_map_entry(run_id, agent_id)

    def mapped_instance(self, agent_id, run_id=RUN_ID):
        path = self.mapping_path(agent_id, run_id)
        return json.loads(path.read_text("utf-8"))["agent_instance_id"] if path.exists() else None

    def queue_lines(self, session_id=SESSION_ID):
        path = self.paths.pending_dispatch_entry(session_id)
        if not path.exists():
            return []
        return [ln for ln in path.read_text("utf-8").splitlines() if ln.strip()]

    def queued_ids(self, session_id=SESSION_ID):
        return [json.loads(ln)["agent_instance_id"] for ln in self.queue_lines(session_id)]

    def usage_ids(self, instance, run_id=RUN_ID):
        return [e["record_id"] for e in self.events(instance, run_id)
                if e["event"] == "usage_record"]

    def assert_no_unknown_agent_folder(self):
        for run in [RUN_ID, "unknown-run"]:
            self.assertNotIn("unknown-agent", self.folders(run))
