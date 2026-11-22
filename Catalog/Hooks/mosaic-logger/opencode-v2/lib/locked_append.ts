/**
 * locked_append.ts - append one whole line to a log file under the cross-language lock.
 *
 * The lock is a sidecar file, <dir>/.<name>.lock, created with the exclusive-create flag
 * ("wx") before the append and deleted after it. A writer that finds the lock file waits and
 * retries until WAIT_LIMIT_MS have passed; a lock file older than STALE_AFTER_MS is the
 * leftover of a dead writer and is broken. A writer that cannot get the lock in time drops the
 * line. Same mechanism as the Python and Go producers (log format section 4.1). No byte-range
 * or advisory lock is used, so readers of the log file are never affected.
 *
 * The append is synchronous: a record is on disk before the caller yields, which keeps
 * records in delivery order and leaves nothing pending when the host process exits.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

export const WAIT_LIMIT_MS = 2000;
export const STALE_AFTER_MS = 10000;
const POLL_MS = 20;

/** Lock file path of target: <dir>/.<name>.lock. */
export function lockPath(target: string): string {
  return nodePath.join(nodePath.dirname(target), "." + nodePath.basename(target) + ".lock");
}

function isStale(lock: string): boolean {
  try {
    return Date.now() - nodeFs.statSync(lock).mtimeMs > STALE_AFTER_MS;
  } catch {
    return false;
  }
}

/** Blocking sleep without spinning the CPU. */
function sleepSync(ms: number): void {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function createLock(lock: string): boolean {
  try {
    nodeFs.closeSync(nodeFs.openSync(lock, "wx"));
    return true;
  } catch (err) {
    // EEXIST: held. EPERM/EACCES: Windows reports a lock file being deleted this way.
    // ENOENT/ENOTDIR mean the folder is unusable and are not retryable.
    const code = (err as NodeJS.ErrnoException | null)?.code;
    if (code === "ENOENT" || code === "ENOTDIR") throw err;
    return false;
  }
}

function acquire(lock: string): boolean {
  const deadline = performance.now() + WAIT_LIMIT_MS;
  for (;;) {
    if (createLock(lock)) return true;
    if (isStale(lock)) {
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

/** Synchronous locked append, creating parent folders. False on timeout or I/O error. Never throws. */
export function lockedAppendSync(target: string, data: string): boolean {
  try {
    nodeFs.mkdirSync(nodePath.dirname(target), { recursive: true });
    const lock = lockPath(target);
    if (!acquire(lock)) return false;
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
  } catch {
    return false;
  }
}
