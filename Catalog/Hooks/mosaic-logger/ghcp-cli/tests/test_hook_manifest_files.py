"""The hook manifest ships the OTel ingest files with the vscode-ghcp variant only.

hook.yaml lists the shared OTel ingest modules (shared source, flat target) for vscode-ghcp,
none for ghcp-cli, and every adapter module of the variant folder for both, every listed source
resolves to a real file, the description block (not comments) mentions COPILOT_OTEL_FILE_EXPORTER_PATH, and the claude-code and opencode variants ship
no OTel ingest file.
"""

import glob
import os
import re
import unittest

_HOOK_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
_MANIFEST = os.path.join(_HOOK_DIR, "hook.yaml")
_VARIANT_RE = re.compile(r"^  ([a-z][a-z-]*):\s*$")
_SOURCE_RE = re.compile(r"^\s+- source:\s*(\S+)\s*$")
_TARGET_RE = re.compile(r"^\s+target:\s*(\S+)\s*$")


def _read_manifest():
    with open(_MANIFEST, encoding="utf-8") as handle:
        return handle.read()


def _entries_by_variant():
    """Map each variant to its (source, target) pairs from the manifest."""
    entries = {}
    in_variants = False
    current = None
    pending_source = None
    for line in _read_manifest().splitlines():
        if line.startswith("variants:"):
            in_variants = True
            continue
        if not in_variants:
            continue
        variant = _VARIANT_RE.match(line)
        if variant:
            current = variant.group(1)
            entries[current] = []
            continue
        source = _SOURCE_RE.match(line)
        if source and current:
            pending_source = source.group(1).strip("\"'")
            continue
        target = _TARGET_RE.match(line)
        if target and current and pending_source is not None:
            entries[current].append((pending_source, target.group(1).strip("\"'")))
            pending_source = None
    return entries


def _description_text():
    """The top-level description block scalar only (not comments or other keys)."""
    lines = _read_manifest().splitlines()
    start = next(i for i, line in enumerate(lines) if line.startswith("description:"))
    block = [lines[start]]
    for line in lines[start + 1:]:
        if line.strip() and not line.startswith((" ", "\t")):
            break
        block.append(line)
    return "\n".join(block)


def _shared_otel_files():
    pattern = os.path.join(_HOOK_DIR, "copilot-shared", "mosaic_logger_otel*.py")
    return sorted(os.path.basename(p) for p in glob.glob(pattern))


def _variant_modules(variant):
    pattern = os.path.join(_HOOK_DIR, variant, "mosaic_logger*.py")
    return sorted(os.path.basename(p) for p in glob.glob(pattern))


class TestCopilotVariantsShipOtelFiles(unittest.TestCase):

    def setUp(self):
        self.entries = _entries_by_variant()

    def _targets(self, variant):
        return [target for _, target in self.entries[variant]]

    def test_the_shared_otel_modules_exist_to_be_listed(self):
        self.assertTrue(_shared_otel_files())

    def test_each_shared_otel_module_is_listed_with_a_shared_source_and_flat_target(self):
        shared_dir = os.path.normpath(os.path.join(_HOOK_DIR, "copilot-shared"))
        variant = "vscode-ghcp"
        for name in _shared_otel_files():
            matches = [e for e in self.entries[variant] if os.path.basename(e[0]) == name]
            self.assertEqual(1, len(matches), "%s: %s listed once" % (variant, name))
            source, target = matches[0]
            self.assertEqual(name, target, "%s: %s target is flat" % (variant, name))
            self.assertNotIn("/", target)
            resolved = os.path.normpath(os.path.join(_HOOK_DIR, variant, source))
            self.assertEqual(os.path.join(shared_dir, name), resolved, variant)
            self.assertTrue(os.path.isfile(resolved), "%s: %s" % (variant, source))

    def test_ghcp_cli_ships_no_otel_module(self):
        otel = [e for e in self.entries["ghcp-cli"]
                if "otel" in os.path.basename(e[0]).lower() or "otel" in e[1].lower()]
        self.assertEqual([], otel)

    def test_ghcp_cli_folder_holds_no_otel_module(self):
        found = [m for m in _variant_modules("ghcp-cli") if "otel" in m]
        self.assertEqual([], found)

    def test_each_copilot_variant_lists_every_adapter_module_in_its_folder(self):
        for variant in ("vscode-ghcp", "ghcp-cli"):
            missing = [m for m in _variant_modules(variant) if m not in self._targets(variant)]
            self.assertEqual([], missing, variant)

    def test_every_listed_variant_source_resolves_to_a_file(self):
        for variant in ("vscode-ghcp", "ghcp-cli"):
            for source, _ in self.entries[variant]:
                path = os.path.normpath(os.path.join(_HOOK_DIR, variant, source))
                self.assertTrue(os.path.isfile(path), "%s: %s" % (variant, source))

    def test_the_description_mentions_the_otel_file_exporter_variable(self):
        self.assertIn("COPILOT_OTEL_FILE_EXPORTER_PATH", _description_text())

    def test_claude_code_and_opencode_ship_no_otel_ingest_files(self):
        for variant in ("claude-code", "opencode"):
            otel = [s for s, _ in self.entries[variant] if "otel" in os.path.basename(s)]
            self.assertEqual([], otel, variant)


