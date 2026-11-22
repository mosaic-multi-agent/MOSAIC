/**
 * tool_records.ts - tool_call_start and tool_call_end records.
 *
 * A start is written at every execute.before. Exactly one end is written per call,
 * from execute.after or from a tool failure event, whichever comes first, into the
 * stream that holds the start. An after without a start writes its end to the
 * session's current stream. The file-writing tools (write, edit, patch) are
 * ordinary tool calls here: their input and result are logged verbatim.
 */

import { fallbackInstanceId } from "./identity.js";
import { get, str, strAt } from "./safe.js";
import { remember, type CallRecord, type InstanceState, type Session } from "./state.js";
import { emit, targetFor } from "./targets.js";

/** An input or result worth logging: an object or a non-empty string. */
function payloadValue(value: unknown): unknown {
  if (typeof value === "object" && value !== null) return value;
  return str(value);
}

/** The error message of a tool failure: a string, or the message member of an object. */
function errorMessage(error: unknown): string | undefined {
  return str(error) ?? strAt(error, "message");
}

function callKey(session: string, callId: string): string {
  return `${session}/${callId}`;
}

/** Write the start of a tool call. A tool without a name, or a repeated start, writes nothing. */
export function recordToolStart(state: InstanceState, session: Session, event: unknown): void {
  const tool = strAt(event, "tool");
  if (tool === undefined) return;
  const target = targetFor(state, session);
  if (!target) return;

  const hostId = str(get(event, "id"));
  const callId = hostId ?? fallbackInstanceId(tool);
  const key = callKey(session.id, callId);
  if (state.calls.has(key)) return;

  emit(target, "tool_call_start", { call_id: callId, tool_name: tool, tool_input: payloadValue(get(event, "input")) });
  remember(state.calls, key, { callId, tool, target, generated: hostId === undefined, ended: false });
  if (hostId === undefined) {
    const pending = state.generatedCalls.get(session.id) ?? [];
    pending.push(key);
    state.generatedCalls.set(session.id, pending);
  }
}

/** The latest started call with a generated id for this tool, taken off the pending list. */
function takeGeneratedCall(state: InstanceState, session: string, tool: string | undefined): CallRecord | undefined {
  const pending = state.generatedCalls.get(session) ?? [];
  for (let i = pending.length - 1; i >= 0; i -= 1) {
    const call = state.calls.get(pending[i]);
    if (call === undefined) {
      pending.splice(i, 1);
    } else if (!call.ended && (tool === undefined || call.tool === tool)) {
      pending.splice(i, 1);
      return call;
    }
  }
  return undefined;
}

function writeEnd(call: CallRecord, fields: Record<string, unknown>): void {
  call.ended = true;
  emit(call.target, "tool_call_end", { call_id: call.callId, tool_name: call.tool, ...fields });
}

/** Write the end of a call from its execute.after payload. */
export function recordToolEnd(state: InstanceState, session: Session, event: unknown): void {
  const tool = strAt(event, "tool");
  const hostId = str(get(event, "id"));
  let call =
    hostId !== undefined ? state.calls.get(callKey(session.id, hostId)) : takeGeneratedCall(state, session.id, tool);
  if (call?.ended) return;

  if (call === undefined) {
    if (tool === undefined) return;
    const target = targetFor(state, session);
    if (!target) return;
    const callId = hostId ?? fallbackInstanceId(tool);
    call = { callId, tool, target, generated: hostId === undefined, ended: false };
    remember(state.calls, callKey(session.id, callId), call);
  }

  const status = get(event, "status");
  const result = payloadValue(get(event, "result"));
  if (status === "completed") writeEnd(call, { status: "success", tool_output: result });
  else if (status === "error") writeEnd(call, { status: "error", error: errorMessage(get(event, "error")) });
  else writeEnd(call, { tool_output: result });
}

/** A tool failure event ends a started, not yet ended call with an error. */
export function recordToolFailure(state: InstanceState, session: Session, data: unknown): void {
  const callId = strAt(data, "id");
  if (callId === undefined) return;
  const call = state.calls.get(callKey(session.id, callId));
  if (call === undefined || call.ended) return;
  writeEnd(call, { status: "error", error: errorMessage(get(data, "error")) });
}
