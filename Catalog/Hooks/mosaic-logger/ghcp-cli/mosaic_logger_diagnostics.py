"""mosaic_logger_diagnostics.py - Default-visible adapter diagnostics.

Appends one JSON line per call to OrchestrationLogs/mosaic-logger-diagnostics.jsonl through
the locked append. The file is written regardless of MOSAIC_LOGGER_DEBUG, sits at the logs
root where log readers do not treat plain files as event streams, and is never written to
stdout or stderr. The writer does not deduplicate: callers decide when a line is due.
"""

import json
import pathlib

import mosaic_logger_core as core
import mosaic_logger_locked_append as locked_append

DIAGNOSTICS_FILENAME = "mosaic-logger-diagnostics.jsonl"


def report(logs_root, kind: str, message: str, *, harness: str,
           run_id: "str | None" = None, session_id: "str | None" = None,
           counts: "dict | None" = None, ids: "list | None" = None) -> bool:
    """Append one diagnostic line under logs_root. True when it was written.

    Never raises, writes no output.
    """
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
        data = (json.dumps(line, ensure_ascii=False) + "\n").encode("utf-8")
        written = locked_append.locked_append(
            pathlib.Path(logs_root) / DIAGNOSTICS_FILENAME, data)
        core.debug_log(f"diagnostic {kind}: {message}")
        return written is True
    except Exception as exc:
        core.debug_log("diagnostics.report failed", exc)
        return False
