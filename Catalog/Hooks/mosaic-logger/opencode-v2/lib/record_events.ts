/**
 * record_events.ts - the subscription events that write turn, usage, tool failure,
 * compaction and permission records.
 *
 * Every handler gets the owned session the event names; an event of another
 * directory, or a location-less event of an unowned session, never reaches one.
 * Not subscribed on purpose: session.usage.updated (cumulative per session, it
 * would double count the step reports) and reasoning events (never logged).
 */

import { sessionOfEvent } from "./ownership.js";
import { onPermissionAsked, onPermissionReplied } from "./permission_records.js";
import { get, str } from "./safe.js";
import {
  onCompactionEnded,
  onStepFinished,
  onStepStarted,
  onTextEnded,
  onToolRequested,
} from "./step_records.js";
import type { InstanceState, Session } from "./state.js";
import { recordToolFailure } from "./tool_records.js";

type Handler = (state: InstanceState, session: Session, data: unknown, eventId: string | undefined) => void;

const HANDLERS: Readonly<Record<string, Handler>> = {
  "session.step.started": (state, session, data) => onStepStarted(state, session, data),
  "session.step.ended": (state, session, data, id) => onStepFinished(state, session, data, id, false),
  "session.step.failed": (state, session, data, id) => onStepFinished(state, session, data, id, true),
  "session.text.ended": (state, session, data) => onTextEnded(state, session, data),
  "session.tool.input.started": (state, session, data) => onToolRequested(state, session, data),
  "session.tool.failed": (state, session, data) => recordToolFailure(state, session, data),
  "session.compaction.ended": (state, session, data, id) => onCompactionEnded(state, session, data, id),
  "permission.asked": (state, session, data) => onPermissionAsked(state, session, data),
  "permission.replied": (state, session, data) => onPermissionReplied(state, session, data),
};

/** Handle the event when it is one of the record events. True when it was. */
export function processRecordEvent(state: InstanceState, type: string, event: unknown, data: unknown): boolean {
  if (!Object.prototype.hasOwnProperty.call(HANDLERS, type)) return false;
  const handler = HANDLERS[type];
  const session = sessionOfEvent(state, event, data);
  if (session) handler(state, session, data, str(get(event, "id")));
  return true;
}
