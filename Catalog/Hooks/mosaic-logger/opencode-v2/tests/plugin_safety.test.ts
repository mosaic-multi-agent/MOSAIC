/**
 * The plugin never fails the host: hook callbacks resolve normally for any
 * payload, never mutate what they are given, and a failure while processing
 * one event skips that event only.
 */

import { describe, expect, it } from "vitest";

import { ROOT, userInbox, execution, beforePayload, afterPayload } from "./events";
import { byEvent, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const OTHER_ROOT = "ses_root0000000000000000002";

/** Start the plugin and require that it registered its hooks, so a passing test exercised them. */
async function startRegistered() {
  const running = await env.start();
  expect(running.host.beforeHooks).toHaveLength(1);
  expect(running.host.afterHooks).toHaveLength(1);
  expect(running.host.subscribeCalls).toHaveLength(1);
  return running;
}

const MALFORMED_HOOK_PAYLOADS: Array<[string, unknown]> = [
  ["null", null],
  ["undefined", undefined],
  ["a string", "subagent"],
  ["a number", 7],
  ["an empty object", {}],
  ["a non-string tool", { tool: 5, sessionID: ROOT, id: "c1", input: {} }],
  ["a non-string session id", { tool: "subagent", sessionID: { nested: true }, id: "c1", input: { agent: "A", prompt: "p" } }],
  ["a null input", { tool: "subagent", sessionID: ROOT, id: "c1", input: null }],
  ["a string input", { tool: "subagent", sessionID: ROOT, id: "c1", input: "agent A" }],
  ["wrong-typed input fields", { tool: "subagent", sessionID: ROOT, id: 12, input: { agent: 3, prompt: ["x"], sessionID: 9 } }],
  ["a non-string status", { tool: "subagent", sessionID: ROOT, id: "c1", input: {}, status: 5, result: 5 }],
  ["a circular result", (() => { const r: Record<string, unknown> = { tool: "subagent", sessionID: ROOT, id: "c1", input: {} }; r.result = { self: r }; return r; })()],
];

describe("hook callbacks never throw into the host", () => {
  it.each(MALFORMED_HOOK_PAYLOADS)("execute.before resolves for %s", async (_label, payload) => {
    const { host, dir } = await startRegistered();
    await host.emit(userInbox(dir, ROOT, "hello"));

    await expect(host.fireBefore(payload)).resolves.toBeUndefined();
  });

  it.each(MALFORMED_HOOK_PAYLOADS)("execute.after resolves for %s", async (_label, payload) => {
    const { host, dir } = await startRegistered();
    await host.emit(userInbox(dir, ROOT, "hello"));

    await expect(host.fireAfter(payload)).resolves.toBeUndefined();
  });

  it.each(MALFORMED_HOOK_PAYLOADS.filter(([, p]) => p !== null && typeof p === "object"))(
    "execute.after resolves for %s even when it answers a pending dispatch",
    async (_label, payload) => {
      const { host, dir } = await startRegistered();
      await host.emit(userInbox(dir, ROOT, "hello"));
      await host.fireBefore(beforePayload(ROOT, "c1", { agent: "Worker", prompt: "agent_instance_id: Worker#1" }));

      await expect(host.fireAfter(payload)).resolves.toBeUndefined();
    },
  );

  it("execute.before and execute.after resolve for payloads whose members throw when read", async () => {
    const { host, dir } = await startRegistered();
    await host.emit(userInbox(dir, ROOT, "hello"));
    const hostile = (): Record<string, unknown> => {
      const payload: Record<string, unknown> = {};
      for (const name of ["tool", "sessionID", "id", "input", "result", "status"]) {
        Object.defineProperty(payload, name, {
          get() {
            throw new Error(`poisoned ${name}`);
          },
          enumerable: true,
        });
      }
      return payload;
    };

    await expect(host.fireBefore(hostile())).resolves.toBeUndefined();
    await expect(host.fireAfter(hostile())).resolves.toBeUndefined();
  });

  it("keeps logging normally after a malformed hook payload", async () => {
    const { host, dir } = await startRegistered();

    await host.fireBefore(null);
    await host.fireAfter({ tool: 5 });
    await host.emit(userInbox(dir, ROOT, "hello"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });
});

describe("hook callbacks observe only", () => {
  it("leave a frozen dispatch before/after payload exactly as it was given", async () => {
    const { host, dir } = await startRegistered();
    await host.emit(userInbox(dir, ROOT, "hello"));
    const input = { agent: "Research", description: "d", prompt: "agent_instance_id: Research#1" };
    const before = Object.freeze(beforePayload(ROOT, "call_1", Object.freeze(input)));
    const after = Object.freeze(afterPayload(ROOT, "call_1", input, "ses_child"));
    const beforeCopy = structuredClone(before);
    const afterCopy = structuredClone(after);

    await expect(host.fireBefore(before)).resolves.toBeUndefined();
    await expect(host.fireAfter(after)).resolves.toBeUndefined();

    expect(before).toEqual(beforeCopy);
    expect(after).toEqual(afterCopy);
  });

  it("do not abort a tool call: the before callback returns no rejection value", async () => {
    const { host, dir } = await startRegistered();
    await host.emit(userInbox(dir, ROOT, "hello"));

    const results = await Promise.all(host.beforeHooks.map(async (cb) => cb(beforePayload(ROOT, "call_2", { agent: "A", prompt: "p" }) as never)));

    for (const result of results) {
      expect(result === undefined || (typeof result === "object" && result !== null && !(result instanceof Error))).toBe(true);
    }
  });
});

describe("event processing survives bad events", () => {
  it("skips an event whose data access throws and still processes the next one", async () => {
    const { host, dir } = await startRegistered();
    const poisoned = { type: "session.inbox.enqueued" } as Record<string, unknown>;
    Object.defineProperty(poisoned, "data", {
      get() {
        throw new Error("poisoned payload");
      },
    });
    Object.defineProperty(poisoned, "location", {
      get() {
        throw new Error("poisoned location");
      },
    });

    await host.emitRaw(poisoned);
    await host.emit(userInbox(dir, ROOT, "valid"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });

  it.each([
    ["null", null],
    ["a string", "event"],
    ["a number", 3],
    ["no type", { data: { sessionID: ROOT } }],
    ["a non-string type", { type: 9, data: { sessionID: ROOT } }],
    ["null data", { type: "session.inbox.enqueued", data: null }],
    ["string data", { type: "session.created", data: "x" }],
    ["a non-string session id", { type: "session.execution.succeeded", data: { sessionID: 12 } }],
    ["a non-object location", { type: "session.created", location: "here", data: { sessionID: ROOT } }],
    ["a non-object inbox item", { type: "session.inbox.enqueued", location: { directory: "x" }, data: { sessionID: ROOT, item: "text" } }],
    ["a wake-up with wrong-typed metadata", { type: "session.inbox.enqueued", data: { sessionID: ROOT, item: { type: "synthetic", payload: { text: 5, metadata: { source: "subagent", childID: {}, state: 3 } } } } }],
  ])("ignores %s and keeps processing later events", async (_label, bad) => {
    const { host, dir } = await startRegistered();

    await host.emitRaw(bad);
    await host.emit(userInbox(dir, ROOT, "valid"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });

  it("writes no record for sessions named only by unusable events", async () => {
    const { host, dir } = await startRegistered();

    await host.emitRaw({ type: "session.execution.succeeded", data: { sessionID: 12 } });
    await host.emitRaw({ type: "session.created", data: "x" });
    await host.emit(execution("", "succeeded"));
    await quiet();

    expect(orchestratorStream(dir, UNKNOWN_RUN)).toEqual([]);
  });

  it("does not throw when the log folder cannot be written", async () => {
    const { host, dir } = await startRegistered();
    const nodeFs = await import("node:fs");
    const nodePath = await import("node:path");
    nodeFs.writeFileSync(nodePath.join(dir, "OrchestrationLogs"), "a file where the folder should be");

    await host.emit(userInbox(dir, ROOT, "hello"));
    await expect(host.fireBefore(beforePayload(ROOT, "c", { agent: "A", prompt: "p" }))).resolves.toBeUndefined();
    await host.emit(execution(ROOT, "succeeded"));

    nodeFs.rmSync(nodePath.join(dir, "OrchestrationLogs"), { force: true });
    await host.emit(userInbox(dir, OTHER_ROOT, "after the folder is writable again"));

    await eventually(() => {
      const starts = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start");
      expect(starts.map((s) => s.session_id)).toContain(OTHER_ROOT);
    });
    expect(host.subscribeCalls).toHaveLength(1);
  });
});
