"""An ingest never raises into its caller, and its lock behaves as the shared state contract says."""

import datetime
import json
import os

import pytest

import mosaic_logger_otel as otel
from otel_ingest_helpers import (
    FIXED_NOW, chat_line, fixed_now, lock_path, make_run, read_json, read_otel_records, state_path,
    write_otel,
)


def raising(*_args, **_kwargs):
    raise RuntimeError("injected failure")


def kinds(res):
    return [d.kind for d in res.diagnostics]


def write_lock(run_dir, holder, acquired_at):
    lock_path(run_dir).parent.mkdir(parents=True, exist_ok=True)
    lock_path(run_dir).write_text(json.dumps({"holder": holder, "acquired_at": acquired_at}),
                                  encoding="utf-8")


def stamp(delta_seconds):
    moment = datetime.datetime.fromtimestamp(FIXED_NOW.timestamp() + delta_seconds,
                                             datetime.timezone.utc)
    return moment.strftime("%Y-%m-%dT%H:%M:%S.000Z")


def filled_run(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line("o1")])
    return run_dir, otel_path


def run_ingest(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=None, now=fixed_now)


# --- failures are swallowed into a diagnostic -----------------------------------------------

@pytest.mark.parametrize("broken", ["clock", "now"])
def test_failing_time_source_returns_failed_with_internal_error(tmp_path, broken):
    run_dir, otel_path = filled_run(tmp_path)
    kwargs = {"budget_seconds": 5.0, "clock": raising if broken == "clock" else None,
              "now": raising if broken == "now" else fixed_now}

    res = otel.ingest(str(run_dir), str(otel_path), **kwargs)

    assert res.status == "failed"
    assert "internal_error" in kinds(res)


def test_internal_failure_leaves_no_lock_and_no_state(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)

    res = otel.ingest(str(run_dir), str(otel_path), budget_seconds=5.0, clock=raising, now=fixed_now)

    assert res.status == "failed"
    assert not lock_path(run_dir).exists()
    assert not state_path(run_dir).exists()


def test_otel_path_that_cannot_be_read_returns_failed_without_raising(tmp_path):
    run_dir, _ = filled_run(tmp_path)
    directory = tmp_path / "not-a-file"
    directory.mkdir()

    res = run_ingest(run_dir, directory)

    assert res.status == "failed"
    assert not lock_path(run_dir).exists()
    assert not state_path(run_dir).exists()


@pytest.mark.parametrize("run_dir,otel_path", [(None, None), (None, "x.jsonl"), (12, 34)])
def test_unusable_arguments_do_not_raise(run_dir, otel_path):
    res = otel.ingest(run_dir, otel_path, budget_seconds=None)

    assert res.status == "failed"
    assert "internal_error" in kinds(res)


def test_read_pending_status_never_raises():
    status = otel.read_pending_status(None)

    assert (status.state_present, status.state_known, status.pending) == (False, False, 0)


# --- pending status ----------------------------------------------------------------------------

