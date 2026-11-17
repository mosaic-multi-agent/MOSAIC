/**
 * ownership.ts - which sessions belong to this instance's project.
 *
 * Events from every project reach every instance on the shared service, and
 * some carry no location. A session S becomes owned, for the rest of the
 * instance's life, at the first of:
 *   1. a hook callback of this instance carrying S (hooks fire only in the
 *      owning project's instance);
 *   2. an event located in this directory naming S;
 *   3. a session.created whose data.location is this directory;
 *   4. S named as a child by an owned session.
 * A location-less event of an unowned session is dropped, never buffered. No
 * rule depends on seeing the session's own session.created.
 */

import * as nodePath from "node:path";

import { strAt } from "./safe.js";
import { ensureSession, type InstanceState, type Session } from "./state.js";

const CASE_INSENSITIVE_PATHS = process.platform === "win32";

function normalize(dir: string): string {
  const resolved = nodePath.resolve(dir);
  const root = nodePath.parse(resolved).root;
  const trimmed = resolved.length > root.length ? resolved.replace(/[\\/]+$/, "") : resolved;
  return CASE_INSENSITIVE_PATHS ? trimmed.toLowerCase() : trimmed;
}

/** True when both are non-empty and name the same directory. */
export function sameDirectory(a: unknown, b: unknown): boolean {
  if (typeof a !== "string" || a === "" || typeof b !== "string" || b === "") return false;
  try {
    return normalize(a) === normalize(b);
  } catch {
    return false;
  }
}

/** Directory of an event's envelope location, if it has one. */
export function eventDirectory(event: unknown): string | undefined {
  return strAt(event, "location", "directory");
}

/**
 * The session as owned by this instance, creating its record on first sight
 * (rules 1 to 3). Undefined when the session is not owned.
 */
export function ownedSession(
  state: InstanceState,
  sessionId: string | undefined,
  evidence: { hook?: boolean; locatedHere?: boolean },
): Session | undefined {
  if (sessionId === undefined) return undefined;
  const known = state.sessions.get(sessionId);
  if (known) return known;
  if (evidence.hook || evidence.locatedHere) return ensureSession(state, sessionId);
  return undefined;
}

/**
 * The owned session an event names, after applying the ownership rules to the
 * event itself. An event located in another directory is dropped.
 */
export function sessionOfEvent(state: InstanceState, event: unknown, data: unknown): Session | undefined {
  const dir = eventDirectory(event);
  if (dir !== undefined && !sameDirectory(dir, state.dir)) return undefined;
  return ownedSession(state, strAt(data, "sessionID"), { locatedHere: dir !== undefined });
}
