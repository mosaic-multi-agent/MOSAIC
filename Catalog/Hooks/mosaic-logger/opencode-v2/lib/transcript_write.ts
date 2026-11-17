/**
 * transcript_write.ts - where a transcript goes, whether an export is usable, and
 * how it is written: the .raw bytes verbatim by atomic replace, then the sidecar
 * (the commit marker) by atomic replace.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

import { currentTimestamp, HARNESS, sanitizeComponent, UNKNOWN_RUN, writeTextAtomic } from "./core.js";
import { get, rec } from "./safe.js";
import { runOfRoot, type InstanceState } from "./state.js";
import { TRANSCRIPT_NATIVE_FORMAT, TRANSCRIPT_SOURCE_MECHANISM } from "./transcript.js";

/** A child's transcript or a root's (the orchestrator transcript). */
export type TranscriptKind = "child" | "root";

export interface TranscriptTarget {
  raw: string;
  meta: string;
  /** The exported session, recorded and guarded on orchestrator transcripts only. */
  sessionId?: string;
}

/** Where the transcript of the session goes now, or undefined when it has no place. */
export function resolveTarget(state: InstanceState, id: string, kind: TranscriptKind): TranscriptTarget | undefined {
  if (kind === "child") {
    const invocation = state.invocations.get(id);
    if (!invocation) return undefined;
    const dir = state.paths.invocationDir(invocation.run, invocation.folder);
    return { raw: nodePath.join(dir, "04_session.raw"), meta: nodePath.join(dir, "04_session.meta.json") };
  }
  const session = state.sessions.get(id);
  if (!session || session.parent !== undefined) return undefined;
  const run = runOfRoot(state, session);
  const scope = run === UNKNOWN_RUN ? `__${HARNESS}__${sanitizeComponent(id).replace(/\./g, "_")}` : "";
  const stem = nodePath.join(state.paths.runRoot(run), `00_orchestrator_session${scope}`);
  return { raw: `${stem}.raw`, meta: `${stem}.meta.json`, sessionId: id };
}

/** Whether the bytes are a JSON object whose info.id is the session and whose messages is an array. */
export function isExportOf(bytes: Buffer, id: string): boolean {
  try {
    const document: unknown = JSON.parse(bytes.toString("utf-8"));
    return rec(document) !== undefined && get(get(document, "info"), "id") === id && Array.isArray(get(document, "messages"));
  } catch {
    return false;
  }
}

/** True when the sidecar at the path names a session other than the given one. */
function ownedByOtherSession(meta: string, id: string): boolean {
  try {
    const owner = get(JSON.parse(nodeFs.readFileSync(meta, "utf-8")), "session_id");
    return typeof owner === "string" && owner !== "" && owner !== id;
  } catch {
    return false;
  }
}

/** Write bytes by temp file and rename, creating the folder. False on any failure. Never throws. */
function writeBytesAtomic(file: string, bytes: Buffer): boolean {
  const tmp = nodePath.join(nodePath.dirname(file), `.tmp_${Date.now()}_${Math.random().toString(36).slice(2)}`);
  try {
    nodeFs.mkdirSync(nodePath.dirname(file), { recursive: true });
    nodeFs.writeFileSync(tmp, bytes);
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

/** Write the transcript and its sidecar. An orchestrator target owned by another session is kept. */
export function writeTranscript(target: TranscriptTarget, bytes: Buffer): void {
  if (target.sessionId !== undefined && ownedByOtherSession(target.meta, target.sessionId)) return;
  if (!writeBytesAtomic(target.raw, bytes)) return;
  const sidecar: Record<string, unknown> = {
    harness: HARNESS,
    native_format: TRANSCRIPT_NATIVE_FORMAT,
    source_mechanism: TRANSCRIPT_SOURCE_MECHANISM,
    captured_at: currentTimestamp(),
    byte_count: bytes.length,
  };
  if (target.sessionId !== undefined) sidecar["session_id"] = target.sessionId;
  writeTextAtomic(target.meta, JSON.stringify(sidecar));
}
