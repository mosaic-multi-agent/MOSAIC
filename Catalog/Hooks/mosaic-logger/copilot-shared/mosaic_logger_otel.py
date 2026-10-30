"""Copilot OTel usage ingest for the mosaic-logger hook bundle (public entry point).

This module mirrors the Go package Tools/Common/otelingest/ (counterpart file:
Tools/Common/otelingest/ingest.go) and must behave identically to it on the same input, including
reading and writing the same .otel-ingest/ state, so a run can be ingested partly by hooks and
partly by the Go tools. Behaviour is defined by the shared conformance fixtures in
Tools/Common/otelingest/testdata/conformance/.

It reads an OTel JSONL file (GHCP CLI or VS Code dialect), turns each per-call chat span into a
usage_record with source "otel" and places it on the stream of the actor a hard key ties it to.

Standard library only; deployed flat next to the adapter modules. Never raises into the caller:
any failure becomes a diagnostic.
"""

import datetime
import os
import time

from mosaic_logger_otel_contract import (
    DEFAULT_BUDGET_SECONDS, ENV_VAR, LOCK_STALE_AFTER_SECONDS, STATE_VERSION, Diagnostic,
    IngestResult, PendingStatus, format_time,
)
from mosaic_logger_otel_keys import read_run_keys
from mosaic_logger_otel_lock import acquire_lock
from mosaic_logger_otel_notices import report_once as _report_once
from mosaic_logger_otel_place import Resolver, place_calls
from mosaic_logger_otel_scan import scan_file
from mosaic_logger_otel_toolend import write_tool_ends
from mosaic_logger_otel_state import (
    cursor_valid, head_digest, load_state, save_state, state_path,
)
from mosaic_logger_otel_write import Writer

__all__ = [
    "ENV_VAR", "STATE_VERSION", "LOCK_STALE_AFTER_SECONDS", "DEFAULT_BUDGET_SECONDS",
    "Diagnostic", "IngestResult", "PendingStatus", "ingest", "read_pending_status", "report_once",
]


def ingest(run_dir, otel_path, *, final=False, whole_file=False, start_offset=None,
           scope=None, budget_seconds=DEFAULT_BUDGET_SECONDS, clock=None, now=None):
    """Run one ingest of the OTel file into the run folder. Never raises.

    final: pending spans in scope go to the unattributed stream. whole_file: read from byte 0
    regardless of the stored cursor. start_offset: first cursor when the state has none yet.
    budget_seconds: time limit (None = unlimited) measured with clock (default time.monotonic);
    an ingest that runs out of time stops at a line boundary and leaves the rest for the next one.
    now: timestamp source for the records and the lock (default: current UTC time).
    """
    try:
        return _ingest(os.fspath(run_dir), os.fspath(otel_path), final, whole_file, start_offset,
                       scope, budget_seconds, clock or time.monotonic, now)
    except Exception as exc:
        return IngestResult("failed", diagnostics=[
            Diagnostic("internal_error", type(exc).__name__ + ": " + str(exc))])


def read_pending_status(run_dir):
    """Read the state without taking the lock and without writing. Never raises."""
    try:
        state, present, unknown = load_state(os.fspath(run_dir))
    except Exception:
        return PendingStatus(state_present=_state_file_exists(run_dir))
    if unknown is not None:
        return PendingStatus(state_present=present)
    return PendingStatus(state_present=present, state_known=present, pending=len(state.pending))


def report_once(run_dir, kind):
    """True when this is the first report of the notice kind for the run (see the notice record)."""
    return _report_once(run_dir, kind)


def _state_file_exists(run_dir):
    try:
        return os.path.exists(state_path(os.fspath(run_dir)))
    except Exception:
        return False


def _ingest(run_dir, otel_path, final, whole_file, start_offset, scope, budget_seconds, clock, now):
    moment = now() if now is not None else datetime.datetime.now(datetime.timezone.utc)
    started = clock() if budget_seconds is not None else 0.0
    try:
        size = os.stat(otel_path).st_size
    except FileNotFoundError:
        return IngestResult("file_missing", diagnostics=[
            Diagnostic("file_missing", "OTel file does not exist")])
    release, held = acquire_lock(run_dir, moment)
    if held:
        return IngestResult("lock_held", diagnostics=[
            Diagnostic("lock_held", "another ingest of this run holds the lock")])
    try:
        state, _present, unknown = load_state(run_dir)
        if unknown is not None:
            diagnostic = Diagnostic("unknown_state_version", "unknown .otel-ingest state version",
                                    ids=[unknown] if unknown else None)
            return IngestResult("unknown_state", diagnostics=[diagnostic])
        if budget_seconds is None:
            out_of_time = lambda: False
        else:
            out_of_time = lambda: clock() - started >= budget_seconds
        start = _start_offset(whole_file, start_offset, state, otel_path, size)
        return _ingest_into(run_dir, otel_path, size, moment, state, start, out_of_time,
                            final, scope)
    finally:
        release()


def _start_offset(whole_file, requested, state, otel_path, size):
    """Where reading starts: byte 0 for a whole-file recovery or when the stored cursor no longer
    describes the file, the stored cursor otherwise, and the requested first offset when there is
    no cursor yet."""
    if whole_file:
        return 0
    if state.cursor is not None:
        return state.cursor["offset"] if cursor_valid(state.cursor, otel_path, size) else 0
    if requested is not None and 0 <= requested <= size:
        return requested
    return 0


def _ingest_into(run_dir, otel_path, size, moment, state, start, out_of_time, final, scope):
    """Read the new lines, place every call it can and save the state."""
    scanned = scan_file(otel_path, start, out_of_time)
    keys = read_run_keys(run_dir)
    calls = list(state.pending)
    failed = []
    known_ids = {call.record_id for call in calls}
    for parsed in scanned.lines:
        if parsed.tool is not None:
            failed.append(parsed.tool)
        link = parsed.link
        if link is not None and keys.tied(link.get("conversation_id", ""), link.get("chat_session_id", ""),
                                          link.get("parent_chat_session_id", "")):
            state.links[parsed.link_id] = link
        call = parsed.call
        if call is not None and call.record_id not in known_ids:
            known_ids.add(call.record_id)
            if not call.timestamp:
                call.timestamp = format_time(moment)
            calls.append(call)
    result = IngestResult("ingested", lines_read=scanned.read, lines_malformed=scanned.malformed)
    writer = Writer(run_dir, moment, keys)
    # A cut-off ingest has not seen the rest of the file, so it must not finalise pending spans.
    place_calls(final and not scanned.cut, scope, state, Resolver(keys, state.links), writer, calls, result)
    write_tool_ends(run_dir, moment, failed, result)
    end = max(scanned.end, start)
    head_len, digest = head_digest(otel_path, max(size, end))
    state.cursor = {"offset": end, "head_len": head_len, "head_sha256": digest}
    save_state(run_dir, state)
    if scanned.malformed:
        result.diagnostics.append(Diagnostic(
            "lines_skipped", "lines that could not be parsed were skipped",
            counts={"malformed": scanned.malformed, "read": scanned.read}))
    if scanned.cut:
        result.diagnostics.append(Diagnostic(
            "budget_exhausted", "the time budget ran out; the rest is left for the next ingest",
            counts={"remaining_bytes": max(size - end, 0)}))
    result.diagnostics.sort(key=lambda d: d.kind)
    return result
