"""The OTel writer appends through the locked append.

Parallel writers on one stream lose, tear or blank no line; a writer that cannot get the lock
in time reports "not written" with False and does not raise; I/O errors still raise OSError.
"""

import json
import pathlib
import time
import types

import pytest

from locked_append_support import hold_lock, line_problems, lock_file_of, release_after, run_writers
from otel_ingest_helpers import FIXED_NOW
import mosaic_logger_otel_write as ow
from mosaic_logger_otel_place import DEST_ORCHESTRATOR

SHARED_DIR = pathlib.Path(__file__).resolve().parents[1]
STREAM = "00_orchestrator_events.jsonl"
BOUND_SLACK = 6.0


def _writer(run_dir):
    keys = types.SimpleNamespace(
        conversation=lambda call: "session-1", harness_by_session={"session-1": "ghcp-cli"},
        inv_by_spawn={}, harness_by_inv={}, first_harness="ghcp-cli")
    return ow.Writer(run_dir, FIXED_NOW, keys)


def _call(record_id="span-1"):
    return types.SimpleNamespace(record_id=record_id, record_index=0, model="m", effort="",
                                 token_usage={"input_tokens": 1}, chat_session_id="")


def test_parallel_writers_of_long_lines_lose_tear_or_blank_no_line_regression_only(tmp_path):
    # Regression coverage only: the OTel writer already writes a whole line with one os.write
    # on an append-mode descriptor, so this passes without the lock. The tests that hold the
    # stream lock are the evidence for the lock behavior of this writer.
    target = tmp_path / "run" / STREAM

    problems = run_writers("line", [SHARED_DIR], target)

    assert problems == []
    assert line_problems(target) == []


def test_append_line_returns_true_when_written_and_leaves_no_lock_file(tmp_path):
    target = tmp_path / "run" / STREAM

    written = ow.append_line(str(target), b'{"a":1}\n')

    assert written is True
    assert target.read_bytes() == b'{"a":1}\n'
    assert not lock_file_of(target).exists()


def test_append_line_returns_false_within_the_bound_when_the_lock_is_held(tmp_path):
    target = tmp_path / "run" / STREAM
    lock = hold_lock(target)

    started = time.monotonic()
    written = ow.append_line(str(target), b'{"a":1}\n')
    elapsed = time.monotonic() - started

    assert written is False
    assert not target.exists()
    assert lock.exists()
    assert elapsed < BOUND_SLACK


def test_append_line_waits_for_a_lock_released_mid_wait_and_then_writes(tmp_path):
    target = tmp_path / "run" / STREAM
    hold_lock(target)
    release_after(lock_file_of(target), 0.5)

    started = time.monotonic()
    written = ow.append_line(str(target), b'{"a":1}\n')
    elapsed = time.monotonic() - started

    assert written is True
    assert target.read_bytes() == b'{"a":1}\n'
    assert elapsed >= 0.4
    assert not lock_file_of(target).exists()


def test_append_line_raises_os_error_when_the_stream_cannot_be_written(tmp_path):
    blocker = tmp_path / "not_a_dir"
    blocker.write_bytes(b"")

    with pytest.raises(OSError):
        ow.append_line(str(blocker / STREAM), b'{"a":1}\n')


def test_writer_write_returns_true_and_appends_one_usage_record(tmp_path):
    run_dir = tmp_path / "20261028T000000Z-0000"
    run_dir.mkdir()

    written = _writer(str(run_dir)).write(_call(), DEST_ORCHESTRATOR)

    assert written is True
    lines = (run_dir / STREAM).read_text(encoding="utf-8").splitlines()
    assert [json.loads(line)["record_id"] for line in lines] == ["span-1"]


def test_writer_write_returns_false_and_writes_nothing_when_the_stream_lock_is_held(tmp_path):
    run_dir = tmp_path / "20261028T000000Z-0000"
    run_dir.mkdir()
    hold_lock(run_dir / STREAM)

    written = _writer(str(run_dir)).write(_call(), DEST_ORCHESTRATOR)

    assert written is False
    assert not (run_dir / STREAM).exists()


def test_writer_write_lets_an_io_error_propagate(tmp_path):
    run_dir = tmp_path / "20261028T000000Z-0000"
    run_dir.mkdir()
    (run_dir / STREAM).mkdir()

    with pytest.raises(OSError):
        _writer(str(run_dir)).write(_call(), DEST_ORCHESTRATOR)
