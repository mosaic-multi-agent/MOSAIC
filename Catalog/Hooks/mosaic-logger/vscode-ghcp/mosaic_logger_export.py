"""mosaic_logger_export.py — Byte-for-byte transcript export with sidecar metadata.

Owns: copying a harness transcript to a *.raw file via atomic replace, and writing
the *.meta.json sidecar that describes the copy.

Key divergence from claude-code (D6): NATIVE_FORMAT is 'vscode-ghcp-transcript-raw'
rather than '...-jsonl' or '...-json' because VS Code documents the transcript format
as unstable and unspecified. Asserting a specific encoding would fabricate metadata —
a violation of the degrade-never-fabricate rule applied to sidecar metadata.
"""

import datetime
import json
import pathlib

import mosaic_logger_core as core


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

NATIVE_FORMAT = "vscode-ghcp-transcript-raw"   # D6
FOREIGN_SESSION_REASON = "target transcript belongs to a different session"


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

class ExportResult:
    """Outcome of an export_transcript call."""

    def __init__(self, ok: bool,
                 raw_path: "pathlib.Path | None" = None,
                 meta_path: "pathlib.Path | None" = None,
                 byte_count: "int | None" = None,
                 reason: "str | None" = None):
        self.ok = ok
        self.raw_path = raw_path
        self.meta_path = meta_path
        self.byte_count = byte_count
        self.reason = reason


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _now_iso() -> str:
    """Current UTC time as ISO 8601 with ms precision and Z suffix."""
    now = datetime.datetime.now(datetime.timezone.utc)
    ms = now.microsecond // 1000
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ms:03d}Z"


def _sidecar_path(raw_path: pathlib.Path) -> pathlib.Path:
    """Derive the sidecar path from the raw path (replace extension with .meta.json)."""
    return raw_path.with_suffix("").with_suffix(".meta.json")


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def _recorded_owner(target_raw: pathlib.Path) -> "str | None":
    """Session recorded in the target's sidecar; None when unowned
    (no sidecar, unreadable, not an object, or no non-empty session_id)."""
    try:
        doc = json.loads(_sidecar_path(target_raw).read_text(encoding="utf-8"))
    except Exception:
        return None
    if not isinstance(doc, dict):
        return None
    owner = doc.get("session_id")
    return owner if isinstance(owner, str) and owner else None


def build_sidecar(source_path: str,
                  source_mechanism: str,
                  byte_count: int,
                  owner_session_id: "str | None" = None) -> dict:
    """Build the sidecar document. Exposed separately so content is assertable
    without performing a copy. session_id is recorded only with an owner."""
    doc = {
        "harness": core.HARNESS,
        "native_format": NATIVE_FORMAT,
        "source_path": source_path,
        "source_mechanism": source_mechanism,
        "captured_at": _now_iso(),
        "byte_count": byte_count,
    }
    if owner_session_id:
        doc["session_id"] = owner_session_id
    return doc


def export_transcript(source_path: "str | None",
                      target_raw: pathlib.Path,
                      source_mechanism: str,
                      owner_session_id: "str | None" = None) -> ExportResult:
    """Copy a harness transcript byte-for-byte to target_raw and write the sidecar.

    Contract:
      - Copy is byte-identical: read binary, write binary, no parsing.
      - Write goes through atomic_replace (no torn files under contention).
      - Sidecar written only when the raw copy succeeded.
      - Missing/unreadable/None/empty source_path returns ok=False, writes nothing.
      - With owner_session_id, a target whose sidecar names a different session
        is kept as is (ok=False, FOREIGN_SESSION_REASON); an unowned target is
        replaced and the sidecar records the exporting session.
      - Never raises.
    """
    try:
        if not source_path:
            return ExportResult(ok=False, reason="empty or None source_path")

        src = pathlib.Path(source_path)
        try:
            data = src.read_bytes()
        except Exception as exc:
            core.debug_log(f"export_transcript: cannot read {source_path}", exc)
            return ExportResult(ok=False, reason=f"unreadable: {exc}")

        if owner_session_id:
            existing = _recorded_owner(target_raw)
            if existing and existing != owner_session_id:
                core.debug_log(
                    f"export_transcript: {target_raw.name} belongs to session "
                    f"{existing!r}; not replacing with session {owner_session_id!r}"
                )
                return ExportResult(ok=False, reason=FOREIGN_SESSION_REASON)

        byte_count = len(data)
        target_raw.parent.mkdir(parents=True, exist_ok=True)
        ok = core.atomic_replace(target_raw, data)
        if not ok:
            return ExportResult(ok=False, reason="atomic_replace failed for raw file")

        # Sidecar written only after successful raw copy
        sidecar_doc = build_sidecar(source_path, source_mechanism, byte_count,
                                   owner_session_id)
        sidecar_json = json.dumps(sidecar_doc, ensure_ascii=False).encode("utf-8")
        meta_path = _sidecar_path(target_raw)
        core.atomic_replace(meta_path, sidecar_json)

        return ExportResult(ok=True, raw_path=target_raw, meta_path=meta_path,
                            byte_count=byte_count)

    except Exception as exc:
        core.debug_log("export_transcript failed unexpectedly", exc)
        return ExportResult(ok=False, reason=str(exc))
