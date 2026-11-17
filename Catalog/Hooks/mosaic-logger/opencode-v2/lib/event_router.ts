/**
 * event_router.ts - the V2 events the logger reads.
 *
 * Handled: session.created, session.deleted, session.inbox.enqueued (user
 * items and synthetic subagent wake-ups), session.synthetic, session.execution.*
 * and session.tool.progress; the step, text, tool failure, compaction and
 * permission events are handled by record_events.ts. The V1 events V2 removed (message.*,
 * session.updated, session.idle, session.status) and every other type are
 * ignored: they never open a session or write a record.
 *
 * Known gaps (V1 features without a V2 signal): V1 recovered a resumed root's
 * run from its message history; V2 has no verified history accessor, so a resumed
 * root stays unbound until its next dispatch. A child whose session.created and
 * every parent-naming signal are all missed is logged as a root.
 */

import { eventDirectory, ownedSession, sameDirectory, sessionOfEvent } from "./ownership.js";
import { endInvocation, joinByCall } from "./invocations.js";
import { processRecordEvent } from "./record_events.js";
import { endIfQuiescent, touch } from "./root_lifecycle.js";
import { get, rec, str, strAt } from "./safe.js";
import { markChild, type InstanceState, type Session } from "./state.js";
import { recordUserTurn } from "./step_records.js";
import { requestChildTranscript, requestRootTranscript } from "./transcript_queue.js";

const TERMINAL_EXECUTION: ReadonlySet<string> = new Set([
  "session.execution.succeeded",
  "session.execution.failed",
  "session.execution.interrupted",
]);

function onCreated(state: InstanceState, event: unknown, data: unknown): void {
  const sessionId = strAt(data, "sessionID");
  if (sessionId === undefined) return;
  const parentId = strAt(data, "parentID");
  const agent = strAt(data, "agent");
  const parentOwned = parentId !== undefined && state.sessions.has(parentId);

  if (parentOwned && parentId !== undefined) {
    markChild(state, sessionId, parentId, agent);
    return;
  }
  const here =
    sameDirectory(eventDirectory(event), state.dir) || sameDirectory(strAt(data, "location", "directory"), state.dir);
  const session = ownedSession(state, sessionId, { locatedHere: here });
  if (!session) return;
  if (session.agent === undefined) session.agent = agent;
  touch(state, session);
}

/** The synthetic wake-up a child's state change delivers to its parent. */
function onWakeUp(state: InstanceState, parent: Session, item: unknown): void {
  const payload = get(item, "payload") ?? item;
  const metadata = get(payload, "metadata");
  if (get(metadata, "source") !== "subagent") return;
  const child = str(get(metadata, "childID"));
  if (child === undefined) return;

  markChild(state, child, parent.id, str(get(metadata, "agent")));
  touch(state, parent);
  const childState = get(metadata, "state");
  if (typeof childState !== "string" || childState === "running") return;
  const text = get(payload, "text");
  endInvocation(state, child, typeof text === "string" ? text : undefined);
  requestChildTranscript(state, child);
}

function onInbox(state: InstanceState, event: unknown, data: unknown): void {
  const session = sessionOfEvent(state, event, data);
  if (!session) return;
  const item = get(data, "item");
  if (rec(item) === undefined) return;
  if (get(item, "type") === "synthetic") {
    onWakeUp(state, session, item);
    return;
  }
  touch(state, session);
  if (get(item, "type") === "user") recordUserTurn(state, session, strAt(data, "inboxID"), get(get(item, "payload"), "text"));
}

function onExecution(state: InstanceState, type: string, event: unknown, data: unknown): void {
  const session = sessionOfEvent(state, event, data);
  if (!session) return;
  if (TERMINAL_EXECUTION.has(type)) {
    endIfQuiescent(state, session, type);
    if (session.parent === undefined) requestRootTranscript(state, session.id);
  } else touch(state, session);
}

function onProgress(state: InstanceState, event: unknown, data: unknown): void {
  const parent = sessionOfEvent(state, event, data);
  if (!parent) return;
  touch(state, parent);
  const child = strAt(data, "metadata", "sessionID");
  if (child === undefined) return;
  markChild(state, child, parent.id);
  const callId = strAt(data, "id");
  if (callId !== undefined) joinByCall(state, callId, parent.id, child);
}

function onDeleted(state: InstanceState, event: unknown, data: unknown): void {
  const session = sessionOfEvent(state, event, data);
  if (session) endIfQuiescent(state, session, "session.deleted");
}

/** Process one subscription event. Never throws; an unusable event is skipped. */
export function processEvent(state: InstanceState, event: unknown): void {
  try {
    const type = str(get(event, "type"));
    const data = get(event, "data");
    if (type === undefined || rec(data) === undefined) return;
    if (processRecordEvent(state, type, event, data)) return;
    if (type === "session.created") onCreated(state, event, data);
    else if (type === "session.inbox.enqueued") onInbox(state, event, data);
    else if (type === "session.synthetic") {
      const parent = sessionOfEvent(state, event, data);
      if (parent) onWakeUp(state, parent, data);
    } else if (type === "session.tool.progress") onProgress(state, event, data);
    else if (type === "session.deleted") onDeleted(state, event, data);
    else if (type.startsWith("session.execution.")) onExecution(state, type, event, data);
  } catch {
    // one bad event skips only itself
  }
}
