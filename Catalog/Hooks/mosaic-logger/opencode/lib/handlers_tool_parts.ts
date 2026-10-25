/**
 * handlers_tool_parts.ts - Tool part update handling (message.part.updated).
 *
 * HIGH-FREQUENCY PATH: in-memory lookups only. Closes failed tool calls from
 * error tool parts through the single-close tool-call registry, writing
 * tool_call_end on the stream that holds the matching tool_call_start.
 */

import { currentTimestamp } from "./core.js";
import type { HandlerDependencies, OpenCodeEvent } from "./handlers_session.js";

/** Tool-call state carried on a message part. Shape verified on OpenCode 1.18.18; access defensively. */
export interface ToolPartState {
  status?: string;
  error?: unknown;
  [key: string]: unknown;
}

/** A message part with type "tool", carried at event.properties.part of message.part.updated. */
export interface ToolPart {
  type?: string;
  callID?: string;
  tool?: string;
  sessionID?: string;
  state?: ToolPartState;
  [key: string]: unknown;
}

export function createToolPartHandlers(deps: HandlerDependencies): {
  /** Handle one message.part.updated bus event. Never throws. */
  handleMessagePartUpdated: (event: OpenCodeEvent) => Promise<void>;
} {
  const { store, paths, buildEvent, appendEvent } = deps;

  async function handleMessagePartUpdated(event: OpenCodeEvent): Promise<void> {
    try {
      const props = event?.properties as Record<string, unknown> | undefined;
      const part = props?.["part"] as ToolPart | undefined;
      if (!part || part.type !== "tool" || part.state?.status !== "error") return;

      const callId = part.callID;
      if (typeof callId !== "string" || callId === "") return;

      // Single close: undefined when never registered or already closed.
      const entry = store.closeToolCall(callId);
      if (!entry) return;

      const errorText = part.state.error;
      const ts = currentTimestamp();
      const sessionId = entry.sessionId;
      await appendEvent(
        store.resolveEventSink(sessionId, paths),
        buildEvent(
          "tool_call_end",
          { sessionId, runId: store.getRunId(sessionId), timestamp: ts },
          {
            call_id: entry.callId,
            status: "error",
            error: typeof errorText === "string" && errorText !== "" ? errorText : undefined,
          },
        ),
      );
    } catch {
      // Never throw into the host.
    }
  }

  return { handleMessagePartUpdated };
}
