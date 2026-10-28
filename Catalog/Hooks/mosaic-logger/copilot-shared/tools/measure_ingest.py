"""One-off measurement of the hook-side Copilot OTel ingest cost (maintainer tool).

Not deployed and not a test. Usage:

    py measure_ingest.py <otel-file.jsonl> [--lines-per-boundary N] [--boundaries N]

Scenarios, each in a fresh scratch run folder (nothing is written next to the sample):
  1. first ingest from the middle of the file (start_offset = size / 2, no time limit)
  2. incremental ingests: the file is copied up to the middle, ingested once, then
     N complete lines are appended per simulated boundary and ingested again
  3. whole-file ingest under the default time budget, and under a 50 ms budget
  4. interpreter start-up plus import of the ingest module (a hook process pays this per event)

The scratch run folder has no run keys, so calls stay pending; the cost of reading, parsing
and saving state is measured, but not the cost of writing usage records to stream files.
"""

import argparse
import os
import shutil
import statistics
import subprocess
import sys
import tempfile
import time

SHARED = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, SHARED)

import mosaic_logger_otel as otel  # noqa: E402


def timed_ingest(run_dir, path, **kwargs):
    started = time.perf_counter()
    result = otel.ingest(run_dir, path, **kwargs)
    return time.perf_counter() - started, result


def fresh_run_dir(root, name):
    path = os.path.join(root, name)
    os.makedirs(path)
    return path


def ms(seconds):
    return "%.1f ms" % (seconds * 1000)


def describe(result):
    return "status=%s lines=%s malformed=%s diagnostics=%s" % (
        result.status, result.lines_read, result.lines_malformed,
        [d.kind for d in result.diagnostics])


def first_ingest_mid_file(root, path, size):
    elapsed, result = timed_ingest(fresh_run_dir(root, "mid"), path, start_offset=size // 2,
                                   budget_seconds=None)
    print("first ingest from offset %d of %d: %s  (%s)" % (size // 2, size, ms(elapsed), describe(result)))


def read_complete_lines(path, start, count):
    """Return (bytes of up to count complete lines starting at start, offset after them)."""
    chunk = []
    with open(path, "rb") as handle:
        handle.seek(start)
        for _ in range(count):
            line = handle.readline()
            if not line.endswith(b"\n"):
                break
            chunk.append(line)
    return b"".join(chunk), start + sum(len(line) for line in chunk)


def incremental(root, path, size, lines_per_boundary, boundaries):
    work = os.path.join(root, "growing.jsonl")
    with open(path, "rb") as source, open(work, "wb") as target:
        source.seek(size // 2)
        source.readline()  # align to a line boundary
        middle = source.tell()
        source.seek(0)
        target.write(source.read(middle))
    run_dir = fresh_run_dir(root, "incremental")
    base_elapsed, base = timed_ingest(run_dir, work, whole_file=True, budget_seconds=None)
    print("incremental base ingest of first %d bytes: %s  (%s)" % (middle, ms(base_elapsed), describe(base)))
    offset, samples, sizes = middle, [], []
    for _ in range(boundaries):
        chunk, after = read_complete_lines(path, offset, lines_per_boundary)
        if not chunk:
            break
        with open(work, "ab") as target:
            target.write(chunk)
        offset = after
        elapsed, _result = timed_ingest(run_dir, work, budget_seconds=None)
        samples.append(elapsed)
        sizes.append(len(chunk))
    if samples:
        print("incremental boundaries: n=%d, %d lines (%d-%d bytes) each: median %s, max %s" % (
            len(samples), lines_per_boundary, min(sizes), max(sizes),
            ms(statistics.median(samples)), ms(max(samples))))


def budgets(root, path):
    for label, budget in (("default budget", otel.DEFAULT_BUDGET_SECONDS), ("50 ms budget", 0.05)):
        run_dir = fresh_run_dir(root, "budget-" + label.split()[0])
        elapsed, result = timed_ingest(run_dir, path, whole_file=True, budget_seconds=budget)
        print("whole file, %s: %s  (%s)" % (label, ms(elapsed), describe(result)))


def startup():
    code = "import mosaic_logger_otel"
    samples = []
    for _ in range(5):
        started = time.perf_counter()
        subprocess.run([sys.executable, "-c", code], cwd=SHARED, check=True)
        samples.append(time.perf_counter() - started)
    print("interpreter start + import of the ingest module: median %s" % ms(statistics.median(samples)))


def main():
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("otel_file")
    parser.add_argument("--lines-per-boundary", type=int, default=20)
    parser.add_argument("--boundaries", type=int, default=10)
    args = parser.parse_args()
    path = os.path.abspath(args.otel_file)
    size = os.path.getsize(path)
    print("python %s on %s, file %s (%d bytes)" % (sys.version.split()[0], sys.platform, path, size))
    root = tempfile.mkdtemp(prefix="measure-ingest-")
    try:
        first_ingest_mid_file(root, path, size)
        incremental(root, path, size, args.lines_per_boundary, args.boundaries)
        budgets(root, path)
        startup()
    finally:
        shutil.rmtree(root, ignore_errors=True)


if __name__ == "__main__":
    main()
