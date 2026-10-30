"""Holds the Python append lock of a target for a while, writing one marker line inside it.

Usage: py locked_append_py_holder.py <module dir> <target> <hold seconds>
Prints "held" once the lock is held, so the test knows when to start its own writer.
"""
import json
import sys
import time

module_dir, target, hold_seconds = sys.argv[1:4]
sys.path.insert(0, module_dir)
import mosaic_logger_locked_append as writer  # noqa: E402

with writer.holding_lock(target) as held:
    if not held:
        sys.exit(4)
    print("held", flush=True)
    time.sleep(float(hold_seconds))
    with open(target, "ab") as handle:
        handle.write((json.dumps({"event": "probe", "writer": "py-holder", "seq": 0}) + "\n").encode("ascii"))
