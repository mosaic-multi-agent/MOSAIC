/**
 * fixtures.ts - load the sanitized spike probe logs and replay them into a fake host.
 *
 * The fixtures keep the observed order of hook calls and events. The directory
 * the probes ran in is rewritten to the directory of the test's workspace.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { fileURLToPath } from "node:url";

import type { FakeHost } from "./fake_host";

const FIXTURE_DIR = nodePath.join(nodePath.dirname(fileURLToPath(import.meta.url)), "fixtures");

export const PROBE_DIR_A = "C:\\oc2-spike\\proj-a";
export const PROBE_DIR_B = "C:\\oc2-spike\\proj-b";

export interface ProbeRecord {
  kind: "event" | "hook";
  type?: string;
  hook?: string;
  location?: { directory?: string };
  data?: unknown;
  e?: unknown;
}

function rewrite(value: unknown, mapping: Record<string, string>): unknown {
  if (typeof value === "string") return mapping[value] ?? value;
  if (Array.isArray(value)) return value.map((item) => rewrite(item, mapping));
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) out[key] = rewrite(item, mapping);
    return out;
  }
  return value;
}

export function loadFixture(name: string, mapping: Record<string, string>): ProbeRecord[] {
  const text = nodeFs.readFileSync(nodePath.join(FIXTURE_DIR, name), "utf-8");
  return text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => rewrite(JSON.parse(line), mapping) as ProbeRecord);
}

/** Feed the records to the host in file order: hooks are fired, events are pushed and awaited. */
export async function replay(host: FakeHost, records: ProbeRecord[]): Promise<void> {
  for (const rec of records) {
    if (rec.kind === "hook" && rec.hook === "tool.execute.before") await host.fireBefore(rec.e);
    else if (rec.kind === "hook" && rec.hook === "tool.execute.after") await host.fireAfter(rec.e);
    else if (rec.kind === "event") {
      await host.emit({ type: rec.type, location: rec.location, data: rec.data as Record<string, unknown> });
    }
  }
}

/** Session ids that appear in the A fixture. */
export const PROBE_ROOT = "ses_e161e29afffe8w3w7eyl30w0bx";
export const PROBE_ALPHA = "ses_e161e12dcffeg9u6dVmLqJaYzh";
export const PROBE_BETA = "ses_e161e12d7ffeiEFVI4ptakFWQk";
export const PROBE_CALL_ALPHA = "call_b40ad85a89d94f7280608683257e4aab";
export const PROBE_CALL_BETA = "call_ae5cf6c21a6144329e311c99d93bdc56";
/** The session the B fixture's own project ran, and the one it only overheard from project A. */
export const PROBE_B_OWN = "ses_e1624e680ffeGrfuwLxvgZj7M0";
export const PROBE_B_FOREIGN = "ses_e1624e717ffeJPhoOQrW3Rtxgl";
