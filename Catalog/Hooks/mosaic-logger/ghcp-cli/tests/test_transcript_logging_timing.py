"""One stop hook's transcript work on a 4.8 MB native orchestrator transcript stays within 5 seconds.

The transcript is generated: usage calls of the orchestrator and of a mapped subagent, turn ends
and large tool events, shaped like a real native session. GHCP CLI hooks are limited to 30 s.
"""

import json
import os
import sys
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from transcript_logging_support import FIRST_RUN, TranscriptLoggingCase, turn_end_event, usage_event  # noqa: E402

TARGET_BYTES = 4_800_000
TIME_LIMIT_SECONDS = 5.0
_FILLER = "x" * 6000


def _build_transcript(path, agent_id):
    """Write events until the file reaches TARGET_BYTES; returns the number of usage calls."""
    calls = 0
    size = 0
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        while size < TARGET_BYTES:
            events = [
                usage_event("call-%d" % calls, agent_id=(agent_id if calls % 4 else None),
                            event_id="u-%d" % calls, toolCounts={"view": 1, "grep": 2}),
                {"type": "tool.execution_complete", "id": "t-%d" % calls,
                 "timestamp": "2026-10-30T13:00:10.000Z", "data": {"result": _FILLER}},
                turn_end_event("te-%d" % calls),
            ]
            for event in events:
                line = json.dumps(event) + "\n"
                handle.write(line)
                size += len(line.encode("utf-8"))
            calls += 1
    return calls


class StopHookTimingTest(TranscriptLoggingCase):
    def setUp(self):
        super().setUp()
        self.start_run()
        self.spawn("agent-1")
        self.calls = _build_transcript(self.transcript, "agent-1")
        self.assertGreaterEqual(os.path.getsize(self.transcript), TARGET_BYTES)

    def test_first_stop_hook_logs_every_call_within_the_limit(self):
        started = time.perf_counter()
        self.stop()
        elapsed = time.perf_counter() - started
        self.assertEqual(self.calls, len(self.usage_records()))
        self.assertLess(elapsed, TIME_LIMIT_SECONDS)

    def test_stop_hook_that_finds_nothing_new_is_within_the_limit(self):
        self.stop()
        started = time.perf_counter()
        self.stop()
        elapsed = time.perf_counter() - started
        self.assertEqual(self.calls, len(self.usage_records()))
        self.assertLess(elapsed, TIME_LIMIT_SECONDS)
        self.assertEqual(FIRST_RUN, self.usage_record("call-0")["run_id"])


if __name__ == "__main__":
    unittest.main()
