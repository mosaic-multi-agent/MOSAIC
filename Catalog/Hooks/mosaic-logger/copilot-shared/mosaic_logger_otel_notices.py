"""The once-per-run notice record (.otel-ingest/notices.json), shared with the Go implementation.

This module mirrors Tools/Common/otelingest/notices.go. The record is shaped
{"reported": {"<kind>": "<ISO time first reported>"}}.
Standard library only; deployed flat next to the adapter modules.
"""

import datetime
import os

from mosaic_logger_otel_contract import STATE_DIR_NAME, format_time
from mosaic_logger_otel_parse import loads_strict
from mosaic_logger_otel_state import write_json_atomically

NOTICES_FILE_NAME = "notices.json"


def _read_reported(path):
    """The reported map of the record; empty when there is no record. Raises when it is unreadable."""
    try:
        with open(path, "rb") as handle:
            data = handle.read()
    except FileNotFoundError:
        return {}
    obj = loads_strict(data.decode("utf-8", errors="replace"))
    if obj is None:
        return {}
    if not isinstance(obj, dict):
        raise ValueError("notice record is not an object")
    reported = obj.get("reported")
    if reported is None:
        return {}
    if not isinstance(reported, dict) or any(v is not None and not isinstance(v, str)
                                             for v in reported.values()):
        raise ValueError("notice record has an unexpected shape")
    return {k: v or "" for k, v in reported.items()}


def _write_reported(directory, path, reported):
    try:
        os.mkdir(directory)
    except FileExistsError:
        pass
    write_json_atomically(directory, path, {"reported": reported}, "notices-")


def report_once(run_dir, kind):
    """Record that a once-per-run notice of kind is being reported for the run and tell the caller
    whether it is the first. When the record cannot be read or written it reports True: a duplicate
    notice is preferred over a lost one. Never raises."""
    try:
        directory = os.path.join(run_dir, STATE_DIR_NAME)
        path = os.path.join(directory, NOTICES_FILE_NAME)
        reported = _read_reported(path)
        if kind in reported:
            return False
        reported[kind] = format_time(datetime.datetime.now(datetime.timezone.utc))
        try:
            _write_reported(directory, path, reported)
        except Exception:
            pass
        return True
    except Exception:
        return True
