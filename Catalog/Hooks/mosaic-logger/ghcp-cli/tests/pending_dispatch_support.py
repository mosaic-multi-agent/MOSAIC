"""Helpers for the pending-dispatch queue tests: queue files and parallel processes.

Queue entries are written straight into the session's JSONL queue file so the
tests control age, type and corruption exactly. Parallel actors are real
processes released together by a start barrier (a file they poll for), with a
bounded wait. The queue is addressed by (run bucket, session).
"""

import datetime
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import unittest

_TESTS = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(_TESTS))
sys.path.insert(0, _TESTS)

from runner_mode_support import LazyModule  # noqa: E402

pending = LazyModule("mosaic_logger_pending_dispatch")

ADAPTER_DIR = pathlib.Path(__file__).resolve().parent.parent
SHARED_DIR = ADAPTER_DIR.parent / "copilot-shared"
RUN = "20261001T100000Z-aa11"
SESSION = "queue-session"
WAIT_SECONDS = 60

_ACTOR = r'''
import json, os, pathlib, sys, time
adapter, shared, workspace, run, session, go, ready, mode, arg1, arg2 = sys.argv[1:11]
sys.path.insert(0, shared)
sys.path.insert(0, adapter)
import mosaic_logger_core as core
import mosaic_logger_pending_dispatch as pending
paths = core.build_paths(pathlib.Path(workspace))
pathlib.Path(ready).write_text("1")
deadline = time.time() + 30
while not os.path.exists(go) and time.time() < deadline:
    time.sleep(0.002)
if mode == "take":
    got = pending.take_pending_dispatch(paths, run, session, arg1 or None)
    print(json.dumps(got.agent_instance_id if got else None))
else:
    ok = pending.put_pending_dispatch(paths, run, session, arg1, None, agent_type=arg2 or None)
    print(json.dumps(bool(ok)))
'''


class QueueCase(unittest.TestCase):
    """A temporary workspace plus take helpers for queue-level tests."""

    def setUp(self):
        import mosaic_logger_core as core
        self._td = tempfile.TemporaryDirectory()
        self.addCleanup(self._td.cleanup)
        self.workspace = pathlib.Path(self._td.name)
        self.paths = core.build_paths(self.workspace)

    def take(self, agent_type, session=SESSION, run=RUN, **kwargs):
        return pending.take_pending_dispatch(self.paths, run, session, agent_type, **kwargs)

    def take_id(self, agent_type, session=SESSION, run=RUN, **kwargs):
        got = self.take(agent_type, session, run, **kwargs)
        return got.agent_instance_id if got else None


def iso(age_seconds=0.0):
    """ISO 8601 UTC timestamp with ms and Z, age_seconds in the past."""
    moment = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=age_seconds)
    return moment.strftime("%Y-%m-%dT%H:%M:%S.") + f"{moment.microsecond // 1000:03d}Z"


def entry(instance, agent_type=None, age=0.0, run_id=None, prompt=None):
    """One queue entry dict (agent_type and prompt keys only when given)."""
    data = {"agent_instance_id": instance, "run_id": run_id, "created_at": iso(age)}
    if agent_type is not None:
        data["agent_type"] = agent_type
    if prompt is not None:
        data["prompt"] = prompt
    return data


def queue_path(paths, session=SESSION, run=RUN):
    return paths.pending_dispatch_entry(run, session)


def write_queue(paths, lines, session=SESSION, run=RUN):
    """Write the queue file; dict items are serialised, str items written verbatim."""
    path = queue_path(paths, session, run)
    path.parent.mkdir(parents=True, exist_ok=True)
    text = "".join((json.dumps(x) if isinstance(x, dict) else x) + "\n" for x in lines)
    path.write_bytes(text.encode("utf-8"))


def read_queue_ids(paths, session=SESSION, run=RUN):
    """Instance ids of the valid entries currently in the queue file, in order."""
    path = queue_path(paths, session, run)
    if not path.exists():
        return []
    ids = []
    for line in path.read_text("utf-8").splitlines():
        if line.strip():
            ids.append(json.loads(line)["agent_instance_id"])
    return ids


def run_actors(workspace, specs, session=SESSION, run=RUN):
    """Run one process per (mode, arg1, arg2) spec, released together.

    Returns each process's decoded JSON output in spec order. Modes: 'take'
    (arg1 = agent type) and 'put' (arg1 = instance id, arg2 = agent type).
    """
    workspace = pathlib.Path(workspace)
    control = workspace / ".actors"
    control.mkdir(parents=True, exist_ok=True)
    go = control / "go"
    procs, readies = [], []
    for index, (mode, arg1, arg2) in enumerate(specs):
        ready = control / f"ready-{index}"
        readies.append(ready)
        procs.append(subprocess.Popen(
            [sys.executable, "-c", _ACTOR, str(ADAPTER_DIR), str(SHARED_DIR), str(workspace),
             run, session, str(go), str(ready), mode, arg1 or "", arg2 or ""],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE))
    deadline = time.time() + WAIT_SECONDS
    while (not all(r.exists() for r in readies) and time.time() < deadline
           and all(p.poll() is None for p in procs)):
        time.sleep(0.01)
    go.write_text("1")
    outputs = []
    for index, proc in enumerate(procs):
        try:
            out, err = proc.communicate(timeout=WAIT_SECONDS)
        except subprocess.TimeoutExpired:
            proc.kill()
            raise
        if proc.returncode != 0:
            raise AssertionError(f"actor {index} failed: {err.decode(errors='replace')[:400]}")
        outputs.append(json.loads(out.decode().strip().splitlines()[-1]))
    return outputs
