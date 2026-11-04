"""mosaic_logger_spawn_key.py -- spawn_call_id capture for native-mode GHCP CLI subagents.

invocation_start.spawn_call_id is the ID of the tool call that spawned the subagent.
`spawn_call_id_from` is the single place that names the payload fields the ID is read
from; adjust it there if a live capture shows other field names. The ID found on the
`task`/`agent` preToolUse payload is remembered with the pending dispatch's
agent_instance_id and taken back at subagentStart. No ID is ever fabricated, and the
value is logged as the harness supplied it (no normalisation).
"""

import datetime
import json
import os
import sys

import mosaic_logger_core as core

_MAX_AGE_SECONDS = 120


def _clean(value) -> "str | None":
    return value if isinstance(value, str) and value else None


def spawn_call_id_from(ctx: "core.HookContext") -> "str | None":
    """Return the tool-call ID in the payload, or None. First present source wins:
    toolCallId, tool_call_id, toolUseId, toolCalls[0].id."""
    payload = ctx.payload
    for key in ("toolCallId", "tool_call_id", "toolUseId"):
        found = _clean(payload.get(key))
        if found:
            return found
    calls = payload.get("toolCalls")
    if isinstance(calls, list) and calls and isinstance(calls[0], dict):
        return _clean(calls[0].get("id"))
    return None


def _entry_path(paths: "core.LogPaths", path_run_id: str, session_id: str):
    name = f"{core.sanitize_component(session_id)}.spawn-calls.jsonl"
    return paths.pending_dispatch_dir(path_run_id) / name


def remember_spawn_call(paths: "core.LogPaths", path_run_id: str, session_id: str,
                        agent_instance_id: str, spawn_call_id: str) -> bool:
    """Append (agent_instance_id, spawn_call_id) for the session. Never raises."""
    try:
        if not (session_id and agent_instance_id and spawn_call_id):
            return False
        entry = {
            "agent_instance_id": agent_instance_id,
            "spawn_call_id": spawn_call_id,
            "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        }
        path = _entry_path(paths, path_run_id, session_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        data = (json.dumps(entry, ensure_ascii=False) + "\n").encode("utf-8")
        fd = os.open(str(path), os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o666)
        try:
            os.write(fd, data)
        finally:
            os.close(fd)
        return True
    except Exception:
        return False


def take_spawn_call(paths: "core.LogPaths", path_run_id: str, session_id: str,
                    agent_instance_id: str) -> "str | None":
    """Return the most recent non-stale remembered ID for the agent instance, else None.
    Never raises."""
    try:
        path = _entry_path(paths, path_run_id, session_id)
        if not path.exists():
            return None
        now = datetime.datetime.now(datetime.timezone.utc)
        found = None
        for line in path.read_text(encoding="utf-8").splitlines():
            try:
                entry = json.loads(line)
                if entry.get("agent_instance_id") != agent_instance_id:
                    continue
                age = (now - datetime.datetime.fromisoformat(entry["created_at"])).total_seconds()
                if age <= _MAX_AGE_SECONDS:
                    found = _clean(entry.get("spawn_call_id")) or found
            except Exception:
                continue
        return found
    except Exception:
        return None
