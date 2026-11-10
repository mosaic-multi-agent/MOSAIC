"""The shared .otel-ingest/state.json contract: cursor, placements, pending spans, links, version.

This module mirrors Tools/Common/otelingest/state.go. Go and Python read and write the same file.
Standard library only; deployed flat next to the adapter modules.
"""

import hashlib
import json
import os
import tempfile

from mosaic_logger_otel_contract import STATE_DIR_NAME, STATE_VERSION
from mosaic_logger_otel_parse import (
    call_from_pending, link_from_state, loads_strict, optional_string,
)

STATE_FILE_NAME = "state.json"
HEAD_BYTES = 4096


class _NumberText(str):
    """A JSON number kept as its literal text."""


class IngestState:
    """The content of .otel-ingest/state.json."""

    def __init__(self):
        self.cursor = None
        self.placements = {}
        self.pending = []
        self.links = {}

    def to_json(self):
        data = {"state_version": STATE_VERSION}
        if self.cursor is not None:
            data["cursor"] = self.cursor
        data["placements"] = self.placements
        data["pending"] = [call.to_pending() for call in self.pending]
        data["links"] = self.links
        return data


def state_path(run_dir):
    return os.path.join(run_dir, STATE_DIR_NAME, STATE_FILE_NAME)


def _raw_version(text):
    """The state_version as written in the file ("" when absent or the file is not an object)."""
    try:
        head = json.loads(text, parse_int=_NumberText, parse_float=_NumberText)
    except (ValueError, RecursionError):
        return ""
    if not isinstance(head, dict) or "state_version" not in head:
        return ""
    version = head["state_version"]
    if isinstance(version, _NumberText):
        return str(version)
    return json.dumps(version, separators=(",", ":"), ensure_ascii=False)


def load_state(run_dir):
    """(state, present, unknown). present is False when there is no state; unknown carries the raw
    version text of a state this implementation does not understand. Raises on an unreadable or
    malformed known state."""
    try:
        with open(state_path(run_dir), "rb") as handle:
            data = handle.read()
    except FileNotFoundError:
        return IngestState(), False, None
    text = data.decode("utf-8", errors="replace")
    raw = _raw_version(text)
    if raw != str(STATE_VERSION):
        return None, True, raw
    return _state_from_json(loads_strict(text)), True, None


def _state_from_json(obj):
    if not isinstance(obj, dict):
        raise ValueError("state is not an object")
    state = IngestState()
    state.cursor = _cursor_from_json(obj.get("cursor"))
    placements = obj.get("placements")
    if placements is not None:
        if not isinstance(placements, dict):
            raise ValueError("placements is not an object")
        state.placements = {str(k): optional_string({"v": v}, "v") for k, v in placements.items()}
    pending = obj.get("pending")
    if pending is not None:
        if not isinstance(pending, list):
            raise ValueError("pending is not a list")
        state.pending = [call_from_pending(item) for item in pending]
    links = obj.get("links")
    if links is not None:
        if not isinstance(links, dict):
            raise ValueError("links is not an object")
        state.links = {str(k): link_from_state(v) for k, v in links.items()}
    return state


def _cursor_from_json(raw):
    if raw is None:
        return None
    if not isinstance(raw, dict):
        raise ValueError("cursor is not an object")
    offset, head_len = raw.get("offset", 0), raw.get("head_len", 0)
    if type(offset) is not int or type(head_len) is not int:
        raise ValueError("cursor numbers are not integers")
    return {"offset": offset, "head_len": head_len,
            "head_sha256": optional_string(raw, "head_sha256")}


def write_json_atomically(directory, final_path, data, prefix):
    """Write the JSON data to final_path through a temporary file in directory, then rename."""
    text = json.dumps(data, indent=2)
    handle, tmp_path = tempfile.mkstemp(dir=directory, prefix=prefix, suffix=".tmp")
    try:
        with os.fdopen(handle, "wb") as out:
            out.write(text.encode("ascii"))
        os.replace(tmp_path, final_path)
    except BaseException:
        try:
            os.remove(tmp_path)
        except OSError:
            pass
        raise


def save_state(run_dir, state):
    """Write the state atomically (temporary file, then rename)."""
    directory = os.path.join(run_dir, STATE_DIR_NAME)
    os.makedirs(directory, exist_ok=True)
    write_json_atomically(directory, state_path(run_dir), state.to_json(), "state-")


def head_digest(path, size):
    """Length and SHA-256 of the first min(HEAD_BYTES, size) bytes of the file."""
    length = min(HEAD_BYTES, size)
    with open(path, "rb") as handle:
        content = handle.read(length)
    return length, hashlib.sha256(content).hexdigest()


def cursor_valid(cursor, path, size):
    """Whether the stored cursor still describes the file: it is not shorter than the cursor and its
    head is unchanged."""
    if cursor is None or size < cursor["offset"] or size < cursor["head_len"]:
        return False
    try:
        length, digest = head_digest(path, cursor["head_len"])
    except OSError:
        return False
    return length == cursor["head_len"] and digest == cursor["head_sha256"]
