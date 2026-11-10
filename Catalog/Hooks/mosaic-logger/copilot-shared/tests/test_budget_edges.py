"""Edge cases of the time budget (Python only): cut-offs in a final ingest, across a parent/child
pair, an exhausted budget at the start, and byte offsets after multi-byte text.

Cut tests rely on the injected clock being read on every line (a clock that advances one second
per reading, with a budget of CUT_BUDGET seconds), which is the contract of the budget check."""

import itertools
import json

import mosaic_logger_otel as otel
from otel_ingest_helpers import (
    SESSION, chat_line, fixed_now, lock_path, make_run, read_json, read_otel_records, state_path,
    write_otel,
)

LINE_COUNT = 40
CUT_BUDGET = 10.0
ORPHAN_PARENT = "invoke-never-seen"
ORCHESTRATOR_LOG = "00_orchestrator_events.jsonl"


def stepping_clock():
    counter = itertools.count()
    return lambda: float(next(counter))


def child_line(span_id, parent):
    """A chat span of a subagent chat session; it stays pending until the spawning tool call (id `parent`) is read."""
    return chat_line(span_id, chat_session=parent)


def parent_line(span_id):
    """The execution_subagent tool span whose call id is `span_id`, run in the orchestrator's own session."""
    span = json.loads(chat_line(span_id))
    span["name"] = "execute_tool"
    span["attributes"]["gen_ai.operation.name"] = "execute_tool"
    span["attributes"]["gen_ai.tool.name"] = "execution_subagent"
    span["attributes"]["gen_ai.tool.call.id"] = span_id
    span["attributes"]["copilot_chat.chat_session_id"] = SESSION
    return json.dumps(span, separators=(",", ":"))


def snapshot(run_dir):
    """All usage records of every stream of the run, plus the state, for equivalence checks."""
    streams = {p.name: read_otel_records(p) for p in sorted(run_dir.glob("*.jsonl"))}
    return streams, read_json(state_path(run_dir))


def ingest(run_dir, otel_path, *, final, budget=None, clock=None):
    return otel.ingest(str(run_dir), str(otel_path), final=final, budget_seconds=budget,
                       clock=clock, now=fixed_now)


def budget_cut(res):
    return any(d.kind == "budget_exhausted" for d in res.diagnostics)


def split_run(tmp_path, lines):
    """Two runs fed the same lines: one to be cut, one to be ingested in a single pass."""
    cut_dir, cut_otel = make_run(tmp_path / "cut")
    whole_dir, whole_otel = make_run(tmp_path / "whole")
    write_otel(cut_otel, lines)
    write_otel(whole_otel, lines)
    return (cut_dir, cut_otel), (whole_dir, whole_otel)


def test_cut_off_final_ingest_does_not_finalize_pending_spans_of_the_unread_rest(tmp_path):
    lines = ([child_line("orphan-0", ORPHAN_PARENT)]
             + [chat_line(f"b-{i:03d}") for i in range(LINE_COUNT)])
    (cut_dir, cut_otel), (whole_dir, whole_otel) = split_run(tmp_path, lines)

    first = ingest(cut_dir, cut_otel, final=True, budget=CUT_BUDGET, clock=stepping_clock())
    state_after_cut = read_json(state_path(cut_dir))
    second = ingest(cut_dir, cut_otel, final=True)
    ingest(whole_dir, whole_otel, final=True)

    assert budget_cut(first)
    assert state_after_cut["cursor"]["offset"] < cut_otel.stat().st_size
    assert not budget_cut(second)
    assert snapshot(cut_dir) == snapshot(whole_dir)


def test_cut_between_a_child_span_and_its_parent_completes_without_duplicates(tmp_path):
    lines = ([child_line("child-0", "inv-0")] + [chat_line(f"b-{i:03d}") for i in range(LINE_COUNT)]
             + [parent_line("inv-0")])
    (cut_dir, cut_otel), (whole_dir, whole_otel) = split_run(tmp_path, lines)

    first = ingest(cut_dir, cut_otel, final=False, budget=CUT_BUDGET, clock=stepping_clock())
    second = ingest(cut_dir, cut_otel, final=False)
    ingest(whole_dir, whole_otel, final=False)

    assert budget_cut(first)
    assert not budget_cut(second)
    cut_records, _ = snapshot(cut_dir)
    ids = [r["record_id"] for records in cut_records.values() for r in records]
    assert len(ids) == len(set(ids))
    # A non-final ingest writes ready calls while an earlier call is still pending, so the line
    # order within a stream may differ between the cut and the single run; the record sets and
    # the final state must be identical.
    cut_streams, cut_state = snapshot(cut_dir)
    whole_streams, whole_state = snapshot(whole_dir)
    assert cut_streams.keys() == whole_streams.keys()
    for name, records in cut_streams.items():
        assert sorted(records, key=lambda r: r["record_id"]) == sorted(
            whole_streams[name], key=lambda r: r["record_id"])
    assert cut_state == whole_state


def test_budget_exhausted_before_the_first_line_leaves_consistent_state_and_no_lock(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [chat_line(f"b-{i:03d}") for i in range(5)])

    res = ingest(run_dir, otel_path, final=False, budget=0.0, clock=stepping_clock())

    assert budget_cut(res)
    assert res.records_written == 0
    assert read_otel_records(run_dir / ORCHESTRATOR_LOG) == []
    assert not lock_path(run_dir).exists()
    if state_path(run_dir).exists():
        state = read_json(state_path(run_dir))
        assert state["cursor"]["offset"] == 0
        assert not state["placements"]
    assert ingest(run_dir, otel_path, final=False).records_written == 5


def multibyte_line(span_id):
    span = json.loads(chat_line(span_id))
    span["attributes"]["note"] = "café ☃ \U0001F600"
    return json.dumps(span, ensure_ascii=False, separators=(",", ":"))


def test_cursor_counts_bytes_so_a_second_ingest_reads_exactly_the_new_line(tmp_path):
    run_dir, otel_path = make_run(tmp_path)
    write_otel(otel_path, [multibyte_line("m1"), multibyte_line("m2")])
    first = ingest(run_dir, otel_path, final=False)
    with open(otel_path, "ab") as handle:
        handle.write((chat_line("m3") + "\n").encode("utf-8"))

    second = ingest(run_dir, otel_path, final=False)

    assert (first.records_written, first.lines_read) == (2, 2)
    assert (second.records_written, second.lines_read) == (1, 1)
    assert read_json(state_path(run_dir))["cursor"]["offset"] == otel_path.stat().st_size
    ids = [r["record_id"] for r in read_otel_records(run_dir / ORCHESTRATOR_LOG)]
    assert sorted(ids) == ["m1", "m2", "m3"]
