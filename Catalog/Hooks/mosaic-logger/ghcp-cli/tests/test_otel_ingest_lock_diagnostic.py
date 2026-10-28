"""The ingest lock diagnostic reaches the default-visible file in the ghcp-cli adapter.

An ingest that finds another ingest of the run holding the lock reports `lock_held`
like every other ingest diagnostic, with MOSAIC_LOGGER_DEBUG unset.
"""

import os
import sys
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import otel_hook_support as support


def _lock_held(original, *args, **kwargs):
    diagnostic = types.SimpleNamespace(
        kind="lock_held", message="another ingest of this run holds the lock",
        counts={}, ids=[])
    return types.SimpleNamespace(
        status="lock_held", records_written=0, finalized=0, pending=0,
        lines_read=0, lines_malformed=0, diagnostics=[diagnostic])


class TestLockHeldDiagnostic(support.OtelHookCase):

    def test_a_held_ingest_lock_is_reported_without_the_debug_switch(self):
        self.assertNotIn("MOSAIC_LOGGER_DEBUG", os.environ)
        self.start_session()
        self.add_spans(support.usage_span("s1"))
        with support.ingest_patched(_lock_held):
            self.agent_stop()
        found = self.diag_lines("lock_held")
        self.assertEqual(1, len(found), f"diagnostics: {self.diag_lines()}")
        self.assertEqual(support.RUN, found[0].get("run_id"))


if __name__ == "__main__":
    unittest.main()
