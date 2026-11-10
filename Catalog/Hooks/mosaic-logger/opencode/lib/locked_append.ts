/**
 * locked_append.ts - append one whole line to a log file under the cross-language lock.
 *
 * The lock is a sidecar file, <dir>/.<name>.lock, created with the exclusive-create flag
 * ("wx") before the append and deleted after it. A writer that finds the lock file waits and
 * retries until WAIT_LIMIT_MS have passed; a lock file older than STALE_AFTER_MS is the
 * leftover of a dead writer and is broken. A writer that cannot get the lock in time drops the
 * line and reports it. Same mechanism as the Python and Go producers (log format section 4.1).
 * No byte-range or advisory lock is used, so readers of the log file are never affected.
 *
 * Imports nothing from core.ts: the debug sink is passed in.
 */

import * as nodeFs from "node:fs";
import * as nodeFsPromises from "node:fs/promises";
import * as nodePath from "node:path";

export const WAIT_LIMIT_MS = 2000;
export const STALE_AFTER_MS = 10000;
const POLL_MS = 20;

/** Same shape as core.ts DebugLogFn. */
export type LockDebugFn = (message: string, error?: unknown) => void;

/** Lock file path of target: <dir>/.<name>.lock. */
export function lockPath(target: string): string {
  return nodePath.join(nodePath.dirname(target), "." + nodePath.basename(target) + ".lock");
}

function report(debug: LockDebugFn | undefined, message: string, error?: unknown): void {
  if (debug === undefined) return;
  try {
    debug(message, error);
  } catch {
    // a failing debug sink must never reach the caller
  }
}

function timeoutMessage(target: string): string {
  return "lock wait limit reached, not written: " + nodePath.basename(target);
}

function isStaleSync(lock: string): boolean {
  try {
    return Date.now() - nodeFs.statSync(lock).mtimeMs > STALE_AFTER_MS;
  } catch {
    return false;
  }
}

async function isStale(lock: string): Promise<boolean> {
  try {
    return Date.now() - (await nodeFsPromises.stat(lock)).mtimeMs > STALE_AFTER_MS;
  } catch {
    return false;
  }
}

function errorCode(err: unknown): string | undefined {
  return (err as NodeJS.ErrnoException | null)?.code;
}

/** Blocking sleep without spinning the CPU. */
function sleepSync(ms: number): void {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function createLockSync(lock: string): boolean {
  try {
    nodeFs.closeSync(nodeFs.openSync(lock, "wx"));
    return true;
  } catch (err) {
    // EEXIST: held. EPERM/EACCES: Windows reports a lock file being deleted this way.
    // ENOENT/ENOTDIR mean the folder is unusable and are not retryable.
    const code = errorCode(err);
    if (code === "ENOENT" || code === "ENOTDIR") throw err;
    return false;
  }
}

async function createLock(lock: string): Promise<boolean> {
  try {
    await (await nodeFsPromises.open(lock, "wx")).close();
    return true;
  } catch (err) {
    const code = errorCode(err);
    if (code === "ENOENT" || code === "ENOTDIR") throw err;
    return false;
  }
}

function acquireSync(lock: string): boolean {
  const deadline = performance.now() + WAIT_LIMIT_MS;
  for (;;) {
    if (createLockSync(lock)) return true;
    if (isStaleSync(lock)) {
      try {
        nodeFs.unlinkSync(lock);
      } catch {
        // someone else broke it first
      }
      continue;
    }
    if (performance.now() >= deadline) return false;
    sleepSync(POLL_MS);
  }
}

async function acquire(lock: string): Promise<boolean> {
  const deadline = performance.now() + WAIT_LIMIT_MS;
  for (;;) {
    if (await createLock(lock)) return true;
    if (await isStale(lock)) {
      await nodeFsPromises.unlink(lock).catch(() => undefined);
      continue;
    }
    if (performance.now() >= deadline) return false;
    await sleep(POLL_MS);
  }
}

/** Synchronous locked append. Returns false on timeout or I/O error. Never throws. */
export function lockedAppendSync(target: string, data: string, debug?: LockDebugFn): boolean {
  try {
    nodeFs.mkdirSync(nodePath.dirname(target), { recursive: true });
    const lock = lockPath(target);
    if (!acquireSync(lock)) {
      report(debug, timeoutMessage(target));
      return false;
    }
    try {
      nodeFs.appendFileSync(target, data, "utf-8");
      return true;
    } finally {
      try {
        nodeFs.unlinkSync(lock);
      } catch {
        // already gone
      }
    }
  } catch (err) {
    report(debug, "append failed: " + nodePath.basename(target), err);
    return false;
  }
}

/** Asynchronous locked append. Never rejects. */
export async function lockedAppend(
  target: string,
  data: string,
  debug?: LockDebugFn,
): Promise<boolean> {
  try {
    await nodeFsPromises.mkdir(nodePath.dirname(target), { recursive: true });
    const lock = lockPath(target);
    if (!(await acquire(lock))) {
      report(debug, timeoutMessage(target));
      return false;
    }
    try {
      await nodeFsPromises.appendFile(target, data, "utf-8");
      return true;
    } finally {
      await nodeFsPromises.unlink(lock).catch(() => undefined);
    }
  } catch (err) {
    report(debug, "append failed: " + nodePath.basename(target), err);
    return false;
  }
}
