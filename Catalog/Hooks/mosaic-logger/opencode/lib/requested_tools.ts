/**
 * requested_tools.ts - Per-message tracking of requested tool parts.
 *
 * Tool parts reach the adapter on message.part.updated and, possibly, on
 * message.updated. Each part is remembered once per message (identified by
 * callID, falling back to the part id), so every usage_record observation of a
 * message can carry all tools known for it so far.
 *
 * Entries live until their session is released (session deleted); a message's
 * entry is never released earlier because a later observation must still carry
 * all of its tools.
 */

/** Tool name -> number of requested tool parts. */
export type RequestedTools = Record<string, number>;

export class RequestedToolsTracker {
  /** sessionId -> messageId -> part identity -> tool name. */
  private readonly sessions = new Map<string, Map<string, Map<string, string>>>();

  /** Record one message part; anything that is not a named, identifiable tool part is ignored. */
  recordPart(sessionId: string, messageId: string, part: unknown): void {
    if (!part || typeof part !== "object") return;
    const p = part as Record<string, unknown>;
    if (p["type"] !== "tool") return;
    const tool = p["tool"];
    if (typeof tool !== "string" || tool === "") return;
    const identity = nonEmptyString(p["callID"]) ?? nonEmptyString(p["id"]);
    if (identity === undefined) return;

    let messages = this.sessions.get(sessionId);
    if (!messages) {
      messages = new Map();
      this.sessions.set(sessionId, messages);
    }
    let parts = messages.get(messageId);
    if (!parts) {
      parts = new Map();
      messages.set(messageId, parts);
    }
    parts.set(identity, tool);
  }

  /** Record every tool part in a parts list. */
  recordParts(sessionId: string, messageId: string, parts: unknown): void {
    if (!Array.isArray(parts)) return;
    for (const part of parts) this.recordPart(sessionId, messageId, part);
  }

  /** Tool name counts for a message; empty when none were requested. */
  snapshot(sessionId: string, messageId: string): RequestedTools {
    const counts: RequestedTools = {};
    const parts = this.sessions.get(sessionId)?.get(messageId);
    if (!parts) return counts;
    for (const tool of parts.values()) counts[tool] = (counts[tool] ?? 0) + 1;
    return counts;
  }

  /** Forget everything tracked for a session. */
  releaseSession(sessionId: string): void {
    this.sessions.delete(sessionId);
  }
}

function nonEmptyString(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}
