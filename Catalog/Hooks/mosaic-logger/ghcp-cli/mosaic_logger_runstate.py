"""mosaic_logger_runstate.py -- Run identity extraction and agent mapping store.

Owns: run_id and agent_instance_id extraction from dispatch content,
agent_id -> agent_instance_id mapping files, pending dispatch queue.
"""

import datetime
import json
import random
import re

import mosaic_logger_core as core
import mosaic_logger_pending_dispatch as pending_dispatch


# ---------------------------------------------------------------------------
# Identifier generation helpers
# ---------------------------------------------------------------------------

def _random_suffix() -> str:
    return "".join(random.choices("0123456789abcdef", k=4))


def _compact_ts(dt: datetime.datetime) -> str:
    """Compact UTC timestamp: YYYYMMDDTHHMMSSz."""
    return dt.strftime("%Y%m%dT%H%M%SZ")


def _iso_ms(dt: datetime.datetime) -> str:
    """ISO 8601 UTC with ms precision and Z suffix."""
    ms = dt.microsecond // 1000
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ms:03d}Z"


def _utcnow(now: "datetime.datetime | None") -> datetime.datetime:
    if now is None:
        return datetime.datetime.now(datetime.timezone.utc)
    return now


# ---------------------------------------------------------------------------
# Identifier minting
# ---------------------------------------------------------------------------

def new_run_id(now: "datetime.datetime | None" = None) -> str:
    """Return {YYYYMMDD}T{HHMMSS}Z-{4 lowercase hex chars}."""
    return f"{_compact_ts(_utcnow(now))}-{_random_suffix()}"


def fallback_instance_id(prefix: "str | None",
                         now: "datetime.datetime | None" = None) -> str:
    """Return {prefix}_{YYYYMMDD}T{HHMMSS}Z-{4 hex chars}.
    Uses literal 'agent' when prefix is None."""
    p = prefix if prefix else "agent"
    return f"{p}_{_compact_ts(_utcnow(now))}-{_random_suffix()}"


def fallback_call_id(tool_name: "str | None",
                     now: "datetime.datetime | None" = None) -> str:
    """Return {tool_name or 'tool'}_{YYYYMMDD}T{HHMMSS}Z-{4 hex chars}."""
    p = tool_name if tool_name else "tool"
    return f"{p}_{_compact_ts(_utcnow(now))}-{_random_suffix()}"


# ---------------------------------------------------------------------------
# agent_instance_id extraction
# ---------------------------------------------------------------------------

# Structured match: agent_instance_id key (with optional quotes/colon) followed
# by a quoted or bare {Name}#{Number} value.
_STRUCTURED_RE = re.compile(
    r'(?<![A-Za-z0-9_])agent_instance_id\s*["\']?\s*[:=]\s*["\']?\s*'
    r'([A-Za-z][A-Za-z0-9_.-]*#[0-9]+)'
)


def extract_instance_id(prompt: "str | None") -> "str | None":
    """Extract a MOSAIC {AgentName}#{Number} instance ID from dispatched prompt text.

    Only an explicit agent_instance_id key followed by the value counts; a bare
    {Name}#{Number} elsewhere in the text is never used. Returns None otherwise.
    Never raises.
    """
    if not prompt:
        return None
    try:
        m = _STRUCTURED_RE.search(prompt)
        return m.group(1) if m else None
    except Exception:
        return None


# ---------------------------------------------------------------------------
# run_id extraction
# ---------------------------------------------------------------------------

# Structured match: run_id key followed by a value matching the format.
_RUN_ID_STRUCTURED_RE = re.compile(
    r'(?<![A-Za-z0-9_])run_id\s*["\']?\s*[:=]\s*["\']?\s*'
    r'([0-9]{8}T[0-9]{6}Z-[0-9a-f]{4})'
)

# Bare match: {YYYYMMDD}T{HHMMSS}Z-{4hex}
_RUN_ID_BARE_RE = re.compile(
    r'(?<![A-Za-z0-9_.-])([0-9]{8}T[0-9]{6}Z-[0-9a-f]{4})'
)


