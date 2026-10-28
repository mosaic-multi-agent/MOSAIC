"""Shared helpers for the session-to-run routing and relocation tests. Not a test module.

Drives hook payloads for several VS Code sessions through mosaic_logger.dispatch()
against a temporary workspace and reads back the orchestrator streams per run folder.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow

RUN_A = "20261028T100000Z-aaaa"
RUN_B = "20261028T110000Z-bbbb"
SESSION_A = "sess-aaaa-0001"
SESSION_B = "sess-bbbb-0002"
UNKNOWN = "unknown-run"


def prompt_payload(workspace, session_id, run_id, text=None):
    """A UserPromptSubmit payload whose prompt names `run_id` (or carries `text`)."""
    prompt = text or f'{{"run_id":"{run_id}","task_description":"work"}}'
    return {
        "timestamp": "2026-10-28T10:00:00.000Z",
        "hook_event_name": "UserPromptSubmit",
        "session_id": session_id,
        "prompt": prompt,
        "cwd": str(workspace),
    }


def tool_payload(workspace, session_id, phase, n):
    """A PreToolUse ('pre') or PostToolUse ('post') payload for a read_file call."""
    data = {
        "timestamp": "2026-10-28T10:00:01.000Z",
        "hook_event_name": "PreToolUse" if phase == "pre" else "PostToolUse",
        "session_id": session_id,
        "tool_name": "read_file",
        "tool_input": {"filePath": f"/workspace/file{n}.md"},
        "tool_use_id": f"toolu_{session_id}_{n}__vscode-{n}",
        "cwd": str(workspace),
    }
    if phase == "post":
        data["tool_response"] = "content"
    return data


def stop_payload(workspace, session_id):
    return flow.payload("orchestrator_stop", workspace, session_id=session_id)


def session_start_payload(workspace, session_id):
    return flow.payload("session_start", workspace, session_id=session_id)


def send(data):
    flow.dispatch(data)


class RoutingWorkspace(flow.Workspace):
    """Workspace with per-run orchestrator stream readers."""

    def send_all(self, *payloads):
        for data in payloads:
            send(data)

    def stream_path(self, run_id):
        return self.paths.orchestrator_events(run_id)

    def stream(self, run_id):
        return flow.read_jsonl(self.stream_path(run_id))

    def stream_text(self, run_id):
        path = self.stream_path(run_id)
        return path.read_text("utf-8") if path.exists() else ""

    def events_of(self, run_id, session_id, name=None):
        return [e for e in self.stream(run_id)
                if e.get("session_id") == session_id
                and (name is None or e.get("event") == name)]

    def names(self, run_id, session_id):
        return [e.get("event") for e in self.events_of(run_id, session_id)]
