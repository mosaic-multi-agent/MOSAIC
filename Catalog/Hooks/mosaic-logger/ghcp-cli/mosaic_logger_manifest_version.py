"""mosaic_logger_manifest_version.py - adapter_version from the workspace manifest.

The version is the `version` of the entry whose ref has `kind: hook` and
`key: mosaic-logger` in <workspace root>/.mosaic/manifest.yaml. A minimal
line-based parse is used (no YAML library). Every failure returns None and
writes a debug line; nothing here raises.
"""

import pathlib

import mosaic_logger_core as core

_HOOK_KEY = "mosaic-logger"
_VERSION_PREFIX = "  version:"
_KIND_PREFIX = "    kind:"
_KEY_PREFIX = "    key:"


def _unquote(value: str) -> "str | None":
    """Strip whitespace and one layer of matching quotes; None if unterminated."""
    value = value.strip()
    if value[:1] in ('"', "'"):
        if len(value) < 2 or value[-1] != value[0]:
            return None
        value = value[1:-1]
    return value


def _split_entries(text: str) -> "list[list[str]]":
    """Group lines into manifest entries.

    An entry starts at a column-0 `- ref:` line and ends at the next non-blank
    line starting at column 0, so trailing top-level blocks are excluded.
    """
    entries: "list[list[str]]" = []
    current: "list[str] | None" = None
    for line in text.splitlines():
        if not line.strip():
            continue
        if line[0] not in (" ", "\t"):
            current = [] if line.startswith("- ref:") else None
            if current is not None:
                entries.append(current)
            continue
        if current is not None:
            current.append(line)
    return entries


def _field(lines: "list[str]", prefix: str) -> "list[str]":
    return [ln[len(prefix):] for ln in lines if ln.startswith(prefix)]


def _is_hook_entry(lines: "list[str]") -> bool:
    kinds = [_unquote(v) for v in _field(lines, _KIND_PREFIX)]
    keys = [_unquote(v) for v in _field(lines, _KEY_PREFIX)]
    return kinds == ["hook"] and keys == [_HOOK_KEY]


def _parse_version(text: str) -> "str | None":
    matches = [e for e in _split_entries(text) if _is_hook_entry(e)]
    if not matches:
        core.debug_log("adapter_version: no hook/mosaic-logger entry in manifest")
        return None
    if len(matches) > 1:
        core.debug_log("adapter_version: multiple hook/mosaic-logger entries in manifest")
        return None
    versions = _field(matches[0], _VERSION_PREFIX)
    if len(versions) != 1:
        core.debug_log(
            f"adapter_version: expected one entry version, found {len(versions)}")
        return None
    value = _unquote(versions[0])
    if not value:
        core.debug_log("adapter_version: entry version is empty or unparseable")
        return None
    return value


def read_hook_version(workspace_root) -> "str | None":
    """Version of the kind: hook / key: mosaic-logger manifest entry, else None."""
    try:
        if workspace_root is None:
            core.debug_log("adapter_version: no workspace root")
            return None
        path = pathlib.Path(workspace_root) / ".mosaic" / "manifest.yaml"
        try:
            with open(path, "r", encoding="utf-8", errors="replace") as fh:
                text = fh.read()
        except OSError as exc:
            core.debug_log(f"adapter_version: manifest unreadable at {path}", exc)
            return None
        return _parse_version(text)
    except Exception as exc:
        core.debug_log("adapter_version: unexpected failure", exc)
        return None


def adapter_version(ctx: "core.HookContext") -> "str | None":
    """Manifest version for the context's resolved workspace root."""
    return read_hook_version(getattr(ctx, "workspace_root", None))
