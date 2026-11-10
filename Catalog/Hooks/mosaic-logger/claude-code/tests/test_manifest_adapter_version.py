"""Tests for the adapter version read from the workspace manifest.

The version is the `version` of the manifest entry whose ref is kind: hook,
key: mosaic-logger, read from <workspace root>/.mosaic/manifest.yaml. Covers
entry position, look-alike entries, the nested catalog version, trailing
top-level blocks, value quoting, CRLF, every unavailable-version outcome
(with a debug line and no exception), and the run_start event.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_core as core
import mosaic_logger_handlers_session as session_handlers
import mosaic_logger_manifest_version as manifest_version

TEST_TIMESTAMP = "2026-01-01T00:00:00.000Z"
TEST_RUN_ID = "20260101T000000Z-abcd"

HEADER = (
    "schema_version: 1\n"
    "harness_id: claude-code\n"
    "updated_at: 2026-01-01T00:00:00Z\n"
    "entries:\n"
)
TRAILER = (
    "tool_version:\n"
    "  version: 7.7.7\n"
    "protocol_source:\n"
    "  version: 8.8.8\n"
    "bundle_source:\n"
    "  version: 9.9.9\n"
)


def entry(kind="hook", key="mosaic-logger", version="1.2.3", catalog=None,
          extra_version=None):
    """One manifest entry shaped like the Deploy tool writes it."""
    lines = [
        "- ref:",
        f"    kind: {kind}",
        f"    key: {key}",
        "  target_path: .claude/hooks/x",
    ]
    if version is not None:
        lines.append(f"  version: {version}")
    if extra_version is not None:
        lines.append(f"  version: {extra_version}")
    lines += [
        "  harness_version: 2.0.0",
        "  injections_version: 3.0.0",
        "  content_hash: abc123",
        "  deployed_at: 2026-01-01T00:00:00Z",
    ]
    if catalog is not None:
        lines += ["  catalog:", f"    version: {catalog}"]
    return "\n".join(lines) + "\n"


def manifest(*entries, trailer=TRAILER):
    return HEADER + "".join(entries) + trailer


def make_ctx(root: pathlib.Path) -> core.HookContext:
    paths = core.build_paths(root)
    ctx = core.HookContext(
        {"hook_event_name": "SessionStart", "session_id": "s1"},
        root, paths, TEST_TIMESTAMP)
    ctx.run_id = TEST_RUN_ID
    return ctx


class ManifestCase(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = pathlib.Path(self._tmp.name)
        self.debug_file = self.root / "debug.log"
        patcher = mock.patch.dict(
            os.environ, {core.DEBUG_ENV_VAR: str(self.debug_file)})
        patcher.start()
        self.addCleanup(patcher.stop)

    def write_manifest(self, content: str, newline="\n"):
        directory = self.root / ".mosaic"
        directory.mkdir(parents=True, exist_ok=True)
        data = content.replace("\n", newline).encode("utf-8")
        (directory / "manifest.yaml").write_bytes(data)

    def read(self):
        return manifest_version.read_hook_version(self.root)

    def debug_text(self) -> str:
        if not self.debug_file.exists():
            return ""
        return self.debug_file.read_text(encoding="utf-8")


class TestEntrySelection(ManifestCase):
    def test_single_entry_version_returned(self):
        self.write_manifest(manifest(entry(version="4.5.6")))
        self.assertEqual("4.5.6", self.read())

    def test_first_of_several_entries(self):
        self.write_manifest(manifest(
            entry(version="1.0.0"),
            entry(key="other-hook", version="2.0.0"),
            entry(kind="agent", key="a", version="3.0.0")))
        self.assertEqual("1.0.0", self.read())

    def test_middle_of_several_entries(self):
        self.write_manifest(manifest(
            entry(key="other-hook", version="2.0.0"),
            entry(version="1.0.0"),
            entry(kind="skill", key="s", version="3.0.0")))
        self.assertEqual("1.0.0", self.read())

    def test_last_of_several_entries(self):
        self.write_manifest(manifest(
            entry(key="other-hook", version="2.0.0"),
            entry(kind="agent", key="a", version="3.0.0"),
            entry(version="1.0.0")))
        self.assertEqual("1.0.0", self.read())

    def test_other_kind_with_same_key_is_not_taken(self):
        self.write_manifest(manifest(
            entry(kind="skill", key="mosaic-logger", version="5.0.0"),
            entry(version="1.0.0")))
        self.assertEqual("1.0.0", self.read())

    def test_only_other_kind_with_same_key_gives_none(self):
        self.write_manifest(manifest(
            entry(kind="agent", key="mosaic-logger", version="5.0.0")))
        self.assertIsNone(self.read())

    def test_other_hook_key_is_not_taken(self):
        self.write_manifest(manifest(entry(key="mosaic-logger-extra", version="6.0.0")))
        self.assertIsNone(self.read())

    def test_nested_catalog_version_is_not_taken(self):
        self.write_manifest(manifest(entry(version="1.0.0", catalog="9.1.1")))
        self.assertEqual("1.0.0", self.read())

    def test_catalog_version_not_taken_when_entry_has_no_version(self):
        self.write_manifest(manifest(entry(version=None, catalog="9.1.1")))
        self.assertIsNone(self.read())

    def test_trailing_blocks_not_taken_for_last_entry_without_version(self):
        self.write_manifest(manifest(
            entry(key="other-hook", version="2.0.0"),
            entry(version=None)))
        self.assertIsNone(self.read())

    def test_trailing_blocks_do_not_override_last_entry_version(self):
        self.write_manifest(manifest(entry(version="1.0.0")))
        self.assertEqual("1.0.0", self.read())


class TestValueSyntax(ManifestCase):
    def test_double_quoted_value(self):
        self.write_manifest(manifest(entry(version='"1.2.3"')))
        self.assertEqual("1.2.3", self.read())

    def test_single_quoted_value(self):
        self.write_manifest(manifest(entry(version="'1.2.3'")))
        self.assertEqual("1.2.3", self.read())

    def test_quoted_kind_and_key(self):
        self.write_manifest(manifest(entry(
            kind='"hook"', key="'mosaic-logger'", version="1.2.3")))
        self.assertEqual("1.2.3", self.read())

    def test_crlf_line_endings(self):
        self.write_manifest(manifest(
            entry(key="other-hook", version="2.0.0"),
            entry(version="1.2.3", catalog="9.9.9")), newline="\r\n")
        self.assertEqual("1.2.3", self.read())

    def test_crlf_with_quoted_value(self):
        self.write_manifest(manifest(entry(version='"1.2.3"')), newline="\r\n")
        self.assertEqual("1.2.3", self.read())


class TestUnavailableVersion(ManifestCase):
    def assert_none_with_debug_line(self):
        self.assertIsNone(self.read())
        self.assertTrue(self.debug_text().strip(),
                        "expected a debug log line explaining the omission")

    def test_missing_manifest(self):
        self.assert_none_with_debug_line()

    def test_unreadable_manifest(self):
        directory = self.root / ".mosaic"
        directory.mkdir()
        (directory / "manifest.yaml").mkdir()  # a directory cannot be read as a file
        self.assert_none_with_debug_line()

    def test_binary_manifest_does_not_raise(self):
        directory = self.root / ".mosaic"
        directory.mkdir()
        (directory / "manifest.yaml").write_bytes(b"\x00\xff\xfe garbage \x01")
        self.assertIsNone(self.read())

    def test_no_workspace_root(self):
        self.assertIsNone(manifest_version.read_hook_version(None))
        self.assertTrue(self.debug_text().strip())

    def test_missing_entry(self):
        self.write_manifest(manifest(entry(key="other-hook", version="2.0.0")))
        self.assert_none_with_debug_line()

    def test_empty_entries(self):
        self.write_manifest(HEADER + TRAILER)
        self.assert_none_with_debug_line()

    def test_entry_without_version(self):
        self.write_manifest(manifest(entry(version=None), entry(key="o", version="2.0.0")))
        self.assert_none_with_debug_line()

    def test_duplicate_matching_entries(self):
        self.write_manifest(manifest(entry(version="1.0.0"), entry(version="1.0.0")))
        self.assert_none_with_debug_line()

    def test_duplicate_entry_versions(self):
        self.write_manifest(manifest(entry(version="1.0.0", extra_version="2.0.0")))
        self.assert_none_with_debug_line()

    def test_empty_version(self):
        self.write_manifest(manifest(entry(version="")))
        self.assert_none_with_debug_line()

    def test_empty_quoted_version(self):
        self.write_manifest(manifest(entry(version='""')))
        self.assert_none_with_debug_line()

    def test_unterminated_quote_version(self):
        self.write_manifest(manifest(entry(version='"1.2.3')))
        self.assert_none_with_debug_line()

    def test_no_debug_line_on_success(self):
        self.write_manifest(manifest(entry(version="1.0.0")))
        self.assertEqual("1.0.0", self.read())
        self.assertEqual("", self.debug_text())


class TestAdapterVersionFromContext(ManifestCase):
    def test_uses_context_workspace_root(self):
        self.write_manifest(manifest(entry(version="3.2.1")))
        self.assertEqual("3.2.1", manifest_version.adapter_version(make_ctx(self.root)))

    def test_none_when_manifest_missing(self):
        self.assertIsNone(manifest_version.adapter_version(make_ctx(self.root)))


class TestRunStartCarriesManifestVersion(ManifestCase):
    def run_start_event(self) -> dict:
        ctx = make_ctx(self.root)
        session_handlers.emit_run_start(ctx)
        path = ctx.paths.orchestrator_events(TEST_RUN_ID)
        lines = [ln for ln in path.read_text("utf-8").splitlines() if ln.strip()]
        self.assertEqual(1, len(lines))
        return json.loads(lines[0])

    def test_run_start_has_manifest_version(self):
        self.write_manifest(manifest(
            entry(key="other-hook", version="2.0.0"),
            entry(version="1.2.3", catalog="9.9.9")))
        self.assertEqual("1.2.3", self.run_start_event()["adapter_version"])

    def test_run_start_omits_version_when_manifest_missing(self):
        event = self.run_start_event()
        self.assertEqual("run_start", event["event"])
        self.assertNotIn("adapter_version", event)

    def test_run_start_omits_version_when_entry_missing(self):
        self.write_manifest(manifest(entry(key="other-hook", version="2.0.0")))
        event = self.run_start_event()
        self.assertNotIn("adapter_version", event)

    def test_run_start_omits_version_when_ambiguous(self):
        self.write_manifest(manifest(entry(version="1.0.0"), entry(version="2.0.0")))
        self.assertNotIn("adapter_version", self.run_start_event())


class TestNoHookYamlReader(unittest.TestCase):
    def test_core_no_longer_reads_hook_yaml(self):
        self.assertFalse(hasattr(core, "read_adapter_version"))
        self.assertFalse(hasattr(core, "adapter_version"))


if __name__ == "__main__":
    unittest.main()
