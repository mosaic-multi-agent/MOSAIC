"""Dialect reading and conversion of one OTel line into a link or a call.

This module mirrors Tools/Common/otelingest/ (span.go, convert.go and the call shape of attribute.go).
Standard library only; deployed flat next to the adapter modules.
"""

import datetime
import json
import re
from collections import namedtuple

from mosaic_logger_otel_contract import PROVIDER_NAME, format_time

USAGE_MARKER = b'"gen_ai.operation.name"'
DIALECT_CLI = "cli"
DIALECT_VSCODE = "vscode"
KIND_INVOKE_AGENT = "invoke_agent"
KIND_EXECUTE_TOOL = "execute_tool"
SPAWN_TOOL_NAMES = {"task", "runSubagent", "execution_subagent"}
STATUS_CODE_ERROR = 2
INT64_MIN =-(2 ** 63)
INT64_MAX = 2 ** 63 - 1
TOKEN_FIELDS = ("input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens",
                "reasoning_tokens")

_NOT_AN_INTEGER = object()

ParsedLine = namedtuple("ParsedLine", "malformed link_id link call tool", defaults=(None,))
_NOTHING = ParsedLine(False, "", None, None)
_MALFORMED = ParsedLine(True, "", None, None)


class Call:
    """One API call (a per-call chat span with usage) waiting for placement."""

    def __init__(self, record_id, record_index, dialect, token_usage):
        self.record_id = record_id
        self.record_index = record_index
        self.dialect = dialect
        self.token_usage = token_usage
        self.parent_span_id = ""
        self.conversation_id = ""
        self.chat_session_id = ""
        self.parent_chat_session_id = ""
        self.model = ""
        self.effort = ""
        self.timestamp = ""
        self.call_label = ""
        self.call_kind = ""

    def to_pending(self):
        """The pending-span shape stored in the state."""
        record = {"record_id": self.record_id, "record_index": self.record_index}
        if self.model:
            record["model"] = self.model
        record["provider"] = PROVIDER_NAME
        if self.effort:
            record["effort"] = self.effort
        record["token_usage"] = self.token_usage
        for key in ("timestamp", "call_label", "call_kind"):
            if getattr(self, key):
                record[key] = getattr(self, key)
        out = {"record": record}
        for key in ("parent_span_id", "conversation_id", "chat_session_id", "parent_chat_session_id"):
            if getattr(self, key):
                out[key] = getattr(self, key)
        return out


def _reject_constant(name):
    raise ValueError("invalid JSON constant " + name)


def _parse_int(text):
    return int(text) if len(text) <= 20 else _NOT_AN_INTEGER


def loads_strict(text):
    """Parse JSON as Go does: no NaN or Infinity, and fractional numbers are kept apart from integers."""
    return json.loads(text, parse_float=lambda _text: _NOT_AN_INTEGER, parse_int=_parse_int,
                      parse_constant=_reject_constant)


def string_value(value):
    return value if isinstance(value, str) else ""


def int_value(value):
    """The value when it is a JSON integer within int64, else None."""
    if type(value) is int and INT64_MIN <= value <= INT64_MAX:
        return value
    return None


def optional_string(obj, key):
    """A string field of a state object; absent or null is empty, any other type is an error."""
    value = obj.get(key)
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError(key + " is not a string")
    return value


def call_from_pending(data):
    """Read a call from the pending-span shape of the state."""
    record = data["record"]
    if not isinstance(record, dict):
        raise ValueError("pending record is not an object")
    index = record.get("record_index", 0)
    if type(index) is not int:
        raise ValueError("record_index is not an integer")
    usage = _usage_from_state(record.get("token_usage"))
    chat_session = optional_string(data, "chat_session_id")
    call = Call(optional_string(record, "record_id"), index,
                DIALECT_VSCODE if chat_session else DIALECT_CLI, usage)
    call.model = optional_string(record, "model")
    call.effort = optional_string(record, "effort")
    call.timestamp = optional_string(record, "timestamp")
    call.call_label = optional_string(record, "call_label")
    call.call_kind = optional_string(record, "call_kind")
    call.parent_span_id = optional_string(data, "parent_span_id")
    call.conversation_id = optional_string(data, "conversation_id")
    call.chat_session_id = chat_session
    call.parent_chat_session_id = optional_string(data, "parent_chat_session_id")
    return call


