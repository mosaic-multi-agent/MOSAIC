"""Shared helpers for the subagent lifecycle and tool output tests.

Builds hook payloads from the captured-shape fixtures in tests/fixtures/ and drives
them through mosaic_logger.dispatch() against a temporary workspace. Not a test module.
"""

import json
import os
import pathlib
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core

FIXTURES = pathlib.Path(__file__).parent / "fixtures"

RUN_ID = "20261027T100000Z-b7c4"
INSTANCE_ID = "requirements-refinement#4"
CALL_ID = "toolu_017pWWTUCtf17bR6MDKoNcMw"
SUFFIX = "__vscode-1793098349285"
TOOL_USE_ID = CALL_ID + SUFFIX
DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"

_FIXTURE_FILES = {
    "session_start": "hook_session_start.json",
    "pre_dispatch": "hook_pre_tool_use_run_subagent.json",
    "start": "hook_subagent_start.json",
    "stop": "hook_subagent_stop.json",
    "post_dispatch": "hook_post_tool_use_run_subagent.json",
    "post_read_empty": "hook_post_tool_use_read_file_empty.json",
    "orchestrator_stop": "hook_stop.json",
}


def payload(kind, workspace, **overrides):
    """Return the fixture payload `kind` with cwd set to `workspace`.

    An override whose value is the string "<absent>" removes that key.
    """
    data = json.loads((FIXTURES / _FIXTURE_FILES[kind]).read_text("utf-8"))
    data["cwd"] = str(workspace)
    for key, value in overrides.items():
        if value == "<absent>":
            data.pop(key, None)
        else:
            data[key] = value
    return data


def dispatch(data):
    """Feed one payload dict through the adapter's dispatcher."""
    mosaic_logger.dispatch(json.dumps(data))


def read_jsonl(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class Workspace:
    """A temporary workspace plus readers over the log tree it accumulates."""

    def __init__(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.path = pathlib.Path(self._tmp.name)
        self.paths = core.build_paths(self.path)

    def cleanup(self):
        self._tmp.cleanup()

    def send(self, kind, **overrides):
        dispatch(payload(kind, self.path, **overrides))

    def start_subagent(self):
        """SessionStart, the runSubagent PreToolUse and SubagentStart (observed order)."""
        self.send("session_start")
        self.send("pre_dispatch")
        self.send("start")

    @property
    def log_root(self):
        return self.paths.root

    def invocation_events(self):
        return read_jsonl(self.paths.invocation_events(RUN_ID, INSTANCE_ID))

    def invocation_ends(self):
        return [e for e in self.invocation_events() if e.get("event") == "invocation_end"]

    def invocation_start(self):
        return next((e for e in self.invocation_events()
                     if e.get("event") == "invocation_start"), None)

    def event_files(self):
        if not self.log_root.exists():
            return []
        return [f for f in self.log_root.rglob("*.jsonl") if f.name != DIAGNOSTICS_FILENAME]

    def all_events(self):
        events = []
        for f in self.event_files():
            events.extend(read_jsonl(f))
        return events

    def orchestrator_events(self):
        events = []
        for f in self.event_files():
            if f.name == "00_orchestrator_events.jsonl":
                events.extend(read_jsonl(f))
        return events

    def invocation_event_count(self):
        """Number of invocation_start / invocation_end events in any stream."""
        return sum(1 for e in self.all_events()
                   if e.get("event") in ("invocation_start", "invocation_end"))

    def invocation_folders(self):
        """Per-invocation folders (non-dot subfolders of any run folder)."""
        if not self.log_root.exists():
            return []
        return [d for run in self.log_root.iterdir() if run.is_dir()
                for d in run.iterdir() if d.is_dir() and not d.name.startswith(".")]

    def diagnostics(self):
        return read_jsonl(self.log_root / DIAGNOSTICS_FILENAME)

    def output_artifact(self):
        return self.paths.invocation_output(RUN_ID, INSTANCE_ID)
