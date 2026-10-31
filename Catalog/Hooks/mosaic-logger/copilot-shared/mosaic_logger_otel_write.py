"""Appends usage_record events to the streams of one run.

This module mirrors Tools/Common/otelingest/write.go.
Standard library only; deployed flat next to the adapter modules.
"""

import json
import os

from mosaic_logger_locked_append import locked_append
from mosaic_logger_otel_contract import PROVIDER_NAME, RECORD_SCHEMA_VERSION, format_time
from mosaic_logger_otel_keys import INVOCATION_STREAM, ORCHESTRATOR_STREAM
from mosaic_logger_otel_place import DEST_INVOCATION, DEST_ORCHESTRATOR

UNATTRIBUTED_STREAM = "00_unattributed_events.jsonl"


def append_line(path, line):
    """Append one whole line under the stream's exclusive lock.

    Returns True when written and False when the lock was not obtained in time (the line is
    dropped). An I/O error raises OSError.
    """
    return locked_append(path, line, raise_os_errors=True)


class Writer:
    """Appends records to the streams of one run."""

    def __init__(self, run_dir, now, keys):
        self.run_dir = run_dir
        self.run_id = os.path.basename(os.path.normpath(run_dir))
        self.fallback_timestamp = format_time(now)
        self.keys = keys

    def write(self, call, dest):
        """Append the call as one whole line to the stream of dest."""
        session = self.keys.conversation(call)
        record = {
            "schema_version": RECORD_SCHEMA_VERSION,
            "event": "usage_record",
            "timestamp": getattr(call, "timestamp", "") or self.fallback_timestamp,
            "harness": self._harness(call, session),
        }
        if session:
            record["session_id"] = session
        record["run_id"] = self.run_id
        record["record_id"] = call.record_id
        record["record_index"] = call.record_index
        record["source"] = "otel"
        path = os.path.join(self.run_dir, UNATTRIBUTED_STREAM)
        if dest == DEST_ORCHESTRATOR:
            path = os.path.join(self.run_dir, ORCHESTRATOR_STREAM)
        elif dest.startswith(DEST_INVOCATION):
            agent = dest[len(DEST_INVOCATION):]
            record["agent_instance_id"] = agent
            path = os.path.join(self.run_dir, agent, INVOCATION_STREAM)
        if call.model:
            record["model"] = call.model
        record["provider"] = PROVIDER_NAME
        if call.effort:
            record["effort"] = call.effort
        if call.token_usage is not None:
            record["token_usage"] = call.token_usage
        for key in ("call_label", "call_kind"):
            if getattr(call, key, ""):
                record[key] = getattr(call, key)
        tools = getattr(call, "requested_tools", None)
        if tools is not None:
            record["requested_tools"] = dict(sorted(tools.items()))
        line = json.dumps(record, separators=(",", ":")).encode("ascii") + b"\n"
        return append_line(path, line)

    def _harness(self, call, session):
        """The envelope harness of the session the call is tied through, else of the invocation
        whose spawn_call_id is the call's chat session."""
        harness = self.keys.harness_by_session.get(session, "")
        if harness:
            return harness
        spawn_ids = self.keys.inv_by_spawn.get(call.chat_session_id)
        if spawn_ids:
            harness = self.keys.harness_by_inv.get(spawn_ids[0], "")
            if harness:
                return harness
        return self.keys.first_harness
