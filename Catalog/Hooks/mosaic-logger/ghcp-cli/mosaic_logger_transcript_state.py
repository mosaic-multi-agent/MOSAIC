"""mosaic_logger_transcript_state.py - What transcript logging has already written for a session.

One append-only file per transcript session, OrchestrationLogs/.transcript-logged/<session>.jsonl.
Each line is {"k": <item key>} (an item written, or a diagnostic reported) or
{"t": <transcript path>} (where the session transcript lives, for read points whose payload
carries no path). A key is added only after the append it stands for succeeded, so a crash
or a dropped append leaves the item eligible for a later read.

The caller holds the state file's lock (holding_lock) from load to close, which makes the
read-write cycle exclusive across hook processes.
"""

import hashlib
import json
import pathlib

import mosaic_logger_core as core

STATE_DIRNAME = ".transcript-logged"


def state_path(paths: "core.LogPaths", session_id: "str | None", transcript_path: "str | None") -> "pathlib.Path | None":
    """The state file of a session; keyed by the transcript path when the payload has no session id."""
    if session_id:
        name = core.sanitize_component(session_id)
    elif transcript_path:
        name = "path-" + hashlib.sha1(transcript_path.encode("utf-8", "replace")).hexdigest()[:16]
    else:
        return None
    return paths.root / STATE_DIRNAME / (name + ".jsonl")


class LoggedState:
    def __init__(self, path: pathlib.Path):
        self.path = path
        self.keys: set = set()
        self.transcript: "str | None" = None
        self._handle = None

    def load(self) -> "LoggedState":
        try:
            with open(self.path, "rb") as handle:
                for raw in handle.read().split(b"\n"):
                    self._absorb(raw)
        except OSError:
            pass
        return self

    def _absorb(self, raw: bytes) -> None:
        try:
            entry = json.loads(raw.decode("utf-8"))
        except Exception:
            return
        if not isinstance(entry, dict):
            return
        if isinstance(entry.get("k"), str):
            self.keys.add(entry["k"])
        if isinstance(entry.get("t"), str):
            self.transcript = entry["t"]

    def has(self, key: str) -> bool:
        return key in self.keys

    def mark(self, key: str) -> None:
        if self._write({"k": key}):
            self.keys.add(key)

    def remember_transcript(self, path: str) -> None:
        if path != self.transcript and self._write({"t": path}):
            self.transcript = path

    def _write(self, entry: dict) -> bool:
        try:
            if self._handle is None:
                self.path.parent.mkdir(parents=True, exist_ok=True)
                self._handle = open(self.path, "ab")
            self._handle.write((json.dumps(entry, ensure_ascii=False) + "\n").encode("utf-8"))
            self._handle.flush()
            return True
        except Exception as exc:
            core.debug_log("transcript state write failed", exc)
            return False

    def close(self) -> None:
        try:
            if self._handle is not None:
                self._handle.close()
        except Exception:
            pass
        self._handle = None
