"""The ingest module is stdlib only and works when its files are copied flat into one folder,
as the deployment does next to each adapter's modules."""

import ast
import json
import shutil
import subprocess
import sys

from otel_ingest_helpers import SHARED_DIR, chat_line, make_run, write_otel

LOCKED_APPEND = SHARED_DIR / "mosaic_logger_locked_append.py"
# The ingest writes through the locked append, so the module is copied flat with the OTel files.
MODULE_FILES = sorted(SHARED_DIR.glob("mosaic_logger_otel*.py")) + [LOCKED_APPEND]

FLAT_SCRIPT = r"""
import json, sys, datetime
flat, run_dir, otel_path = sys.argv[1:4]
sys.path.insert(0, flat)
import mosaic_logger_otel as m
assert m.__file__.startswith(flat), m.__file__
res = m.ingest(run_dir, otel_path, budget_seconds=None,
               now=lambda: datetime.datetime(2026, 10, 28, 9, 0, 0, tzinfo=datetime.timezone.utc))
print(json.dumps({"status": res.status, "records": res.records_written,
                  "notice": m.report_once(run_dir, "variable_unset")}))
"""


def imported_modules(path):
    """(top-level module name, relative-import level) for every import in a source file."""
    tree = ast.parse(path.read_text(encoding="utf-8"))
    found = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            found += [(alias.name.split(".")[0], 0) for alias in node.names]
        elif isinstance(node, ast.ImportFrom):
            found.append(((node.module or "").split(".")[0], node.level))
    return found


def test_public_entry_module_exists():
    assert (SHARED_DIR / "mosaic_logger_otel.py") in MODULE_FILES
    assert LOCKED_APPEND in MODULE_FILES


def test_locked_append_module_exists_and_is_not_named_for_otel():
    assert LOCKED_APPEND.is_file()
    assert "otel" not in LOCKED_APPEND.name


def test_modules_import_only_standard_library_or_each_other_flat():
    own = {p.stem for p in MODULE_FILES}
    for path in MODULE_FILES:
        for name, level in imported_modules(path):
            assert level == 0, f"{path.name}: relative import of {name!r} breaks flat deployment"
            assert name in sys.stdlib_module_names or name in own, \
                f"{path.name}: imports {name!r}, neither standard library nor a sibling module"


def test_copied_flat_into_one_folder_the_module_imports_and_ingests(tmp_path):
    flat = tmp_path / "flat"
    flat.mkdir()
    for path in MODULE_FILES:
        shutil.copyfile(path, flat / path.name)
    run_dir, otel_path = make_run(tmp_path / "work")
    write_otel(otel_path, [chat_line("flat-1")])

    # -I -S: no user site, no site-packages, no PYTHONPATH: only the standard library and the flat folder.
    proc = subprocess.run(
        [sys.executable, "-I", "-S", "-c", FLAT_SCRIPT, str(flat), str(run_dir), str(otel_path)],
        capture_output=True, text=True, timeout=60, cwd=str(tmp_path),
    )

    assert proc.returncode == 0, proc.stderr
    assert json.loads(proc.stdout) == {"status": "ingested", "records": 1, "notice": True}