def _usage_from_state(raw):
    if raw is None:
        return None
    if not isinstance(raw, dict):
        raise ValueError("token_usage is not an object")
    usage = {}
    for key in TOKEN_FIELDS:
        value = raw.get(key)
        if value is None:
            continue
        if int_value(value) is None:
            raise ValueError(key + " is not an integer")
        usage[key] = value
    return usage


def link_from_state(data):
    """Validate a link read from the state and return it with only its known string fields."""
    if not isinstance(data, dict):
        raise ValueError("link is not an object")
    link = {"kind": optional_string(data, "kind")}
    for key in ("parent_span_id", "conversation_id", "chat_session_id", "parent_chat_session_id",
                "tool_call_id", "tool_name"):
        value = optional_string(data, key)
        if value:
            link[key] = value
    return link


def make_link(kind, fields, tool_call_id="", tool_name=""):
    """A link record of the state; empty fields are omitted."""
    link = {"kind": kind}
    for key, value in (("parent_span_id", fields[0]), ("conversation_id", fields[1]),
                       ("chat_session_id", fields[2]), ("parent_chat_session_id", fields[3]),
                       ("tool_call_id", tool_call_id), ("tool_name", tool_name)):
        if value:
            link[key] = value
    return link


def parse_line(line, offset):
    """Parse one complete line that contains the usage marker."""
    try:
        obj = loads_strict(line.decode("utf-8", errors="replace"))
    except (ValueError, RecursionError):
        return _MALFORMED
    if not isinstance(obj, dict):
        return _MALFORMED
    dialect = DIALECT_VSCODE
    if isinstance(obj.get("type"), str):
        if obj["type"] != "span":
            return _NOTHING
        dialect = DIALECT_CLI
    span_id = string_value(obj.get("spanId"))
    attrs = obj.get("attributes")
    if span_id == "" or not isinstance(attrs, dict):
        return _NOTHING
    parent = string_value(obj.get("parentSpanId"))
    context = obj.get("parentSpanContext")
    if isinstance(context, dict) and parent == "":
        parent = string_value(context.get("spanId"))
    fields = (parent, string_value(attrs.get("gen_ai.conversation.id")),
              string_value(attrs.get("copilot_chat.chat_session_id")),
              string_value(attrs.get("copilot_chat.parent_chat_session_id")))
    operation = string_value(attrs.get("gen_ai.operation.name"))
    if operation == "chat":
        call = convert_chat(attrs, dialect)
        if call is None:
            return _NOTHING
        call.record_id, call.record_index = span_id, offset
        call.timestamp = start_timestamp(obj.get("startTime"))
        (call.parent_span_id, call.conversation_id, call.chat_session_id,
         call.parent_chat_session_id) = fields
        return ParsedLine(False, "", None, call)
    if operation == KIND_INVOKE_AGENT:
        return ParsedLine(False, span_id, make_link(KIND_INVOKE_AGENT, fields), None)
    if operation == KIND_EXECUTE_TOOL:
        name = string_value(attrs.get("gen_ai.tool.name"))
        tool = failed_tool_of(obj, attrs, dialect)
        if name in SPAWN_TOOL_NAMES:
            link = make_link(KIND_EXECUTE_TOOL, fields,
                             string_value(attrs.get("gen_ai.tool.call.id")), name)
            return ParsedLine(False, span_id, link, None, tool)
        return ParsedLine(False, "", None, None, tool)
    return _NOTHING


class FailedTool:
    """A failed execute_tool span with a call id: the facts the end of its tool call is written from."""

    def __init__(self, dialect, call_id, tool_name, message, end_time):
        self.dialect = dialect
        self.call_id = call_id
        self.tool_name = tool_name
        self.message = message
        self.end_time = end_time  # formatted span end time; "" when the span has no usable endTime


def failed_tool_of(obj, attrs, dialect):
    """The failed tool call a span describes, or None when the span did not fail or has no call
    id. Content attributes (arguments, results) are never read."""
    status = obj.get("status")
    if not isinstance(status, dict):
        status = {}
    if int_value(status.get("code")) != STATUS_CODE_ERROR:
        return None
    call_id = string_value(attrs.get("gen_ai.tool.call.id"))
    if call_id == "":
        return None
    return FailedTool(dialect, call_id, string_value(attrs.get("gen_ai.tool.name")),
                      string_value(status.get("message")), start_timestamp(obj.get("endTime")))


