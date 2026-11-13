"""Shared helpers for the subagentStart/subagentStop correlation tests.

Builds harness-shaped payloads (preToolUse agent dispatch, subagentStart,
subagentStop), replays them through the hook entry point, and reads back the
resulting log tree. Native mode: the session is bound to the run (as an earlier
dispatch would have bound it) unless a test sets bind_run to False. The session
transcript carries the agents' usage calls, which are logged at the read points.
"""

import json
import os
import pathlib
import re
import subprocess
import sys
import threading

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from transcript_logging_support import (  # noqa: E402
    FIRST_RUN, SESSION_ID, TranscriptLoggingCase, invocation, ts, usage_event,
)
from runner_mode_support import read_jsonl  # noqa: E402

RUN_ID = FIRST_RUN
FALLBACK_RE = r"^{prefix}_\d{{8}}T\d{{6}}Z-[0-9a-f]{{4}}$"
_ENTRY_POINT = str(pathlib.Path(__file__).resolve().parent.parent / "mosaic_logger.py")
_PROCESS_TIMEOUT_SECONDS = 60

__all__ = ["CorrelationCase", "ConcurrentHooksCase", "RUN_ID", "SESSION_ID", "dispatch_prompt",
           "fallback_pattern", "invocation", "ts", "usage_event"]


def fallback_pattern(prefix):
    """Regex of the unique fallback id for a given prefix."""
    return FALLBACK_RE.format(prefix=re.escape(prefix))


def dispatch_prompt(instance, run_id=RUN_ID):
    """A dispatch prompt carrying an explicit agent_instance_id line."""
    body = {"agent_instance_id": instance, "task_description": "do the work"}
    if run_id:
        body["run_id"] = run_id
    return json.dumps(body)


class CorrelationCase(TranscriptLoggingCase):
    """A temporary native-mode workspace with the session bound to RUN_ID."""

    bind_run = True

    def setUp(self):
        super().setUp()
        if self.bind_run:
            self.start_run(RUN_ID)

    # -- payloads -------------------------------------------------------------

    def dispatch_agent(self, instance, use_id=None, prompt=None, session_id=SESSION_ID,
                       run_id=RUN_ID, **extra):
        """preToolUse of the agent tool, as the orchestrator launching a subagent."""
        self.dispatch("preToolUse", {
            "toolName": "agent", "toolUseId": use_id or "tu-" + instance,
            "toolArgs": {"prompt": prompt or dispatch_prompt(instance, run_id)},
            **extra}, session_id=session_id)

    def start(self, agent_id, agent_type=None, agent_name=None, prompt=None,
              session_id=SESSION_ID, **extra):
        """subagentStart; agent_type / agent_name None leave the key out."""
        payload = {"agentId": agent_id, "transcriptPath": str(self.transcript), **extra}
        if agent_type is not None:
            payload["agentType"] = agent_type
        if agent_name is not None:
            payload["agentName"] = agent_name
        if prompt is not None:
            payload["agentPrompt"] = prompt
        self.dispatch("subagentStart", payload, session_id=session_id)

    def stop_agent(self, agent_id, message="done", session_id=SESSION_ID):
        self.dispatch("subagentStop", {
            "agentId": agent_id, "response": message,
            "transcriptPath": str(self.transcript)}, session_id=session_id)

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

    def mapping_path(self, agent_id, run_id=RUN_ID):
        return self.paths.agent_map_entry(run_id, agent_id)

    def mapped_instance(self, agent_id, run_id=RUN_ID):
        path = self.mapping_path(agent_id, run_id)
        return json.loads(path.read_text("utf-8"))["agent_instance_id"] if path.exists() else None

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

    def run_concurrently(self, event, payloads):
        """Launch one hook process per payload at (nearly) the same moment."""
        env = dict(os.environ)
        barrier = threading.Barrier(len(payloads))
        results = [None] * len(payloads)

        def run(index, payload):
            body = {"cwd": str(self.workspace), "sessionId": SESSION_ID, **payload}
            try:
                barrier.wait(timeout=30)
                results[index] = subprocess.run(
                    [sys.executable, _ENTRY_POINT, event], input=json.dumps(body).encode(),
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
            self.assertIsInstance(result, subprocess.CompletedProcess, f"process {index}: {result!r}")
            self.assertEqual(0, result.returncode, f"process {index}: {result.stderr!r}")

    def start_payload(self, agent_id, agent_type):
        return {"agentId": agent_id, "agentType": agent_type,
                "transcriptPath": str(self.transcript)}
