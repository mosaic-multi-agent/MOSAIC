/**
 * handlers_tool_parts_support.ts - Shared setup for the tool-part and tool
 * outcome test files: in-memory handler dependencies with a capturing
 * appendEvent, and small helpers to drive tool calls and read the results.
 */

import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";
import { beforeEach, afterEach } from "vitest";

import { createToolHandlers } from "../lib/handlers_tools";
import { createToolPartHandlers } from "../lib/handlers_tool_parts";
import type { HandlerDependencies, OpenCodeEvent } from "../lib/handlers_session";
import { SessionCorrelationStore } from "../lib/correlation";
import { LogPaths, setDebugLogger } from "../lib/core";
import {
  taskDispatchPayload,
  toolBeforePayload,
  toolAfterPayload,
} from "./fixtures/opencode_api";

export const RUN_ID = "20260101T170000Z-a3f9";
export const ORCH_SESSION = "sess-orch-001";
export const SUB_SESSION = "sess-sub-001";
export const AGENT_INSTANCE_ID = "Research#1";

export type CollectedEvent = { filePath: string; event: Record<string, unknown> };

export interface Harness {
  store: SessionCorrelationStore;
  paths: LogPaths;
  collected: CollectedEvent[];
  before: (tool: string, sessionID: string, callID: string) => Promise<void>;
  beforeTask: (sessionID: string, callID: string) => Promise<void>;
  after: (
    tool: string,
    sessionID: string,
    callID: string,
    output?: Record<string, unknown>,
  ) => Promise<void>;
  /** After-hook invoked with an undefined second argument. */
  afterNoOutput: (tool: string, sessionID: string, callID: string) => Promise<void>;
  part: (event: OpenCodeEvent) => Promise<void>;
  closeDispatch: (childSessionId: string) => Promise<void>;
  /** tool_call_end events, optionally narrowed to one call_id. */
  ends: (callId?: string) => CollectedEvent[];
}

/** Registers a temp workspace per test and returns an accessor for its root. */
export function installTempDir(): { dir: () => string } {
  let tmpDir = "";
  beforeEach(() => {
    tmpDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-tp-test-"));
    setDebugLogger(() => {});
  });
  afterEach(() => {
    nodeFs.rmSync(tmpDir, { recursive: true, force: true });
    setDebugLogger(() => {});
  });
  return { dir: () => tmpDir };
}

let counter = 0;

/** Build a handler harness over one shared store, with an orchestrator and a subagent session registered. */
export function makeHarness(root: string): Harness {
  const store = new SessionCorrelationStore();
  const paths = new LogPaths(root);
  const collected: CollectedEvent[] = [];
  const deps: HandlerDependencies = {
    store,
    paths,
    sdkClient: {
      session: { get: async () => undefined, messages: async () => [] },
      app: { log: async () => {} },
    },
    adapterVersion: "0.1.0",
    buildEvent: (event, envelope, fields) => ({
      schema_version: "1.3.0",
      event,
      timestamp: envelope.timestamp,
      harness: "opencode",
      ...(envelope.sessionId ? { session_id: envelope.sessionId } : {}),
      ...(envelope.runId ? { run_id: envelope.runId } : {}),
      ...Object.fromEntries(
        Object.entries(fields).filter(([, v]) => v !== undefined && v !== null && v !== ""),
      ),
    }),
    appendEvent: async (filePath, event) => {
      collected.push({ filePath, event });
    },
    fallbackCallId: (toolName) => `${toolName ?? "tool"}_fallback_${++counter}`,
  };

  store.register(ORCH_SESSION, { runId: RUN_ID });
  store.register(SUB_SESSION, {
    parentId: ORCH_SESSION,
    agentInstanceId: AGENT_INSTANCE_ID,
  });

  const tools = createToolHandlers(deps);
  const parts = createToolPartHandlers(deps);

  return {
    store,
    paths,
    collected,
    before: async (tool, sessionID, callID) => {
      const [i, o] = toolBeforePayload({ tool, sessionID, callID });
      await tools.handleToolBefore(i, o);
    },
    beforeTask: async (sessionID, callID) => {
      store.register(SUB_SESSION, {
        parentId: ORCH_SESSION,
        agentInstanceId: AGENT_INSTANCE_ID,
        dispatchCallId: callID,
      });
      const [i, o] = taskDispatchPayload({
        sessionID,
        callID,
        agentInstanceId: AGENT_INSTANCE_ID,
        runId: RUN_ID,
      });
      await tools.handleToolBefore(i, o);
    },
    after: async (tool, sessionID, callID, output) => {
      const [i, o] = toolAfterPayload({ tool, sessionID, callID, ...(output ? { output } : {}) });
      await tools.handleToolAfter(i, o);
    },
    afterNoOutput: async (tool, sessionID, callID) => {
      const [i] = toolAfterPayload({ tool, sessionID, callID });
      await tools.handleToolAfter(i, undefined);
    },
    part: (event) => parts.handleMessagePartUpdated(event),
    closeDispatch: (childSessionId) => tools.closeDispatchCall(childSessionId),
    ends: (callId) =>
      collected.filter(
        (e) => e.event.event === "tool_call_end" && (callId === undefined || e.event.call_id === callId),
      ),
  };
}
