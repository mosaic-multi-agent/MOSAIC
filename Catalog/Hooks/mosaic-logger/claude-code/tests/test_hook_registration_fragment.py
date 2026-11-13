"""The claude-code registration fragment of hook.yaml registers the right hooks.

Claude Code starts a subagent without waiting for async hooks, so the dispatch
capture must run as a synchronous PreToolUse entry matching Agent|Task, while the
catch-all PreToolUse stays async. The fragment is read by line scanning (no YAML
library) and parsed as JSON; the tests assert structure, not argument text or
group order.
"""

import json
import os
import re
import sys
import textwrap
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger

_VARIANT = "claude-code"
_HOOK_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
_MANIFEST = os.path.join(_HOOK_DIR, "hook.yaml")
_VARIANT_RE = re.compile(r"^  ([a-z][a-z-]*):\s*$")
_FRAGMENT_RE = re.compile(r"^(\s*)fragment:\s*\|\s*$")

# Event -> async flag of its single entry, other than PreToolUse.
_OTHER_EVENTS = {
    "SessionStart": True,
    "SessionEnd": False,
    "UserPromptSubmit": True,
    "Stop": False,
    "PostToolUse": True,
    "PostToolUseFailure": True,
    "SubagentStart": False,
    "SubagentStop": True,
    "Notification": True,
    "PreCompact": True,
    "PostCompact": True,
}


def _fragment_text():
    """The fragment block of the claude-code variant, dedented."""
    current = None
    block = None
    indent = None
    with open(_MANIFEST, encoding="utf-8") as handle:
        lines = handle.read().splitlines()
    in_variants = False
    for line in lines:
        if line.startswith("variants:"):
            in_variants = True
            continue
        if not in_variants:
            continue
        if block is not None:
            if line.strip() == "" or len(line) - len(line.lstrip()) > indent:
                block.append(line)
                continue
            break
        variant = _VARIANT_RE.match(line)
        if variant:
            current = variant.group(1)
            continue
        match = _FRAGMENT_RE.match(line)
        if current == _VARIANT and match:
            block = []
            indent = len(match.group(1))
    return textwrap.dedent("\n".join(block)) if block else ""


def _hooks():
    return json.loads(_fragment_text())["hooks"]


def _entries(event):
    """All hook entries of an event, flattened out of its groups."""
    return [entry for group in _hooks()[event] for entry in group["hooks"]]


class TestFragmentIsReadable(unittest.TestCase):

    def test_fragment_is_found_and_is_valid_json_with_a_hooks_object(self):
        self.assertTrue(_fragment_text().strip())
        self.assertIsInstance(_hooks(), dict)

    def test_every_expected_event_is_registered(self):
        expected = set(_OTHER_EVENTS) | {"PreToolUse"}
        self.assertEqual(expected, set(_hooks()))


class TestPreToolUseRegistration(unittest.TestCase):

    def test_pre_tool_use_has_exactly_two_groups(self):
        self.assertEqual(2, len(_hooks()["PreToolUse"]))

    def test_exactly_one_synchronous_entry_and_it_matches_agent_and_task(self):
        sync_groups = [g for g in _hooks()["PreToolUse"]
                       if any(not e["async"] for e in g["hooks"])]
        self.assertEqual(1, len(sync_groups))
        self.assertEqual("Agent|Task", sync_groups[0].get("matcher"))
        self.assertEqual(1, len(sync_groups[0]["hooks"]))

    def test_synchronous_entry_passes_the_adapters_capture_argument(self):
        (group,) = [g for g in _hooks()["PreToolUse"] if g.get("matcher")]
        (entry,) = group["hooks"]
        self.assertIs(False, entry["async"])
        self.assertEqual("command", entry["type"])
        self.assertEqual(2, len(entry["args"]))
        self.assertTrue(entry["args"][0].endswith("mosaic_logger.py"))
        self.assertEqual(mosaic_logger.CAPTURE_DISPATCH_ARG, entry["args"][1])

    def test_exactly_one_async_catch_all_entry_with_script_path_only(self):
        groups = [g for g in _hooks()["PreToolUse"] if not g.get("matcher")]
        self.assertEqual(1, len(groups))
        (entry,) = groups[0]["hooks"]
        self.assertIs(True, entry["async"])
        self.assertEqual(1, len(entry["args"]))
        self.assertTrue(entry["args"][0].endswith("mosaic_logger.py"))

    def test_both_pre_tool_use_entries_run_the_same_script(self):
        scripts = {e["args"][0] for e in _entries("PreToolUse")}
        self.assertEqual(1, len(scripts))

    def test_both_pre_tool_use_entries_use_the_same_command_and_timeout_style(self):
        entries = _entries("PreToolUse")
        self.assertEqual(1, len({e["command"] for e in entries}))
        self.assertEqual(1, len({e["timeout"] for e in entries}))


class TestOtherEventsAreUnchanged(unittest.TestCase):

    def test_every_other_event_has_one_entry_with_its_current_async_flag(self):
        for event, expected_async in _OTHER_EVENTS.items():
            with self.subTest(event=event):
                groups = _hooks()[event]
                self.assertEqual(1, len(groups))
                entries = _entries(event)
                self.assertEqual(1, len(entries))
                self.assertIs(expected_async, entries[0]["async"])
                self.assertEqual(1, len(entries[0]["args"]))


if __name__ == "__main__":
    unittest.main()
