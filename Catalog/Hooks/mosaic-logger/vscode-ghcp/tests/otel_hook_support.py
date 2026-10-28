"""Shared helpers for the OTel ingest-point tests of the vscode-ghcp adapter. Not a test module.

Makes the shared Copilot module folder importable (the deployed layout is flat, so the
adapter finds it beside its own modules), builds OTel lines in the VS Code dialect, and
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

import session_routing_flow as rf
import vscode_flow as flow

ENV_VAR = "COPILOT_OTEL_FILE_EXPORTER_PATH"
HARNESS = "vscode-ghcp"
DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"
RUN = "20261028T100000Z-aaaa"
OTHER_RUN = "20261028T110000Z-bbbb"
SESSION = "sess-aaaa-0001"
OTHER_SESSION = "sess-aaaa-0002"


def usage_span(span_id, conversation=SESSION, parent=None, input_tokens=100, output_tokens=10,
               chat_session=None):
    """A VS Code dialect chat span line carrying usage; root unless `parent` is given."""
    span = {
        "traceId": "0123456789abcdef0123456789abcdef", "spanId": span_id,
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
        span["parentSpanContext"] = {"spanId": parent}
    if chat_session:
        span["attributes"]["copilot_chat.chat_session_id"] = chat_session
    return json.dumps(span, separators=(",", ":"))


def cli_usage_span(span_id, conversation, input_tokens=100, output_tokens=10):
    """A GHCP CLI dialect root chat span line carrying usage."""
    return json.dumps({
        "type": "span", "traceId": "0123456789abcdef0123456789abcdef", "spanId": span_id,
        "name": "chat",
        "attributes": {
            "gen_ai.operation.name": "chat",
            "gen_ai.conversation.id": conversation,
            "gen_ai.response.model": "claude-sonnet-5.5",
            "gen_ai.usage.input_tokens": input_tokens,
            "gen_ai.usage.output_tokens": output_tokens,
        },
    }, separators=(",", ":"))


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
    return [e for e in flow.read_jsonl(stream_path)
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


def run_hook_process(data):
    """Run the adapter as its own process for one payload, as the harness does per event."""
    env = dict(os.environ)
    env["PYTHONPATH"] = os.pathsep.join([str(SHARED_DIR), env.get("PYTHONPATH", "")])
    return subprocess.run([sys.executable, ADAPTER], capture_output=True, timeout=60,
                          env=env, input=json.dumps(data).encode())


class OtelWorkspace(rf.RoutingWorkspace):
    """Routing workspace plus an OTel export file outside the log tree."""

    def __init__(self):
        super().__init__()
        self.otel_path = self.path / "otel-export" / "copilot-otel.jsonl"

    def run_dir(self, run_id=RUN):
        return self.log_root / run_id

    def add_spans(self, *lines):
        append_lines(self.otel_path, lines)

    def prompt(self, session_id=SESSION, run_id=RUN):
        """UserPromptSubmit that makes `run_id` the run of `session_id`."""
        rf.send(rf.prompt_payload(self.path, session_id, run_id))

    def session_start(self, session_id=SESSION):
        rf.send(rf.session_start_payload(self.path, session_id))

    def stop(self, session_id=SESSION):
        rf.send(rf.stop_payload(self.path, session_id))

    def start_session(self, session_id=SESSION, run_id=RUN):
        """SessionStart then the first prompt: the run is known afterwards."""
        self.session_start(session_id)
        self.prompt(session_id, run_id)

    def process_send(self, data):
        """Dispatch `data` in a fresh hook process; it must exit 0 with empty stdout."""
        result = run_hook_process(data)
        assert result.returncode == 0 and result.stdout == b"", (result.returncode, result.stdout)

    def process_session_start(self, session_id=SESSION):
        self.process_send(rf.session_start_payload(self.path, session_id))

    def process_prompt(self, session_id=SESSION, run_id=RUN):
        self.process_send(rf.prompt_payload(self.path, session_id, run_id))

    def process_stop(self, session_id=SESSION):
        self.process_send(rf.stop_payload(self.path, session_id))

    def orchestrator_usage(self, run_id=RUN):
        return usage_records(self.stream_path(run_id))

    def orchestrator_record_ids(self, run_id=RUN):
        return record_ids(self.stream_path(run_id))

    def state(self, run_id=RUN):
        path = self.run_dir(run_id) / ".otel-ingest" / "state.json"
        return json.loads(path.read_text("utf-8")) if path.exists() else None

    def diag_lines(self, kind=None):
        found = flow.read_jsonl(self.log_root / DIAGNOSTICS_FILENAME)
        return [d for d in found if kind is None or d.get("kind") == kind]

    def event_names(self, run_id=RUN):
        return [e.get("event") for e in self.stream(run_id) if e.get("event") != "usage_record"]


def clean_environment(otel_path=None):
    """Patch os.environ: debug logging off, the OTel variable set to `otel_path` or removed."""
    patcher = mock.patch.dict(os.environ)
    patcher.start()
    os.environ.pop("MOSAIC_LOGGER_DEBUG", None)
    os.environ.pop(ENV_VAR, None)
    if otel_path is not None:
        os.environ[ENV_VAR] = str(otel_path)
    return patcher
