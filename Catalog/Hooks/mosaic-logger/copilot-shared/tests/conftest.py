"""Puts the shared Copilot module folder on sys.path so tests import it flat, as deployed."""

import pathlib
import sys

SHARED_DIR = pathlib.Path(__file__).resolve().parents[1]
if str(SHARED_DIR) not in sys.path:
    sys.path.insert(0, str(SHARED_DIR))