def extract_run_id(prompt: "str | None") -> "str | None":
    """Extract a MOSAIC run_id from text.

    Matching order (stops at first hit):
      1. Structured: run_id key followed by a value matching the format.
      2. Bare: earliest occurrence of the format pattern in the text.
    Returns None when no confident match exists. Never raises.
    """
    if not prompt:
        return None
    try:
        m = _RUN_ID_STRUCTURED_RE.search(prompt)
        if m:
            return m.group(1)
        m = _RUN_ID_BARE_RE.search(prompt)
        if m:
            return m.group(1)
        return None
    except Exception:
        return None


# ---------------------------------------------------------------------------
# Session run-id cache
# ---------------------------------------------------------------------------

SESSION_RUN_ID_MAX_AGE_SECONDS = 86400


def put_session_run_id(paths: "core.LogPaths",
                       session_id: "str | None",
                       run_id: "str | None",
                       created_at: "str | None" = None) -> bool:
    """Persist a session_id -> run_id mapping via atomic replace.

    Latest-wins: unconditionally replaces any existing record for this session id.
    Creates the parent directory when missing.
    Returns True on success, False when session_id or run_id is falsy or on any
    failure. Never raises.

    created_at: optional ISO-8601 UTC timestamp with ms precision and Z suffix for
    the record. When absent, defaults to the current UTC time. Callers may supply
    a hook-firing timestamp so that the cache entry's staleness is evaluated relative
    to the hook that observed the run_id (not the wall-clock time of the write).
    """
    try:
        if not session_id or not run_id:
            return False
        if created_at is None:
            now = datetime.datetime.now(datetime.timezone.utc)
            created_at = _iso_ms(now)
        data = {
            "session_id": session_id,
            "run_id": run_id,
            "created_at": created_at,
        }
        path = paths.session_run_id_entry(session_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        return core.atomic_replace(
            path, json.dumps(data, ensure_ascii=False).encode("utf-8")
        )
    except Exception:
        return False


def get_session_run_id(paths: "core.LogPaths",
                       session_id: "str | None",
                       max_age_seconds: int = SESSION_RUN_ID_MAX_AGE_SECONDS
                       ) -> "str | None":
    """Return the cached run_id for session_id, or None on any miss or staleness.

    Returns None when: session_id is falsy, the file is absent, empty, or
    malformed, the record is not a dict, the run_id field is missing or empty,
    or created_at is older than max_age_seconds. Never raises. Never deletes
    the file on a stale read.
    """
    try:
        if not session_id:
            return None
        path = paths.session_run_id_entry(session_id)
        try:
            text = path.read_text(encoding="utf-8")
        except Exception:
            return None
        if not text.strip():
            return None
        try:
            data = json.loads(text)
        except Exception:
            return None
        if not isinstance(data, dict):
            return None
        run_id = data.get("run_id")
        if not run_id:
            return None
        created_at = data.get("created_at", "")
        try:
            ts = datetime.datetime.fromisoformat(created_at.replace("Z", "+00:00"))
            now = datetime.datetime.now(datetime.timezone.utc)
            age_seconds = (now - ts).total_seconds()
            if age_seconds > max_age_seconds:
                return None
        except Exception:
            return None
        return run_id
    except Exception:
        return None


# ---------------------------------------------------------------------------
# Agent-to-run association (workspace-level)
# ---------------------------------------------------------------------------

AGENT_RUN_MAX_AGE_SECONDS = 86400


def put_agent_run(paths: "core.LogPaths",
                  agent_id: "str | None",
                  run_id: "str | None") -> bool:
    """Persist an agent_id -> run_id association via atomic replace.

    Latest-wins: unconditionally replaces any existing record for this agent id.
    Creates the parent directory when missing.
    Returns True on success, False when agent_id or run_id is falsy or on any
    failure. Never raises.
    """
    try:
        if not agent_id or not run_id:
            return False
        now = datetime.datetime.now(datetime.timezone.utc)
        data = {
            "agent_id": agent_id,
            "run_id": run_id,
            "created_at": _iso_ms(now),
        }
        path = paths.agent_run_entry(agent_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        return core.atomic_replace(
            path, json.dumps(data, ensure_ascii=False).encode("utf-8")
        )
    except Exception:
        return False


def get_agent_run(paths: "core.LogPaths",
                  agent_id: "str | None",
                  max_age_seconds: int = AGENT_RUN_MAX_AGE_SECONDS
                  ) -> "dict | None":
    """Return the parsed AgentRunRecord dict for agent_id, or None on any miss or staleness.

    Returns None when: agent_id is falsy, the file is absent, empty, or malformed,
    the record is not a dict, or created_at is older than max_age_seconds.
    Never raises. Never deletes the file on a stale read.
    """
    try:
        if not agent_id:
            return None
        path = paths.agent_run_entry(agent_id)
        try:
            text = path.read_text(encoding="utf-8")
        except Exception:
            return None
        if not text.strip():
            return None
        try:
            data = json.loads(text)
        except Exception:
            return None
        if not isinstance(data, dict):
            return None
        created_at = data.get("created_at", "")
        try:
            ts = datetime.datetime.fromisoformat(created_at.replace("Z", "+00:00"))
            now = datetime.datetime.now(datetime.timezone.utc)
            age_seconds = (now - ts).total_seconds()
            if age_seconds > max_age_seconds:
                return None
        except Exception:
            return None
        return data
    except Exception:
        return None


def resolve_run_for_agent(paths: "core.LogPaths",
                          agent_id: "str | None",
                          max_age_seconds: int = AGENT_RUN_MAX_AGE_SECONDS
                          ) -> "str | None":
    """Return the run_id from the agent-to-run association, or None on any miss.

    Unlike resolve_invocation_id, returns None on a miss rather than a fabricated
    fallback — the caller decides the fallback because 'no association' and
    'association to run X' require different routing. Never raises.
    """
    try:
        record = get_agent_run(paths, agent_id, max_age_seconds=max_age_seconds)
        if record is None:
            return None
        run_id = record.get("run_id")
        if not run_id:
            return None
        return run_id
    except Exception:
        return None


# ---------------------------------------------------------------------------
# Pending-dispatch state management
# ---------------------------------------------------------------------------

PENDING_DISPATCH_MAX_AGE_SECONDS = pending_dispatch.PENDING_DISPATCH_MAX_AGE_SECONDS


def put_pending_dispatch(
    paths: "core.LogPaths",
    path_run_id: str,
    session_id: str,
    agent_instance_id: str,
    extracted_run_id: "str | None",
    prompt: "str | None" = None,
    agent_type: "str | None" = None,
) -> bool:
    """Queue a pending dispatch; delegates to mosaic_logger_pending_dispatch."""
    return pending_dispatch.put_pending_dispatch(
        paths, path_run_id, session_id, agent_instance_id, extracted_run_id,
        prompt=prompt, agent_type=agent_type,
    )


# ---------------------------------------------------------------------------
# agent_id -> agent_instance_id mapping
# ---------------------------------------------------------------------------

def put_agent_mapping(paths: "core.LogPaths",
                      run_id: str,
                      agent_id: str,
                      agent_instance_id: str,
                      agent_type: "str | None") -> bool:
    """Persist the agent_id -> agent_instance_id mapping via atomic replace.
    Returns True on success. Never raises."""
    try:
        now = datetime.datetime.now(datetime.timezone.utc)
        data = {
            "agent_id": agent_id,
            "agent_instance_id": agent_instance_id,
            "agent_type": agent_type,
            "created_at": _iso_ms(now),
        }
        path = paths.agent_map_entry(run_id, agent_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        return core.atomic_replace(
            path, json.dumps(data, ensure_ascii=False).encode("utf-8")
        )
    except Exception:
        return False


def get_agent_mapping(paths: "core.LogPaths",
                      run_id: str,
                      agent_id: str) -> "dict | None":
    """Read one mapping. Returns None when absent, unreadable, or malformed.
    Never raises."""
    try:
        path = paths.agent_map_entry(run_id, agent_id)
        text = path.read_text(encoding="utf-8")
        data = json.loads(text)
        if isinstance(data, dict):
            return data
        return None
    except Exception:
        return None


def resolve_invocation_id(paths: "core.LogPaths",
                          run_id: str,
                          agent_id: str) -> str:
    """Return the agent_instance_id for routing a subagent-scoped event.

    Falls back to the deterministic 'unmapped_{agent_id}' string when no
    mapping exists. Never returns None.
    """
    mapping = get_agent_mapping(paths, run_id, agent_id)
    if mapping is not None:
        iid = mapping.get("agent_instance_id")
        if iid:
            return iid
    return f"unmapped_{agent_id}"
