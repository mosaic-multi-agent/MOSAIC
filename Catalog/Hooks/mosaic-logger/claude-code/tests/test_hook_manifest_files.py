"""The hook manifest ships a complete claude-code adapter.

hook.yaml lists every adapter module (mosaic_logger*.py) of the claude-code folder in the
claude-code variant file list, and every listed source resolves to a real file, so a deployed
adapter never fails on a missing module.
"""

import glob
import os
import re
import unittest

_VARIANT = "claude-code"
_HOOK_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
_MANIFEST = os.path.join(_HOOK_DIR, "hook.yaml")
_VARIANT_RE = re.compile(r"^  ([a-z][a-z-]*):\s*$")
_SOURCE_RE = re.compile(r"^\s+- source:\s*(\S+)\s*$")
_TARGET_RE = re.compile(r"^\s+target:\s*(\S+)\s*$")


def _claude_code_entries():
    """The (source, target) pairs listed under the claude-code variant."""
    entries = []
    in_variants = False
    current = None
    pending_source = None
    with open(_MANIFEST, encoding="utf-8") as handle:
        lines = handle.read().splitlines()
    for line in lines:
        if line.startswith("variants:"):
            in_variants = True
            continue
        if not in_variants:
            continue
        variant = _VARIANT_RE.match(line)
        if variant:
            current = variant.group(1)
            continue
        if current != _VARIANT:
            continue
        source = _SOURCE_RE.match(line)
        if source:
            pending_source = source.group(1).strip("\"'")
            continue
        target = _TARGET_RE.match(line)
        if target and pending_source is not None:
            entries.append((pending_source, target.group(1).strip("\"'")))
            pending_source = None
    return entries


def _adapter_modules():
    pattern = os.path.join(_HOOK_DIR, _VARIANT, "mosaic_logger*.py")
    return sorted(os.path.basename(p) for p in glob.glob(pattern))


class TestClaudeCodeManifestIsComplete(unittest.TestCase):

    def test_the_variant_lists_files_to_check(self):
        self.assertTrue(_claude_code_entries())
        self.assertTrue(_adapter_modules())

    def test_every_adapter_module_in_the_folder_is_listed(self):
        targets = [target for _, target in _claude_code_entries()]

        missing = [m for m in _adapter_modules() if m not in targets]

        self.assertEqual([], missing)

    def test_every_listed_source_resolves_to_a_file(self):
        missing = []
        for source, _ in _claude_code_entries():
            path = os.path.normpath(os.path.join(_HOOK_DIR, _VARIANT, source))
            if not os.path.isfile(path):
                missing.append(source)

        self.assertEqual([], missing)


class TestClaudeCodeShipsTheLockedAppendModule(unittest.TestCase):

    _NAME = "mosaic_logger_locked_append.py"

    def test_the_module_exists_in_the_folder_and_is_listed_with_a_local_flat_entry(self):
        self.assertTrue(os.path.isfile(os.path.join(_HOOK_DIR, _VARIANT, self._NAME)))
        self.assertIn((self._NAME, self._NAME), _claude_code_entries())

    def test_no_listed_file_is_named_for_otel(self):
        otel = [e for e in _claude_code_entries()
                if "otel" in e[0].lower() or "otel" in e[1].lower()]

        self.assertEqual([], otel)


if __name__ == "__main__":
    unittest.main()
