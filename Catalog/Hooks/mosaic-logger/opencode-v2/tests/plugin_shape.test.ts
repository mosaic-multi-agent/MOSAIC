/**
 * The V2 plugin model: a plain {id, setup} default export that registers the
 * tool hooks and the event subscription, never waits for the subscription
 * loop, and returns a synchronous, idempotent cleanup.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it, vi } from "vitest";

import * as pluginModule from "../plugin";
import { RUN_A, ROOT, userInbox, execution, beforePayload } from "./events";
import { byEvent, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, sleep, useTestEnv } from "./harness";

const env = useTestEnv();

describe("plugin module shape", () => {
  it("default-exports a plain object with the id and a setup function and nothing else", () => {
    const plugin = pluginModule.default as Record<string, unknown>;

    expect(Object.keys(pluginModule)).toEqual(["default"]);
    expect(Object.getPrototypeOf(plugin)).toBe(Object.prototype);
    expect(plugin.id).toBe("mosaic-logger");
    expect(typeof plugin.setup).toBe("function");
  });
});

describe("setup registrations", () => {
  it("registers both tool hooks and one event subscription carrying an abort signal", async () => {
    const { host } = await env.start();

    expect(host.beforeHooks).toHaveLength(1);
    expect(host.afterHooks).toHaveLength(1);
    expect(host.subscribeCalls).toHaveLength(1);
    expect(host.subscribeCalls[0].signal).toBeInstanceOf(AbortSignal);
    expect(host.subscribeCalls[0].signal?.aborted).toBe(false);
  });

  it("subscribes to every event type the delivered behaviour reads, when it filters by type", async () => {
    const { host } = await env.start();

    const types = host.subscribeCalls[0]?.types;
    expect(host.subscribeCalls).toHaveLength(1);
    if (types === undefined) return;
    for (const needed of [
      "session.created",
      "session.deleted",
      "session.inbox.enqueued",
      "session.execution.succeeded",
      "session.execution.failed",
      "session.execution.interrupted",
      "session.tool.progress",
    ]) {
      expect(types).toContain(needed);
    }
  });

  it("also subscribes to the step, text, tool, permission and compaction events the logging reads, when it filters by type", async () => {
    const { host } = await env.start();

    const types = host.subscribeCalls[0]?.types;
    expect(host.subscribeCalls).toHaveLength(1);
    if (types === undefined) return;
    for (const needed of [
      "session.step.started",
      "session.step.ended",
      "session.step.failed",
      "session.text.ended",
      "session.tool.input.started",
      "session.tool.failed",
      "session.compaction.ended",
      "permission.asked",
      "permission.replied",
    ]) {
      expect(types).toContain(needed);
    }
  });

  it("never subscribes to the V1 events that V2 removed", async () => {
    const { host } = await env.start();

    expect(host.subscribeCalls).toHaveLength(1);
    const types = host.subscribeCalls[0].types ?? [];
    for (const removed of types) {
      expect(removed).not.toMatch(/^message\./);
      expect(["session.updated", "session.idle", "session.status"]).not.toContain(removed);
    }
  });

  it("never reads ctx.client, ctx.$ or ctx.directory, which V2 does not provide, even inside a guard", async () => {
    const reads: string[] = [];
    const { host, dir } = await env.start({
      extraCtx: (ctx) => {
        for (const name of ["client", "$", "directory"]) {
          Object.defineProperty(ctx, name, {
            get() {
              reads.push(name);
              return undefined;
            },
            enumerable: true,
          });
        }
      },
    });
    expect(reads).toEqual([]);

    await host.emit(userInbox(dir, ROOT, "hello"));
    await host.fireBefore(beforePayload(ROOT, "call_1", { agent: "A", prompt: "p" }));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    expect(reads).toEqual([]);
  });
});

describe("setup timing and survival", () => {
  it("returns promptly while the subscription's first pull never resolves and a manifest exists", async () => {
    const dir = env.workspace();
    nodeFs.mkdirSync(nodePath.join(dir, ".mosaic"));
    nodeFs.writeFileSync(
      nodePath.join(dir, ".mosaic", "manifest.yaml"),
      "- ref:\n    kind: hook\n    key: mosaic-logger\n  version: 1.12.0\n",
    );

    const started = Date.now();
    const handle = await Promise.race([
      env.start({ dir }),
      sleep(1500).then(() => "hung" as const),
    ]);

    expect(handle).not.toBe("hung");
    expect(Date.now() - started).toBeLessThan(1500);
    if (handle !== "hung") expect(handle.host.subscribeCalls).toHaveLength(1);
  });

  it("returns a cleanup function even when ctx is missing", async () => {
    const plugin = pluginModule.default;

    const cleanupNull = await plugin.setup(null as never);
    const cleanupUndefined = await plugin.setup(undefined as never);

    expect(typeof cleanupNull).toBe("function");
    expect(typeof cleanupUndefined).toBe("function");
    expect(() => cleanupNull()).not.toThrow();
  });

  it.each([
    ["undefined", undefined],
    ["empty", ""],
    ["a number", 42],
    ["an object", {}],
  ])("registers nothing when ctx.location.directory is %s", async (_label, directory) => {
    const host = (await env.start({ extraCtx: (ctx) => void (ctx.location = { directory }) })).host;

    expect(host.beforeHooks).toHaveLength(0);
    expect(host.afterHooks).toHaveLength(0);
    expect(host.subscribeCalls).toHaveLength(0);
  });

  it("proceeds with the subscription when the tool domain is missing", async () => {
    const { host } = await env.start({ omit: ["tool"] });

    expect(host.subscribeCalls).toHaveLength(1);
  });

  it("proceeds with the tool hooks when the event domain is missing", async () => {
    const { host } = await env.start({ omit: ["event"] });

    expect(host.beforeHooks).toHaveLength(1);
    expect(host.afterHooks).toHaveLength(1);
  });

  it.each(["throw", "reject"] as const)(
    "still subscribes when a tool hook registration fails by %s",
    async (how) => {
      const { host } = await env.start({ toolHookFails: how });

      expect(host.subscribeCalls).toHaveLength(1);
    },
  );

  it("still registers the tool hooks when the subscription throws", async () => {
    const { host } = await env.start({ subscribeThrows: true });

    expect(host.beforeHooks).toHaveLength(1);
    expect(host.afterHooks).toHaveLength(1);
  });
});

describe("cleanup", () => {
  it("aborts the subscription's signal synchronously and a second call does nothing", async () => {
    const { host, cleanup } = await env.start();
    const signal = host.subscribeCalls[0]?.signal;

    const first = cleanup();
    const abortedAfterFirst = signal?.aborted;

    expect(first).toBeUndefined();
    expect(abortedAfterFirst).toBe(true);
    expect(() => cleanup()).not.toThrow();
  });

  it("does not subscribe again after cleanup, even once the iterator has ended", async () => {
    const { host, cleanup } = await env.start();
    expect(host.subscribeCalls).toHaveLength(1);

    cleanup();
    host.endStream();
    await sleep(1300);

    expect(host.subscribeCalls).toHaveLength(1);
  });
});

describe("subscription loop", () => {
  it("subscribes again no sooner than a second after the iterator ends", async () => {
    const { host } = await env.start();
    expect(host.subscribeCalls).toHaveLength(1);

    host.endStream();
    await sleep(800);
    const countAfterShortWait = host.subscribeCalls.length;
    await eventually(() => {
      expect(host.subscribeCalls).toHaveLength(2);
    }, 2500);

    expect(countAfterShortWait).toBe(1);
  }, 8000);

  it("subscribes again after the iterator throws, and the new subscription logs events", async () => {
    const { host, dir } = await env.start();

    host.failStream(new Error("transport lost"));
    await eventually(() => {
      expect(host.subscribeCalls).toHaveLength(2);
    }, 3500);
    await host.emit(userInbox(dir, ROOT, "after reconnect"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  }, 8000);

  it("processes events in delivery order, writing each event's records before the next is pulled", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "first"));
    const afterFirst = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start").length;
    await host.emit(execution(ROOT, "succeeded"));
    const afterSecond = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end").length;

    expect(afterFirst).toBe(1);
    expect(afterSecond).toBe(1);
  });
});

describe("console output", () => {
  it("writes nothing to stdout, stderr or the console during a full session", async () => {
    const { host, dir } = await env.start();
    const writes: string[] = [];
    const consoleSpies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => vi.spyOn(console, m));
    const stdout = process.stdout.write.bind(process.stdout);
    const stderr = process.stderr.write.bind(process.stderr);
    const capture = (chunk: unknown): boolean => {
      writes.push(String(chunk));
      return true;
    };
    process.stdout.write = capture as never;
    process.stderr.write = capture as never;
    try {
      await host.emit(userInbox(dir, ROOT, "hello"));
      await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input: { agent: "A", prompt: `agent_instance_id: A#1\nrun_id: ${RUN_A}` } });
      await host.fireBefore(null);
      await host.emit({ type: "session.created", data: null as never });
      await host.emit(execution(ROOT, "succeeded"));
      await eventually(() => {
        expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
      });
    } finally {
      process.stdout.write = stdout;
      process.stderr.write = stderr;
    }

    const consoleCalls = consoleSpies.reduce((n, spy) => n + spy.mock.calls.length, 0);
    consoleSpies.forEach((spy) => spy.mockRestore());

    expect(writes).toEqual([]);
    expect(consoleCalls).toBe(0);
  });
});
