"""mosaic_logger_session_run.py - Workspace-level session -> run association and relocation.

Every VS Code hook payload carries a session_id, but only prompt text names the run. Once
a prompt reveals a session's run, this module remembers it (OrchestrationLogs/
.session-run-id/<session>.json, skipped by log readers) so later firings of that session
- tool calls, SessionStart, Stop - resolve the same run. It also moves the session's
earlier lines from unknown-run/00_orchestrator_events.jsonl into the run's orchestrator
stream, serialised by an exclusive-create lock file because hook processes run
concurrently. Other modules of the variant read the association through get_session_run_id.
"""

import datetime
import json
import os
import time

import mosaic_logger_core as core
from mosaic_logger_locked_append import holding_lock, locked_append

SESSION_RUN_ID_MAX_AGE_SECONDS = 86400
UNKNOWN_RUN = "unknown-run"
_LOCK_STALE_SECONDS = 60.0
_LOCK_WAIT_SECONDS = 10.0


def put_session_run_id(paths: "core.LogPaths", session_id: "str | None",
                       run_id: "str | None", created_at: "str | None" = None) -> bool:
    """Persist session_id -> run_id (latest wins). Never raises."""
    try:
        if not session_id or not run_id:
            return False
        if created_at is None:
            created_at = core.current_timestamp()
        data = {"session_id": session_id, "run_id": run_id, "created_at": created_at}
        path = paths.session_run_id_entry(session_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        return core.atomic_replace(path, json.dumps(data, ensure_ascii=False).encode("utf-8"))
    except Exception:
        return False


def get_session_run_id(paths: "core.LogPaths", session_id: "str | None",
                       max_age_seconds: int = SESSION_RUN_ID_MAX_AGE_SECONDS
                       ) -> "str | None":
    """Return the cached run for session_id, or None on a miss, bad record or staleness."""
    try:
        if not session_id:
            return None
        data = json.loads(paths.session_run_id_entry(session_id).read_text(encoding="utf-8"))
        run_id = data.get("run_id") if isinstance(data, dict) else None
        if not run_id:
            return None
        created = datetime.datetime.fromisoformat(data.get("created_at", "").replace("Z", "+00:00"))
        age = (datetime.datetime.now(datetime.timezone.utc) - created).total_seconds()
        return run_id if age <= max_age_seconds else None
    except Exception:
        return None


def _acquire_lock(lock_path) -> bool:
    deadline = time.monotonic() + _LOCK_WAIT_SECONDS
    while True:
        try:
            fd = os.open(str(lock_path), os.O_CREAT | os.O_EXCL | os.O_WRONLY)
            os.close(fd)
            return True
        except FileExistsError:
            try:
                if time.time() - lock_path.stat().st_mtime > _LOCK_STALE_SECONDS:
                    lock_path.unlink()
                    continue
            except OSError:
                pass
            if time.monotonic() >= deadline:
                return False
            time.sleep(0.02)


def _report(message, exc):
    core.debug_log("relocation: " + message, exc)


def _split_session_lines(raw: bytes, session_id: str, run_id: str):
    moved, kept = [], []
    for line in raw.splitlines(keepends=True):
        event = None
        try:
            event = json.loads(line)
        except Exception:
            pass
        if isinstance(event, dict) and event.get("session_id") == session_id:
            event["run_id"] = run_id
            moved.append(json.dumps(event, ensure_ascii=False) + "\n")
        else:
            kept.append(line)
    return moved, kept


def _snapshot_stream(source):
    """Rename the stream to a private snapshot while holding its append lock; None if not done.

    Writers then start a fresh file, so no concurrent append is lost. When the append lock is
    not obtained in time the stream is left untouched.
    """
    snapshot = source.with_name(source.name + ".relocating-" + str(os.getpid()))
    with holding_lock(source, _report) as held:
        if not held:
            return None
        os.replace(str(source), str(snapshot))
    return snapshot


def _relocate_locked(paths: "core.LogPaths", session_id: str, run_id: str) -> None:
    """Move the session's lines without ever rewriting the file concurrent writers append to.

    The snapshot is removed only after its lines are written: the session's lines to the
    run's stream and the others back to the unknown-run stream, each under that stream's
    append lock. If the run's stream cannot be written the whole snapshot goes back to the
    unknown-run stream; if that fails too the snapshot stays in place.
    """
    source = paths.orchestrator_events(UNKNOWN_RUN)
    if not source.is_file():
        return
    snapshot = _snapshot_stream(source)
    if snapshot is None:
        return
    raw = snapshot.read_bytes()
    moved, kept = _split_session_lines(raw, session_id, run_id)
    if moved:
        target = paths.orchestrator_events(run_id)
        if not locked_append(target, "".join(moved).encode("utf-8"), _report):
            if locked_append(source, raw, _report):
                snapshot.unlink()
            else:
                core.debug_log("relocation left snapshot in place: " + str(snapshot))
            return
    if kept and not locked_append(source, b"".join(kept), _report):
        core.debug_log("relocation left snapshot in place: " + str(snapshot))
        return
    snapshot.unlink()


def relocate_unknown_run_events(paths: "core.LogPaths", session_id: "str | None",
                                run_id: "str | None") -> None:
    """Move the session's unknown-run orchestrator lines into run_id's stream. Never raises."""
    try:
        if not session_id or not run_id or run_id == UNKNOWN_RUN:
            return
        if not paths.orchestrator_events(UNKNOWN_RUN).is_file():
            return
        lock_dir = paths.session_run_id_dir()
        lock_dir.mkdir(parents=True, exist_ok=True)
        lock_path = lock_dir / "relocate.lock"
        if not _acquire_lock(lock_path):
            return
        try:
            _relocate_locked(paths, session_id, run_id)
        finally:
            try:
                lock_path.unlink()
            except OSError:
                pass
    except Exception as exc:
        core.debug_log("relocate_unknown_run_events failed", exc)
