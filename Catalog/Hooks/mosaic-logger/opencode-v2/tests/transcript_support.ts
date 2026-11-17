/**
 * transcript_support.ts - shared drivers and readers for the transcript tests.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { fileURLToPath } from "node:url";
import { expect } from "vitest";

import { RUN_A, dispatchChild, missionPrompt, userInbox, wakeUp, wrapWakeUp } from "./events";
import type { FakeHost } from "./fake_host";
import { LOGS } from "./log_reader";
import { eventually, quiet } from "./harness";

const FIXTURES = nodePath.join(nodePath.dirname(fileURLToPath(import.meta.url)), "fixtures");

/** The spike's parent export (info.id PARENT) and its child's export (info.id CHILD). */
export const PARENT = "ses_e161e29afffe8w3w7eyl30w0bx";
export const CHILD = "ses_e161e12d7ffeiEFVI4ptakFWQk";
export const PARENT_EXPORT = nodeFs.readFileSync(nodePath.join(FIXTURES, "t2-parent-export.json"));
export const CHILD_EXPORT = nodeFs.readFileSync(nodePath.join(FIXTURES, "t2-child-export.json"));
export const OK = '{"status_code":"SUCCESS","status_message":"done"}';

export const SIDECAR_KEYS = ["harness", "native_format", "source_mechanism", "captured_at", "byte_count"];

export function runDir(dir: string, run: string = RUN_A): string {
  return nodePath.join(dir, LOGS, run);
}

export function readBytes(file: string): Buffer | undefined {
  return nodeFs.existsSync(file) ? nodeFs.readFileSync(file) : undefined;
}

export function readSidecar(file: string): Record<string, unknown> | undefined {
  const bytes = readBytes(file);
  return bytes ? (JSON.parse(bytes.toString("utf-8")) as Record<string, unknown>) : undefined;
}

/** Every transcript file (.raw or transcript .meta.json) under OrchestrationLogs, relative paths. */
export function transcriptFiles(dir: string): string[] {
  const out: string[] = [];
  const walk = (current: string): void => {
    if (!nodeFs.existsSync(current)) return;
    for (const entry of nodeFs.readdirSync(current, { withFileTypes: true })) {
      const full = nodePath.join(current, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (/\.raw$|_session.*\.meta\.json$/.test(entry.name)) {
        out.push(nodePath.relative(nodePath.join(dir, LOGS), full).split(nodePath.sep).join("/"));
      }
    }
  };
  walk(nodePath.join(dir, LOGS));
  return out.sort();
}

/** A root session that dispatches one child bound to RUN_A (invocation folder Worker#1). */
export async function startAndDispatch(host: FakeHost, dir: string, child: string = CHILD): Promise<void> {
  await host.emit(userInbox(dir, PARENT, "go"));
  await dispatchChild(host, {
    dir,
    parent: PARENT,
    child,
    callId: `call_${child}`,
    agent: "Worker",
    prompt: missionPrompt("Worker#1", RUN_A),
  });
}

/** The child's completion wake-up (not running). */
export async function complete(host: FakeHost, child: string = CHILD, body: string = OK): Promise<void> {
  await host.emit(wakeUp(PARENT, child, "Worker", "completed", wrapWakeUp(child, "completed", body)));
}

/** Wait until the fake command has been started at least n times, then let the queue drain. */
export async function settle(starts: () => unknown[], n: number): Promise<void> {
  await eventually(() => expect(starts().length).toBeGreaterThanOrEqual(n), 8000);
  await quiet(700);
}
