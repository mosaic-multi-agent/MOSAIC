/**
 * step_records.ts - turn, usage_record and compaction records.
 *
 * One step is one assistant message. Its text parts become one assistant turn and
 * its tokens one usage_record, both written when the step ends (or fails), the turn
 * first. Usage comes only from the step's own report, never from the cumulative
 * per-session event, and no cost is ever written. Reasoning text is never logged.
 * Under the Runner, a step that finishes with "stop" ends the primary session.
 */

import { get, rec, str, strAt } from "./safe.js";
import { endIfQuiescent, touch } from "./root_lifecycle.js";
import { remember, rememberKey, type InstanceState, type Session, type StepInfo, type Target } from "./state.js";
import { emit, targetFor } from "./targets.js";
import { tokenUsageFromV2 } from "./usage.js";

function finiteNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stepKey(session: string, message: string): string {
  return `${session}/${message}`;
}

/** The step's record, created on first sight. */
function stepOf(state: InstanceState, session: string, message: string): StepInfo {
  const key = stepKey(session, message);
  let info = state.steps.get(key);
  if (!info) {
    info = { tools: new Map(), toolIds: new Set(), texts: [] };
    remember(state.steps, key, info);
  }
  return info;
}

/** A user inbox item becomes a user turn, once per inbox id. */
export function recordUserTurn(state: InstanceState, session: Session, inboxId: string | undefined, text: unknown): void {
  const content = typeof text === "string" && text !== "" ? text : undefined;
  if (content === undefined) return;
  if (inboxId !== undefined) {
    const key = stepKey(session.id, inboxId);
    if (state.inboxSeen.has(key)) return;
    rememberKey(state.inboxSeen, key);
  }
  const target = targetFor(state, session, content);
  if (target) emit(target, "turn", { role: "user", content });
}

export function onStepStarted(state: InstanceState, session: Session, data: unknown): void {
  const message = strAt(data, "assistantMessageID");
  if (message === undefined) return;
  const info = stepOf(state, session.id, message);
  info.model = strAt(data, "model", "id") ?? info.model;
  info.provider = strAt(data, "model", "providerID") ?? info.provider;
  info.started = finiteNumber(get(data, "started")) ?? info.started;
}

export function onToolRequested(state: InstanceState, session: Session, data: unknown): void {
  const message = strAt(data, "assistantMessageID");
  const name = strAt(data, "name");
  if (message === undefined || name === undefined) return;
  const info = stepOf(state, session.id, message);
  const callId = strAt(data, "id");
  if (callId !== undefined) {
    if (info.toolIds.has(callId)) return;
    info.toolIds.add(callId);
  }
  info.tools.set(name, (info.tools.get(name) ?? 0) + 1);
}

export function onTextEnded(state: InstanceState, session: Session, data: unknown): void {
  const message = strAt(data, "assistantMessageID");
  const text = get(data, "text");
  if (message === undefined || typeof text !== "string") return;
  const info = stepOf(state, session.id, message);
  info.texts.push({ ordinal: finiteNumber(get(data, "ordinal")) ?? 0, seq: info.texts.length, text });
}

/** The assistant turn of an ended step: its texts by ordinal, then arrival, joined by newlines. */
function writeAssistantTurn(state: InstanceState, session: Session, target: Target, message: string, info: StepInfo | undefined): void {
  const key = stepKey(session.id, message);
  if (state.turnsDone.has(key)) return;
  rememberKey(state.turnsDone, key);
  const texts = [...(info?.texts ?? [])].sort((a, b) => a.ordinal - b.ordinal || a.seq - b.seq);
  const content = texts.map((t) => t.text).filter((t) => t !== "").join("\n");
  if (content === "") return;
  session.lastAssistant = content;
  emit(target, "turn", { role: "assistant", content, model: info?.model, provider: info?.provider });
}

/** Write a usage record once per record id, numbered in the order written for its session. */
function writeUsage(
  state: InstanceState,
  session: Session,
  target: Target,
  recordId: string,
  fields: Record<string, unknown>,
  options: { timestamp?: string; raw?: Record<string, unknown> },
): void {
  let ledger = state.usage.get(session.id);
  if (!ledger) {
    ledger = { next: 0, ids: new Set() };
    state.usage.set(session.id, ledger);
  }
  if (ledger.ids.has(recordId)) return;
  ledger.ids.add(recordId);
  const index = ledger.next;
  ledger.next += 1;
  emit(
    target,
    "usage_record",
    { record_id: recordId, record_index: index, source: target.source, agent_instance_id: target.instanceId, ...fields },
    options,
  );
}

function stepUsage(state: InstanceState, session: Session, target: Target, data: unknown, eventId: string | undefined, failed: boolean): void {
  const message = strAt(data, "assistantMessageID");
  const recordId = message ?? (eventId !== undefined ? `${session.id}:${eventId}` : undefined);
  if (recordId === undefined) return;
  const tokens = tokenUsageFromV2(get(data, "tokens"));
  if (failed && tokens === undefined) return;

  const info = message !== undefined ? state.steps.get(stepKey(session.id, message)) : undefined;
  const requested: Record<string, number> = {};
  for (const [name, count] of info?.tools ?? []) requested[name] = count;
  const timestamp = info?.started !== undefined ? isoOf(info.started) : undefined;
  writeUsage(
    state,
    session,
    target,
    recordId,
    { model: info?.model, provider: info?.provider, token_usage: tokens },
    { timestamp, raw: { requested_tools: requested } },
  );
}

function isoOf(epochMs: number): string | undefined {
  const date = new Date(epochMs);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

/** A step ended or failed: its turn, its usage record, and under the Runner a stop step ends the primary. */
export function onStepFinished(
  state: InstanceState,
  session: Session,
  data: unknown,
  eventId: string | undefined,
  failed: boolean,
): void {
  touch(state, session);
  const target = targetFor(state, session);
  if (!target) return;
  const message = strAt(data, "assistantMessageID");
  if (message !== undefined) {
    writeAssistantTurn(state, session, target, message, state.steps.get(stepKey(session.id, message)));
  }
  stepUsage(state, session, target, data, eventId, failed);
  if (message !== undefined) state.steps.delete(stepKey(session.id, message));
  if (!failed && state.mode !== undefined && str(get(data, "finish")) === "stop") {
    endIfQuiescent(state, session, "session.step.ended");
  }
}

/** The model of a compaction report: a string, or an object with id and providerID. */
function compactionModel(model: unknown): { model?: string; provider?: string } {
  if (typeof model === "string") return { model: str(model) };
  if (rec(model) === undefined) return {};
  return { model: strAt(model, "id"), provider: strAt(model, "providerID") };
}

/**
 * An ended compaction: one self-contained compaction event, then its usage record
 * when the report carries tokens and the event has an id of its own.
 */
export function onCompactionEnded(state: InstanceState, session: Session, data: unknown, eventId: string | undefined): void {
  touch(state, session);
  if (eventId !== undefined && state.compactionsSeen.has(eventId)) return;
  const target = targetFor(state, session);
  if (!target) return;
  if (eventId !== undefined) rememberKey(state.compactionsSeen, eventId);

  const reason = str(get(data, "reason"));
  emit(target, "compaction", { trigger: reason === "auto" || reason === "manual" ? reason : undefined });

  const tokens = tokenUsageFromV2(get(data, "tokens"));
  if (eventId === undefined || tokens === undefined) return;
  const { model, provider } = compactionModel(get(data, "model"));
  writeUsage(state, session, target, `compaction:${eventId}`, { call_kind: "compaction", model, provider, token_usage: tokens }, {});
}
