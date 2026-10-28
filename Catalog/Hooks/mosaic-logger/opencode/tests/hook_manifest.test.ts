/**
 * hook_manifest.test.ts - The opencode variant of hook.yaml ships every adapter module.
 *
 * The deployer copies only the files listed under the opencode variant, so a lib module
 * missing from that list makes the deployed plugin fail to load.
 */

import { describe, it, expect } from "vitest";
import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

const ADAPTER_DIR = nodePath.resolve(__dirname, "..");
const MANIFEST = nodePath.resolve(ADAPTER_DIR, "..", "hook.yaml");

/** Source paths listed under the opencode variant, relative to the adapter folder. */
function opencodeSources(): string[] {
  const lines = nodeFs.readFileSync(MANIFEST, "utf-8").split(/\r?\n/);
  const sources: string[] = [];
  let inVariants = false;
  let inOpencode = false;
  for (const line of lines) {
    if (line.startsWith("variants:")) {
      inVariants = true;
      continue;
    }
    if (!inVariants) continue;
    const variant = /^ {2}([a-z][a-z-]*):\s*$/.exec(line);
    if (variant) {
      inOpencode = variant[1] === "opencode";
      continue;
    }
    if (!inOpencode) continue;
    const source = /^\s+- source:\s*(\S+)\s*$/.exec(line);
    if (source) sources.push(source[1].replace(/^["']|["']$/g, ""));
  }
  return sources;
}

describe("opencode variant file list in hook.yaml", () => {
  it("lists a non-empty set of sources", () => {
    expect(opencodeSources().length).toBeGreaterThan(0);
  });

  it("lists every lib module of the adapter", () => {
    const listed = new Set(opencodeSources());
    const modules = nodeFs
      .readdirSync(nodePath.join(ADAPTER_DIR, "lib"))
      .filter((name) => name.endsWith(".ts"))
      .map((name) => `lib/${name}`);

    const missing = modules.filter((m) => !listed.has(m));

    expect(missing).toEqual([]);
  });

  it("lists only sources that resolve to existing files", () => {
    const missing = opencodeSources().filter(
      (source) => !nodeFs.existsSync(nodePath.resolve(ADAPTER_DIR, source)),
    );

    expect(missing).toEqual([]);
  });
});
