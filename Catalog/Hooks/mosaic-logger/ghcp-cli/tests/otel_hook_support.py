"""Shared helpers for the OTel ingest-point tests of the ghcp-cli adapter. Not a test module.

Makes the shared Copilot module folder importable (the deployed layout is flat, so the
adapter finds it beside its own modules), builds OTel lines in the GHCP CLI dialect, and
offers a patch point for the ingest entry that every module of the adapter sees.
"""

import contextlib
import json
import os
import pathlib
import subprocess
import sys
from unittest import mock

_HERE = pathlib.Path(__file__).resolve().parent
SHARED_DIR = _HERE.parents[1] / "copilot-shared"
ADAPTER = str(_HERE.parent / "mosaic_logger.py")
sys.path.insert(0, str(_HERE))
sys.path.insert(0, str(_HERE.parent))
sys.path.insert(0, str(SHARED_DIR))

import runner_mode_support as rms

ENV_VAR = "COPILOT_OTEL_FILE_EXPORTER_PATH"
HARNESS = "ghcp-cli"
DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"
RUN = "20261028T100000Z-aaaa"
OTHER_RUN = "20261028T110000Z-bbbb"
SESSION = "cli-session-aaaa-0001"
OTHER_SESSION = "cli-session-aaaa-0002"
INSTANCE = "TestWriter#7"
AGENT_ID = "cli-agent-0001"


def usage_span(span_id, conversation=SESSION, parent=None, input_tokens=100, output_tokens=10):
    """A GHCP CLI dialect chat span line carrying usage; root unless `parent` is given."""
    span = {
        "type": "span", "traceId": "0123456789abcdef0123456789abcdef", "spanId": span_id,
        "name": "chat", "kind": 2,
        "attributes": {
            "gen_ai.operation.name": "chat",
            "gen_ai.conversation.id": conversation,
            "gen_ai.response.model": "claude-sonnet-5.5",
            "gen_ai.usage.input_tokens": input_tokens,
            "gen_ai.usage.output_tokens": output_tokens,
        },
    }
    if parent:
        span["parentSpanId"] = parent
    return json.dumps(span, separators=(",", ":"))


def corrupt_line():
    """A line that carries the usage marker but is not a JSON object."""
    return '{"gen_ai.operation.name":"chat","broken'


def append_lines(path, lines):
    path = pathlib.Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "ab") as handle:
        handle.write("".join(line + "\n" for line in lines).encode("utf-8"))


def usage_records(stream_path):
    """usage_record lines with source 'otel' of one JSONL stream."""
    return [e for e in rms.read_jsonl(pathlib.Path(stream_path))
            if e.get("event") == "usage_record" and e.get("source") == "otel"]


def record_ids(stream_path):
    return sorted(r.get("record_id") for r in usage_records(stream_path))


@contextlib.contextmanager
def ingest_patched(replacement=None):
    """Replace the shared module's ingest() wherever the adapter reaches it.

    `replacement(original, *args, **kwargs)` supplies the result (or raises); None
    delegates to the real ingest. Yields the list of (args, kwargs) calls seen.

    Only modules already imported that expose `ingest` are patched, so the adapter has to
    bind the entry at import time (`from mosaic_logger_otel import ingest`, or any module
    attribute named `ingest`); `mosaic_logger_otel.ingest` itself is always patched.
    """
    import mosaic_logger_otel as otel
    original = otel.ingest
    calls = []

    def fake(*args, **kwargs):
        calls.append((args, kwargs))
        if replacement is None:
            return original(*args, **kwargs)
        return replacement(original, *args, **kwargs)

    with contextlib.ExitStack() as stack:
        for name, module in list(sys.modules.items()):
            if name.startswith("mosaic_logger") and getattr(module, "ingest", None) is original:
                stack.enter_context(mock.patch.object(module, "ingest", fake))
        yield calls


