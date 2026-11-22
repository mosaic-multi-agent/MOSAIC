"""Shared helpers for the SubagentStart/SubagentStop correlation tests.

Builds VS Code-shaped payloads (PreToolUse of a subagent-dispatch tool, SubagentStart,
SubagentStop, the dispatch's PostToolUse, the orchestrator's Stop), replays them through
mosaic_logger.dispatch() against a temporary workspace and reads the resulting log tree
and spawn index back. Native mode: the session is bound to RUN_ID (as an earlier
dispatch would have bound it) unless a test sets bind_run to False. Not a test module.
"""

import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import threading
import unittest

_TESTS = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(_TESTS))
sys.path.insert(0, _TESTS)

import mosaic_logger  # noqa: E402
import mosaic_logger_core as core  # noqa: E402
import mosaic_logger_session_run as session_run  # noqa: E402
import mosaic_logger_subagent_state as subagent_state  # noqa: E402

RUN_ID = "20261112T100000Z-c0a1"
SESSION_ID = "corr-session-main"
SUFFIX = "__vscode-1793098349285"
FALLBACK_RE = r"^{prefix}_\d{{8}}T\d{{6}}Z-[0-9a-f]{{4}}$"
_ADAPTER_DIR = pathlib.Path(_TESTS).parent
_SHARED_DIR = _ADAPTER_DIR.parent / "copilot-shared"
_ENTRY_POINT = str(_ADAPTER_DIR / "mosaic_logger.py")
_PROCESS_TIMEOUT_SECONDS = 60


def fallback_pattern(prefix):
    """Regex of the unique fallback id for a given prefix."""
    return FALLBACK_RE.format(prefix=re.escape(prefix))


def dispatch_prompt(instance, run_id=RUN_ID):
    """A dispatch prompt carrying an explicit agent_instance_id line."""
    body = {"agent_instance_id": instance, "task_description": "do the work"}
    if run_id:
        body["run_id"] = run_id
    return json.dumps(body)


def read_jsonl(path):
    path = pathlib.Path(path)
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]


