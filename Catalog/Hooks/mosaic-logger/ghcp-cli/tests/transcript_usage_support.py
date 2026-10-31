"""Shared helpers for the transcript usage reader tests: event builders and transcript writers."""

import json
import os
import pathlib
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_transcript_usage as reader  # noqa: E402

FIXTURES = pathlib.Path(__file__).resolve().parent / "fixtures"

USAGE_FIXTURE = FIXTURES / "ghcp_cli_transcript_usage.jsonl"
COMPACTION_FIXTURE = FIXTURES / "ghcp_cli_transcript_compaction.jsonl"
NOT_APPLIED_FIXTURE = FIXTURES / "ghcp_cli_transcript_compaction_not_applied.jsonl"
ALARM_FIXTURE = FIXTURES / "ghcp_cli_transcript_renamed_fields.jsonl"
UNREADABLE_FIXTURE = FIXTURES / "ghcp_cli_transcript_unreadable_fields.jsonl"
PARTIAL_FIXTURE = FIXTURES / "ghcp_cli_transcript_partial_line.jsonl"

DEFAULT_TS = "2026-10-30T13:00:10.000Z"


def usage_event(api_call_id="call-1", ts=DEFAULT_TS, agent_id=None, event_id="ev-1", drop=(), **overrides):
    """A session.usage_record event. `drop` removes usage keys; `api_call_id`/`event_id` None removes them."""
    usage = {
        "model": "claude-sonnet-5.5",
        "provider": {"id": "copilot", "kind": "copilot", "label": "GitHub Copilot"},
        "inputTokens": 1000,
        "outputTokens": 50,
        "cacheReadTokens": 300,
        "cacheWriteTokens": 200,
        "reasoningTokens": 20,
        "duration": 1000,
        "reasoningEffort": "medium",
        "toolCounts": {},
    }
    if api_call_id is not None:
        usage["apiCallId"] = api_call_id
    usage.update(overrides)
    for key in drop:
        usage.pop(key, None)
    return _envelope("session.usage_record", {"usage": usage}, ts, agent_id, event_id)


def compaction_event(attempt_id="attempt-1", ts=DEFAULT_TS, agent_id=None, event_id="ev-c1", drop=(),
                     used_drop=(), **overrides):
    """A session.compaction_complete event; `overrides` replace data keys, `used_drop` trims compactionTokensUsed."""
    used = {
        "provider": {"id": "copilot", "kind": "copilot", "label": "GitHub Copilot"},
        "inputTokens": 1000,
        "outputTokens": 40,
        "cacheReadTokens": 300,
        "cacheWriteTokens": 100,
        "duration": 2000,
        "model": "claude-sonnet-5.5",
        "requestId": "req-1",
        "serviceRequestId": "svc-1",
    }
    for key in used_drop:
        used.pop(key, None)
    data = {
        "success": True,
        "outcome": "applied",
        "trigger": "threshold",
        "preCompactionTokens": 9000,
        "postCompactionTokens": 1500,
        "compactionTokensUsed": used,
    }
    if attempt_id is not None:
        data["compactionAttemptId"] = attempt_id
    data.update(overrides)
    for key in drop:
        data.pop(key, None)
    return _envelope("session.compaction_complete", data, ts, agent_id, event_id)


def turn_end_event(event_id="ev-t1", ts=DEFAULT_TS):
    return _envelope("assistant.turn_end", {"turnId": "0"}, ts, None, event_id)


def _envelope(kind, data, ts, agent_id, event_id):
    event = {"type": kind, "data": data, "timestamp": ts}
    if event_id is not None:
        event["id"] = event_id
    if agent_id is not None:
        event["agentId"] = agent_id
    return event


def write_transcript(tmp_path, events, trailing=""):
    """Write events as JSONL (events may be dicts or raw strings); `trailing` is appended verbatim."""
    path = pathlib.Path(tmp_path) / "events.jsonl"
    lines = [e if isinstance(e, str) else json.dumps(e) for e in events]
    path.write_text("".join(line + "\n" for line in lines) + trailing, encoding="utf-8", newline="")
    return path


def read_events(tmp_path, events, trailing=""):
    return reader.read_transcript_usage(write_transcript(tmp_path, events, trailing))


def problem_keys(result):
    return [(p.record_id, p.field) for p in result.problems]
