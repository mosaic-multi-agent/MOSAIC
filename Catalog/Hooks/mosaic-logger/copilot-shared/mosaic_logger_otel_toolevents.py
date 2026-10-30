"""Tool-call starts and ends of the stream files of one run, and the matching of a failed span to a start.

This module mirrors Tools/Common/otelingest/toolend_events.go.
Standard library only; deployed flat next to the adapter modules.
"""

import os
import re
from collections import namedtuple

from mosaic_logger_otel_keys import INVOCATION_STREAM, ORCHESTRATOR_STREAM
from mosaic_logger_otel_parse import DIALECT_CLI, loads_strict

ToolEvent = namedtuple("ToolEvent", "event call_id tool_name harness session_id")

# The suffix the VS Code hook appends to a logged call id.
_VSCODE_CALL_SUFFIX = re.compile(r"__vscode-[0-9]+$")


class ToolStream:
    """The tool calls of one stream file: the starts in file order and the call ids that have an end."""

    def __init__(self, path):
        self.path = path
        self.starts = []
        self.ends = set()

    def first_start(self, tool):
        """The first start in the stream that belongs to the failed span, or None."""
        for start in self.starts:
            if matches(tool.dialect, start.call_id, tool.call_id):
                return start
        return None


def stream_paths(run_dir):
    """The stream files that can hold tool events: the orchestrator stream and the invocation streams."""
    paths = [os.path.join(run_dir, ORCHESTRATOR_STREAM)]
    try:
        names = sorted(os.listdir(run_dir))
    except OSError:
        names = []
    for name in names:
        if not name.startswith(".") and os.path.isdir(os.path.join(run_dir, name)):
            paths.append(os.path.join(run_dir, name, INVOCATION_STREAM))
    return paths


def _tool_event(line):
    try:
        obj = loads_strict(line.decode("utf-8", errors="replace"))
    except (ValueError, RecursionError):
        return None
    if obj is None:
        return ToolEvent("", "", "", "", "")
    if not isinstance(obj, dict):
        return None
    values = []
    for name in ToolEvent._fields:
        value = obj.get(name)
        if value is not None and not isinstance(value, str):
            return None
        values.append(value or "")
    return ToolEvent(*values)


def read_tool_stream(path):
    """Read the tool starts and ends of one stream file; a missing file is an empty stream."""
    stream = ToolStream(path)
    try:
        handle = open(path, "rb")
    except OSError:
        return stream
    with handle:
        for line in handle:
            if not line.strip():
                continue
            event = _tool_event(line)
            if event is None or event.call_id == "":
                continue
            if event.event == "tool_call_start":
                stream.starts.append(event)
            elif event.event == "tool_call_end":
                stream.ends.add(event.call_id)
    return stream


def matches(dialect, start_call_id, span_call_id):
    """Whether a logged start call id belongs to the span call id: verbatim for a GHCP CLI span, and
    after removing the VS Code suffix for a VS Code span."""
    if dialect == DIALECT_CLI:
        return start_call_id == span_call_id
    return _VSCODE_CALL_SUFFIX.sub("", start_call_id) == span_call_id