def convert_chat(attrs, dialect):
    """Map the usage, model and effort attributes of a chat span; None when it carries no usage."""
    usage = convert_tokens(attrs)
    if usage is None:
        return None
    call = Call("", 0, dialect, usage)
    call.model = (string_value(attrs.get("gen_ai.response.model"))
                  or string_value(attrs.get("gen_ai.request.model")))
    if dialect == DIALECT_CLI:
        call.effort = string_value(attrs.get("gen_ai.request.reasoning.level"))
    else:
        call.effort = vscode_effort(string_value(attrs.get("copilot_chat.request.options")))
        call.call_label = string_value(attrs.get("gen_ai.agent.name"))
        call.call_kind = call_kind_of(call.call_label)
    return call


_RETRY_PREFIX = re.compile(r"^retry-[A-Za-z]+-")
_MAIN_LABELS = ("panel/editAgent", "tool/runSubagent")
_AUXILIARY_LABELS = ("backgroundTodoAgent", "title", "progressMessages")


def call_kind_of(label):
    """Map a VS Code chat span's purpose name to a call kind ("" when unknown). Leading
    retry-<letters>- prefixes are ignored; the comparison is case-sensitive."""
    name = label
    while True:
        stripped = _RETRY_PREFIX.sub("", name, count=1)
        if stripped == name:
            break
        name = stripped
    if name in _MAIN_LABELS or name.startswith("tool/runSubagent-"):
        return "main"
    if name.startswith("summarizeConversationHistory"):
        return "compaction"
    if name in _AUXILIARY_LABELS or name.startswith("copilotLanguageModelWrapper/"):
        return "auxiliary"
    return ""


def start_timestamp(value):
    """A span's startTime ([seconds, nanoseconds], two integers) as a UTC timestamp truncated to
    milliseconds; "" for any other shape (the caller then uses the ingest moment)."""
    if not isinstance(value, list) or len(value) != 2:
        return ""
    seconds, nanos = int_value(value[0]), int_value(value[1])
    if seconds is None or nanos is None:
        return ""
    try:
        epoch = datetime.datetime(1970, 1, 1, tzinfo=datetime.timezone.utc)
        moment = epoch + datetime.timedelta(seconds=seconds, microseconds=nanos // 1000)
    except OverflowError:
        return ""
    return format_time(moment)


def _first_int(attrs, *keys):
    for key in keys:
        value = int_value(attrs.get(key))
        if value is not None:
            return value
    return None


def convert_tokens(attrs):
    """Token mapping. The reported input includes the cache, so the record's input is the reported
    input minus cache read and cache write; a negative difference is not recorded."""
    output = int_value(attrs.get("gen_ai.usage.output_tokens"))
    cache_read = int_value(attrs.get("gen_ai.usage.cache_read.input_tokens"))
    cache_creation = _first_int(attrs, "gen_ai.usage.cache_write.input_tokens",
                                "gen_ai.usage.cache_creation.input_tokens")
    reasoning = _first_int(attrs, "gen_ai.usage.reasoning.output_tokens",
                           "gen_ai.usage.reasoning_tokens")
    reported = int_value(attrs.get("gen_ai.usage.input_tokens"))
    input_tokens = None
    if reported is not None:
        diff = reported - (cache_read or 0) - (cache_creation or 0)
        if diff >= 0:
            input_tokens = diff
    values = (input_tokens, output, cache_read, cache_creation, reasoning)
    if all(v is None for v in values):
        return None
    return {k: v for k, v in zip(TOKEN_FIELDS, values) if v is not None}


def vscode_effort(options):
    """Effort verbatim from the request options JSON: output_config.effort before reasoning.effort."""
    if options == "":
        return ""
    try:
        parsed = loads_strict(options)
    except (ValueError, RecursionError):
        return ""
    if not isinstance(parsed, dict):
        return ""
    sections = []
    for key in ("output_config", "reasoning"):
        section = parsed.get(key)
        if section is not None and not isinstance(section, dict):
            return ""
        sections.append(section or {})
    return string_value(sections[0].get("effort")) or string_value(sections[1].get("effort"))
