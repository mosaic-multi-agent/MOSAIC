/**
 * events.ts - builders for V2 events and tool-hook payloads in the shapes the
 * spike probes observed, plus a helper that plays one complete subagent dispatch.
 */

import type { V2Event } from "../lib/host";
import type { FakeHost } from "./fake_host";

export const RUN_A = "20261117T150000Z-abcd";
export const RUN_B = "20261117T150100Z-ef01";

export const ROOT = "ses_root0000000000000000001";
export const CHILD_PREFIX = "You are a subagent spawned by another session.\n";

/** An event located in directory dir. */
export function located(dir: string, type: string, data: Record<string, unknown>): V2Event {
  return { type, location: { directory: dir }, data };
}

/** An event without a location (execution, usage, synthetic inbox items). */
export function unlocated(type: string, data: Record<string, unknown>): V2Event {
  return { type, data };
}

export function userInbox(dir: string, sessionID: string, text: string): V2Event {
  return located(dir, "session.inbox.enqueued", {
    sessionID,
    inboxID: `msg_${Math.random().toString(16).slice(2, 10)}`,
    item: { type: "user", payload: { text, files: [] }, delivery: "steer" },
  });
}

export function execution(
  sessionID: string,
  outcome: "succeeded" | "failed" | "interrupted" | "started" = "succeeded",
): V2Event {
  return unlocated(`session.execution.${outcome}`, { sessionID });
}

export function created(dir: string, sessionID: string, parentID: string | undefined, agent: string): V2Event {
  return located(dir, "session.created", {
    sessionID,
    projectID: "global",
    location: { directory: dir },
    subpath: "",
    ...(parentID ? { parentID } : {}),
    slug: "slug",
    title: "title",
    agent,
    model: { id: "gpt-6-luna", providerID: "openai" },
    version: "2.3.0",
  });
}

export function progress(dir: string, parent: string, callId: string, child: string): V2Event {
  return located(dir, "session.tool.progress", {
    sessionID: parent,
    assistantMessageID: "msg_assistant",
    id: callId,
    metadata: { sessionID: child, status: "running" },
  });
}

export function wrapWakeUp(child: string, state: string, body: string, description = "task"): string {
  return `<subagent sessionID="${child}" state="${state}" description="${description}">\n${body}\n</subagent>`;
}

/** Synthetic inbox item delivered to the parent when a child changes state. */
export function wakeUp(
  parent: string,
  child: string,
  agent: string,
  state: string,
  text: string,
): V2Event {
  return unlocated("session.inbox.enqueued", {
    sessionID: parent,
    inboxID: `msg_${Math.random().toString(16).slice(2, 10)}`,
    item: {
      type: "synthetic",
      payload: { text, description: "task", metadata: { source: "subagent", childID: child, agent, state } },
      delivery: "steer",
    },
  });
}

export interface DispatchInput {
  agent: string;
  description?: string;
  prompt: string;
  sessionID?: unknown;
}

export function beforePayload(session: string, callId: string, input: DispatchInput): Record<string, unknown> {
  return { tool: "subagent", sessionID: session, agent: "orchestrator", messageID: "msg_a", id: callId, input };
}

/** The launch-time result of a subagent call: the child is running. */
export function afterPayload(
  session: string,
  callId: string,
  input: DispatchInput,
  child: string | undefined,
  status = "completed",
): Record<string, unknown> {
  const result = child
    ? { output: { sessionID: child, status: "running", output: "working" }, metadata: { sessionID: child, status: "running" } }
    : { output: "nothing launched" };
  return { tool: "subagent", sessionID: session, agent: "orchestrator", messageID: "msg_a", id: callId, input, status, result };
}

export interface DispatchOptions {
  dir: string;
  parent: string;
  child: string;
  callId: string;
  agent: string;
  prompt: string;
}

/** One complete native dispatch: before, after (launch result), session.created and tool progress. */
export async function dispatchChild(host: FakeHost, o: DispatchOptions): Promise<void> {
  const input: DispatchInput = { agent: o.agent, description: "task", prompt: o.prompt };
  await host.fireBefore(beforePayload(o.parent, o.callId, input));
  await host.fireAfter(afterPayload(o.parent, o.callId, input, o.child));
  await host.emit(created(o.dir, o.child, o.parent, o.agent));
  await host.emit(progress(o.dir, o.parent, o.callId, o.child));
  await host.emit(userInbox(o.dir, o.child, CHILD_PREFIX + o.prompt));
}

/** A dispatch prompt carrying the MOSAIC identity the orchestrator would write. */
export function missionPrompt(instanceId: string, runId: string | undefined, task = "Do the work."): string {
  const lines = [`agent_instance_id: ${instanceId}`];
  if (runId) lines.push(`run_id: ${runId}`);
  lines.push("", task);
  return lines.join("\n");
}
