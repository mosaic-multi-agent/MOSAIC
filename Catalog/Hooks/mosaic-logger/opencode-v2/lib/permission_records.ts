/**
 * permission_records.ts - permission prompts as notification records.
 *
 * permission.evaluate fires for allow and ask and never for a configured deny; only
 * an ask needs a human, so only an ask writes a notification. When the ephemeral
 * permission.asked event links the request to that ask by call id, the later
 * permission.replied event writes one resolution notification with the same id.
 */

import { ownedSession } from "./ownership.js";
import { touch } from "./root_lifecycle.js";
import { get, str, strAt } from "./safe.js";
import { remember, type InstanceState, type Session } from "./state.js";
import { emit, targetFor } from "./targets.js";

const PERMISSION_PROMPT = "permission_prompt";

/** The text shown for an ask: the event's message, else the action and its joined resources. */
function askMessage(event: unknown): string | undefined {
  const given = strAt(event, "message");
  if (given !== undefined) return given;
  const resources = get(event, "resources");
  const joined = Array.isArray(resources)
    ? resources.filter((r): r is string => typeof r === "string" && r !== "").join(", ")
    : "";
  return [strAt(event, "action"), joined].filter((part) => part !== undefined && part !== "").join(" ") || undefined;
}

/** The permission.evaluate callback: owns the session and writes a notification for an ask. */
export function onPermissionEvaluate(state: InstanceState, event: unknown): void {
  try {
    const session = ownedSession(state, strAt(event, "sessionID"), { hook: true });
    if (!session) return;
    touch(state, session);
    if (get(event, "effect") !== "ask") return;
    const target = targetFor(state, session);
    if (!target) return;

    const notificationId = strAt(event, "source", "id");
    emit(target, "notification", {
      notification_type: PERMISSION_PROMPT,
      message: askMessage(event),
      requires_response: true,
      notification_id: notificationId,
    });
    if (notificationId !== undefined) remember(state.asks, notificationId, { target, resolved: false });
  } catch {
    // a hook never fails the host
  }
}

/** permission.asked links its request id to the ask notification of the same call. */
export function onPermissionAsked(state: InstanceState, _session: Session, data: unknown): void {
  const requestId = strAt(data, "id");
  const callId = strAt(data, "source", "id");
  if (requestId === undefined || callId === undefined || !state.asks.has(callId)) return;
  remember(state.askRequests, requestId, callId);
}

/** permission.replied writes the resolution of a linked ask, once. */
export function onPermissionReplied(state: InstanceState, _session: Session, data: unknown): void {
  const requestId = strAt(data, "requestID");
  const notificationId = requestId !== undefined ? state.askRequests.get(requestId) : undefined;
  const ask = notificationId !== undefined ? state.asks.get(notificationId) : undefined;
  if (notificationId === undefined || ask === undefined || ask.resolved) return;
  ask.resolved = true;
  emit(ask.target, "notification", {
    notification_type: PERMISSION_PROMPT,
    notification_id: notificationId,
    resolution: str(get(data, "reply")),
  });
}
