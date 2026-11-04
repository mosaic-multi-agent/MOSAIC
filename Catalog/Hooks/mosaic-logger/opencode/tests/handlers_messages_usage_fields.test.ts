/**
 * handlers_messages_usage_fields.test.ts - usage_record token usage, provider and
 * service_tier rules for the OpenCode message handler.
 *
 *   - output_tokens counts reasoning; reasoning_tokens is reported separately
 *   - A missing tokens.reasoning means no reasoning (never written as 0)
 *   - provider is copied verbatim from info.providerID, omitted when absent or empty
 *   - model is unchanged; provider never appears inside it
 *   - No usage_record carries service_tier, whatever info.mode holds
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";

import { createMessageHandlers, buildTokenUsage } from "../lib/handlers_messages";
import type { HandlerDependencies, SdkClient, MessageInfo } from "../lib/handlers_session";
import { SessionCorrelationStore } from "../lib/correlation";
import { LogPaths, setDebugLogger } from "../lib/core";
import { assistantMessageEntry, messageUpdatedEvent } from "./fixtures/opencode_api";

const RUN_ID = "20260101T170000Z-a3f9";
const SESSION = "sess-orch-001";

let tmpDir: string;

beforeEach(() => {
  tmpDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-usage-fields-"));
  setDebugLogger(() => {});
});

afterEach(() => {
  nodeFs.rmSync(tmpDir, { recursive: true, force: true });
  setDebugLogger(() => {});
});

type Emitted = Record<string, unknown>;

/** Run one assistant message through the handler and return its usage_record (or undefined). */
async function usageRecordFor(
  overrides: Partial<MessageInfo> & Record<string, unknown>,
): Promise<{ record: Emitted | undefined; all: Emitted[] }> {
  const store = new SessionCorrelationStore();
  const collected: Emitted[] = [];
  const sdk: SdkClient = {
    session: { get: async () => undefined, messages: async () => [] },
    app: { log: async () => {} },
  };
  const deps: HandlerDependencies = {
    store,
    paths: new LogPaths(tmpDir),
    sdkClient: sdk,
    adapterVersion: "0.1.0",
    // Keeps empty strings so "omitted when empty" is decided by the handler, not the test double.
    buildEvent: (event, envelope, fields) => ({
      schema_version: "1.3.0",
      event,
      timestamp: envelope.timestamp,
      harness: "opencode",
      ...(envelope.runId ? { run_id: envelope.runId } : {}),
      ...Object.fromEntries(
        Object.entries(fields).filter(([, v]) => v !== undefined && v !== null),
      ),
    }),
    appendEvent: async (_filePath, event) => {
      collected.push(event as Emitted);
    },
    fallbackCallId: () => "fallback-call-id",
  };
  const { handleMessageEvent } = createMessageHandlers(deps);
  store.register(SESSION, { runId: RUN_ID });
  const msg = assistantMessageEntry("Answer.", {
    id: "msg-fields-1",
    sessionID: SESSION,
    ...overrides,
  } as Partial<MessageInfo>);
  await handleMessageEvent(messageUpdatedEvent(msg));
  return { record: collected.find((e) => e.event === "usage_record"), all: collected };
}

type Usage = Record<string, unknown>;

describe("usage_record token_usage - reasoning", () => {
  it("adds reasoning to output_tokens and reports reasoning_tokens separately", async () => {
    const { record } = await usageRecordFor({
      tokens: { input: 10, output: 241, reasoning: 80, cache: { read: 5, write: 7 } },
    });

    const usage = record!.token_usage as Usage;
    expect(usage.output_tokens).toBe(321);
    expect(usage.reasoning_tokens).toBe(80);
  });

  it("keeps input, cache read and cache write unchanged", async () => {
    const { record } = await usageRecordFor({
      tokens: { input: 10, output: 20, reasoning: 30, cache: { read: 5, write: 7 } },
    });

    const usage = record!.token_usage as Usage;
    expect(usage.input_tokens).toBe(10);
    expect(usage.cache_read_tokens).toBe(5);
    expect(usage.cache_creation_tokens).toBe(7);
  });

  it("reports reasoning_tokens 0 and output unchanged when tokens.reasoning is 0", async () => {
    const { record } = await usageRecordFor({ tokens: { input: 1, output: 50, reasoning: 0 } });

    const usage = record!.token_usage as Usage;
    expect(usage.reasoning_tokens).toBe(0);
    expect(usage.output_tokens).toBe(50);
  });

  it("writes output alone and omits reasoning_tokens when tokens.reasoning is missing", async () => {
    const { record } = await usageRecordFor({ tokens: { input: 1, output: 50 } });

    const usage = record!.token_usage as Usage;
    expect(usage.output_tokens).toBe(50);
    expect(usage).not.toHaveProperty("reasoning_tokens");
  });

  it("omits output_tokens when tokens.output is not a number even if reasoning is", async () => {
    const { record } = await usageRecordFor({
      tokens: { input: 1, output: "50" as unknown as number, reasoning: 9 },
    });

    const usage = record!.token_usage as Usage;
    expect(usage).not.toHaveProperty("output_tokens");
    expect(usage.reasoning_tokens).toBe(9);
  });

  it("omits reasoning_tokens and keeps output alone when tokens.reasoning is not a number", async () => {
    const { record } = await usageRecordFor({
      tokens: { input: 1, output: 50, reasoning: "80" as unknown as number },
    });

    const usage = record!.token_usage as Usage;
    expect(usage.output_tokens).toBe(50);
    expect(usage).not.toHaveProperty("reasoning_tokens");
  });
});

