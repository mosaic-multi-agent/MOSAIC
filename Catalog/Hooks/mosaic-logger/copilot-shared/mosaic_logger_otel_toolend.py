"""Writes the tool_call_end of failed tool calls from their execute_tool spans.

This module mirrors Tools/Common/otelingest/toolend.go.
Standard library only; deployed flat next to the adapter modules.
"""

import json
import os

from mosaic_logger_locked_append import holding_lock
from mosaic_logger_otel_contract import Diagnostic, RECORD_SCHEMA_VERSION, format_time
from mosaic_logger_otel_toolevents import read_tool_stream, stream_paths


def write_tool_ends(run_dir, now, tools, result):
    """Write one tool_call_end for every failed tool whose call has exactly one stream holding its
    start and no end there yet. Nothing is kept for failed tools without a start: a whole-file
    ingest matches them later."""
    if not tools:
        return
    run_id = os.path.basename(os.path.normpath(run_dir))
    streams = [read_tool_stream(path) for path in stream_paths(run_dir)]
    timed_out = 0
    for tool in tools:
        stream, start = _owning_stream(streams, tool)
        if stream is None or start.call_id in stream.ends:
            continue
        end = _tool_end_of(run_id, now, tool, start)
        outcome = _append_tool_end(stream, start, end)
        if outcome is None:
            timed_out += 1
            continue
        if outcome:
            result.tool_ends_written += 1
        stream.ends.add(start.call_id)
    if timed_out:
        result.diagnostics.append(Diagnostic(
            "append_timed_out",
            "lines were not written because the append lock wait limit was reached; "
            "a later ingest writes them",
            counts={"lines": timed_out}))


def _owning_stream(streams, tool):
    """The only stream that holds a start for the failed tool and that start; a start in two or
    more streams is ambiguous and gives none."""
    found = None
    found_start = None
    for stream in streams:
        start = stream.first_start(tool)
        if start is None:
            continue
        if found is not None:
            return None, None
        found, found_start = stream, start
    return found, found_start


def _tool_end_of(run_id, now, tool, start):
    """The end of a failed tool call, built from the span and the start it belongs to."""
    end = {
        "schema_version": RECORD_SCHEMA_VERSION,
        "event": "tool_call_end",
        "timestamp": tool.end_time or format_time(now),
        "harness": start.harness,
    }
    if start.session_id:
        end["session_id"] = start.session_id
    end["run_id"] = run_id
    end["call_id"] = start.call_id
    name = tool.tool_name or start.tool_name
    if name:
        end["tool_name"] = name
    end["status"] = "error"
    if tool.message:
        end["error"] = tool.message
    return end


def _append_tool_end(stream, start, end):
    """Append the end under the stream's lock, after checking again under the lock that a hook has
    not written an end meanwhile. True when written, False when a hook had, None when the lock was
    not obtained in time. An I/O error raises OSError."""
    line = json.dumps(end, separators=(",", ":")).encode("ascii") + b"\n"
    os.makedirs(os.path.dirname(stream.path), exist_ok=True)
    with holding_lock(stream.path) as held:
        if not held:
            return None
        if start.call_id in read_tool_stream(stream.path).ends:
            return False
        with open(stream.path, "ab") as handle:
            handle.write(line)
        return True
