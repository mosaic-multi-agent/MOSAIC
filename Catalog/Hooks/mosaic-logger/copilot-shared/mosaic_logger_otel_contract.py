"""Shared constants, result types and time formatting of the Copilot OTel ingest.

This module mirrors the contract of the Go package Tools/Common/otelingest/ (contract.go, diag/).
Standard library only; deployed flat next to the adapter modules.
"""

import datetime
import re

ENV_VAR = "COPILOT_OTEL_FILE_EXPORTER_PATH"
STATE_VERSION = 1
LOCK_STALE_AFTER_SECONDS = 600
DEFAULT_BUDGET_SECONDS = 5.0
STATE_DIR_NAME = ".otel-ingest"
RECORD_SCHEMA_VERSION = "1.7.0"
PROVIDER_NAME = "github-copilot"

_TIME_PATTERN = re.compile(r"^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$")


class Diagnostic:
    """One ingest diagnostic (kind string, message, counts, sorted ids)."""

    def __init__(self, kind, message="", counts=None, ids=None):
        self.kind = kind
        self.message = message
        self.counts = counts
        self.ids = ids


class IngestResult:
    """Outcome of one ingest."""

    def __init__(self, status, records_written=0, finalized=0, pending=0,
                 lines_read=0, lines_malformed=0, diagnostics=None, tool_ends_written=0):
        self.status = status
        self.records_written = records_written
        self.finalized = finalized
        self.pending = pending
        self.lines_read = lines_read
        self.lines_malformed = lines_malformed
        self.tool_ends_written = tool_ends_written  # tool_call_end lines written for failed tool calls
        self.diagnostics = diagnostics if diagnostics is not None else []


class PendingStatus:
    """Read-only view of a run's ingest state."""

    def __init__(self, state_present=False, state_known=False, pending=0):
        self.state_present = state_present
        self.state_known = state_known
        self.pending = pending


def to_utc(moment):
    """An aware UTC datetime; a naive one is taken to be UTC already."""
    if moment.tzinfo is None:
        return moment.replace(tzinfo=datetime.timezone.utc)
    return moment.astimezone(datetime.timezone.utc)


def format_time(moment):
    """ISO time with millisecond precision, e.g. 2026-10-28T09:00:00.000Z."""
    moment = to_utc(moment)
    return f"{moment:%Y-%m-%dT%H:%M:%S}.{moment.microsecond // 1000:03d}Z"


def parse_time(text):
    """Inverse of format_time; None when the text does not have exactly that shape."""
    if not isinstance(text, str) or not _TIME_PATTERN.match(text):
        return None
    try:
        parsed = datetime.datetime.strptime(text, "%Y-%m-%dT%H:%M:%S.%fZ")
    except ValueError:
        return None
    return parsed.replace(tzinfo=datetime.timezone.utc)
