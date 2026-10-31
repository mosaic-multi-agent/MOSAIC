"""Shared helpers for the transcript logging tests: a hook-driving test case with a session transcript.

The case drives the real dispatcher (native mode by default, Runner mode through the MOSAIC_*
environment) against a temporary workspace, keeps one session transcript that tests write and grow,
and reads back every event stream, the diagnostics file and the stream each event landed on.
"""

import contextlib
import json
import os
import pathlib
import sys
from unittest import mock

_TESTS_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(_TESTS_DIR))
sys.path.insert(0, _TESTS_DIR)

import mosaic_logger_core  # noqa: E402,F401  (puts the shared append module on sys.path)
import mosaic_logger_locked_append as locked  # noqa: E402

from tests.runner_mode_support import (  # noqa: E402
    INSTANCE_ID, RUN_ID, SESSION_ID, RunnerModeTestCase, read_jsonl)
import transcript_usage_support as tus  # noqa: E402

FIRST_RUN = "20261030T130000Z-aaaa"
SECOND_RUN = "20261030T150000Z-bbbb"
DIAGNOSTICS_FILE = "mosaic-logger-diagnostics.jsonl"
ORCHESTRATOR_STREAM = "00_orchestrator_events.jsonl"
INVOCATION_STREAM = "03_events.jsonl"

usage_event = tus.usage_event
compaction_event = tus.compaction_event
turn_end_event = tus.turn_end_event


def ts(second):
    """A transcript timestamp, `second` seconds into a fixed past minute, in the transcript format."""
    return "2026-10-30T13:00:%02d.000Z" % second


def call_start(second, seconds_before=1):
    """The call-start timestamp of a usage event stamped ts(second) whose duration is whole seconds."""
    return ts(second - seconds_before)


class TranscriptLoggingCase(RunnerModeTestCase):
    """Temp workspace, environment control and one growing session transcript."""

    def setUp(self):
        super().setUp()
        self.transcript = self.workspace / "session-state" / "events.jsonl"
        self.transcript.parent.mkdir(parents=True)
        self.logs = self.workspace / "OrchestrationLogs"

    # -- transcript ---------------------------------------------------------

    def write(self, events, trailing=""):
        """Replace the session transcript with events (dicts or raw lines)."""
        lines = [e if isinstance(e, str) else json.dumps(e) for e in events]
        self.transcript.write_text("".join(line + "\n" for line in lines) + trailing,
                                   encoding="utf-8", newline="")

    def append(self, events, trailing=""):
        """Append events to the session transcript, as the CLI does while the session runs."""
        lines = [e if isinstance(e, str) else json.dumps(e) for e in events]
        with open(self.transcript, "a", encoding="utf-8", newline="") as handle:
            handle.write("".join(line + "\n" for line in lines) + trailing)

    # -- native mode driving -------------------------------------------------

    def start_run(self, run_id=FIRST_RUN):
        """A user prompt naming run_id: the session now belongs to that run."""
        self.dispatch("userPromptSubmitted", {"prompt": "Start the work. run_id: %s" % run_id})

    def spawn(self, agent_id, instance_id=INSTANCE_ID, run_id=FIRST_RUN):
        """Dispatch a task tool call and subagentStart so agent_id is mapped to instance_id."""
        prompt = json.dumps({"agent_instance_id": instance_id, "run_id": run_id,
                             "task_description": "Do it."})
        self.dispatch("preToolUse", {"toolName": "task", "toolUseId": "tu-" + agent_id,
                                     "toolArgs": {"prompt": prompt}})
        self.dispatch("subagentStart", {"agentId": agent_id, "agentType": instance_id.split("#")[0],
                                        "transcriptPath": str(self.transcript)})

    def stop(self, event="agentStop", **payload):
        """Fire a read point with the session transcript path."""
        body = {"transcriptPath": str(self.transcript)}
        body.update(payload)
        self.dispatch(event, body)

    def subagent_stop(self, agent_id):
        self.stop("subagentStop", agentId=agent_id,
                  response='{"status_code": "SUCCESS"}')

    # -- append failures ---------------------------------------------------------

    @contextlib.contextmanager
    def failing_appends(self, matches, how="drop"):
        """While active, every appended event for which matches(event) is true fails.

        how="drop" makes the append report a dropped line (as a held lock does); how="raise" makes
        it raise, as a process dying in the middle of a read would. Other appends go through.
        """
        real = locked.locked_append

        def wrapper(target, line, *args, **kwargs):
            try:
                event = json.loads(line)
            except ValueError:
                event = {}
            if isinstance(event, dict) and matches(event):
                if how == "raise":
                    raise RuntimeError("simulated crash during the read")
                return False
            return real(target, line, *args, **kwargs)

        with mock.patch.object(locked, "locked_append", wrapper):
            yield

    def stop_surviving_a_crash(self, event="agentStop"):
        """Fire a read point; a crash inside it is allowed to escape (the process is gone anyway)."""
        try:
            self.stop(event)
        except Exception:  # noqa: BLE001 - the simulated crash
            pass

    # -- reading back ----------------------------------------------------------

    def all_events(self):
        """(stream label, event) for every event of every stream; label is 'run/stream-or-instance'."""
        found = []
        if not self.logs.exists():
            return found
        for path in sorted(self.logs.rglob(ORCHESTRATOR_STREAM)):
            label = orchestrator(path.relative_to(self.logs).parts[0])
            found.extend((label, event) for event in read_jsonl(path))
        for path in sorted(self.logs.rglob(INVOCATION_STREAM)):
            label = "/".join(path.relative_to(self.logs).parts[:-1])
            found.extend((label, event) for event in read_jsonl(path))
        return found

    def events_of(self, kind):
        return [(label, event) for label, event in self.all_events() if event.get("event") == kind]

    def usage_records(self):
        return self.events_of("usage_record")

    def streams_of_record(self, record_id):
        """Labels of every stream holding a usage_record with this record_id (one entry per line)."""
        return [label for label, event in self.usage_records() if event.get("record_id") == record_id]

    def usage_record(self, record_id):
        """The single usage_record line with this record_id; fails when it is not exactly one."""
        found = [event for _, event in self.usage_records() if event.get("record_id") == record_id]
        self.assertEqual(1, len(found), "usage_record %r written %d times" % (record_id, len(found)))
        return found[0]

    def record_ids(self):
        return sorted(event.get("record_id") for _, event in self.usage_records())

    def diagnostics(self):
        return read_jsonl(self.logs / DIAGNOSTICS_FILE)

    def diagnostics_of(self, kind):
        return [line for line in self.diagnostics() if line.get("kind") == kind]


def orchestrator(run_id):
    return run_id + "/orchestrator"


def invocation(run_id, instance_id=INSTANCE_ID):
    return "%s/%s" % (run_id, instance_id)


__all__ = ["TranscriptLoggingCase", "usage_event", "compaction_event", "turn_end_event", "ts",
           "call_start", "orchestrator", "invocation", "FIRST_RUN", "SECOND_RUN", "RUN_ID",
           "SESSION_ID", "INSTANCE_ID", "DIAGNOSTICS_FILE"]
