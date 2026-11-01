"""Session guard on the shared orchestrator transcript export.

The shared target `00_orchestrator_session.raw` is owned by the session whose transcript it
holds (recorded in the sidecar). A different session never replaces it; the owner's
re-export does; an unowned target (no sidecar, or a sidecar without a recorded session) is
replaced and then records the exporter. Without an owner there is no guard.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_export as export_module

_OLD = b'{"type": "assistant", "n": 1}\n'
_NEW = b'{"type": "assistant", "n": 1}\n{"type": "assistant", "n": 2}\n'
_FOREIGN = b'{"type": "assistant", "other": true}\n'


class _ExportCase(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = pathlib.Path(self._tmp.name)
        self.target = self.dir / "run" / "00_orchestrator_session.raw"
        self.sidecar = self.dir / "run" / "00_orchestrator_session.meta.json"

    def source(self, content, name="src.jsonl"):
        path = self.dir / name
        path.write_bytes(content)
        return str(path)

    def export(self, content, session, name="src.jsonl"):
        return export_module.export_transcript(
            self.source(content, name), self.target, "transcript_path",
            owner_session_id=session)

    def read_sidecar(self):
        return json.loads(self.sidecar.read_text(encoding="utf-8"))

    def seed_unowned(self, raw=b"legacy raw\n"):
        self.target.parent.mkdir(parents=True, exist_ok=True)
        self.target.write_bytes(raw)


class TestForeignSessionKeepsTarget(_ExportCase):
    def test_second_session_leaves_raw_and_sidecar_unchanged(self):
        self.export(_OLD, "session-a")
        raw_before = self.target.read_bytes()
        sidecar_before = self.sidecar.read_bytes()

        result = self.export(_FOREIGN, "session-b", name="other.jsonl")

        self.assertFalse(result.ok)
        self.assertEqual(raw_before, self.target.read_bytes())
        self.assertEqual(sidecar_before, self.sidecar.read_bytes())

    def test_second_session_failure_names_the_reason(self):
        self.export(_OLD, "session-a")
        result = self.export(_FOREIGN, "session-b", name="other.jsonl")
        self.assertEqual(export_module.FOREIGN_SESSION_REASON, result.reason)

    def test_first_export_records_the_exporting_session(self):
        result = self.export(_OLD, "session-a")
        self.assertTrue(result.ok)
        self.assertEqual("session-a", self.read_sidecar()["session_id"])

    def test_sidecar_keeps_existing_fields_besides_session_id(self):
        self.export(_OLD, "session-a")
        sidecar = self.read_sidecar()
        for key in ("harness", "native_format", "source_path",
                    "source_mechanism", "captured_at", "byte_count"):
            self.assertIn(key, sidecar)
        self.assertEqual(len(_OLD), sidecar["byte_count"])

    def test_unusable_source_reports_source_failure_not_foreign_session(self):
        self.export(_OLD, "session-a")
        result = export_module.export_transcript(
            str(self.dir / "missing.jsonl"), self.target, "transcript_path",
            owner_session_id="session-b")
        self.assertFalse(result.ok)
        self.assertNotEqual(export_module.FOREIGN_SESSION_REASON, result.reason)
        self.assertEqual(_OLD, self.target.read_bytes())


class TestSameSessionReexport(_ExportCase):
    def test_owner_reexport_replaces_the_transcript(self):
        self.export(_OLD, "session-a")
        result = self.export(_NEW, "session-a")
        self.assertTrue(result.ok)
        self.assertEqual(_NEW, self.target.read_bytes())

    def test_owner_reexport_updates_byte_count_and_keeps_owner(self):
        self.export(_OLD, "session-a")
        self.export(_NEW, "session-a")
        sidecar = self.read_sidecar()
        self.assertEqual(len(_NEW), sidecar["byte_count"])
        self.assertEqual("session-a", sidecar["session_id"])

    def test_owner_reexport_after_foreign_attempt_still_updates(self):
        self.export(_OLD, "session-a")
        self.export(_FOREIGN, "session-b", name="other.jsonl")
        self.export(_NEW, "session-a")
        self.assertEqual(_NEW, self.target.read_bytes())


class TestUnownedTargetIsReplaced(_ExportCase):
    def test_target_without_sidecar_is_replaced_and_records_exporter(self):
        self.seed_unowned()
        result = self.export(_NEW, "session-b")
        self.assertTrue(result.ok)
        self.assertEqual(_NEW, self.target.read_bytes())
        self.assertEqual("session-b", self.read_sidecar()["session_id"])

    def test_sidecar_without_session_is_replaced_and_records_exporter(self):
        self.seed_unowned()
        self.sidecar.write_text(json.dumps(
            {"harness": "vscode-ghcp", "byte_count": 11}), encoding="utf-8")
        result = self.export(_NEW, "session-b")
        self.assertTrue(result.ok)
        self.assertEqual(_NEW, self.target.read_bytes())
        self.assertEqual("session-b", self.read_sidecar()["session_id"])

    def test_sidecar_with_empty_session_is_treated_as_unowned(self):
        self.seed_unowned()
        self.sidecar.write_text(json.dumps({"session_id": ""}), encoding="utf-8")
        self.assertTrue(self.export(_NEW, "session-b").ok)
        self.assertEqual("session-b", self.read_sidecar()["session_id"])

    def test_corrupt_sidecar_is_treated_as_unowned(self):
        self.seed_unowned()
        self.sidecar.write_text("{not json", encoding="utf-8")
        self.assertTrue(self.export(_NEW, "session-b").ok)
        self.assertEqual(_NEW, self.target.read_bytes())
        self.assertEqual("session-b", self.read_sidecar()["session_id"])

    def test_non_object_sidecar_is_treated_as_unowned(self):
        self.seed_unowned()
        self.sidecar.write_text("[1, 2]", encoding="utf-8")
        self.assertTrue(self.export(_NEW, "session-b").ok)
        self.assertEqual("session-b", self.read_sidecar()["session_id"])


class TestExportWithoutOwner(_ExportCase):
    """No owner means no guard and no session_id, as for invocation transcripts."""

    def test_export_without_owner_replaces_and_records_no_session(self):
        self.export(_OLD, "session-a")
        result = export_module.export_transcript(
            self.source(_NEW, "n.jsonl"), self.target, "transcript_path")
        self.assertTrue(result.ok)
        self.assertEqual(_NEW, self.target.read_bytes())
        self.assertNotIn("session_id", self.read_sidecar())

    def test_build_sidecar_includes_session_id_only_with_an_owner(self):
        with_owner = export_module.build_sidecar(
            "p", "transcript_path", 3, owner_session_id="s1")
        without = export_module.build_sidecar("p", "transcript_path", 3)
        self.assertEqual("s1", with_owner["session_id"])
        self.assertNotIn("session_id", without)


if __name__ == "__main__":
    unittest.main()
