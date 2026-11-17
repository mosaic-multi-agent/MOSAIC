/**
 * The opencode-v2 variant of hook.yaml ships the plugin entry as the one
 * top-level plugin file and every lib module as a helper below lib/.
 *
 * The deployer copies only the files listed under the variant, and V2 loads
 * only top-level files of .opencode/plugins/, so a missing or misplaced entry
 * makes the deployed plugin fail to load.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

const ADAPTER_DIR = nodePath.resolve(__dirname, "..");
const MANIFEST = nodePath.resolve(ADAPTER_DIR, "..", "hook.yaml");

interface FileEntry {
  source: string;
  target: string;
}

interface Variant {
  found: boolean;
  supported: string;
  files: FileEntry[];
  registrationLine: string;
}

const unquote = (s: string): string => s.replace(/^["']|["']$/g, "");

/** Reads the named variant's block from hook.yaml with line patterns, as the V1 manifest test does. */
function readVariant(name: string): Variant {
  const lines = nodeFs.readFileSync(MANIFEST, "utf-8").split(/\r?\n/);
  const out: Variant = { found: false, supported: "", files: [], registrationLine: "" };
  let inVariants = false;
  let inVariant = false;
  let pendingSource: string | undefined;
  for (const line of lines) {
    if (line.startsWith("variants:")) {
      inVariants = true;
      continue;
    }
    if (!inVariants) continue;
    const header = /^ {2}([a-z][a-z0-9-]*):\s*$/.exec(line);
    if (header) {
      inVariant = header[1] === name;
      out.found = out.found || inVariant;
      continue;
    }
    if (!inVariant) continue;
    const supported = /^ {4}supported:\s*(\S+)\s*$/.exec(line);
    if (supported) out.supported = supported[1];
    const registration = /^ {4}registration:\s*(.*)$/.exec(line);
    if (registration) out.registrationLine = registration[1].trim();
    const source = /^\s+- source:\s*(\S+)\s*$/.exec(line);
    if (source) pendingSource = unquote(source[1]);
    const target = /^\s+target:\s*(\S+)\s*$/.exec(line);
    if (target && pendingSource !== undefined) {
      out.files.push({ source: pendingSource, target: unquote(target[1]) });
      pendingSource = undefined;
    }
  }
  return out;
}

describe("opencode-v2 variant in hook.yaml", () => {
  const variant = readVariant("opencode-v2");

  it("exists and is supported", () => {
    expect(variant.found).toBe(true);
    expect(variant.supported).toBe("true");
  });

  it("registers nothing: V2 plugins activate by presence", () => {
    expect(variant.registrationLine).toBe("[]");
  });

  it("lists the plugin entry first, as the top-level mosaic-logger.ts", () => {
    expect(variant.files[0]).toEqual({ source: "plugin.ts", target: "mosaic-logger.ts" });
  });

  it("lists every lib module exactly once, under the same name below lib/", () => {
    const modules = nodeFs
      .readdirSync(nodePath.join(ADAPTER_DIR, "lib"))
      .filter((name) => name.endsWith(".ts"))
      .sort();

    const helpers = variant.files.slice(1);

    expect(helpers.map((f) => f.source).sort()).toEqual(modules.map((m) => `lib/${m}`));
    for (const f of helpers) expect(f.target).toBe(f.source);
  });

  it("lists no other file: no tests, fixtures, package files, notes or dependencies", () => {
    const others = variant.files.slice(1).filter((f) => !/^lib\/[^/]+\.ts$/.test(f.source));

    expect(others).toEqual([]);
  });

  it("places exactly one file at the top level and every helper one level down in lib/", () => {
    const topLevel = variant.files.filter((f) => !f.target.includes("/"));
    const nested = variant.files.filter((f) => f.target.includes("/"));

    expect(topLevel).toHaveLength(1);
    expect(nested.every((f) => /^lib\/[^/]+\.ts$/.test(f.target))).toBe(true);
  });

  it("lists only sources that resolve to existing files", () => {
    const missing = variant.files.filter((f) => !nodeFs.existsSync(nodePath.resolve(ADAPTER_DIR, f.source)));

    expect(missing).toEqual([]);
  });
});

describe("opencode (V1) variant in hook.yaml", () => {
  const variant = readVariant("opencode");

  it("is still supported and ships its plugin entry and export module", () => {
    expect(variant.found).toBe(true);
    expect(variant.supported).toBe("true");
    expect(variant.files[0]).toEqual({ source: "plugin.ts", target: "mosaic-logger.ts" });
    expect(variant.files).toContainEqual({ source: "lib/export.ts", target: "lib/export.ts" });
  });
});

describe("bundle description", () => {
  it("names OpenCode V2 and its need for the opencode command", () => {
    const text = nodeFs.readFileSync(MANIFEST, "utf-8");
    const description = /^description: >-\r?\n((?: {2}.*\r?\n)+)/m.exec(text)?.[1] ?? "";
    const flat = description.replace(/\s+/g, " ");

    expect(flat).toContain("OpenCode V2");
    expect(flat).toContain("opencode command on the PATH");
  });
});
