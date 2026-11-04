"""Helpers for the locked-append tests: writer processes appending very long lines to one file.

The same file sits in each tests folder that needs it (the folders are separate packages).
A line carries 100000 characters, more than the OS write buffer, so an unlocked append
from several processes is able to interleave.
"""

import json
import os
import pathlib
import subprocess
import sys
import threading
import time

PAD = 100_000
PROCS = 4
LINES = 8

WRITER = r'''
import json, pathlib, sys
kind, target, tag, count, pad = sys.argv[1:6]
dirs = sys.argv[6:]
for d in dirs:
    sys.path.insert(0, d)
count, pad = int(count), int(pad)
if kind == "locked":
    import mosaic_logger_locked_append as writer
elif kind == "event":
    import mosaic_logger_core as writer
else:
    import mosaic_logger_otel_write as writer
for i in range(count):
    event = {"event": "probe", "writer": tag, "seq": i, "pad": "p" * pad}
    if kind == "event":
        writer.append_event(pathlib.Path(target), event)
        continue
    line = (json.dumps(event) + "\n").encode("ascii")
    if kind == "locked":
        writer.locked_append(target, line)
    else:
        writer.append_line(target, line)
'''


def lock_file_of(target):
    """The lock file the log format names for target: <dir>/.<name>.lock."""
    target = pathlib.Path(target)
    return target.parent / ("." + target.name + ".lock")


def hold_lock(target, age_seconds=0.0):
    """Create the lock file of target as another live writer would have; returns its path."""
    path = lock_file_of(target)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b"")
    if age_seconds:
        stamp = time.time() - age_seconds
        os.utime(path, (stamp, stamp))
    return path


def release_after(path, seconds):
    """Delete the lock file at path after seconds, from a helper thread; returns the started timer."""
    timer = threading.Timer(seconds, lambda: pathlib.Path(path).unlink())
    timer.daemon = True
    timer.start()
    return timer

def _reader_problems(path):
    """Problems seen by one unlocked read of path: any complete line that is not valid JSON."""
    try:
        data = pathlib.Path(path).read_bytes()
    except FileNotFoundError:
        return []
    except OSError as exc:
        return ["reader failed: %r" % (exc,)]
    complete = data[:data.rfind(b"\n") + 1]
    problems = []
    for raw in complete.split(b"\n")[:-1]:
        try:
            json.loads(raw)
        except ValueError:
            problems.append("reader saw a torn or blank line of %d bytes" % len(raw))
    return problems


def run_writers(kind, dirs, target, procs=PROCS, lines=LINES):
    """Run procs writer processes of kind ("locked", "event", "line") at once.

    Returns the problems found: failed writers, and anything a reader saw while they wrote.
    """
    children = []
    for n in range(procs):
        args = [sys.executable, "-I", "-c", WRITER, kind, str(target), "w%d" % n,
                str(lines), str(PAD)] + [str(d) for d in dirs]
        children.append(subprocess.Popen(args, stderr=subprocess.PIPE, stdout=subprocess.DEVNULL))
    problems = []
    while any(c.poll() is None for c in children):
        problems += _reader_problems(target)
        time.sleep(0.01)
    for c in children:
        err = c.stderr.read().decode("utf-8", "replace")
        c.stderr.close()
        if c.returncode != 0:
            problems.append("writer exited %s: %s" % (c.returncode, err[-300:]))
    return problems


def line_problems(path, procs=PROCS, lines=LINES):
    """Lost, duplicated, torn or blank lines of path after run_writers (empty list = intact)."""
    data = pathlib.Path(path).read_bytes()
    if not data.endswith(b"\n"):
        return ["file does not end with a newline"]
    problems = []
    seen = []
    for raw in data.split(b"\n")[:-1]:
        if not raw.strip():
            problems.append("blank line")
            continue
        try:
            event = json.loads(raw)
            ok = event["pad"] == "p" * PAD
            seen.append((event["writer"], event["seq"]))
        except (ValueError, KeyError, TypeError):
            problems.append("torn line of %d bytes" % len(raw))
            continue
        if not ok:
            problems.append("line with damaged payload")
    expected = sorted(("w%d" % n, i) for n in range(procs) for i in range(lines))
    if sorted(seen) != expected:
        problems.append("expected %d distinct lines, found %d" % (len(expected), len(seen)))
    return problems
