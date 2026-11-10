"""Missing OTel file, unparseable state, pending-status read-only rules and concurrent ingests."""

import json
import threading

import mosaic_logger_otel as otel
from otel_ingest_helpers import (
    chat_line, fixed_now, lock_path, make_run, read_otel_records, state_path, write_otel,
)

ORCHESTRATOR_LOG = "00_orchestrator_events.jsonl"


def kinds(res):
    return [d.kind for d in res.diagnostics]


def run_ingest(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=None, now=fixed_now)


def lock_stamp():
    return json.dumps({"holder": "go/4242", "acquired_at": "2026-10-28T08:59:00.000Z"})


# --- missing OTel file ----------------------------------------------------------------------

def test_missing_otel_file_reports_the_cause_and_leaves_the_run_untouched(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    log_before = (run_dir / ORCHESTRATOR_LOG).read_bytes()

    res = run_ingest(run_dir, otel_path)

    assert res.status == "file_missing"
    assert "file_missing" in kinds(res)
    assert (res.records_written, res.pending) == (0, 0)
    assert not state_path(run_dir).exists()
    assert not lock_path(run_dir).exists()
    assert (run_dir / ORCHESTRATOR_LOG).read_bytes() == log_before


def test_late_otel_file_is_ingested_normally_after_a_missing_one(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    run_ingest(run_dir, otel_path)
    write_otel(otel_path, [chat_line("o1"), chat_line("o2")])

    res = run_ingest(run_dir, otel_path)

    assert (res.status, res.records_written, res.lines_read) == ("ingested", 2, 2)
    assert "file_missing" not in kinds(res)
    assert state_path(run_dir).exists()


# --- unparseable state ----------------------------------------------------------------------

def test_unparseable_state_is_refused_and_left_byte_identical(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line("o1")])
    state_path(run_dir).parent.mkdir()
    garbage = b"this is { not json"
    state_path(run_dir).write_bytes(garbage)
    log_before = (run_dir / ORCHESTRATOR_LOG).read_bytes()

    res = run_ingest(run_dir, otel_path)

    assert res.status == "unknown_state"
    assert (res.records_written, res.lines_read) == (0, 0)
    assert "unknown_state_version" in kinds(res)
    assert state_path(run_dir).read_bytes() == garbage
    assert (run_dir / ORCHESTRATOR_LOG).read_bytes() == log_before
    assert not lock_path(run_dir).exists()


# --- pending status ------------------------------------------------------------------------------

def test_pending_status_does_not_take_the_lock_or_rewrite_the_state(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line("c1", chat_session="session-never-seen")])
    run_ingest(run_dir, otel_path)
    lock_path(run_dir).write_text(lock_stamp(), encoding="utf-8")
    state_before = state_path(run_dir).read_bytes()

    status = otel.read_pending_status(str(run_dir))

    assert (status.state_present, status.state_known, status.pending) == (True, True, 1)
    assert state_path(run_dir).read_bytes() == state_before
    assert lock_path(run_dir).read_text(encoding="utf-8") == lock_stamp()


def test_pending_status_flags_a_state_that_is_not_json(tmp_path):
    run_dir, _ = make_run(tmp_path)
    state_path(run_dir).parent.mkdir()
    state_path(run_dir).write_text('{"state_version": 1, "pend', encoding="utf-8")

    status = otel.read_pending_status(str(run_dir))

    assert (status.state_present, status.state_known, status.pending) == (True, False, 0)


# --- concurrent ingests ---------------------------------------------------------------------

def test_concurrent_ingests_write_each_record_exactly_once(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    ids = [f"o{i:02d}" for i in range(20)]
    write_otel(otel_path, [chat_line(i) for i in ids])
    results = [None] * 4

    def work(index):
        results[index] = otel.ingest(str(run_dir), str(otel_path), budget_seconds=None)

    threads = [threading.Thread(target=work, args=(i,)) for i in range(len(results))]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    followup = otel.ingest(str(run_dir), str(otel_path), budget_seconds=None)

    assert {r.status for r in results} <= {"ingested", "lock_held"}
    assert "ingested" in {r.status for r in results}
    assert followup.status == "ingested"
    written = [r["record_id"] for r in read_otel_records(run_dir / ORCHESTRATOR_LOG)]
    assert sorted(written) == ids
    assert not lock_path(run_dir).exists()
