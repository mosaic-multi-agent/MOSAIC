/**
 * locked_append_python.test.ts - the OpenCode writer and the Python locked append respect
 * each other's lock on one file.
 *
 * Skipped when no Python interpreter is available (`py` on Windows, `python3` elsewhere).
 * The Python module under test is the one shipped in copilot-shared.
 *
 * Covers:
 *   - both implementations name the lock file the same way
 *   - an OpenCode writer (async and sync) waits for a lock the Python writer holds
 *   - a Python writer waits for a lock file named by the OpenCode module
 *   - mixed Python, async and sync OpenCode writers keep every long line whole
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import { spawnSync } from "node:child_process";

import { lockPath, lockedAppend, lockedAppendSync } from "../lib/locked_append";
import {
  PY_MODULE_DIR,
  makeTempDir,
  longLine,
  readLog,
  seqsOf,
  range,
  holdLock,
  releaseAfter,
  runNodeWriter,
  runPythonWriter,
  startPythonHolder,
  findPython,
} from "./locked_append_support";

const python = findPython();
const SKIP_REASON = "no Python interpreter (py on Windows, python3 elsewhere)";

describe.skipIf(python === null)(`OpenCode and Python locked append (needs Python; skipped: ${SKIP_REASON})`, () => {
  let dir: string;
  let target: string;

  beforeEach(() => {
    dir = makeTempDir();
    target = nodePath.join(dir, "run", "00_orchestrator_events.jsonl");
  });

  afterEach(() => {
    nodeFs.rmSync(dir, { recursive: true, force: true });
  });

  it("names the lock file the same way as the Python module", () => {
    const code = "import sys; sys.path.insert(0, sys.argv[1]); " +
      "import mosaic_logger_locked_append as m; print(m.lock_path(sys.argv[2]))";
    const out = spawnSync(python as string, ["-c", code, PY_MODULE_DIR, target], { encoding: "utf-8" });

    expect(nodePath.resolve(out.stdout.trim())).toBe(nodePath.resolve(lockPath(target)));
  });

  it("async OpenCode writer waits for a lock the Python writer holds", async () => {
    const holder = await startPythonHolder(python as string, target, 1.0);
    const started = performance.now();

    const written = await lockedAppend(target, longLine("node", 0, 10));
    await holder.done;

    expect(written).toBe(true);
    expect(performance.now() - started).toBeGreaterThanOrEqual(500);
    expect(readLog(target).lines.map((l) => l.writer)).toEqual(["py-holder", "node"]);
  }, 30_000);

  it("sync OpenCode writer waits for a lock the Python writer holds", async () => {
    const holder = await startPythonHolder(python as string, target, 1.0);
    const started = performance.now();

    const written = lockedAppendSync(target, longLine("node", 0, 10));
    await holder.done;

    expect(written).toBe(true);
    expect(performance.now() - started).toBeGreaterThanOrEqual(500);
    expect(readLog(target).lines.map((l) => l.writer)).toEqual(["py-holder", "node"]);
  }, 30_000);

  it("Python writer waits for a lock file named by the OpenCode module", async () => {
    const lock = lockPath(target);
    holdLock(lock);
    releaseAfter(lock, 700);
    const started = performance.now();

    const result = await runPythonWriter(python as string, target, "py", 1, 10);

    expect(result.code).toBe(0);
    expect(performance.now() - started).toBeGreaterThanOrEqual(500);
    expect(seqsOf(readLog(target), "py")).toEqual([0]);
  }, 30_000);

  it("Python, async and sync OpenCode writers together lose, tear or blank no long line", async () => {
    const perWriter = 5;

    const results = await Promise.all([
      runPythonWriter(python as string, target, "py", perWriter),
      runNodeWriter("async", target, "nodeA", perWriter),
      runNodeWriter("sync", target, "nodeS", perWriter),
      Promise.all(range(perWriter).map((seq) => lockedAppend(target, longLine("local", seq)))),
    ]);

    for (const result of results.slice(0, 3) as Array<{ code: number | null; stderr: string }>) {
      expect(result.code).toBe(0);
    }
    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect(reading.blank).toBe(0);
    for (const tag of ["py", "nodeA", "nodeS"]) {
      expect(seqsOf(reading, tag)).toEqual(range(perWriter));
    }
    expect(seqsOf(reading, "local").sort((a, b) => a - b)).toEqual(range(perWriter));
  }, 120_000);
});
