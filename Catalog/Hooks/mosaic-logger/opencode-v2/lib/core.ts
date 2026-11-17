/**
 * core.ts - constants, timestamps, the event envelope, the on-disk layout and
 * the write primitives of the mosaic-logger OpenCode V2 variant.
 *
 * Records follow Development/Designs/MosaicLogFormat.md. Optional fields are
 * absent, never null, empty or placeholders. Only node: built-ins are used.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

import { lockedAppendSync } from "./locked_append.js";
import { schemaVersionFor } from "./schema_versions.js";

export const HARNESS = "opencode-v2";
export const LOGS_DIRNAME = "OrchestrationLogs";
export const UNKNOWN_RUN = "unknown-run";

/** Characters reserved by Windows and cross-platform filesystem conventions. */
const RESERVED_CHARS = new Set([...'<>:"/\\|?*']);

/** Current UTC time as ISO 8601 with milliseconds and a Z suffix. */
export function currentTimestamp(): string {
  return new Date().toISOString();
}

/**
 * Recursive prune: [value, keep]. Dropped: null, undefined, "", empty object or
 * array, and any object whose entries are all dropped. Kept: false, 0, any
 * number, non-empty strings and arrays (arrays are kept as they are).
 */
export function prune(value: unknown): [unknown, boolean] {
  if (value === null || value === undefined) return [undefined, false];
  if (typeof value === "string") return value === "" ? [undefined, false] : [value, true];
  if (Array.isArray(value)) return value.length === 0 ? [undefined, false] : [value, true];
  if (typeof value === "object") {
    const pruned: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      const [kept, keep] = prune(item);
      if (keep) pruned[key] = kept;
    }
    return Object.keys(pruned).length === 0 ? [undefined, false] : [pruned, true];
  }
  return [value, true];
}

export interface Envelope {
  sessionId?: string;
  runId?: string;
  timestamp: string;
}

/** A complete event: the common envelope followed by the pruned caller fields. */
export function buildEvent(
  event: string,
  envelope: Envelope,
  fields: Record<string, unknown>,
): Record<string, unknown> {
  const result: Record<string, unknown> = {
    schema_version: schemaVersionFor(event),
    event,
    timestamp: envelope.timestamp,
    harness: HARNESS,
  };
  if (envelope.sessionId) result["session_id"] = envelope.sessionId;
  if (envelope.runId) result["run_id"] = envelope.runId;
  for (const [key, value] of Object.entries(fields)) {
    const [kept, keep] = prune(value);
    if (keep) result[key] = kept;
  }
  return result;
}

/** One filesystem path component: reserved and control characters become _, trailing dots and spaces go. */
export function sanitizeComponent(name: string): string {
  if (!name) return "_";
  let out = "";
  for (const ch of name) {
    out += RESERVED_CHARS.has(ch) || ch.charCodeAt(0) < 0x20 ? "_" : ch;
  }
  return out.replace(/[. ]+$/, "") || "_";
}

/** The on-disk layout, expressed once. No call site concatenates paths. */
export class LogPaths {
  readonly root: string;

  constructor(workspaceRoot: string) {
    this.root = nodePath.join(workspaceRoot, LOGS_DIRNAME);
  }

  runRoot(run: string): string {
    return nodePath.join(this.root, run);
  }

  orchestratorEvents(run: string): string {
    return nodePath.join(this.runRoot(run), "00_orchestrator_events.jsonl");
  }

  /** Folder of one invocation; folder is a name already made unique within the run. */
  invocationDir(run: string, folder: string): string {
    return nodePath.join(this.runRoot(run), sanitizeComponent(folder));
  }

  invocationEvents(run: string, folder: string): string {
    return nodePath.join(this.invocationDir(run, folder), "03_events.jsonl");
  }

  invocationInput(run: string, folder: string): string {
    return nodePath.join(this.invocationDir(run, folder), "01_input.md");
  }

  invocationOutput(run: string, folder: string): string {
    return nodePath.join(this.invocationDir(run, folder), "02_output.md");
  }
}

/** Append one JSON object as one line under the lock. False on any failure. Never throws. */
export function appendEvent(file: string, event: Record<string, unknown>): boolean {
  try {
    return lockedAppendSync(file, JSON.stringify(event) + "\n");
  } catch {
    return false;
  }
}

/**
 * Write a text file by temp file and rename, creating its folder.
 * False on any failure. Never throws.
 */
export function writeTextAtomic(file: string, text: string): boolean {
  const tmp = nodePath.join(
    nodePath.dirname(file),
    `.tmp_${Date.now()}_${Math.random().toString(36).slice(2)}`,
  );
  try {
    nodeFs.mkdirSync(nodePath.dirname(file), { recursive: true });
    nodeFs.writeFileSync(tmp, text, "utf-8");
    nodeFs.renameSync(tmp, file);
    return true;
  } catch {
    try {
      nodeFs.unlinkSync(tmp);
    } catch {
      // nothing to clean up
    }
    return false;
  }
}
