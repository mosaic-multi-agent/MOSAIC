"""Puts the shared Copilot module folder on sys.path before any test module imports the adapter,
mirroring the deployed flat layout where the shared modules sit beside the adapter modules."""

import pathlib
import sys

SHARED_DIR = pathlib.Path(__file__).resolve().parents[2] / "copilot-shared"
if str(SHARED_DIR) not in sys.path:
    sys.path.insert(0, str(SHARED_DIR))
