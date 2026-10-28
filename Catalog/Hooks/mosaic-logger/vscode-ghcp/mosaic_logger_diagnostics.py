"""mosaic_logger_diagnostics.py — Default-visible adapter diagnostics.

Appends one JSON line per real failure (for example a subagent firing that cannot be
mapped to an invocation) to OrchestrationLogs/mosaic-logger-diagnostics.jsonl. The file
is written regardless of MOSAIC_LOGGER_DEBUG, sits at the logs root where log readers
do not treat plain files as event streams, and is never written to stdout or stderr.
Other modules of the variant may call report() for further diagnostics.
"""

import pathlib

import mosaic_logger_core as core

DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"


def report(logs_root: pathlib.Path, kind: str, message: str, *, harness: str,
           run_id: "str | None" = None, session_id: "str | None" = None,
           counts: "dict | None" = None, ids: "list | None" = None) -> None:
    """Append one diagnostic line under logs_root. Never raises, writes no output."""
    try:
        line = {
            "timestamp": core.current_timestamp(),
            "harness": harness,
            "kind": kind,
            "message": message,
        }
        optional = {"run_id": run_id, "session_id": session_id,
                    "counts": counts, "ids": ids}
        for key, value in optional.items():
            if value:
                line[key] = value
        core.append_event(pathlib.Path(logs_root) / DIAGNOSTICS_FILENAME, line)
        core.debug_log(f"diagnostic {kind}: {message}")
    except Exception as exc:
        core.debug_log("diagnostics.report failed", exc)
