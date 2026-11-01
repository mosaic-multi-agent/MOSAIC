"""Concurrent usage emission on one stream: no bulk rewrite.

Several real processes fire usage emission over the same transcript at the
same moment, several times each. The invariant asserted is the absence of a
bulk rewrite: every record is written at least once, and the number of extra
writes stays within a small tolerance for rare single duplicates. No
interleaving order is asserted.
"""

import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_usage as usage

_ADAPTER_DIR = str(pathlib.Path(__file__).parent.parent)
_RUN_ID = "20260101T170000Z-c9a1"
_TS = "2026-01-01T17:00:00.000Z"

_PROCESSES = 4
_ROUNDS = 5
_RECORDS = 30
_DUPLICATE_TOLERANCE = 3

_FIRER = r'''
import pathlib, sys, time
adapter_dir, workspace, transcript, start_at, rounds = sys.argv[1:6]
sys.path.insert(0, adapter_dir)
import mosaic_logger_core as core
import mosaic_logger_usage as usage
root = pathlib.Path(workspace)
ctx = core.HookContext({"hook_event_name": "Stop", "session_id": "s-conc"},
                       root, core.build_paths(root), sys.argv[6])
ctx.run_id = sys.argv[7]
while time.time() < float(start_at):
    time.sleep(0.001)
for _ in range(int(rounds)):
    usage.emit_usage_records(ctx, transcript, None, "orchestrator_transcript")
'''


def _write_transcript(path, count):
    lines = []
    for i in range(count):
        lines.append(json.dumps({"type": "assistant", "message": {
            "id": "msg_%03d" % i, "model": "claude-opus-4-5",
            "usage": {"input_tokens": 10 + i, "output_tokens": 3}}}))
    pathlib.Path(path).write_text("\n".join(lines) + "\n", encoding="utf-8")


class ConcurrentUsageEmissionTests(unittest.TestCase):
    def test_concurrent_firings_on_one_stream_do_not_rewrite_records_in_bulk(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            transcript = root / "t.jsonl"
            _write_transcript(transcript, _RECORDS)
            start_at = time.time() + 1.5
            children = [
                subprocess.Popen(
                    [sys.executable, "-I", "-c", _FIRER, _ADAPTER_DIR, str(root),
                     str(transcript), str(start_at), str(_ROUNDS), _TS, _RUN_ID],
                    stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
                for _ in range(_PROCESSES)
            ]
            failures = []
            for child in children:
                err = child.communicate(timeout=120)[1].decode("utf-8", "replace")
                if child.returncode != 0:
                    failures.append(err[-300:])
            self.assertEqual([], failures)

            paths = core.build_paths(root)
            events_path = paths.orchestrator_events(_RUN_ID)
            events = [json.loads(ln) for ln in
                      events_path.read_text("utf-8").splitlines() if ln.strip()]
            written = [e["record_id"] for e in events if e.get("event") == usage.USAGE_EVENT]

            self.assertEqual({"msg_%03d" % i for i in range(_RECORDS)}, set(written))
            self.assertLessEqual(
                len(written) - _RECORDS, _DUPLICATE_TOLERANCE,
                "records written %d times in total for %d distinct records"
                % (len(written), _RECORDS))


if __name__ == "__main__":
    unittest.main()
