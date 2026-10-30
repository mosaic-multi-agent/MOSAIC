"""Shared helpers for the OTel ingest tests: a throw-away run folder and OTel line builders."""

import datetime
import json
import pathlib

SHARED_DIR = pathlib.Path(__file__).resolve().parents[1]
REPO_ROOT = SHARED_DIR.parents[3]
CONFORMANCE_DIR = REPO_ROOT / "Tools" / "Common" / "otelingest" / "testdata" / "conformance"

RUN_ID = "20261028T000000Z-0000"
SESSION = "cli-session-6"
FIXED_NOW = datetime.datetime(2026, 10, 28, 9, 0, 0, tzinfo=datetime.timezone.utc)


def fixed_now():
    return FIXED_NOW


def make_run(tmp_path, session=SESSION):
    """Create <tmp>/OrchestrationLogs/<run>/ whose orchestrator stream carries one session.

    Returns (run_dir, otel_path). Root chat spans of that conversation go to the orchestrator.
    """
    run_dir = tmp_path / "OrchestrationLogs" / RUN_ID
    run_dir.mkdir(parents=True)
    start = {
        "schema_version": "1.0.0", "event": "session_start",
        "timestamp": "2026-10-28T08:00:00.000Z", "harness": "ghcp-cli",
        "session_id": session, "run_id": RUN_ID,
    }
    (run_dir / "00_orchestrator_events.jsonl").write_text(json.dumps(start) + "\n", encoding="utf-8")
    otel_dir = tmp_path / "otel"
    otel_dir.mkdir()
    return run_dir, otel_dir / "copilot-otel.jsonl"


def chat_line(span_id, conversation=SESSION, input_tokens=100, output_tokens=10):
    """A GHCP CLI dialect root chat span line (no parent) carrying usage."""
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
    return json.dumps(span, separators=(",", ":"))


def write_otel(otel_path, lines):
    """Replace the OTel file with the given complete lines."""
    otel_path.write_bytes(("".join(line + "\n" for line in lines)).encode("utf-8"))


def read_json(path):
    return json.loads(pathlib.Path(path).read_text(encoding="utf-8"))


def read_otel_records(path, keep_timestamp=False):
    """usage_record lines with source 'otel' of a JSONL stream; timestamp removed unless keep_timestamp."""
    records = []
    for line in pathlib.Path(path).read_bytes().split(b"\n"):
        if not line.strip():
            continue
        obj = json.loads(line)
        if obj.get("event") == "usage_record" and obj.get("source") == "otel":
            if not keep_timestamp:
                obj.pop("timestamp", None)
            records.append(obj)
    return records


def state_path(run_dir):
    return pathlib.Path(run_dir) / ".otel-ingest" / "state.json"


def lock_path(run_dir):
    return pathlib.Path(run_dir) / ".otel-ingest" / "lock"
