/**
 * adapter_version on run_start comes from the workspace manifest's single
 * mosaic-logger hook entry; anything ambiguous or unusable omits the field.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import { ROOT, RUN_A, beforePayload, missionPrompt, userInbox } from "./events";
import { byEvent, orchestratorStream } from "./log_reader";
import { eventually, sleep, useTestEnv } from "./harness";

const env = useTestEnv();

function entry(key: string, version: string): string {
  return [
    "- ref:",
    "    kind: hook",
    `    key: ${key}`,
    `  version: ${version}`,
    "  installed_at: 2026-11-01T10:00:00Z",
    "  files:",
    "    - path: .opencode/plugins/mosaic-logger.ts",
    "      checksum: abc123",
    "",
  ].join("\n");
}

function manifest(...entries: string[]): string {
  return ["schema: 1", "entries:", ...entries].join("\n");
}

/** Start with the given manifest text, bind a run, and return the run_start record. */
async function runStartFor(manifestText: string | undefined) {
  const dir = env.workspace();
  if (manifestText !== undefined) {
    nodeFs.mkdirSync(nodePath.join(dir, ".mosaic"));
    nodeFs.writeFileSync(nodePath.join(dir, ".mosaic", "manifest.yaml"), manifestText);
  }
  const { host } = await env.start({ dir });
  await host.emit(userInbox(dir, ROOT, "start"));
  await sleep(300);
  await host.fireBefore(beforePayload(ROOT, "call_1", { agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) }));

  let start: Record<string, unknown> | undefined;
  await eventually(() => {
    start = byEvent(orchestratorStream(dir, RUN_A), "run_start")[0];
    expect(start).toBeDefined();
  });
  return start as Record<string, unknown>;
}

describe("adapter_version from the manifest", () => {
  it("reads the version from a manifest in the deployed shape, among other entries", async () => {
    const start = await runStartFor(manifest(entry("other-hook", "3.0.0"), entry("mosaic-logger", "1.12.0")));

    expect(start.adapter_version).toBe("1.12.0");
  });

  it.each([
    ["two mosaic-logger hook entries", manifest(entry("mosaic-logger", "1.12.0"), entry("mosaic-logger", "1.13.0"))],
    ["an empty version", manifest(entry("mosaic-logger", ""))],
    ["a bracketed version", manifest(entry("mosaic-logger", "[1.12.0]"))],
    ["no mosaic-logger entry", manifest(entry("other-hook", "3.0.0"))],
    ["an unparseable file", "\u0000\u0001 not a manifest {{{"],
  ])("omits adapter_version for %s", async (_label, text) => {
    const start = await runStartFor(text);

    expect("adapter_version" in start).toBe(false);
  });
});
