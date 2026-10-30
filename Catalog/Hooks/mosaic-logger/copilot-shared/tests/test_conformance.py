"""Runs every case of the shared conformance fixtures against the Python ingest.

The fixtures live with the Go package and are read in place. For each step the runner builds the
run folder, places the step's OTel file and log overlay, runs one ingest and compares streams,
state, result and diagnostics with the expected files, exactly as the Go runner does.
"""

import json
import pathlib
import shutil

import pytest

import mosaic_logger_otel as otel
from otel_ingest_helpers import CONFORMANCE_DIR, FIXED_NOW, read_json

CASE_NAMES = sorted(p.name for p in CONFORMANCE_DIR.iterdir() if p.is_dir())
STREAM_TOP_LEVEL = {"00_orchestrator_events.jsonl", "00_unattributed_events.jsonl"}


def is_stream_file(rel):
    parts = rel.split("/")
    if len(parts) == 1:
        return parts[0] in STREAM_TOP_LEVEL
    return len(parts) == 2 and parts[1] == "03_events.jsonl" and not parts[0].startswith(".")


def read_stream_events(path):
    """In file order, the OTel usage_record lines and every tool_call_end line of a stream, timestamp kept."""
    events = []
    for line in path.read_bytes().split(b"\n"):
        if not line.strip():
            continue
        obj = json.loads(line)
        is_otel_usage = obj.get("event") == "usage_record" and obj.get("source") == "otel"
        if is_otel_usage or obj.get("event") == "tool_call_end":
            events.append(obj)
    return events


def collect_streams(root, selected):
    """OTel usage records and tool_call_end lines per selected stream file (relative posix path) under root."""
    out = {}
    if not root.is_dir():
        return out
    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(root).as_posix()
        if not selected(rel):
            continue
        records = read_stream_events(path)
        if records:
            out[rel] = records
    return out


def copy_tree(src, dst):
    """Copy every file under src into dst, overwriting. A missing src is ignored."""
    if not src.is_dir():
        return
    for path in src.rglob("*"):
        target = dst / path.relative_to(src)
        if path.is_dir():
            target.mkdir(parents=True, exist_ok=True)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)


def snapshot_logs(run_dir):
    """Bytes of every run-folder file outside dot-prefixed folders."""
    out = {}
    for path in run_dir.rglob("*"):
        rel = path.relative_to(run_dir)
        if path.is_file() and not any(part.startswith(".") for part in rel.parts):
            out[rel.as_posix()] = path.read_bytes()
    return out


class CaseWorkspace:
    """The temporary workspace one case runs in."""

    def __init__(self, tmp_path, case_dir, spec):
        self.case_dir = case_dir
        self.logs_root = tmp_path / "OrchestrationLogs"
        self.run_dir = self.logs_root / spec["run_id"]
        self.otel_path = tmp_path / "otel" / "copilot-otel.jsonl"
        self.otel_path.parent.mkdir(parents=True)
        self.run_dir.mkdir(parents=True)
        copy_tree(case_dir / "logs", self.run_dir)
        self.logs_before = {}

    def prepare_step(self, step):
        if step["otel"] is None:
            if self.otel_path.exists():
                self.otel_path.unlink()
        else:
            shutil.copyfile(self.case_dir / step["otel"], self.otel_path)
        if step["logs"] is not None:
            copy_tree(self.case_dir / step["logs"], self.run_dir)
        self.logs_before = snapshot_logs(self.run_dir)

    def run_ingest(self, step):
        return otel.ingest(
            str(self.run_dir), str(self.otel_path),
            final=step["final"], whole_file=step["whole_file"],
            start_offset=step["start_offset"], scope=step["scope"] or None,
            budget_seconds=None, now=lambda: FIXED_NOW,
        )


def check_result(case_dir, step, res):
    want = read_json(case_dir / "expected" / step / "result.json")
    got = {
        "status": res.status, "records_written": res.records_written,
        "finalized": res.finalized, "pending": res.pending,
        "lines_read": res.lines_read, "lines_malformed": res.lines_malformed,
        "tool_ends_written": res.tool_ends_written,
    }
    assert got == want, f"{step}: result differs"


def check_diagnostics(case_dir, step, res):
    path = case_dir / "expected" / step / "diagnostics.json"
    want = read_json(path) if path.exists() else []
    got = []
    for d in res.diagnostics:
        entry = {"kind": d.kind}
        if d.counts:
            entry["counts"] = d.counts
        if d.ids:
            entry["ids"] = d.ids
        got.append(entry)
    assert got == want, f"{step}: diagnostics differ"


def check_streams(ws, step):
    want = collect_streams(ws.case_dir / "expected" / step / "streams", lambda rel: True)
    got = collect_streams(ws.run_dir, is_stream_file)
    assert got == want, f"{step}: ingested usage_record and tool_call_end streams differ"


def check_state(ws, step):
    state = ws.run_dir / ".otel-ingest" / "state.json"
    want_path = ws.case_dir / "expected" / step / "state.json"
    if not want_path.exists():
        assert not state.exists(), f"{step}: state.json exists but the case expects no state file"
        return
    assert state.exists(), f"{step}: state.json was not written"
    assert read_json(state) == read_json(want_path), f"{step}: state.json differs"


def check_logs_append_only(ws, step):
    for rel, before in ws.logs_before.items():
        after = (ws.run_dir / rel).read_bytes()
        assert after.startswith(before), f"{step}: {rel}: existing log content was changed"
    assert not (ws.logs_root / "unknown-run").exists(), f"{step}: ingest wrote to unknown-run/"


def run_case(tmp_path, name):
    case_dir = CONFORMANCE_DIR / name
    spec = read_json(case_dir / "case.json")
    ws = CaseWorkspace(tmp_path, case_dir, spec)
    for step in spec["steps"]:
        ws.prepare_step(step)
        otel_before = ws.otel_path.read_bytes() if ws.otel_path.exists() else b""
        res = ws.run_ingest(step)
        check_result(case_dir, step["name"], res)
        check_diagnostics(case_dir, step["name"], res)
        check_streams(ws, step["name"])
        check_state(ws, step["name"])
        check_logs_append_only(ws, step["name"])
        otel_after = ws.otel_path.read_bytes() if ws.otel_path.exists() else b""
        assert otel_before == otel_after, "the OTel file was modified by the ingest"
    return ws


def test_conformance_cases_are_present():
    assert CASE_NAMES, f"no conformance cases in {CONFORMANCE_DIR}"


@pytest.mark.parametrize("name", CASE_NAMES)
def test_conformance_case(tmp_path, name):
    run_case(tmp_path, name)


def test_split_ingest_ends_like_single_ingest(tmp_path):
    """Two partial ingests and one full ingest end in the same streams and the same state."""
    split = run_case(tmp_path / "split", "split_inside_parent_child_pair")
    full = run_case(tmp_path / "full", "split_inside_parent_child_pair_full")

    split_streams = collect_streams(split.run_dir, is_stream_file)
    full_streams = collect_streams(full.run_dir, is_stream_file)
    assert split_streams and split_streams == full_streams
    assert read_json(split.run_dir / ".otel-ingest" / "state.json") == \
        read_json(full.run_dir / ".otel-ingest" / "state.json")
