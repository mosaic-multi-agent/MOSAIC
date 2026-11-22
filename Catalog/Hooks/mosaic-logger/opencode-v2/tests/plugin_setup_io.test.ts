/**
 * setup never waits for file I/O: with every asynchronous read of the workspace
 * manifest made to hang forever, setup must still resolve promptly with the
 * hooks and the subscription registered.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it, vi } from "vitest";

import { sleep, useTestEnv } from "./harness";

const slow = vi.hoisted(() => ({
  manifestReads: 0,
  isManifest: (target: unknown): boolean => /manifest\.yaml$/.test(String(target)),
}));

vi.mock("node:fs/promises", async (importOriginal) => {
  const actual = await importOriginal<typeof import("node:fs/promises")>();
  const readFile = (...args: unknown[]): Promise<unknown> => {
    if (slow.isManifest(args[0])) {
      slow.manifestReads += 1;
      return new Promise(() => {});
    }
    return (actual.readFile as (...a: unknown[]) => Promise<unknown>)(...args);
  };
  return { ...actual, readFile, default: { ...actual, readFile } };
});

vi.mock("node:fs", async (importOriginal) => {
  const actual = await importOriginal<typeof import("node:fs")>();
  const readFile = (...args: unknown[]): Promise<unknown> => {
    if (slow.isManifest(args[0])) {
      slow.manifestReads += 1;
      return new Promise(() => {});
    }
    return (actual.promises.readFile as (...a: unknown[]) => Promise<unknown>)(...args);
  };
  const promises = { ...actual.promises, readFile };
  return { ...actual, promises, default: { ...actual, promises } };
});

const env = useTestEnv();

describe("setup with a manifest read that never completes", () => {
  it("resolves promptly with both tool hooks and the subscription registered", async () => {
    const dir = env.workspace();
    nodeFs.mkdirSync(nodePath.join(dir, ".mosaic"));
    nodeFs.writeFileSync(
      nodePath.join(dir, ".mosaic", "manifest.yaml"),
      "- ref:\n    kind: hook\n    key: mosaic-logger\n  version: 1.12.0\n",
    );

    const started = Date.now();
    const handle = await Promise.race([env.start({ dir }), sleep(1000).then(() => "hung" as const)]);

    expect(handle).not.toBe("hung");
    expect(Date.now() - started).toBeLessThan(1000);
    if (handle === "hung") return;
    expect(handle.host.beforeHooks).toHaveLength(1);
    expect(handle.host.afterHooks).toHaveLength(1);
    expect(handle.host.subscribeCalls).toHaveLength(1);
  });
});