def run_hook_process(event, body):
    """Run the adapter as its own process for one event, as the harness does."""
    env = dict(os.environ)
    env["PYTHONPATH"] = os.pathsep.join([str(SHARED_DIR), env.get("PYTHONPATH", "")])
    return subprocess.run([sys.executable, ADAPTER, event], capture_output=True, timeout=60,
                          env=env, input=json.dumps(body).encode())


class OtelHookCase(rms.RunnerModeTestCase):
    """Temp workspace with an OTel export file outside the log tree; debug logging off."""

    otel_set = True

    def setUp(self):
        super().setUp()
        self.otel_path = self.workspace / "otel-export" / "copilot-otel.jsonl"
        patcher = mock.patch.dict(os.environ)
        patcher.start()
        self.addCleanup(patcher.stop)
        os.environ.pop("MOSAIC_LOGGER_DEBUG", None)
        os.environ.pop(ENV_VAR, None)
        if self.otel_set:
            os.environ[ENV_VAR] = str(self.otel_path)

    def run_dir(self, run_id=RUN):
        return self.paths.root / run_id

    def add_spans(self, *lines):
        append_lines(self.otel_path, lines)

    def prompt(self, session_id=SESSION, run_id=RUN):
        """userPromptSubmitted that makes `run_id` the run of `session_id`."""
        return self.dispatch("userPromptSubmitted",
                             {"prompt": f'run_id: "{run_id}"\nDo the work'}, session_id)

    def session_start(self, session_id=SESSION):
        return self.dispatch("sessionStart", {}, session_id)

    def agent_stop(self, session_id=SESSION):
        return self.dispatch("agentStop", {"stopReason": "end_turn"}, session_id)

    def subagent_start(self, session_id=SESSION, run_id=RUN):
        prompt = f'agent_instance_id: "{INSTANCE}"\nrun_id: "{run_id}"\nDo the task'
        return self.dispatch("subagentStart", {
            "agentId": AGENT_ID, "agentName": "TestWriter", "agentPrompt": prompt}, session_id)

    def subagent_stop(self, session_id=SESSION, run_id=RUN):
        return self.dispatch("subagentStop", {
            "agentId": AGENT_ID, "response": f'run_id: "{run_id}"\ndone'}, session_id)

    def process_send(self, event, payload=None, session_id=SESSION):
        """Dispatch one event in a fresh hook process; it must exit 0 with empty stdout."""
        body = {"cwd": str(self.workspace), "sessionId": session_id}
        body.update(payload or {})
        result = run_hook_process(event, body)
        assert result.returncode == 0 and result.stdout == b"", (result.returncode, result.stdout)

    def process_session_start(self, session_id=SESSION):
        self.process_send("sessionStart", {}, session_id)

    def process_prompt(self, session_id=SESSION, run_id=RUN):
        self.process_send("userPromptSubmitted",
                          {"prompt": f'run_id: "{run_id}"\nDo the work'}, session_id)

    def process_agent_stop(self, session_id=SESSION):
        self.process_send("agentStop", {"stopReason": "end_turn"}, session_id)

    def start_session(self, session_id=SESSION, run_id=RUN):
        """sessionStart then the first prompt: the run is known afterwards."""
        self.session_start(session_id)
        self.prompt(session_id, run_id)

    def orchestrator_record_ids(self, run_id=RUN):
        return record_ids(self.paths.orchestrator_events(run_id))

    def orchestrator_usage(self, run_id=RUN):
        return usage_records(self.paths.orchestrator_events(run_id))

    def state(self, run_id=RUN):
        path = self.run_dir(run_id) / ".otel-ingest" / "state.json"
        return json.loads(path.read_text("utf-8")) if path.exists() else None

    def diag_lines(self, kind=None):
        found = rms.read_jsonl(self.paths.root / DIAGNOSTICS_FILENAME)
        return [d for d in found if kind is None or d.get("kind") == kind]

    def event_names(self, run_id=RUN):
        return [e.get("event") for e in self.orchestrator_events(run_id)
                if e.get("event") != "usage_record"]

    def any_ingest_state(self):
        """True when any run folder holds an .otel-ingest directory."""
        return any(self.paths.root.glob("*/.otel-ingest"))