class TestTranscriptLoggingModulesAreShipped(unittest.TestCase):
    """The ghcp-cli variant lists the transcript reader, the transcript logging and its own
    diagnostics writer, each with a local source and a flat target, and every one exists."""

    _NAMES = ("mosaic_logger_transcript_usage.py", "mosaic_logger_transcript_logging.py",
              "mosaic_logger_diagnostics.py")

    def setUp(self):
        self.entries = _entries_by_variant()["ghcp-cli"]

    def test_each_module_is_listed_once_with_a_local_source_and_flat_target(self):
        for name in self._NAMES:
            with self.subTest(module=name):
                matches = [e for e in self.entries if e[1] == name]
                self.assertEqual(1, len(matches))
                self.assertEqual(name, matches[0][0])

    def test_each_module_exists_in_the_variant_folder(self):
        for name in self._NAMES:
            with self.subTest(module=name):
                self.assertTrue(os.path.isfile(os.path.join(_HOOK_DIR, "ghcp-cli", name)))


class TestLockedAppendModuleIsShipped(unittest.TestCase):
    """The locked append module is shared source, flat target for both Copilot variants; the
    claude-code variant ships its own copy; opencode (TypeScript) ships none; and it is not
    named for OTel, so the claude-code variant still ships no OTel-named file."""

    _NAME = "mosaic_logger_locked_append.py"

    def setUp(self):
        self.entries = _entries_by_variant()

    def test_the_shared_module_exists(self):
        self.assertTrue(os.path.isfile(os.path.join(_HOOK_DIR, "copilot-shared", self._NAME)))

    def test_each_copilot_variant_lists_it_with_a_shared_source_and_flat_target(self):
        shared = os.path.normpath(os.path.join(_HOOK_DIR, "copilot-shared", self._NAME))
        for variant in ("vscode-ghcp", "ghcp-cli"):
            matches = [e for e in self.entries[variant] if os.path.basename(e[0]) == self._NAME]
            self.assertEqual(1, len(matches), variant)
            source, target = matches[0]
            self.assertEqual(self._NAME, target, variant)
            self.assertEqual(shared, os.path.normpath(os.path.join(_HOOK_DIR, variant, source)), variant)

    def test_claude_code_lists_its_own_copy_with_a_local_source(self):
        matches = [e for e in self.entries["claude-code"] if e[1] == self._NAME]
        self.assertEqual(1, len(matches))
        resolved = os.path.normpath(os.path.join(_HOOK_DIR, "claude-code", matches[0][0]))
        self.assertEqual(os.path.join(_HOOK_DIR, "claude-code", self._NAME), resolved)
        self.assertTrue(os.path.isfile(resolved))

    def test_opencode_does_not_ship_the_python_module(self):
        shipped = [e for e in self.entries["opencode"] if self._NAME in (e[0], e[1])]
        self.assertEqual([], shipped)

    def test_no_otel_named_file_is_shipped_to_claude_code_or_opencode(self):
        for variant in ("claude-code", "opencode"):
            otel = [e for e in self.entries[variant] if "otel" in os.path.basename(e[0]).lower()
                    or "otel" in e[1].lower()]
            self.assertEqual([], otel, variant)


if __name__ == "__main__":
    unittest.main()
