/**
 * compaction.test.ts - Per-event schema stamping and compaction event shape.
 *
 * Covers:
 *   - buildEvent stamps compaction events with schema 1.5.0
 *   - every other event keeps the 1.3.0 stamp
 *   - the compaction handler (wired to the real buildEvent) emits a single phase-less event
 *   - routing to the subagent stream or the orchestrator stream
 *   - token and trigger fields only from genuine payload values
 *   - append failures stay non-fatal
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";

import { createNotificationHandlers } from "../lib/handlers_notifications";
import type {
  HandlerDependencies,
  SdkClient,
  SessionCompactingInput,
} from "../lib/handlers_session";
import { SessionCorrelationStore } from "../lib/correlation";
import { LogPaths, buildEvent, setDebugLogger } from "../lib/core";

const RUN_ID = "20260101T170000Z-a3f9";
const ORCH_SESSION = "sess-orch-001";
const SUB_SESSION = "sess-sub-001";
const AGENT_INSTANCE_ID = "Research#1";
const ENVELOPE = { sessionId: ORCH_SESSION, runId: RUN_ID, timestamp: "2026-01-01T17:00:00.000Z" };

let tmpDir: string;

beforeEach(() => {
  tmpDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-compaction-test-"));
  setDebugLogger(() => {});
});

afterEach(() => {
  nodeFs.rmSync(tmpDir, { recursive: true, force: true });
  setDebugLogger(() => {});
});

type Collected = { filePath: string; event: Record<string, unknown> };

function makeNullSdk(): SdkClient {
  return {
    session: { get: async () => undefined, messages: async () => [] },
    app: { log: async () => {} },
  };
}

function setup(
  appendOverride?: (filePath: string, event: Record<string, unknown>) => Promise<void>,
) {
  const store = new SessionCorrelationStore();
  const paths = new LogPaths(tmpDir);
  const collected: Collected[] = [];
  const deps: HandlerDependencies = {
    store,
    paths,
    sdkClient: makeNullSdk(),
    adapterVersion: "0.1.0",
    buildEvent,
    appendEvent:
      appendOverride ??
      (async (filePath, event) => {
        collected.push({ filePath, event });
      }),
    fallbackCallId: (toolName) => `${toolName ?? "tool"}_call_fallback`,
  };
  store.register(ORCH_SESSION, { runId: RUN_ID });
  const { handleSessionCompacting } = createNotificationHandlers(deps);
  return { store, paths, collected, handleSessionCompacting };
}

describe("buildEvent - per-event schema stamping", () => {
  it("stamps a compaction event with schema 1.5.0", () => {
    const event = buildEvent("compaction", ENVELOPE, {});
    expect(event.schema_version).toBe("1.5.0");
  });

  it.each(["notification", "session_start", "tool_use", "subagent_start", "session_end"])(
    "keeps schema 1.3.0 for a %s event",
    (name) => {
      const event = buildEvent(name, ENVELOPE, {});
      expect(event.schema_version).toBe("1.3.0");
    },
  );
});

describe("handleSessionCompacting - emitted compaction event", () => {
  it("stamps the emitted event with schema 1.5.0", async () => {
    const { collected, handleSessionCompacting } = setup();

    await handleSessionCompacting({ sessionID: ORCH_SESSION });

    expect(collected).toHaveLength(1);
    expect(collected[0].event.schema_version).toBe("1.5.0");
  });

  it("emits one self-contained event without a phase", async () => {
    const { collected, handleSessionCompacting } = setup();

    await handleSessionCompacting({ sessionID: ORCH_SESSION, trigger: "auto" });

    const compactions = collected.filter((e) => e.event.event === "compaction");
    expect(compactions).toHaveLength(1);
    expect(compactions[0].event.phase).toBeUndefined();
  });

  it("does not copy a phase value from the payload", async () => {
    const { collected, handleSessionCompacting } = setup();
    const payload = { sessionID: ORCH_SESSION, phase: "pre" } as unknown as SessionCompactingInput;

    await handleSessionCompacting(payload);

    expect(collected[0].event.phase).toBeUndefined();
  });
});

describe("handleSessionCompacting - stream routing", () => {
  it("writes a subagent session compaction to the subagent invocation stream", async () => {
    const { store, paths, collected, handleSessionCompacting } = setup();
    store.register(SUB_SESSION, { parentId: ORCH_SESSION, agentInstanceId: AGENT_INSTANCE_ID });

    await handleSessionCompacting({ sessionID: SUB_SESSION });

    expect(collected).toHaveLength(1);
    expect(collected[0].filePath).toBe(paths.invocationEvents(RUN_ID, AGENT_INSTANCE_ID));
    expect(collected[0].event.schema_version).toBe("1.5.0");
  });

  it("writes a primary session compaction to the orchestrator stream", async () => {
    const { paths, collected, handleSessionCompacting } = setup();

    await handleSessionCompacting({ sessionID: ORCH_SESSION });

    expect(collected[0].filePath).toBe(paths.orchestratorEvents(RUN_ID));
  });
});

describe("handleSessionCompacting - payload-derived fields", () => {
  it("emits trigger and token counts when the payload supplies genuine values", async () => {
    const { collected, handleSessionCompacting } = setup();

    await handleSessionCompacting({
      sessionID: ORCH_SESSION,
      trigger: "manual",
      tokens_before: 9000,
      tokens_after: 2500,
    });

    expect(collected[0].event.trigger).toBe("manual");
    expect(collected[0].event.tokens_before).toBe(9000);
    expect(collected[0].event.tokens_after).toBe(2500);
  });

  it("omits token counts that are absent or not numeric", async () => {
    const { collected, handleSessionCompacting } = setup();
    const payload = {
      sessionID: ORCH_SESSION,
      tokens_before: "9000",
    } as unknown as SessionCompactingInput;

    await handleSessionCompacting(payload);

    expect(collected[0].event.tokens_before).toBeUndefined();
    expect(collected[0].event.tokens_after).toBeUndefined();
  });
});

describe("handleSessionCompacting - failure handling", () => {
  it("does not throw when the append fails", async () => {
    const { handleSessionCompacting } = setup(async () => {
      throw new Error("disk full");
    });

    await expect(handleSessionCompacting({ sessionID: ORCH_SESSION })).resolves.toBeUndefined();
  });
});
