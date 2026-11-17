/**
 * hooks.ts - the tool-hook callbacks.
 *
 * Observe-only: no callback assigns a field of a hook event, returns a value or
 * throws. Any payload is accepted, including null, wrong-typed fields and
 * members that throw when read; such a payload is skipped.
 *
 * Known gap: V1 recorded a dispatch's closing tool call from its own signals;
 * V2 fires the subagent execute.after at launch, so the call ends there.
 */

import { ownedSession } from "./ownership.js";
import { captureDispatch, discardDispatch, joinByCall } from "./invocations.js";
import { get, str, strAt } from "./safe.js";
import { markChild, type InstanceState, type Session } from "./state.js";
import { bindRun, touch } from "./root_lifecycle.js";
import { recordToolEnd, recordToolStart } from "./tool_records.js";

const SUBAGENT_TOOL = "subagent";

/** The child session a launch result names: result.metadata.sessionID, else result.output.sessionID. */
function launchedChild(result: unknown): string | undefined {
  return strAt(result, "metadata", "sessionID") ?? strAt(result, "output", "sessionID");
}

/** A dispatch starts a new invocation; a call naming a session continues that child. */
function isReCall(input: unknown): boolean {
  return str(get(input, "sessionID")) !== undefined;
}

function guarded(work: () => void): void {
  try {
    work();
  } catch {
    // a hook never fails the host
  }
}

/** A dispatch is captured, and binds the root to the run its prompt names (native mode). */
function handleDispatch(state: InstanceState, session: Session, event: unknown): void {
  const input = get(event, "input");
  if (isReCall(input)) return;
  captureDispatch(state, session.id, str(get(event, "id")), input);
  bindRun(state, session, strAt(input, "prompt"));
}

/** The launch result of a dispatch joins it to its child. */
function handleLaunch(state: InstanceState, session: Session, event: unknown): void {
  const callId = str(get(event, "id"));
  const child = get(event, "status") === "completed" ? launchedChild(get(event, "result")) : undefined;
  if (child !== undefined) {
    markChild(state, child, session.id, strAt(event, "input", "agent"));
    if (callId !== undefined && !isReCall(get(event, "input"))) joinByCall(state, callId, session.id, child);
  } else if (callId !== undefined) {
    discardDispatch(state, callId);
  }
}

/** execute.before: owns the session, captures a dispatch, binds the root's run, writes the call's start. */
export function onToolBefore(state: InstanceState, event: unknown): void {
  guarded(() => {
    const session = ownedSession(state, strAt(event, "sessionID"), { hook: true });
    if (!session) return;
    touch(state, session);
    if (str(get(event, "tool")) === SUBAGENT_TOOL) handleDispatch(state, session, event);
    recordToolStart(state, session, event);
  });
}

/** The session compaction callback: an ownership and activity signal only; it writes nothing. */
export function onCompactionHook(state: InstanceState, event: unknown): void {
  guarded(() => {
    const session = ownedSession(state, strAt(event, "sessionID"), { hook: true });
    if (session) touch(state, session);
  });
}

/** execute.after: a dispatch's launch result joins it to its child; the call's end is written. */
export function onToolAfter(state: InstanceState, event: unknown): void {
  guarded(() => {
    const session = ownedSession(state, strAt(event, "sessionID"), { hook: true });
    if (!session) return;
    touch(state, session);
    if (str(get(event, "tool")) === SUBAGENT_TOOL) handleLaunch(state, session, event);
    recordToolEnd(state, session, event);
  });
}