def test_pending_status_reports_pending_spans_of_a_known_state(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    child = json.loads(chat_line("c1"))
    child["parentSpanId"] = "parent-never-seen"
    write_otel(otel_path, [json.dumps(child)])
    run_ingest(run_dir, otel_path)

    status = otel.read_pending_status(str(run_dir))

    assert (status.state_present, status.state_known, status.pending) == (True, True, 1)


def test_pending_status_without_state_is_all_empty(tmp_path):
    run_dir, _ = make_run(tmp_path)

    status = otel.read_pending_status(str(run_dir))

    assert (status.state_present, status.state_known, status.pending) == (False, False, 0)


def test_pending_status_flags_an_unknown_state_version(tmp_path):
    run_dir, _ = make_run(tmp_path)
    state_path(run_dir).parent.mkdir()
    state_path(run_dir).write_text('{"state_version": 99, "pending": [{}]}', encoding="utf-8")

    status = otel.read_pending_status(str(run_dir))

    assert (status.state_present, status.state_known, status.pending) == (True, False, 0)


# --- lock -----------------------------------------------------------------------------------------

def test_fresh_lock_of_another_ingest_skips_the_turn_and_is_left_alone(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    write_lock(run_dir, "go/4242", stamp(-60))
    before = lock_path(run_dir).read_bytes()
    log_before = (run_dir / "00_orchestrator_events.jsonl").read_bytes()

    res = run_ingest(run_dir, otel_path)

    assert res.status == "lock_held"
    assert "lock_held" in kinds(res)
    assert (res.records_written, res.lines_read) == (0, 0)
    assert lock_path(run_dir).read_bytes() == before
    assert (run_dir / "00_orchestrator_events.jsonl").read_bytes() == log_before
    assert not state_path(run_dir).exists()


def test_work_skipped_by_a_held_lock_is_done_by_the_next_ingest(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    write_lock(run_dir, "go/4242", stamp(-60))
    run_ingest(run_dir, otel_path)
    os.remove(lock_path(run_dir))

    res = run_ingest(run_dir, otel_path)

    assert (res.status, res.records_written) == ("ingested", 1)


def test_stale_lock_is_broken_and_the_ingest_proceeds(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    write_lock(run_dir, "go/4242", stamp(-otel.LOCK_STALE_AFTER_SECONDS - 60))

    res = run_ingest(run_dir, otel_path)

    assert (res.status, res.records_written) == ("ingested", 1)
    assert not lock_path(run_dir).exists()


@pytest.mark.parametrize("age_seconds,status,records", [
    (60, "lock_held", 0),
    (3600, "ingested", 1),
])
def test_lock_without_readable_time_ages_by_its_file_modification_time(
        tmp_path, age_seconds, status, records):
    run_dir, otel_path = filled_run(tmp_path)
    lock_path(run_dir).parent.mkdir()
    lock_path(run_dir).write_text("not json", encoding="utf-8")
    modified = FIXED_NOW.timestamp() - age_seconds
    os.utime(lock_path(run_dir), (modified, modified))

    res = run_ingest(run_dir, otel_path)

    assert (res.status, res.records_written) == (status, records)
    assert lock_path(run_dir).exists() == (status == "lock_held")


def test_lock_just_inside_the_stale_age_still_blocks(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    write_lock(run_dir, "go/4242", stamp(-otel.LOCK_STALE_AFTER_SECONDS + 60))

    res = run_ingest(run_dir, otel_path)

    assert res.status == "lock_held"


def test_lock_exactly_as_old_as_the_stale_age_is_broken(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    write_lock(run_dir, "go/4242", stamp(-otel.LOCK_STALE_AFTER_SECONDS))

    res = run_ingest(run_dir, otel_path)

    assert (res.status, res.records_written) == ("ingested", 1)


def test_lock_is_released_after_a_normal_ingest(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)

    run_ingest(run_dir, otel_path)

    assert not lock_path(run_dir).exists()


def test_lock_is_released_when_the_otel_file_is_missing(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    otel_path.unlink()

    res = run_ingest(run_dir, otel_path)

    assert res.status == "file_missing"
    assert not lock_path(run_dir).exists()


def test_lock_is_released_when_the_state_version_is_unknown(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)
    state_path(run_dir).parent.mkdir()
    state_path(run_dir).write_text('{"state_version": 99}', encoding="utf-8")

    res = run_ingest(run_dir, otel_path)

    assert res.status == "unknown_state"
    assert not lock_path(run_dir).exists()


def test_records_are_whole_json_lines_and_state_carries_the_state_version(tmp_path):
    run_dir, otel_path = filled_run(tmp_path)

    run_ingest(run_dir, otel_path)

    assert (run_dir / "00_orchestrator_events.jsonl").read_bytes().endswith(b"\n")
    assert [r["record_id"] for r in read_otel_records(run_dir / "00_orchestrator_events.jsonl")] == ["o1"]
    assert read_json(state_path(run_dir))["state_version"] == otel.STATE_VERSION
