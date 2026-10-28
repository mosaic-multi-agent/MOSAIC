"""Time-budget behaviour of the ingest (Python only): a cut-off stops at a line boundary, keeps
the state consistent, releases the lock, and a later ingest finishes with the same final records
as one uninterrupted ingest.

Contract: the budget is checked against the injected clock before every line, so a clock that
advances one second per reading with a 10 s budget cuts a 40-line file well before the end."""

import itertools

import mosaic_logger_otel as otel
from otel_ingest_helpers import (
    SESSION, chat_line, fixed_now, lock_path, make_run, read_json, read_otel_records, state_path,
    write_otel,
)

LINE_COUNT = 40
CUT_BUDGET = 10.0


def stepping_clock():
    """A monotonic clock that advances one second on every reading."""
    counter = itertools.count()
    return lambda: float(next(counter))


def orchestrator_records(run_dir):
    return read_otel_records(run_dir / "00_orchestrator_events.jsonl")


def make_filled_run(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line(f"b-{i:03d}") for i in range(LINE_COUNT)])
    return run_dir, otel_path


def ingest_with_cut(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=CUT_BUDGET,
                       clock=stepping_clock(), now=fixed_now)


def ingest_unbudgeted(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=None, now=fixed_now)


def budget_diagnostic(res):
    return next((d for d in res.diagnostics if d.kind == "budget_exhausted"), None)


def test_cut_off_ingest_reports_budget_exhausted_with_remaining_bytes(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    res = ingest_with_cut(run_dir, otel_path)

    diag = budget_diagnostic(res)
    assert res.status == "ingested"
    assert diag is not None, [d.kind for d in res.diagnostics]
    cursor = read_json(state_path(run_dir))["cursor"]["offset"]
    assert diag.counts == {"remaining_bytes": otel_path.stat().st_size - cursor}
    assert diag.counts["remaining_bytes"] > 0


def test_cut_off_ingest_stops_at_a_line_boundary_with_consistent_state(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    res = ingest_with_cut(run_dir, otel_path)

    data = otel_path.read_bytes()
    state = read_json(state_path(run_dir))
    offset = state["cursor"]["offset"]
    written = orchestrator_records(run_dir)
    assert 0 < len(written) < LINE_COUNT
    assert res.records_written == len(written)
    assert offset == 0 or data[offset - 1:offset] == b"\n"
    assert sorted(state["placements"]) == sorted(r["record_id"] for r in written)
    assert all(r["record_index"] < offset for r in written)


def test_cut_off_ingest_releases_the_lock(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    ingest_with_cut(run_dir, otel_path)

    assert not lock_path(run_dir).exists()


def test_next_ingest_finishes_with_the_records_of_an_uninterrupted_ingest(tmp_path):
    cut_dir, cut_otel = make_filled_run(tmp_path / "cut")
    whole_dir, whole_otel = make_filled_run(tmp_path / "whole")
    ingest_with_cut(cut_dir, cut_otel)

    res = ingest_unbudgeted(cut_dir, cut_otel)
    ingest_unbudgeted(whole_dir, whole_otel)

    assert budget_diagnostic(res) is None
    assert orchestrator_records(cut_dir) == orchestrator_records(whole_dir)
    assert len(orchestrator_records(cut_dir)) == LINE_COUNT
    assert read_json(state_path(cut_dir)) == read_json(state_path(whole_dir))


def test_repeated_cut_off_ingests_make_progress_until_done_without_duplicates(tmp_path):
    cut_dir, cut_otel = make_filled_run(tmp_path / "cut")
    whole_dir, whole_otel = make_filled_run(tmp_path / "whole")

    for _ in range(LINE_COUNT + 2):
        if budget_diagnostic(ingest_with_cut(cut_dir, cut_otel)) is None:
            break
    else:
        raise AssertionError("repeated budgeted ingests never finished")
    ingest_unbudgeted(whole_dir, whole_otel)

    assert orchestrator_records(cut_dir) == orchestrator_records(whole_dir)
    assert read_json(state_path(cut_dir)) == read_json(state_path(whole_dir))


def test_no_budget_means_the_ingest_reads_everything(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)
    huge_jumps = itertools.count(0, 1_000_000)

    res = otel.ingest(str(run_dir), str(otel_path), budget_seconds=None,
                      clock=lambda: float(next(huge_jumps)), now=fixed_now)

    assert budget_diagnostic(res) is None
    assert res.records_written == LINE_COUNT


def test_generous_budget_does_not_cut_the_ingest(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    res = otel.ingest(str(run_dir), str(otel_path), budget_seconds=1000.0,
                      clock=stepping_clock(), now=fixed_now)

    assert budget_diagnostic(res) is None
    assert res.records_written == LINE_COUNT


def test_default_budget_is_a_positive_number():
    assert isinstance(otel.DEFAULT_BUDGET_SECONDS, (int, float))
    assert otel.DEFAULT_BUDGET_SECONDS > 0


def test_session_id_is_kept_on_records_written_after_a_cut_off(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    ingest_with_cut(run_dir, otel_path)

    assert {r.get("session_id") for r in orchestrator_records(run_dir)} == {SESSION}
