"""Reading a transcript of about 4.8 MB finishes within the hook's time budget."""

import json
import os
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transcript_usage_support as support  # noqa: E402

TARGET_BYTES = 4_800_000
TIME_LIMIT_SECONDS = 5.0
_FILLER = "x" * 6000


def _build_transcript(path):
    """Write a transcript shaped like a real one: usage records, turn ends and large tool events."""
    calls = 0
    size = 0
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        while size < TARGET_BYTES:
            events = [
                support.usage_event("call-%d" % calls, agent_id=("agent-1" if calls % 4 else None),
                                    event_id="u-%d" % calls, toolCounts={"view": 1, "grep": 2}),
                {"type": "tool.execution_complete", "id": "t-%d" % calls, "timestamp": support.DEFAULT_TS,
                 "data": {"result": _FILLER}},
                support.turn_end_event("te-%d" % calls),
            ]
            for event in events:
                line = json.dumps(event) + "\n"
                handle.write(line)
                size += len(line.encode("utf-8"))
            calls += 1
    return calls


class ReadTimingTest(unittest.TestCase):
    def test_large_transcript_is_read_within_the_limit(self):
        import mosaic_logger_transcript_usage as reader
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "events.jsonl")
            expected = _build_transcript(path)
            self.assertGreaterEqual(os.path.getsize(path), TARGET_BYTES)

            started = time.perf_counter()
            result = reader.read_transcript_usage(path)
            elapsed = time.perf_counter() - started

        self.assertEqual(len(result.calls), expected)
        self.assertFalse(result.alarm)
        self.assertLess(elapsed, TIME_LIMIT_SECONDS)


if __name__ == "__main__":
    unittest.main()
