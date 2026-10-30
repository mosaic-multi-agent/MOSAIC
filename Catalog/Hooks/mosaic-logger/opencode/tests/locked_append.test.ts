/**
 * locked_append.test.ts - the OpenCode locked append (async and sync).
 *
 * Covers:
 *   - lock file name and the shared wait/stale constants
 *   - a plain append, parent folder creation, lock file removal afterwards
 *   - parallel writers (in-process async, separate processes) with lines longer than the
 *     OS write buffer: no line lost, torn or blank (end-to-end support; the held-lock,
 *     stale, timeout and lock-file tests are what discriminate the lock mechanism)
 *   - waiting for a released lock, breaking a stale lock
 *   - lock timeout: event dropped, one debug message, no throw, bounded wait
 *   - I/O errors and a throwing debug function never escape
 *   - the sync path is fully synchronous and does not spin the CPU while waiting
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";

import {
  WAIT_LIMIT_MS,
  lockPath,
  STALE_AFTER_MS,


  lockedAppend,
  lockedAppendSync,
} from "../lib/locked_append";
import {
  PAD,
  makeTempDir,
  longLine,
  readLog,
  seqsOf,
  range,
  holdLock,
  lockFileOf,
  lockFileOf,
  releaseAfter,
  runNodeWriter,
} from "./locked_append_support";

let dir: string;
let target: string;

beforeEach(() => {
  dir = makeTempDir();
  target = nodePath.join(dir, "run", "00_orchestrator_events.jsonl");
});

afterEach(() => {
  nodeFs.rmSync(dir, { recursive: true, force: true });
});

describe("lock file naming and constants", () => {
  it("names the lock file .<name>.lock in the target's folder", () => {
    expect(lockPath(target)).toBe(nodePath.join(dir, "run", ".00_orchestrator_events.jsonl.lock"));
  });

  it("uses the shared 2 s wait limit and 10 s stale age", () => {
    expect(WAIT_LIMIT_MS).toBe(2000);
    expect(STALE_AFTER_MS).toBe(10000);
  });
});

describe("single append", () => {
  it("async: writes the line, creates the folder and removes the lock file", async () => {
    const written = await lockedAppend(target, longLine("a", 0, 10));

    expect(written).toBe(true);
    expect(seqsOf(readLog(target), "a")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("sync: writes the line, creates the folder and removes the lock file", () => {
    const written = lockedAppendSync(target, longLine("a", 0, 10));

    expect(written).toBe(true);
    expect(seqsOf(readLog(target), "a")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("keeps a line longer than the OS write buffer whole", async () => {
    await lockedAppend(target, longLine("big", 0, PAD * 4));

    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect((reading.lines[0].pad as string).length).toBe(PAD * 4);
  });

  it("appends to an existing file and keeps the earlier lines", async () => {
    lockedAppendSync(target, longLine("first", 0, 10));
    await lockedAppend(target, longLine("second", 0, 10));

    const reading = readLog(target);
    expect(reading.lines.map((l) => l.writer)).toEqual(["first", "second"]);
  });
});

describe("parallel writers", () => {
  it("async: concurrent appends of long lines lose, tear or blank no line", async () => {
    const writers = 8;
    const perWriter = 3;

    await Promise.all(
      range(writers).map(async (w) => {
        for (const seq of range(perWriter)) {
          await lockedAppend(target, longLine(`w${w}`, seq));
        }
      }),
    );

    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect(reading.blank).toBe(0);
    for (const w of range(writers)) {
      expect(seqsOf(reading, `w${w}`)).toEqual(range(perWriter));
    }
  }, 60_000);

  it("separate processes (async and sync) plus in-process appends lose, tear or blank no line", async () => {
    const perWriter = 6;
    const children = [
      runNodeWriter("async", target, "childA", perWriter),
      runNodeWriter("sync", target, "childS", perWriter),
      runNodeWriter("async", target, "childB", perWriter),
    ];
    const local = Promise.all(
      range(perWriter).map((seq) => lockedAppend(target, longLine("local", seq))),
    );

    const results = await Promise.all([...children, local]);

    for (const result of results.slice(0, 3) as Array<{ code: number | null; stderr: string }>) {
      expect(result.stderr).toBe("");
      expect(result.code).toBe(0);
    }
    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect(reading.blank).toBe(0);
    for (const tag of ["childA", "childS", "childB"]) {
      expect(seqsOf(reading, tag)).toEqual(range(perWriter));
    }
    expect(seqsOf(reading, "local").sort((a, b) => a - b)).toEqual(range(perWriter));
  }, 120_000);
});

describe("waiting for a held lock", () => {
  it("async: waits for a lock released within the limit, then writes", async () => {
    const lock = lockFileOf(target);
    holdLock(lock);
    releaseAfter(lock, 500);
    const started = performance.now();

    const written = await lockedAppend(target, longLine("late", 0, 10));

    expect(written).toBe(true);
    expect(performance.now() - started).toBeGreaterThanOrEqual(300);
    expect(seqsOf(readLog(target), "late")).toEqual([0]);
  });

  it("sync: waits for a lock released within the limit, then writes", () => {
    const lock = lockFileOf(target);
    holdLock(lock);
    releaseAfter(lock, 500);
    const started = performance.now();

    const written = lockedAppendSync(target, longLine("late", 0, 10));

    expect(written).toBe(true);
    expect(performance.now() - started).toBeGreaterThanOrEqual(300);
    expect(seqsOf(readLog(target), "late")).toEqual([0]);
  });

  it("async: breaks a lock file older than the stale age and writes promptly", async () => {
    holdLock(lockFileOf(target), STALE_AFTER_MS + 2000);
    const started = performance.now();

    const written = await lockedAppend(target, longLine("stale", 0, 10));

    expect(written).toBe(true);
    expect(performance.now() - started).toBeLessThan(WAIT_LIMIT_MS - 500);
    expect(seqsOf(readLog(target), "stale")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("sync: breaks a lock file older than the stale age and writes promptly", () => {
    holdLock(lockFileOf(target), STALE_AFTER_MS + 2000);
    const started = performance.now();

    const written = lockedAppendSync(target, longLine("stale", 0, 10));

    expect(written).toBe(true);
    expect(performance.now() - started).toBeLessThan(WAIT_LIMIT_MS - 500);
    expect(seqsOf(readLog(target), "stale")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });
});

describe("lock timeout", () => {
  it("async: drops the event, calls debug once naming the target, never rejects, stays bounded", async () => {
    holdLock(lockFileOf(target));
    const messages: string[] = [];
    const started = performance.now();

    const written = await lockedAppend(target, longLine("lost", 0, 10), (m) => messages.push(m));
    const elapsed = performance.now() - started;

    expect(written).toBe(false);
    expect(nodeFs.existsSync(target)).toBe(false);
    expect(messages).toHaveLength(1);
    expect(messages[0]).toContain(nodePath.basename(target));
    expect(elapsed).toBeGreaterThanOrEqual(WAIT_LIMIT_MS - 200);
    expect(elapsed).toBeLessThan(WAIT_LIMIT_MS + 3000);
  }, 20_000);

  it("sync: drops the event, calls debug once naming the target, never throws, stays bounded", () => {
    holdLock(lockFileOf(target));
    const messages: string[] = [];
    const started = performance.now();

    const written = lockedAppendSync(target, longLine("lost", 0, 10), (m) => messages.push(m));
    const elapsed = performance.now() - started;

    expect(written).toBe(false);
    expect(nodeFs.existsSync(target)).toBe(false);
    expect(messages).toHaveLength(1);
    expect(messages[0]).toContain(nodePath.basename(target));
    expect(elapsed).toBeGreaterThanOrEqual(WAIT_LIMIT_MS - 200);
    expect(elapsed).toBeLessThan(WAIT_LIMIT_MS + 3000);
  }, 20_000);

  it("leaves a lock file held by another writer untouched on timeout", () => {
    const lock = lockFileOf(target);
    holdLock(lock);

    lockedAppendSync(target, longLine("lost", 0, 10));

    expect(nodeFs.existsSync(lock)).toBe(true);
  }, 20_000);

  it("sync: does not spin the CPU while waiting for the lock", () => {
    holdLock(lockFileOf(target));
    const before = process.cpuUsage();

    lockedAppendSync(target, longLine("lost", 0, 10));

    const used = process.cpuUsage(before);
    expect((used.user + used.system) / 1000).toBeLessThan(500);
  }, 20_000);
});

describe("errors never escape", () => {
  function blockParent(): string {
    const blocker = nodePath.join(dir, "blocker");
    nodeFs.writeFileSync(blocker, "not a folder");
    return nodePath.join(blocker, "sub", "events.jsonl");
  }

  it("async: an unwritable location returns false after one debug call", async () => {
    const messages: string[] = [];

    const written = await lockedAppend(blockParent(), longLine("x", 0, 10), (m) => messages.push(m));

    expect(written).toBe(false);
    expect(messages).toHaveLength(1);
  });

  it("sync: an unwritable location returns false after one debug call", () => {
    const messages: string[] = [];

    const written = lockedAppendSync(blockParent(), longLine("x", 0, 10), (m) => messages.push(m));

    expect(written).toBe(false);
    expect(messages).toHaveLength(1);
  });

  it("async: a throwing debug function does not reject", async () => {
    const throwing = () => {
      throw new Error("debug failed");
    };

    await expect(lockedAppend(blockParent(), longLine("x", 0, 10), throwing)).resolves.toBe(false);
  });

  it("sync: a throwing debug function does not throw", () => {
    const throwing = () => {
      throw new Error("debug failed");
    };

    expect(lockedAppendSync(blockParent(), longLine("x", 0, 10), throwing)).toBe(false);
  });
});

describe("sync path is fully synchronous", () => {
  it("returns a plain boolean with the line already on disk, before any queued callback runs", () => {
    let callbackRan = false;
    setTimeout(() => (callbackRan = true), 0);
    queueMicrotask(() => (callbackRan = true));

    const result = lockedAppendSync(target, longLine("now", 0, 10));

    expect(result).toBe(true);
    expect(callbackRan).toBe(false);
    expect(seqsOf(readLog(target), "now")).toEqual([0]);
  });
});