class CorrelationCase(unittest.TestCase):
    """A temporary native-mode workspace with the session bound to RUN_ID."""

    bind_run = True

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.workspace = pathlib.Path(self._tmp.name)
        self.paths = core.build_paths(self.workspace)
        self.transcript = self.workspace / "orchestrator-transcript.jsonl"
        self.transcript.write_text('{"type": "user.message"}\n', "utf-8")
        if self.bind_run:
            session_run.put_session_run_id(self.paths, SESSION_ID, RUN_ID)

    # -- payloads -------------------------------------------------------------

    def payload(self, event, session_id=SESSION_ID, **fields):
        data = {"timestamp": "2026-11-12T10:00:00.000Z", "hook_event_name": event,
                "session_id": session_id, "transcript_path": str(self.transcript),
                "cwd": str(self.workspace)}
        data.update(fields)
        return data

    def send(self, data):
        mosaic_logger.dispatch(json.dumps(data))

    def dispatch_agent(self, instance, call_id=None, prompt=None, session_id=SESSION_ID,
                       run_id=RUN_ID, tool_name="runSubagent", **tool_input):
        """PreToolUse of the dispatch tool, as the orchestrator launching a subagent."""
        call_id = call_id or "toolu_" + re.sub(r"\W", "", instance)
        self.send(self.payload(
            "PreToolUse", session_id, tool_name=tool_name, tool_use_id=call_id + SUFFIX,
            tool_input={"prompt": prompt or dispatch_prompt(instance, run_id), **tool_input}))

    def start_payload(self, agent_id=None, agent_type=None, agent_name=None,
                      session_id=SESSION_ID, **extra):
        """SubagentStart payload; None values leave the key out."""
        fields = {"agent_id": agent_id, "agent_type": agent_type, "agent_name": agent_name}
        data = self.payload("SubagentStart", session_id, **extra)
        data.update({k: v for k, v in fields.items() if v is not None})
        return data

    def start(self, agent_id=None, agent_type=None, agent_name=None,
              session_id=SESSION_ID, **extra):
        self.send(self.start_payload(agent_id, agent_type, agent_name, session_id, **extra))

    def stop_agent(self, agent_id=None, agent_type=None, agent_name=None, session_id=SESSION_ID):
        fields = {"agent_id": agent_id, "agent_type": agent_type, "agent_name": agent_name}
        data = self.payload("SubagentStop", session_id, stop_hook_active=False)
        data.update({k: v for k, v in fields.items() if v is not None})
        self.send(data)

    def post_dispatch(self, call_id, response="done", tool_name="runSubagent",
                      session_id=SESSION_ID):
        """PostToolUse of the dispatch tool: the completion firing."""
        self.send(self.payload("PostToolUse", session_id, tool_name=tool_name,
                               tool_use_id=call_id + SUFFIX, tool_response=response))

    def orchestrator_stop(self, session_id=SESSION_ID):
        self.send(self.payload("Stop", session_id, stop_hook_active=False))

    # -- reading back ---------------------------------------------------------

    def run_root(self, run_id=RUN_ID):
        return self.paths.run_root(run_id)

    def folders(self, run_id=RUN_ID):
        """Names of the invocation folders of a run (sidecar dot-folders excluded)."""
        root = self.run_root(run_id)
        if not root.exists():
            return []
        return sorted(d.name for d in root.iterdir()
                      if d.is_dir() and not d.name.startswith("."))

    def events(self, instance, run_id=RUN_ID):
        return read_jsonl(self.paths.invocation_events(run_id, instance))

    def names(self, instance, run_id=RUN_ID):
        return [e["event"] for e in self.events(instance, run_id)]

    def ends(self, instance, run_id=RUN_ID):
        return [e for e in self.events(instance, run_id) if e["event"] == "invocation_end"]

    def mapping_path(self, key, run_id=RUN_ID):
        return self.paths.agent_map_entry(run_id, key)

    def mapped_instance(self, key, run_id=RUN_ID):
        path = self.mapping_path(key, run_id)
        return json.loads(path.read_text("utf-8"))["agent_instance_id"] if path.exists() else None

    def spawn_entry(self, key):
        return subagent_state.get_spawn_entry(self.paths, key)

    def spawn_entries(self):
        """Every spawn index entry of the workspace."""
        folder = self.paths.subagent_index_dir()
        if not folder.exists():
            return []
        return [json.loads(f.read_text("utf-8")) for f in sorted(folder.glob("*.json"))]

    def instance_of(self, key):
        """Instance id the spawn index holds for key (None when absent)."""
        entry = self.spawn_entry(key)
        return entry["agent_instance_id"] if entry else None

    def queue_lines(self, session_id=SESSION_ID, run_id=RUN_ID):
        path = self.paths.pending_dispatch_entry(run_id, session_id)
        if not path.exists():
            return []
        return [ln for ln in path.read_text("utf-8").splitlines() if ln.strip()]

    def queued_ids(self, session_id=SESSION_ID, run_id=RUN_ID):
        return [json.loads(ln)["agent_instance_id"]
                for ln in self.queue_lines(session_id, run_id)]

    def assert_no_unknown_agent_folder(self):
        for run in (RUN_ID, "unknown-run"):
            self.assertNotIn("unknown-agent", self.folders(run))


class ConcurrentHooksCase(CorrelationCase):
    """Fires hooks as separate processes through the real hook entry point."""

    def run_concurrently(self, payloads):
        """Launch one hook process per payload dict at (nearly) the same moment."""
        env = dict(os.environ)
        env["PYTHONPATH"] = os.pathsep.join(
            [str(_SHARED_DIR), env.get("PYTHONPATH", "")]).rstrip(os.pathsep)
        barrier = threading.Barrier(len(payloads))
        results = [None] * len(payloads)

        def run(index, payload):
            try:
                barrier.wait(timeout=30)
                results[index] = subprocess.run(
                    [sys.executable, _ENTRY_POINT], input=json.dumps(payload).encode(),
                    capture_output=True, env=env, timeout=_PROCESS_TIMEOUT_SECONDS)
            except Exception as exc:  # reported by the assertion below
                results[index] = exc

        threads = [threading.Thread(target=run, args=(i, p), daemon=True)
                   for i, p in enumerate(payloads)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=_PROCESS_TIMEOUT_SECONDS + 30)
        for index, result in enumerate(results):
            self.assertIsInstance(result, subprocess.CompletedProcess,
                                  f"process {index}: {result!r}")
            self.assertEqual(0, result.returncode, f"process {index}: {result.stderr!r}")
