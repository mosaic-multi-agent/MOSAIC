"""The per-run ingest lock (.otel-ingest/lock), shared with the Go implementation.

This module mirrors Tools/Common/otelingest/lock.go.
Standard library only; deployed flat next to the adapter modules.
"""

import json
import os

from mosaic_logger_otel_contract import (
    LOCK_STALE_AFTER_SECONDS, STATE_DIR_NAME, format_time, parse_time, to_utc,
)

LOCK_FILE_NAME = "lock"


def lock_path(run_dir):
    return os.path.join(run_dir, STATE_DIR_NAME, LOCK_FILE_NAME)


def _remove_quietly(path):
    try:
        os.remove(path)
    except OSError:
        pass


def acquire_lock(run_dir, now):
    """Take the run's ingest lock. Returns (release, held): held is True when another ingest holds a
    fresh lock; a lock at least LOCK_STALE_AFTER_SECONDS old was left by a crashed ingest and is
    broken. release removes the lock."""
    os.makedirs(os.path.join(run_dir, STATE_DIR_NAME), exist_ok=True)
    path = lock_path(run_dir)
    for _attempt in range(2):
        if _create_lock(path, now):
            return (lambda: _remove_quietly(path)), False
        if not _lock_is_stale(path, now):
            return None, True
        _remove_quietly(path)
    return None, True


def _create_lock(path, now):
    """Create the lock file exclusively; False when it already exists."""
    flags = os.O_CREAT | os.O_EXCL | os.O_WRONLY | getattr(os, "O_BINARY", 0)
    try:
        fd = os.open(path, flags, 0o644)
    except FileExistsError:
        return False
    except PermissionError:
        if os.path.exists(path):
            return False
        raise
    content = {"holder": "py/" + str(os.getpid()), "acquired_at": format_time(now)}
    try:
        os.write(fd, json.dumps(content).encode("ascii"))
    finally:
        os.close(fd)
    return True


def _lock_is_stale(path, now):
    """Whether the lock is at least LOCK_STALE_AFTER_SECONDS old. Its age comes from acquired_at,
    else from the modification time of the lock file."""
    now_seconds = to_utc(now).timestamp()
    acquired = None
    try:
        with open(path, "rb") as handle:
            content = json.loads(handle.read().decode("utf-8", errors="replace"))
        if isinstance(content, dict):
            moment = parse_time(content.get("acquired_at"))
            if moment is not None:
                acquired = moment.timestamp()
    except (OSError, ValueError, RecursionError):
        pass
    if acquired is None:
        try:
            acquired = os.stat(path).st_mtime
        except OSError:
            return True
    return now_seconds - acquired >= LOCK_STALE_AFTER_SECONDS
