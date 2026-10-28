"""mosaic_logger_subagent_state.py — Workspace-level subagent spawn index and claims.

SubagentStart records, per correlation key, the run, invocation and session of the
subagent. Later firings (SubagentStop, the orchestrator's PostToolUse for the spawning
tool, the Stop sweep) run in separate hook processes and find the invocation through
this index, since they carry no prompt text. The index lives under a dot-prefixed folder
of OrchestrationLogs/, which log readers skip.

Per key, three files: <key>.json (the index entry), <key>.stopped (SubagentStop seen)
and <key>.claimed (the completion was claimed; created exclusively so that only one
firing writes the invocation_end).
"""

import json
import os
import re

import mosaic_logger_core as core

_VSCODE_SUFFIX_RE = re.compile(r"__vscode-\d+$")


def normalize_call_id(value) -> "str | None":
    """Strip a trailing '__vscode-<digits>' suffix; None for absent/empty."""
    if not isinstance(value, str) or not value:
        return None
    return _VSCODE_SUFFIX_RE.sub("", value) or None


def put_spawn_entry(paths: "core.LogPaths", key: str, run_id: str,
                    agent_instance_id: str, session_id: "str | None",
                    agent_type: "str | None") -> bool:
    """Persist the index entry for key. Never raises."""
    try:
        data = {"key": key, "run_id": run_id, "agent_instance_id": agent_instance_id}
        if session_id:
            data["session_id"] = session_id
        if agent_type:
            data["agent_type"] = agent_type
        path = paths.subagent_index_file(key, "json")
        path.parent.mkdir(parents=True, exist_ok=True)
        return core.atomic_replace(path, json.dumps(data, ensure_ascii=False).encode("utf-8"))
    except Exception:
        return False


def get_spawn_entry(paths: "core.LogPaths", key: str) -> "dict | None":
    """Read the index entry for key; None when absent or unreadable. Never raises."""
    try:
        data = json.loads(paths.subagent_index_file(key, "json").read_text("utf-8"))
        if isinstance(data, dict) and data.get("agent_instance_id"):
            return data
        return None
    except Exception:
        return None


def _create_exclusive(path) -> bool:
    """Create path exclusively; True only for the caller that created it."""
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        os.close(os.open(str(path), os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o666))
        return True
    except Exception:
        return False


def mark_stopped(paths: "core.LogPaths", key: str) -> bool:
    """Record that SubagentStop was seen for key. Idempotent. Never raises."""
    marker = paths.subagent_index_file(key, "stopped")
    return marker.exists() or _create_exclusive(marker)


def claim_completion(paths: "core.LogPaths", key: str) -> bool:
    """True for exactly one caller per key: the one that may write the invocation_end."""
    return _create_exclusive(paths.subagent_index_file(key, "claimed"))


def stopped_unclaimed(paths: "core.LogPaths", session_id: str) -> list:
    """Index entries of session_id that were marked stopped and are not yet claimed."""
    found = []
    try:
        for entry_file in sorted(paths.subagent_index_dir().glob("*.json")):
            entry = _read_entry_file(entry_file)
            if not entry or entry.get("session_id") != session_id:
                continue
            key = entry.get("key")
            if (key and paths.subagent_index_file(key, "stopped").exists()
                    and not paths.subagent_index_file(key, "claimed").exists()):
                found.append(entry)
    except Exception:
        pass
    return found


def _read_entry_file(path) -> "dict | None":
    try:
        data = json.loads(path.read_text("utf-8"))
        return data if isinstance(data, dict) else None
    except Exception:
        return None


# ---------------------------------------------------------------------------
# Compaction routing
# ---------------------------------------------------------------------------

# A compaction belongs to the stream of the actor that was compacted. The subagent
# is identified only from what the hook payload or the adapter state actually
# carries: the payload's agent_id (the spawning tool-call ID, looked up in the
# spawn index), or a payload session_id that the spawn index maps to a subagent
# and that is not an orchestrator session. A subagent merely running is never
# enough. Anything else is the orchestrator's own compaction.

_UNKNOWN_RUN = "unknown-run"


def _read_entries(paths: "core.LogPaths"):
    try:
        files = sorted(paths.subagent_index_dir().glob("*.json"))
    except Exception:
        return
    for entry_file in files:
        try:
            data = json.loads(entry_file.read_text("utf-8"))
        except Exception:
            continue
        if isinstance(data, dict) and data.get("agent_instance_id") and data.get("run_id"):
            yield data


def _is_orchestrator_session(paths: "core.LogPaths", run_id: str, session_id: str) -> bool:
    """True when a session_start for session_id sits in the run's orchestrator stream."""
    for rid in (run_id, _UNKNOWN_RUN):
        try:
            text = paths.orchestrator_events(rid).read_text("utf-8")
        except Exception:
            continue
        for line in text.splitlines():
            try:
                event = json.loads(line)
            except Exception:
                continue
            if (isinstance(event, dict) and event.get("event") == "session_start"
                    and event.get("session_id") == session_id):
                return True
    return False


def _entry_for_session(ctx: "core.HookContext") -> "dict | None":
    if not ctx.session_id:
        return None
    for entry in _read_entries(ctx.paths):
        if entry.get("session_id") != ctx.session_id:
            continue
        if _is_orchestrator_session(ctx.paths, entry["run_id"], ctx.session_id):
            return None
        return entry
    return None


def compaction_destination(ctx: "core.HookContext"):
    """Return the events file a compaction firing belongs to. Never raises."""
    try:
        key = normalize_call_id(ctx.agent_id)
        entry = get_spawn_entry(ctx.paths, key) if key else None
        if entry is None:
            entry = _entry_for_session(ctx)
        if entry is not None and entry.get("run_id"):
            return ctx.paths.invocation_events(entry["run_id"], entry["agent_instance_id"])
    except Exception as exc:
        core.debug_log("compaction routing failed", exc)
    return ctx.paths.orchestrator_events(core.effective_run_id(ctx))
