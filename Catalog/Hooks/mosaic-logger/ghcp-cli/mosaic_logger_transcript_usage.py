"""GHCP CLI transcript usage and compaction reader.

Reads every `session.usage_record` and `session.compaction_complete` event of a GHCP CLI
session transcript (`events.jsonl`) into usage calls and compactions. Stdlib only, never
raises, writes nothing.

The whole file is read in one pass. Only lines that mention one of the three event types of
interest are parsed as JSON, so a multi-megabyte transcript is read well inside the hook
time limit.
"""

from __future__ import annotations

import datetime
import json
import re
from dataclasses import dataclass, field
from typing import List, Optional

_USAGE_TYPE = "session.usage_record"
_COMPACTION_TYPE = "session.compaction_complete"
_TURN_END_TYPE = "assistant.turn_end"
_MARKERS = (b'"session.usage_record"', b'"session.compaction_complete"', b'"assistant.turn_end"')
_TOKEN_FIELDS = ("inputTokens", "outputTokens", "cacheReadTokens", "cacheWriteTokens", "reasoningTokens")
_TRIGGERS = {"threshold": "auto", "manual": "manual"}
_TIMESTAMP = re.compile(r"^(\d{4})-(\d\d)-(\d\d)T(\d\d):(\d\d):(\d\d)(?:\.(\d+))?Z?$")


@dataclass
class FieldProblem:
    """One field the reader could not read; the record degraded without it."""
    record_id: Optional[str]  # None when the record has no key and was left out
    field: str                # transcript field name, e.g. "toolCounts"


@dataclass
class UsageCall:
    """One session.usage_record event, mapped to usage_record fields."""
    record_id: str
    record_index: int
    agent_id: Optional[str]          # None = the session's own (orchestrator) call
    timestamp: str                   # call start, ISO 8601 UTC with milliseconds and "Z"
    model: Optional[str]
    provider: Optional[str]          # "github-copilot" or None
    effort: Optional[str]
    token_usage: Optional[dict]      # None when no sub-field resolved
    requested_tools: Optional[dict]  # None = absent


@dataclass
class CompactionUsage:
    """compactionTokensUsed, mapped to usage_record fields."""
    timestamp: str
    model: Optional[str]
    provider: Optional[str]
    token_usage: Optional[dict]


@dataclass
class CompactionFacts:
    """One session.compaction_complete event."""
    record_id: str
    record_index: int
    agent_id: Optional[str]
    completed_at: str
    applied: bool
    trigger: Optional[str]
    tokens_before: Optional[int]
    tokens_after: Optional[int]
    usage: Optional[CompactionUsage]


@dataclass
class TranscriptRead:
    readable: bool = False
    calls: List[UsageCall] = field(default_factory=list)
    compactions: List[CompactionFacts] = field(default_factory=list)
    alarm: bool = False
    assistant_turns: int = 0
    problems: List[FieldProblem] = field(default_factory=list)


def read_transcript_usage(transcript_path) -> TranscriptRead:
    """Read every usage call and compaction of a GHCP CLI session transcript.

    Never raises. A missing or unreadable file gives readable=False and empty lists.
    Malformed lines and a trailing partial line are skipped and are not problems.
    """
    result = TranscriptRead()
    try:
        with open(transcript_path, "rb") as handle:
            data = handle.read()
    except Exception:
        return result
    result.readable = True
    try:
        _scan(data, result)
    except Exception:
        pass
    return result


def _scan(data: bytes, result: TranscriptRead) -> None:
    turn_ends = 0
    readable_usage = 0
    ordinal = 0
    for raw in data.split(b"\n"):
        if not any(marker in raw for marker in _MARKERS):
            continue
        event = _parse_line(raw)
        if event is None:
            continue
        kind = event.get("type")
        if kind == _TURN_END_TYPE:
            turn_ends += 1
        elif kind == _USAGE_TYPE:
            if _usage_block_readable(event):
                readable_usage += 1
            _read_usage_event(event, ordinal, result)
            ordinal += 1
        elif kind == _COMPACTION_TYPE:
            _read_compaction_event(event, ordinal, result)
            ordinal += 1
    result.assistant_turns = turn_ends
    result.alarm = turn_ends > 0 and readable_usage == 0


def _parse_line(raw: bytes) -> Optional[dict]:
    try:
        event = json.loads(raw.decode("utf-8"))
    except Exception:
        return None
    return event if isinstance(event, dict) else None


def _usage_block_readable(event: dict) -> bool:
    data = event.get("data")
    usage = data.get("usage") if isinstance(data, dict) else None
    if not isinstance(usage, dict):
        return False
    return any(_number(usage.get(name)) is not None for name in _TOKEN_FIELDS)


def _number(value) -> Optional[int]:
    """An integer figure, or None. Booleans and non-integral numbers are not figures."""
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value == value and abs(value) != float("inf") and value == int(value):
        return int(value)
    return None


def _text(value) -> Optional[str]:
    return value if isinstance(value, str) and value else None


def _agent_id(event: dict) -> Optional[str]:
    return _text(event.get("agentId"))


def _provider(block: dict) -> Optional[str]:
    provider = block.get("provider")
    if isinstance(provider, dict) and "copilot" in (provider.get("id"), provider.get("kind")):
        return "github-copilot"
    return None


