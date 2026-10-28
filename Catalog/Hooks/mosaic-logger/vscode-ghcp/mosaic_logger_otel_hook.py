"""mosaic_logger_otel_hook.py - Glue between the hook adapter and the shared Copilot OTel ingest.

The OTel file path comes only from COPILOT_OTEL_FILE_EXPORTER_PATH. SessionStart records the
file's current size for the session; Stop and SubagentStop run one bounded, non-final ingest of
the file into the firing's run, starting (for a run with no cursor yet) at the smallest size
recorded by any session of that run. Every diagnostic goes to the default-visible diagnostics
file; an unset variable and a missing file are each reported once per run. Never raises, never
writes to stdout, never modifies the OTel file.
"""

import json
import os

import mosaic_logger_core as core
import mosaic_logger_diagnostics as diagnostics

try:
    from mosaic_logger_otel import DEFAULT_BUDGET_SECONDS, ENV_VAR, ingest, report_once
except Exception as _exc:  # shared module not deployed beside the adapter
    DEFAULT_BUDGET_SECONDS, ENV_VAR = 5.0, "COPILOT_OTEL_FILE_EXPORTER_PATH"
    ingest = report_once = None
    core.debug_log("shared OTel ingest module unavailable", _exc)

OFFSETS_DIRNAME = ".otel-start-offsets"


def _report(ctx, kind, message, run_id, counts=None, ids=None):
    diagnostics.report(ctx.paths.root, kind, message, harness=core.HARNESS, run_id=run_id,
                       session_id=ctx.session_id, counts=counts, ids=ids)


def _offset_entry(ctx, session_id):
    return ctx.paths.root / OFFSETS_DIRNAME / f"{core.sanitize_component(session_id)}.json"


def record_session_start(ctx) -> None:
    """Remember the OTel file's size at the first SessionStart of the session."""
    try:
        if ingest is None or not ctx.session_id or not os.environ.get(ENV_VAR):
            return
        entry = _offset_entry(ctx, ctx.session_id)
        if entry.exists():
            return
        try:
            size = os.stat(os.environ[ENV_VAR]).st_size
        except OSError:
            size = 0
        entry.parent.mkdir(parents=True, exist_ok=True)
        data = {"session_id": ctx.session_id, "offset": size}
        core.atomic_replace(entry, json.dumps(data).encode("utf-8"))
    except Exception as exc:
        core.debug_log("record_session_start failed", exc)


def _read_json(path):
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def first_offset(ctx, run_id):
    """Smallest recorded start offset among the sessions of the run, or None when unknown."""
    offsets = []
    try:
        for entry in ctx.paths.session_run_id_dir().glob("*.json"):
            data = _read_json(entry)
            session = data.get("session_id")
            if data.get("run_id") != run_id or not session:
                continue
            offset = _read_json(_offset_entry(ctx, session)).get("offset")
            if isinstance(offset, int) and not isinstance(offset, bool) and offset >= 0:
                offsets.append(offset)
    except Exception as exc:
        core.debug_log("first_offset failed", exc)
    return min(offsets) if offsets else None


def _report_result(ctx, run_id, run_dir, result) -> None:
    for diag in getattr(result, "diagnostics", None) or []:
        kind = diag.kind
        if kind == "file_missing" and not report_once(run_dir, kind):
            continue
        _report(ctx, kind, diag.message, run_id, counts=diag.counts or None, ids=diag.ids or None)


def ingest_at_stop(ctx) -> None:
    """Run one non-final ingest into the firing's run and report what it found."""
    try:
        run_id = ctx.run_id
        if ingest is None or not run_id:
            return
        run_dir = ctx.paths.run_root(run_id)
        otel_path = os.environ.get(ENV_VAR)
        if not otel_path:
            if report_once(run_dir, "variable_unset"):
                _report(ctx, "variable_unset", ENV_VAR + " is not set; no usage is ingested",
                        run_id)
            return
        result = ingest(run_dir, otel_path, final=False,
                        start_offset=first_offset(ctx, run_id),
                        budget_seconds=DEFAULT_BUDGET_SECONDS)
        _report_result(ctx, run_id, run_dir, result)
    except Exception as exc:
        core.debug_log("ingest_at_stop failed", exc)
        try:
            _report(ctx, "internal_error", f"{type(exc).__name__}: {exc}", ctx.run_id)
        except Exception:
            pass
