/**
 * run_binding.ts -- Run-binding sources for native OpenCode sessions.
 *
 * A native session is bound to a run only by a MOSAIC dispatch, i.e. a prompt
 * handed to a subagent through the Task tool that carries an agent instance id
 * as well as a run id. Message text (user or assistant) never binds.
 */

import { extractInstanceId, extractRunId } from "./runstate.js";

/**
 * Run id of a MOSAIC dispatch prompt, else undefined.
 * A dispatch carries an agent instance id as well as a run id.
 * Pure. Never throws.
 */
export function dispatchRunId(prompt: unknown): string | undefined {
  if (typeof prompt !== "string" || prompt === "") return undefined;
  if (extractInstanceId(prompt) === undefined) return undefined;
  return extractRunId(prompt);
}

/** Dispatch run id of one history part when it is a Task tool part, else undefined. */
function partDispatchRunId(part: unknown): string | undefined {
  if (typeof part !== "object" || part === null) return undefined;
  const p = part as Record<string, unknown>;
  if (p["type"] !== "tool" || p["tool"] !== "task") return undefined;
  const state = p["state"];
  if (typeof state !== "object" || state === null) return undefined;
  const input = (state as Record<string, unknown>)["input"];
  if (typeof input !== "object" || input === null) return undefined;
  return dispatchRunId((input as Record<string, unknown>)["prompt"]);
}

/**
 * Run id of the LAST MOSAIC dispatch found in a session's message history
 * (the value returned by sdkClient.session.messages()), else undefined.
 * Only Task tool parts count; message text never does. Never throws.
 */
export function historyDispatchRunId(messages: unknown): string | undefined {
  try {
    if (!Array.isArray(messages)) return undefined;
    let found: string | undefined;
    for (const entry of messages) {
      if (typeof entry !== "object" || entry === null) continue;
      const parts = (entry as Record<string, unknown>)["parts"];
      if (!Array.isArray(parts)) continue;
      for (const part of parts) {
        const runId = partDispatchRunId(part);
        if (runId !== undefined) found = runId;
      }
    }
    return found;
  } catch {
    return undefined;
  }
}