def _format_time(moment: datetime.datetime) -> str:
    return "%04d-%02d-%02dT%02d:%02d:%02d.%03dZ" % (
        moment.year, moment.month, moment.day, moment.hour, moment.minute, moment.second,
        moment.microsecond // 1000)


def _parse_time(value) -> Optional[datetime.datetime]:
    if not isinstance(value, str):
        return None
    match = _TIMESTAMP.match(value)
    if not match:
        return None
    year, month, day, hour, minute, second, fraction = match.groups()
    micro = int(((fraction or "") + "000000")[:6])
    try:
        return datetime.datetime(int(year), int(month), int(day), int(hour), int(minute), int(second), micro)
    except ValueError:
        return None


def _start_time(event: dict, block: dict, key: Optional[str], problems: list) -> Optional[str]:
    """Call start: envelope timestamp minus duration; the envelope timestamp when duration is unreadable."""
    end = _parse_time(event.get("timestamp"))
    if end is None:
        problems.append(FieldProblem(key, "timestamp"))
        return None
    duration = block.get("duration")
    valid = not isinstance(duration, bool) and isinstance(duration, (int, float)) \
        and duration == duration and 0 <= duration < 1e12
    if not valid:
        problems.append(FieldProblem(key, "duration"))
        return _format_time(end)
    try:
        return _format_time(end - datetime.timedelta(milliseconds=duration))
    except OverflowError:
        problems.append(FieldProblem(key, "duration"))
        return _format_time(end)


def _token_usage(block: dict, key: str, problems: list, with_reasoning: bool) -> Optional[dict]:
    """Map transcript token figures to the log-format token_usage block."""
    figures = {}
    for name in _TOKEN_FIELDS:
        figures[name] = _number(block.get(name))
        needed = name in ("inputTokens", "cacheReadTokens", "cacheWriteTokens")
        if figures[name] is None and (needed or name in block) and (with_reasoning or name != "reasoningTokens"):
            problems.append(FieldProblem(key, name))
    usage = {}
    cache_read, cache_write, total = figures["cacheReadTokens"], figures["cacheWriteTokens"], figures["inputTokens"]
    if None not in (cache_read, cache_write, total):
        uncached = total - cache_read - cache_write
        if uncached >= 0:
            usage["input_tokens"] = uncached
        else:
            problems.append(FieldProblem(key, "inputTokens"))
    if cache_read is not None:
        usage["cache_read_tokens"] = cache_read
    if cache_write is not None:
        usage["cache_creation_tokens"] = cache_write
    if figures["outputTokens"] is not None:
        usage["output_tokens"] = figures["outputTokens"]
    reasoning = figures["reasoningTokens"]
    if with_reasoning and reasoning is not None:
        if figures["outputTokens"] is not None and reasoning <= figures["outputTokens"]:
            usage["reasoning_tokens"] = reasoning
        else:
            problems.append(FieldProblem(key, "reasoningTokens"))
    return usage or None


def _requested_tools(block: dict, key: str, problems: list) -> Optional[dict]:
    counts = block.get("toolCounts")
    if not isinstance(counts, dict):
        problems.append(FieldProblem(key, "toolCounts"))
        return None
    tools = {}
    for name, count in counts.items():
        value = _number(count)
        if not name or value is None or value < 0:
            problems.append(FieldProblem(key, "toolCounts"))
            return None
        if value >= 1:
            tools[name] = value
    return tools


def _record_key(event: dict, own_id, prefix: str, field_name: str, problems: list) -> Optional[str]:
    """Record key from the event's own id, else from the envelope id; None when neither exists."""
    if _text(own_id):
        return prefix + own_id
    envelope_id = _text(event.get("id"))
    if envelope_id is None:
        problems.append(FieldProblem(None, field_name))
        return None
    key = prefix + "event:" + envelope_id
    problems.append(FieldProblem(key, field_name))
    return key


def _read_usage_event(event: dict, ordinal: int, result: TranscriptRead) -> None:
    data = event.get("data")
    block = data.get("usage") if isinstance(data, dict) else None
    if not isinstance(block, dict):
        return
    problems: list = []
    key = _record_key(event, block.get("apiCallId"), "", "apiCallId", problems)
    started = _start_time(event, block, key, problems) if key is not None else None
    if key is None or started is None:
        result.problems.extend(problems)
        return
    result.calls.append(UsageCall(
        record_id=key, record_index=ordinal, agent_id=_agent_id(event), timestamp=started,
        model=_text(block.get("model")), provider=_provider(block), effort=_text(block.get("reasoningEffort")),
        token_usage=_token_usage(block, key, problems, True),
        requested_tools=_requested_tools(block, key, problems)))
    result.problems.extend(problems)


def _read_compaction_event(event: dict, ordinal: int, result: TranscriptRead) -> None:
    data = event.get("data")
    data = data if isinstance(data, dict) else {}
    problems: list = []
    key = _record_key(event, data.get("compactionAttemptId"), "compaction:", "compactionAttemptId", problems)
    completed = _parse_time(event.get("timestamp"))
    if key is not None and completed is None:
        problems.append(FieldProblem(key, "timestamp"))
    if key is None or completed is None:
        result.problems.extend(problems)
        return
    trigger = data.get("trigger")
    result.compactions.append(CompactionFacts(
        record_id=key, record_index=ordinal, agent_id=_agent_id(event), completed_at=_format_time(completed),
        applied=data.get("success") is True and data.get("outcome") == "applied",
        trigger=_TRIGGERS.get(trigger) if isinstance(trigger, str) else None,
        tokens_before=_number(data.get("preCompactionTokens")),
        tokens_after=_number(data.get("postCompactionTokens")),
        usage=_compaction_usage(event, data.get("compactionTokensUsed"), key, problems)))
    result.problems.extend(problems)


def _compaction_usage(event: dict, cost, key: str, problems: list) -> Optional[CompactionUsage]:
    if not isinstance(cost, dict):
        return None
    started = _start_time(event, cost, key, problems)
    if started is None:
        return None
    return CompactionUsage(timestamp=started, model=_text(cost.get("model")), provider=_provider(cost),
                           token_usage=_token_usage(cost, key, problems, False))
