"""Python writer for the cross-language lock test: appends long lines with locked_append.

Usage: py locked_append_py_writer.py <module dir> <target> <tag> <count> <pad>
"""
import json
import sys

module_dir, target, tag, count, pad = sys.argv[1:6]
sys.path.insert(0, module_dir)
import mosaic_logger_locked_append as writer  # noqa: E402

failed = 0
for seq in range(int(count)):
    event = {"event": "probe", "writer": tag, "seq": seq, "pad": "p" * int(pad)}
    if not writer.locked_append(target, (json.dumps(event) + "\n").encode("ascii")):
        failed += 1
sys.exit(0 if failed == 0 else 3)
