/**
 * harness.ts - shared setup for the plugin tests: temp workspaces, a clean
 * MOSAIC_* environment, plugin start-up against a fake host, and polling.
 */

import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";
import * as nodePath from "node:path";
import { afterEach, beforeEach } from "vitest";

import plugin from "../plugin";
import { FakeHost, type FakeHostOptions } from "./fake_host";

const ROLE_VARS = ["MOSAIC_ROLE", "MOSAIC_RUN_ID", "MOSAIC_AGENT_INSTANCE_ID"];

export interface Running {
  host: FakeHost;
  dir: string;
  cleanup: () => void;
}

export interface TestEnv {
  /** Create a fresh workspace directory (removed after the test). */
  workspace(): string;
  /** Run the plugin's setup against a fake host rooted at dir (default: new workspace). */
  start(options?: FakeHostOptions & { dir?: string }): Promise<Running>;
}

/** Registers hooks: clean MOSAIC_* env before each test, workspaces and plugins removed after. */
export function useTestEnv(): TestEnv {
  const created: string[] = [];
  const running: Running[] = [];
  const saved: Record<string, string | undefined> = {};

  beforeEach(() => {
    for (const name of ROLE_VARS) {
      saved[name] = process.env[name];
      delete process.env[name];
    }
  });

  afterEach(() => {
    for (const r of running.splice(0)) {
      try {
        r.cleanup();
      } catch {
        // cleanup failures are asserted by the tests that care
      }
    }
    for (const name of ROLE_VARS) {
      if (saved[name] === undefined) delete process.env[name];
      else process.env[name] = saved[name];
    }
    for (const dir of created.splice(0)) {
      nodeFs.rmSync(dir, { recursive: true, force: true });
    }
  });

  const workspace = (): string => {
    const dir = nodeFs.realpathSync(nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-v2-")));
    created.push(dir);
    return dir;
  };

  const start = async (options: FakeHostOptions & { dir?: string } = {}): Promise<Running> => {
    const dir = options.dir ?? workspace();
    const host = new FakeHost(dir, options);
    const cleanup = await plugin.setup(host.ctx);
    const handle = { host, dir, cleanup };
    running.push(handle);
    return handle;
  };

  return { workspace, start };
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Retry an assertion until it passes; rethrows the last failure after the timeout. */
export async function eventually(assertion: () => void | Promise<void>, timeoutMs = 3000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    try {
      await assertion();
      return;
    } catch (err) {
      if (Date.now() >= deadline) throw err;
      await sleep(15);
    }
  }
}

/**
 * Give asynchronous writes (lock protocol with retries, slow disks) time to land
 * before asserting that something was NOT written.
 */
export function quiet(ms = 400): Promise<void> {
  return sleep(ms);
}
