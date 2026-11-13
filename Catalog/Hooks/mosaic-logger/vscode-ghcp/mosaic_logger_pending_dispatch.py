"""mosaic_logger_pending_dispatch.py - the per-session pending-dispatch queue.

Owns: appending a captured dispatch (put) and consuming the oldest fresh dispatch of a
given agent type (take). Both run under the same sidecar lock on the queue file, so a take's
read-select-rewrite and every append are mutually exclusive, also across processes.

The type of a dispatch is read from the prefix before '#' of its agent_instance_id (the
MOSAIC {AgentType}#{Number} form), because the dispatch tool's type key is not verified;
an explicitly stored agent_type wins when present.
"""

import collections
import datetime
import json
import re

import mosaic_logger_core as core
import mosaic_logger_locked_append as locked_append

PENDING_DISPATCH_MAX_AGE_SECONDS = 120

PendingDispatch = collections.namedtuple(
    "PendingDispatch", ["agent_instance_id", "run_id", "prompt"])

_NON_ALNUM_RE = re.compile(r"[\W_]+", re.UNICODE)


def _iso_ms(now: datetime.datetime) -> str:
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


def dispatch_type_of(agent_instance_id: "str | None") -> "str | None":
    """The part of agent_instance_id before the first '#', or None. Never raises."""
    try:
        if not isinstance(agent_instance_id, str) or "#" not in agent_instance_id:
            return None
        head = agent_instance_id.split("#", 1)[0]
        return head or None
    except Exception:
        return None


def _normalize_type(value: "str | None") -> str:
    if not isinstance(value, str):
        return ""
    return _NON_ALNUM_RE.sub("", value.casefold())


def agent_types_match(a: "str | None", b: "str | None") -> bool:
    """True when both types are non-empty and equal ignoring case and punctuation."""
    try:
        na, nb = _normalize_type(a), _normalize_type(b)
        return bool(na) and na == nb
    except Exception:
        return False


def put_pending_dispatch(
    paths: "core.LogPaths",
    path_run_id: str,
    session_id: str,
    agent_instance_id: str,
    extracted_run_id: "str | None",
    prompt: "str | None" = None,
    agent_type: "str | None" = None,
) -> bool:
    """Append one dispatch line to the session queue under the queue lock.

    Returns True when written; False when the lock was not obtained in time or on any
    I/O error. Never raises.
    """
    try:
        entry = {
            "agent_instance_id": agent_instance_id,
            "run_id": extracted_run_id,
            "created_at": _iso_ms(datetime.datetime.now(datetime.timezone.utc)),
        }
        if isinstance(agent_type, str) and agent_type:
            entry["agent_type"] = agent_type
        if prompt is not None:
            entry["prompt"] = prompt
        data = (json.dumps(entry, ensure_ascii=False) + "\n").encode("utf-8")
        return locked_append.locked_append(
            paths.pending_dispatch_entry(path_run_id, session_id), data, core.debug_log)
    except Exception as exc:
        core.debug_log("pending-dispatch: put failed", exc)
        return False


def _is_fresh(entry: dict, now: datetime.datetime, max_age_seconds: float) -> bool:
    try:
        created = datetime.datetime.fromisoformat(entry.get("created_at", "").replace("Z", "+00:00"))
        return (now - created).total_seconds() <= max_age_seconds
    except Exception:
        return False


def _parse_valid(line: str) -> "dict | None":
    try:
        entry = json.loads(line)
    except ValueError:
        return None
    if not isinstance(entry, dict):
        return None
    instance = entry.get("agent_instance_id")
    if not isinstance(instance, str) or not instance:
        return None
    return entry


def _entry_type(entry: dict) -> "str | None":
    stored = entry.get("agent_type")
    if isinstance(stored, str) and stored:
        return stored
    return dispatch_type_of(entry["agent_instance_id"])


def _write_queue(path, lines: "list[str]") -> bool:
    """Replace the queue content; atomic first, in place when the atomic replace fails."""
    data = "".join(ln + "\n" for ln in lines).encode("utf-8")
    if core.atomic_replace(path, data):
        return True
    try:
        with open(path, "wb") as handle:
            handle.write(data)
        return True
    except OSError as exc:
        core.debug_log("pending-dispatch: queue rewrite failed", exc)
        return False


def take_pending_dispatch(
    paths: "core.LogPaths",
    path_run_id: str,
    session_id: str,
    agent_type: "str | None",
    max_age_seconds: int = PENDING_DISPATCH_MAX_AGE_SECONDS,
) -> "PendingDispatch | None":
    """Consume and return the oldest fresh dispatch whose type matches agent_type.

    Returns None for an empty/absent type (queue untouched), when no fresh matching
    entry exists, when the lock was not obtained, or when the consumption could not be
    made durable. Stale and corrupt lines are dropped on rewrite. Never raises.
    """
    try:
        if not isinstance(agent_type, str) or not agent_type:
            return None
        path = paths.pending_dispatch_entry(path_run_id, session_id)
        if not path.exists():
            return None
        with locked_append.holding_lock(path, core.debug_log) as held:
            if not held:
                return None
            return _take_locked(path, agent_type, max_age_seconds)
    except Exception as exc:
        core.debug_log("pending-dispatch: take failed", exc)
        return None


def _take_locked(path, agent_type: str, max_age_seconds: float) -> "PendingDispatch | None":
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, ValueError):
        return None
    now = datetime.datetime.now(datetime.timezone.utc)
    all_lines = [ln for ln in text.splitlines() if ln.strip()]
    kept = []  # (raw line, entry) of fresh valid entries, in order
    for raw in all_lines:
        entry = _parse_valid(raw)
        if entry is not None and _is_fresh(entry, now, max_age_seconds):
            kept.append((raw, entry))
    chosen = next((i for i, (_, e) in enumerate(kept)
                   if agent_types_match(_entry_type(e), agent_type)), None)
    remaining = [raw for i, (raw, _) in enumerate(kept) if i != chosen]
    if chosen is None:
        if len(kept) != len(all_lines):
            _write_queue(path, remaining)
        return None
    if not _write_queue(path, remaining):
        return None
    entry = kept[chosen][1]
    return PendingDispatch(entry["agent_instance_id"], entry.get("run_id"), entry.get("prompt"))