describe("usage_record token_usage - zero output boundary", () => {
  it("counts reasoning as output when tokens.output is 0", async () => {
    const { record } = await usageRecordFor({ tokens: { input: 1, output: 0, reasoning: 5 } });

    const usage = record!.token_usage as Usage;
    expect(usage.output_tokens).toBe(5);
    expect(usage.reasoning_tokens).toBe(5);
  });

  it("reports both output and reasoning as 0 when both are 0", async () => {
    const { record } = await usageRecordFor({ tokens: { input: 1, output: 0, reasoning: 0 } });

    const usage = record!.token_usage as Usage;
    expect(usage.output_tokens).toBe(0);
    expect(usage.reasoning_tokens).toBe(0);
  });
});

describe("buildTokenUsage - reasoning", () => {
  it("sums output and reasoning and reports reasoning separately", () => {
    const result = buildTokenUsage({ tokens: { output: 241, reasoning: 80 } });
    expect(result?.output_tokens).toBe(321);
    expect(result?.reasoning_tokens).toBe(80);
  });

  it("returns reasoning_tokens alone when it is the only resolvable number", () => {
    const result = buildTokenUsage({ tokens: { reasoning: 12 } });
    expect(result).toEqual({ reasoning_tokens: 12 });
  });

  it("returns undefined when reasoning is not a number and nothing else resolves", () => {
    const info: MessageInfo = { tokens: { reasoning: null as unknown as number } };
    expect(buildTokenUsage(info)).toBeUndefined();
  });
});

describe("usage_record provider", () => {
  it("copies info.providerID verbatim", async () => {
    const { record } = await usageRecordFor({ providerID: "github-copilot", modelID: "gpt-5" });

    expect(record!.provider).toBe("github-copilot");
  });

  it("leaves model unchanged and free of the provider", async () => {
    const { record } = await usageRecordFor({
      providerID: "github-copilot",
      modelID: "claude-sonnet-4.5",
    });

    expect(record!.model).toBe("claude-sonnet-4.5");
    expect(record!.provider).toBe("github-copilot");
  });

  it("omits provider when info.providerID is absent", async () => {
    const { record } = await usageRecordFor({ modelID: "gpt-5" });

    expect(record).toBeDefined();
    expect(record).not.toHaveProperty("provider");
  });

  it("omits provider when info.providerID is an empty string", async () => {
    const { record } = await usageRecordFor({ providerID: "", modelID: "gpt-5" });

    expect(record).toBeDefined();
    expect(record).not.toHaveProperty("provider");
  });

  it("omits provider when info.providerID is not a string", async () => {
    const { record } = await usageRecordFor({
      providerID: 123 as unknown as string,
      modelID: "gpt-5",
    });

    expect(record).toBeDefined();
    expect(record).not.toHaveProperty("provider");
  });

  it("does not add provider to the turn event", async () => {
    const { all } = await usageRecordFor({ providerID: "github-copilot", modelID: "gpt-5" });

    const turns = all.filter((e) => e.event === "turn");
    for (const turn of turns) {
      expect(turn).not.toHaveProperty("provider");
    }
  });
});

describe("usage_record service_tier", () => {
  it("does not carry service_tier when info.mode is set", async () => {
    const { record } = await usageRecordFor({ mode: "build-review", modelID: "gpt-5" });

    expect(record).toBeDefined();
    expect(record).not.toHaveProperty("service_tier");
  });

  it("carries no service_tier on any emitted event", async () => {
    const { all } = await usageRecordFor({
      mode: "auto",
      providerID: "github-copilot",
      tokens: { input: 1, output: 2 },
    });

    expect(all.length).toBeGreaterThan(0);
    for (const e of all) {
      expect(e).not.toHaveProperty("service_tier");
    }
  });
});
