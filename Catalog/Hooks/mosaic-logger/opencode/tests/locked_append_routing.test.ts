/**
 * locked_append_routing.test.ts - appendEvent and appendEventSync use the locked append.
 *
 * A fresh lock file held by another writer is the discriminator: a path that appends
 * without the lock writes at once, a locked path waits, drops on timeout and logs a
 * debug message.
 *
 * Covers:
 *   - appendEvent / appendEventSync wait for a released lock before writing
 *   - on timeout both drop the event, log a debug message and do not throw
 *   - a stale lock is broken
 *   - parallel appends through each path keep every line whole
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";

import { appendEvent, setDebugLogger } from "../lib/core";
import { appendEventSync } from "../lib/sync_append";
import { STALE_AFTER_MS } from "../lib/locked_append";
import {
  makeTempDir,
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
let debugMessages: string[];

function event(tag: string, seq: number, pad: number = 10): Record<string, unknown> {
  return { event: "probe", writer: tag, seq, pad: "p".repeat(pad) };
}

beforeEach(() => {
  dir = makeTempDir();
  target = nodePath.join(dir, "00_orchestrator_events.jsonl");
  debugMessages = [];
  setDebugLogger((message) => debugMessages.push(message));
});

afterEach(() => {
  setDebugLogger(() => {});
  nodeFs.rmSync(dir, { recursive: true, force: true });
});

describe("appendEvent", () => {
  it("waits for a held lock to be released, then writes", async () => {
    const lock = lockFileOf(target);
    holdLock(lock);
    releaseAfter(lock, 500);
    const started = performance.now();

    await appendEvent(target, event("late", 0));

    expect(performance.now() - started).toBeGreaterThanOrEqual(300);
    expect(seqsOf(readLog(target), "late")).toEqual([0]);
  });

  it("drops the event on lock timeout, logs a debug message and does not throw", async () => {
    holdLock(lockFileOf(target));

    await expect(appendEvent(target, event("lost", 0))).resolves.toBeUndefined();

    expect(nodeFs.existsSync(target)).toBe(false);
    expect(debugMessages.length).toBeGreaterThanOrEqual(1);
  }, 20_000);

  it("breaks a stale lock, writes and leaves no lock file behind", async () => {
    holdLock(lockFileOf(target), STALE_AFTER_MS + 2000);

    await appendEvent(target, event("stale", 0));

    expect(seqsOf(readLog(target), "stale")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("removes its lock file after a successful write", async () => {
    await appendEvent(target, event("one", 0));

    expect(seqsOf(readLog(target), "one")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("leaves a lock held by another writer untouched when it drops the event", async () => {
    const lock = lockFileOf(target);
    holdLock(lock);

    await appendEvent(target, event("lost", 0));

    expect(nodeFs.existsSync(lock)).toBe(true);
  }, 20_000);

  it("creates a missing parent folder", async () => {
    const nested = nodePath.join(dir, "a", "b", "events.jsonl");

    await appendEvent(nested, event("nested", 0));

    expect(seqsOf(readLog(nested), "nested")).toEqual([0]);
  });

  it("writes nothing while the lock is held, then keeps every long line whole after release", async () => {
    const lock = lockFileOf(target);
    holdLock(lock);
    const perWriter = 2;
    const child = runNodeWriter("async", target, "child", perWriter);

    const writers = Promise.all(
      range(4).map(async (w) => {
        for (const seq of range(perWriter)) {
          await appendEvent(target, event(`w${w}`, seq, 100_000));
        }
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 400));
    const writtenWhileHeld = nodeFs.existsSync(target);
    nodeFs.unlinkSync(lock);
    await writers;
    const result = await child;

    expect(writtenWhileHeld).toBe(false);
    expect(result.code).toBe(0);
    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect(reading.blank).toBe(0);
    for (const w of range(4)) {
      expect(seqsOf(reading, `w${w}`)).toEqual(range(perWriter));
    }
    expect(seqsOf(reading, "child")).toEqual(range(perWriter));
  }, 120_000);
});

describe("appendEventSync", () => {
  it("waits for a held lock to be released, then writes and returns true", () => {
    const lock = lockFileOf(target);
    holdLock(lock);
    releaseAfter(lock, 500);
    const started = performance.now();

    const written = appendEventSync(target, event("late", 0));

    expect(written).toBe(true);
    expect(performance.now() - started).toBeGreaterThanOrEqual(300);
    expect(seqsOf(readLog(target), "late")).toEqual([0]);
  });

  it("drops the event on lock timeout, returns false, logs a debug message and does not throw", () => {
    holdLock(lockFileOf(target));

    const written = appendEventSync(target, event("lost", 0));

    expect(written).toBe(false);
    expect(nodeFs.existsSync(target)).toBe(false);
    expect(debugMessages.length).toBeGreaterThanOrEqual(1);
  }, 20_000);

  it("breaks a stale lock, writes and leaves no lock file behind", () => {
    holdLock(lockFileOf(target), STALE_AFTER_MS + 2000);

    expect(appendEventSync(target, event("stale", 0))).toBe(true);
    expect(seqsOf(readLog(target), "stale")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("removes its lock file after a successful write", () => {
    expect(appendEventSync(target, event("one", 0))).toBe(true);

    expect(seqsOf(readLog(target), "one")).toEqual([0]);
    expect(nodeFs.existsSync(lockFileOf(target))).toBe(false);
  });

  it("leaves a lock held by another writer untouched when it drops the event", () => {
    const lock = lockFileOf(target);
    holdLock(lock);

    appendEventSync(target, event("lost", 0));

    expect(nodeFs.existsSync(lock)).toBe(true);
  }, 20_000);

  it("creates a missing parent folder", () => {
    const nested = nodePath.join(dir, "a", "b", "events.jsonl");

    expect(appendEventSync(nested, event("nested", 0))).toBe(true);
    expect(seqsOf(readLog(nested), "nested")).toEqual([0]);
  });

  it("waits for a held lock, then keeps every long line whole next to parallel processes", async () => {
    const perWriter = 5;
    const lock = lockFileOf(target);
    holdLock(lock);
    releaseAfter(lock, 500);
    const children = [
      runNodeWriter("sync", target, "childS", perWriter),
      runNodeWriter("async", target, "childA", perWriter),
    ];
    const started = performance.now();

    for (const seq of range(perWriter)) {
      expect(appendEventSync(target, event("local", seq, 100_000))).toBe(true);
    }
    const totalMs = performance.now() - started;
    const results = await Promise.all(children);

    expect(totalMs).toBeGreaterThanOrEqual(300);

    for (const result of results) expect(result.code).toBe(0);
    const reading = readLog(target);
    expect(reading.torn).toBe(0);
    expect(reading.blank).toBe(0);
    expect(seqsOf(reading, "local")).toEqual(range(perWriter));
    expect(seqsOf(reading, "childS")).toEqual(range(perWriter));
    expect(seqsOf(reading, "childA")).toEqual(range(perWriter));
  }, 120_000);
});
