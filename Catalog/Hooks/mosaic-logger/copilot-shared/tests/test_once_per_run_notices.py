"""The once-per-run notice record: .otel-ingest/notices.json shaped {"reported": {kind: ISO time}},
shared with the Go implementation."""

import json
import re

import mosaic_logger_otel as otel
from otel_ingest_helpers import chat_line, fixed_now, make_run, read_json, state_path, write_otel

ISO_TIME = re.compile(r"^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z$")


def notices_path(run_dir):
    return run_dir / ".otel-ingest" / "notices.json"


def test_first_call_reports_and_repeat_does_not(tmp_path):
    run_dir, _ = make_run(tmp_path)

    first = otel.report_once(str(run_dir), "variable_unset")
    second = otel.report_once(str(run_dir), "variable_unset")

    assert (first, second) == (True, False)


def test_kinds_are_independent(tmp_path):
    run_dir, _ = make_run(tmp_path)
    otel.report_once(str(run_dir), "variable_unset")

    other_kind_first = otel.report_once(str(run_dir), "file_missing")
    other_kind_again = otel.report_once(str(run_dir), "file_missing")
    first_kind_again = otel.report_once(str(run_dir), "variable_unset")

    assert (other_kind_first, other_kind_again, first_kind_again) == (True, False, False)


def test_runs_are_independent(tmp_path):
    run_a, _ = make_run(tmp_path / "a")
    run_b, _ = make_run(tmp_path / "b")
    otel.report_once(str(run_a), "variable_unset")

    assert otel.report_once(str(run_b), "variable_unset") is True


def test_record_is_written_in_the_documented_shape(tmp_path):
    run_dir, _ = make_run(tmp_path)

    otel.report_once(str(run_dir), "variable_unset")
    otel.report_once(str(run_dir), "file_missing")

    record = read_json(notices_path(run_dir))
    assert list(record) == ["reported"]
    assert sorted(record["reported"]) == ["file_missing", "variable_unset"]
    assert all(ISO_TIME.match(v) for v in record["reported"].values()), record


def test_first_report_time_is_kept_when_a_repeat_is_refused(tmp_path):
    run_dir, _ = make_run(tmp_path)
    otel.report_once(str(run_dir), "variable_unset")
    first_time = read_json(notices_path(run_dir))["reported"]["variable_unset"]

    otel.report_once(str(run_dir), "variable_unset")

    assert read_json(notices_path(run_dir))["reported"]["variable_unset"] == first_time


def test_record_written_by_the_other_implementation_is_honoured(tmp_path):
    run_dir, _ = make_run(tmp_path)
    notices_path(run_dir).parent.mkdir()
    notices_path(run_dir).write_text(
        json.dumps({"reported": {"variable_unset": "2026-10-28T08:00:00.000Z"}}, indent=2),
        encoding="utf-8")

    already = otel.report_once(str(run_dir), "variable_unset")
    new_kind = otel.report_once(str(run_dir), "file_missing")

    assert (already, new_kind) == (False, True)
    reported = read_json(notices_path(run_dir))["reported"]
    assert reported["variable_unset"] == "2026-10-28T08:00:00.000Z"
    assert "file_missing" in reported


def test_no_temporary_files_are_left_beside_the_record(tmp_path):
    run_dir, _ = make_run(tmp_path)

    otel.report_once(str(run_dir), "variable_unset")

    assert sorted(p.name for p in notices_path(run_dir).parent.iterdir()) == ["notices.json"]


def test_unreadable_record_reports_yes(tmp_path):
    run_dir, _ = make_run(tmp_path)
    notices_path(run_dir).parent.mkdir()
    notices_path(run_dir).write_text("{not json", encoding="utf-8")

    assert otel.report_once(str(run_dir), "variable_unset") is True


def test_record_that_cannot_be_read_as_a_file_reports_yes(tmp_path):
    run_dir, _ = make_run(tmp_path)
    notices_path(run_dir).mkdir(parents=True)

    assert otel.report_once(str(run_dir), "variable_unset") is True


def test_unwritable_record_reports_yes_every_time(tmp_path):
    run_dir, _ = make_run(tmp_path)
    (run_dir / ".otel-ingest").write_text("a file where the state folder should be", encoding="utf-8")

    results = [otel.report_once(str(run_dir), "variable_unset") for _ in range(2)]

    assert results == [True, True]


def test_missing_run_folder_reports_yes_without_raising(tmp_path):
    results = [otel.report_once(str(tmp_path / "no-such-run"), "variable_unset") for _ in range(2)]

    assert results == [True, True]


def test_unusable_run_dir_reports_yes_without_raising():
    assert otel.report_once(None, "variable_unset") is True


def test_record_and_ingest_state_coexist(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line("n1")])
    otel.report_once(str(run_dir), "variable_unset")

    res = otel.ingest(str(run_dir), str(otel_path), budget_seconds=None, now=fixed_now)

    assert (res.status, res.records_written) == ("ingested", 1)
    assert state_path(run_dir).exists()
    assert "variable_unset" in read_json(notices_path(run_dir))["reported"]
    assert otel.report_once(str(run_dir), "variable_unset") is False
