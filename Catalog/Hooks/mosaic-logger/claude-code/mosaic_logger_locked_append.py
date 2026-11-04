"""mosaic_logger_locked_append.py - append one whole line to a log file under an exclusive lock.

KEEP IDENTICAL: this file exists twice, in copilot-shared/ (shipped with the ghcp-cli and
vscode-ghcp variants) and in claude-code/ (that variant ships no copilot-shared file).
Change both copies together; a test asserts they are byte-identical.

The lock is a sidecar file, <dir>/.<name>.lock, created with exclusive-create semantics
(O_CREAT | O_EXCL) before the append and deleted after it. A writer that finds the lock file
waits and retries until WAIT_LIMIT_SECONDS have passed; a lock file older than
STALE_AFTER_SECONDS is the leftover of a dead writer and is broken. A writer that cannot get
the lock in time drops the line and reports it. The mechanism is described in
Development/Designs/MosaicLogFormat.md section 4.1. No byte-range or advisory file lock is
used, so readers of the log file are never affected by the lock.
"""

import contextlib
import os
import pathlib
import time

WAIT_LIMIT_SECONDS = 2.0
STALE_AFTER_SECONDS = 10.0
_POLL_SECONDS = 0.02


def lock_path(target):
    """The lock file of target: <dir>/.<name>.lock."""
    target = pathlib.Path(target)
    return target.parent / ("." + target.name + ".lock")


def _is_stale(lock):
    try:
        return time.time() - os.stat(lock).st_mtime > STALE_AFTER_SECONDS
    except OSError:
        return False


def _try_create(lock):
    """Create the lock file; True if it was created, False if another writer holds it."""
    try:
        fd = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644)
    except FileExistsError:
        return False
    except OSError:
        # PermissionError: Windows reports a lock file that is being deleted as access denied.
        # Any other create failure also counts as not acquired; retry until the wait limit.
        return False
    os.close(fd)
    return True


def _acquire(lock):
    """Wait up to WAIT_LIMIT_SECONDS for the lock file; True when this writer now holds it."""
    deadline = time.monotonic() + WAIT_LIMIT_SECONDS
    while True:
        if _try_create(lock):
            return True
        if _is_stale(lock):
            try:
                os.unlink(lock)
            except OSError:
                pass
            continue
        if time.monotonic() >= deadline:
            return False
        time.sleep(_POLL_SECONDS)


def _release(lock):
    try:
        os.unlink(lock)
    except OSError:
        pass


def _report(debug, message, exc=None):
    if debug is None:
        return
    try:
        debug(message, exc)
    except Exception:
        pass


@contextlib.contextmanager
def holding_lock(target, debug=None):
    """Hold the lock of target for the block; yields True, or False if it was not obtained in time.

    The lock is released on exit, also when the block raises. debug(message, exc) is called
    once on a timeout (exc is None).
    """
    target = pathlib.Path(target)
    lock = lock_path(target)
    try:
        target.parent.mkdir(parents=True, exist_ok=True)
        acquired = _acquire(lock)
    except OSError:
        acquired = False
    if not acquired:
        _report(debug, "lock wait limit reached, not written: " + target.name)
        yield False
        return
    try:
        yield True
    finally:
        _release(lock)


def locked_append(target, line, debug=None, raise_os_errors=False):
    """Append line (bytes, one whole line) to target while holding its lock.

    Returns True when written, False when the line was dropped (lock wait limit, or an I/O
    error that was reported through debug). Never raises, except an OSError when
    raise_os_errors is true. debug(message, exc) receives a message and the exception or None.
    """
    try:
        # Created here as well so a directory that cannot be created is reported as an I/O error.
        pathlib.Path(target).parent.mkdir(parents=True, exist_ok=True)
        with holding_lock(target, debug) as held:
            if not held:
                return False
            with open(target, "ab") as handle:
                handle.write(line)
            return True
    except OSError as exc:
        if raise_os_errors:
            raise
        _report(debug, "append failed: " + pathlib.Path(target).name, exc)
        return False
