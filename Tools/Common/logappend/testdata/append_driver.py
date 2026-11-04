"""Test driver for the Go locked-append tests: runs the Python locked append in a child process.

Usage:
  append_driver.py append <target> <tag-prefix> <count> <size>
      appends <count> long lines "<prefix>-<i>|<filler>" with locked_append; exit 1 if one is dropped
  append_driver.py hold <target> <hold-seconds> <text>
      takes the lock with holding_lock, prints "locked", holds it, writes <text> as a plain line, exits
"""

import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mosaic_logger_locked_append as la  # noqa: E402


def do_append(target, prefix, count, size):
    for i in range(count):
        tag = "%s-%d" % (prefix, i)
        line = (tag + "|" + prefix[0] * size + "\n").encode()
        if not la.locked_append(target, line):
            print("dropped " + tag)
            return 1
    return 0


def do_hold(target, seconds, text):
    with la.holding_lock(target) as held:
        if not held:
            print("not locked")
            return 1
        print("locked")
        sys.stdout.flush()
        time.sleep(seconds)
        with open(target, "ab") as handle:
            handle.write((text + "\n").encode())
    return 0


if __name__ == "__main__":
    mode = sys.argv[1]
    if mode == "append":
        sys.exit(do_append(sys.argv[2], sys.argv[3], int(sys.argv[4]), int(sys.argv[5])))
    sys.exit(do_hold(sys.argv[2], float(sys.argv[3]), sys.argv[4]))
