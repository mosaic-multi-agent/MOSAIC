"""Attribution keys of one run, read from the run's MOSAIC log events.

This module mirrors Tools/Common/otelingest/keys.go.
Standard library only; deployed flat next to the adapter modules.
"""

import os
from collections import namedtuple

from mosaic_logger_otel_parse import loads_strict

ORCHESTRATOR_STREAM = "00_orchestrator_events.jsonl"
INVOCATION_STREAM = "03_events.jsonl"

LogEvent = namedtuple("LogEvent", "event source harness session_id spawn_call_id")


def _append_unique(mapping, key, value):
    values = mapping.setdefault(key, [])
    if value not in values:
        values.append(value)


class RunKeys:
    """Session ids of the orchestrator and invocation streams plus invocation spawn_call_ids."""

    def __init__(self):
        self.orch_sessions = set()
        self.inv_by_session = {}
        self.inv_by_spawn = {}
        self.harness_by_session = {}
        self.harness_by_inv = {}
        self.first_harness = ""

    def note_harness(self, session, harness):
        if not harness:
            return
        self.harness_by_session.setdefault(session, harness)
        if not self.first_harness:
            self.first_harness = harness

    def known(self, session):
        """Whether a conversation or session id belongs to the run."""
        return bool(session) and (session in self.orch_sessions or bool(self.inv_by_session.get(session)))

    def conversation(self, call):
        """The run session a call belongs to: its conversation id, else its parent chat session."""
        if self.known(call.conversation_id):
            return call.conversation_id
        if self.known(call.parent_chat_session_id):
            return call.parent_chat_session_id
        return ""

    def tied(self, conversation, chat_session, parent_chat_session):
        """Whether any hard key ties the span to the run."""
        return (self.known(conversation) or self.known(parent_chat_session)
                or (bool(chat_session) and bool(self.inv_by_spawn.get(chat_session))))

    def session_ids(self):
        return sorted(self.orch_sessions | set(self.inv_by_session))


def each_log_event(path):
    """Parsable events of a JSONL log, skipping OTel-sourced usage records so the ingest never
    derives keys from its own output."""
    try:
        handle = open(path, "rb")
    except OSError:
        return
    with handle:
        for line in handle:
            if not line.strip():
                continue
            event = _log_event(line)
            if event is not None and not (event.event == "usage_record" and event.source == "otel"):
                yield event


def _log_event(line):
    try:
        obj = loads_strict(line.decode("utf-8", errors="replace"))
    except (ValueError, RecursionError):
        return None
    if obj is None:
        return LogEvent("", "", "", "", "")
    if not isinstance(obj, dict):
        return None
    values = []
    for name in LogEvent._fields:
        value = obj.get(name)
        if value is not None and not isinstance(value, str):
            return None
        values.append(value or "")
    return LogEvent(*values)


def read_run_keys(run_dir):
    """Read the attribution keys from the log folder of a run."""
    keys = RunKeys()
    for event in each_log_event(os.path.join(run_dir, ORCHESTRATOR_STREAM)):
        if event.session_id:
            keys.orch_sessions.add(event.session_id)
            keys.note_harness(event.session_id, event.harness)
    try:
        names = sorted(os.listdir(run_dir))
    except OSError:
        names = []
    for name in names:
        folder = os.path.join(run_dir, name)
        if name.startswith(".") or not os.path.isdir(folder):
            continue
        for event in each_log_event(os.path.join(folder, INVOCATION_STREAM)):
            if event.session_id:
                _append_unique(keys.inv_by_session, event.session_id, name)
                keys.note_harness(event.session_id, event.harness)
            if event.spawn_call_id:
                _append_unique(keys.inv_by_spawn, event.spawn_call_id, name)
            if name not in keys.harness_by_inv and event.harness:
                keys.harness_by_inv[name] = event.harness
    return keys
