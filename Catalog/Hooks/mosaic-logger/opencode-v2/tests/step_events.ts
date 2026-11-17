/**
 * step_events.ts - builders for the V2 step, text, usage, tool, permission and
 * compaction events and the matching tool-hook payloads, in the shapes the spike
 * probes and the plugin research documented.
 */

import type { V2Event } from "../lib/host";
import { located, unlocated } from "./events";

export interface V2Tokens {
  input?: number;
  output?: number;
  reasoning?: number;
  cache?: { read?: number; write?: number };
}

export function tokens(input: number, output: number, reasoning = 0, cacheRead = 0, cacheWrite = 0): V2Tokens {
  return { input, output, reasoning, cache: { read: cacheRead, write: cacheWrite } };
}

export function stepStarted(
  dir: string,
  sessionID: string,
  assistantMessageID: string,
  model: { id: string; providerID: string } = { id: "gpt-6-luna", providerID: "openai" },
  started = 1794925254870,
): V2Event {
  return located(dir, "session.step.started", { sessionID, assistantMessageID, agent: "build", model, snapshot: "abc", started });
}

export function stepEnded(
  dir: string,
  sessionID: string,
  assistantMessageID: string,
  finish: string,
  stepTokens: unknown,
  extra: Record<string, unknown> = {},
): V2Event {
  return located(dir, "session.step.ended", { sessionID, assistantMessageID, finish, cost: 0.42, tokens: stepTokens, files: [], ...extra });
}

export function stepFailed(dir: string, sessionID: string, assistantMessageID: string, stepTokens?: unknown): V2Event {
  const data: Record<string, unknown> = { sessionID, assistantMessageID, error: { message: "provider failed" } };
  if (stepTokens !== undefined) data.tokens = stepTokens;
  return located(dir, "session.step.failed", data);
}

export function textEnded(dir: string, sessionID: string, assistantMessageID: string, text: string, ordinal = 0): V2Event {
  return located(dir, "session.text.ended", { sessionID, assistantMessageID, ordinal, text });
}

export function reasoningEnded(dir: string, sessionID: string, assistantMessageID: string, text: string): V2Event {
  return located(dir, "session.reasoning.ended", { sessionID, assistantMessageID, ordinal: 0, text });
}

export function toolInputStarted(dir: string, sessionID: string, assistantMessageID: string, id: string, name: string): V2Event {
  return located(dir, "session.tool.input.started", { sessionID, assistantMessageID, id, name });
}

/** Cumulative per-session usage; carries no location. */
export function usageUpdated(sessionID: string, cumulative: unknown): V2Event {
  return unlocated("session.usage.updated", { sessionID, cost: 1.5, tokens: cumulative });
}

export function toolFailed(dir: string, sessionID: string, id: string, message: string): V2Event {
  return located(dir, "session.tool.failed", {
    sessionID,
    assistantMessageID: "msg_a",
    id,
    error: { type: "tool.execution", message },
    executed: false,
  });
}

export function compactionEnded(
  dir: string,
  sessionID: string,
  reason: unknown,
  compactionTokens?: unknown,
  model?: unknown,
): V2Event {
  const data: Record<string, unknown> = { sessionID, reason, text: "summary" };
  if (compactionTokens !== undefined) data.tokens = compactionTokens;
  if (model !== undefined) data.model = model;
  return located(dir, "session.compaction.ended", data);
}

export function compactionOther(dir: string, sessionID: string, kind: "started" | "delta" | "failed"): V2Event {
  return located(dir, `session.compaction.${kind}`, { sessionID, reason: "auto", error: { message: "x" }, text: "partial" });
}

export function permissionAsked(dir: string, sessionID: string, requestID: string, callId: string): V2Event {
  return located(dir, "permission.asked", {
    id: requestID,
    sessionID,
    action: "edit",
    resources: ["src/a.ts"],
    source: { type: "tool", id: callId },
  });
}

export function permissionReplied(dir: string, sessionID: string, requestID: string, reply: string): V2Event {
  return located(dir, "permission.replied", { sessionID, requestID, reply });
}

export function toolBefore(session: string, id: string | undefined, tool: unknown, input?: unknown): Record<string, unknown> {
  const payload: Record<string, unknown> = { tool, sessionID: session, agent: "build", messageID: "msg_a", input };
  if (id !== undefined) payload.id = id;
  return payload;
}

export function toolAfter(
  session: string,
  id: string | undefined,
  tool: string,
  status: string,
  detail: { result?: unknown; error?: unknown; input?: unknown } = {},
): Record<string, unknown> {
  const payload: Record<string, unknown> = { tool, sessionID: session, agent: "build", messageID: "msg_a", status, input: detail.input };
  if (id !== undefined) payload.id = id;
  if ("result" in detail) payload.result = detail.result;
  if ("error" in detail) payload.error = detail.error;
  return payload;
}

export function evaluatePayload(session: string, effect: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    sessionID: session,
    agent: "build",
    action: "edit",
    resources: ["src/a.ts"],
    source: { type: "tool", id: "call_perm" },
    effect,
    ...extra,
  };
}
