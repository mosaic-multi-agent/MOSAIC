"""Time-budget behaviour of the ingest with spans that carry gen_ai.output.messages (Python only):
reading the requested tool names must not break the cut-off at a line boundary, and a cut-off ingest
followed by a second one ends with the records of one uninterrupted ingest, tool names included."""

import itertools
import json

import mosaic_logger_otel as otel
from otel_ingest_helpers import (
    chat_line, fixed_now, make_run, read_json, read_otel_records, state_path, write_otel,
)

LINE_COUNT = 40
CUT_BUDGET = 10.0
TOOLS_PER_SPAN = 3


def stepping_clock():
    """A monotonic clock that advances one second on every reading."""
    counter = itertools.count()
    return lambda: float(next(counter))


def line_with_output_messages(span_id):
    """A chat span whose output messages hold text and several tool calls (a repeated name included)."""
    span = json.loads(chat_line(span_id))
    parts = [{"type": "text", "content": "x" * 2000}]
    parts += [{"type": "tool_call", "id": f"toolu_{i}", "name": name, "arguments": "{}"}
              for i, name in enumerate(["read_file", "read_file", "glob"][:TOOLS_PER_SPAN])]
    span["attributes"]["gen_ai.output.messages"] = json.dumps(
        [{"role": "assistant", "parts": parts}], separators=(",", ":"))
    return json.dumps(span, separators=(",", ":"))


def make_filled_run(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [line_with_output_messages(f"om-{i:03d}") for i in range(LINE_COUNT)])
    return run_dir, otel_path


def orchestrator_records(run_dir):
    return read_otel_records(run_dir / "00_orchestrator_events.jsonl")


def ingest_with_cut(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=CUT_BUDGET,
                       clock=stepping_clock(), now=fixed_now)


def ingest_unbudgeted(run_dir, otel_path):
    return otel.ingest(str(run_dir), str(otel_path), budget_seconds=None, now=fixed_now)


def test_records_written_before_a_cut_off_carry_their_requested_tools(tmp_path):
    run_dir, otel_path = make_filled_run(tmp_path)

    ingest_with_cut(run_dir, otel_path)

    written = orchestrator_records(run_dir)
    assert 0 < len(written) < LINE_COUNT
    assert all(r.get("requested_tools") == {"read_file": 2, "glob": 1} for r in written)


def test_cut_off_then_second_ingest_equals_an_uninterrupted_ingest_with_output_messages(tmp_path):
    cut_dir, cut_otel = make_filled_run(tmp_path / "cut")
    whole_dir, whole_otel = make_filled_run(tmp_path / "whole")
    ingest_with_cut(cut_dir, cut_otel)

    ingest_unbudgeted(cut_dir, cut_otel)
    ingest_unbudgeted(whole_dir, whole_otel)

    cut_records = orchestrator_records(cut_dir)
    assert len(cut_records) == LINE_COUNT
    assert all(r.get("requested_tools") == {"read_file": 2, "glob": 1} for r in cut_records)
    assert cut_records == orchestrator_records(whole_dir)
    assert read_json(state_path(cut_dir)) == read_json(state_path(whole_dir))
