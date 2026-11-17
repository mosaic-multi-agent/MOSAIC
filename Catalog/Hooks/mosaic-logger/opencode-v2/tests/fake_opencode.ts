/**
 * fake_opencode.ts - puts a stand-in `opencode` command first on PATH so the
 * plugin's transcript exports run against fixture documents. Restores the
 * environment after every test.
 */

import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";
import * as nodePath from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach } from "vitest";

const SCRIPT = nodePath.join(nodePath.dirname(fileURLToPath(import.meta.url)), "fixtures", "fake_opencode.mjs");
const VARS = ["PATH", "FAKE_OPENCODE_DIR", "FAKE_OPENCODE_LOG", "FAKE_OPENCODE_DELAY", "FAKE_OPENCODE_KEEP"];

export interface FakeCall {
  phase: "start" | "end";
  at: number;
  args: string[];
  cwd: string;
  pwd: string | null;
  mosaicRole: string | null;
  mosaicRunId: string | null;
  mosaicInstance: string | null;
  keepMarker: string | null;
  stdinIsTTY: boolean;
}

export interface FakeOpencode {
  /** Serve content verbatim as the export of sessionID; exitCode makes the command fail after printing. */
  serve(sessionID: string, content: string | Buffer, exitCode?: number): void;
  /** Delay every answer by ms. */
  delay(ms: number): void;
  /** Calls that reached the command, in start order. */
  starts(): FakeCall[];
  /** Calls that finished writing their answer. */
  ends(): FakeCall[];
  /** Replace PATH with a folder holding no opencode command. */
  removeFromPath(): void;
}

export function useFakeOpencode(): FakeOpencode {
  const saved: Record<string, string | undefined> = {};
  const created: string[] = [];
  let root = "";

  const calls = (): FakeCall[] => {
    const file = nodePath.join(root, "calls.jsonl");
    if (!nodeFs.existsSync(file)) return [];
    return nodeFs
      .readFileSync(file, "utf-8")
      .split("\n")
      .filter((line) => line.trim() !== "")
      .map((line) => JSON.parse(line) as FakeCall);
  };

  beforeEach(() => {
    for (const name of VARS) saved[name] = process.env[name];
    root = nodeFs.realpathSync(nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-v2-bin-")));
    created.push(root);
    const bin = nodePath.join(root, "bin");
    nodeFs.mkdirSync(bin);
    nodeFs.mkdirSync(nodePath.join(root, "exports"));
    if (process.platform === "win32") {
      nodeFs.writeFileSync(nodePath.join(bin, "opencode.cmd"), `@echo off\r\n"${process.execPath}" "${SCRIPT}" %*\r\n`);
    } else {
      const shim = nodePath.join(bin, "opencode");
      nodeFs.writeFileSync(shim, `#!/bin/sh\nexec "${process.execPath}" "${SCRIPT}" "$@"\n`);
      nodeFs.chmodSync(shim, 0o755);
    }
    process.env.PATH = `${bin}${nodePath.delimiter}${saved.PATH ?? ""}`;
    process.env.FAKE_OPENCODE_DIR = nodePath.join(root, "exports");
    process.env.FAKE_OPENCODE_LOG = nodePath.join(root, "calls.jsonl");
    process.env.FAKE_OPENCODE_KEEP = "kept-through-export";
    delete process.env.FAKE_OPENCODE_DELAY;
  });

  afterEach(() => {
    for (const name of VARS) {
      if (saved[name] === undefined) delete process.env[name];
      else process.env[name] = saved[name];
    }
    for (const dir of created.splice(0)) nodeFs.rmSync(dir, { recursive: true, force: true });
  });

  return {
    serve(sessionID, content, exitCode) {
      nodeFs.writeFileSync(nodePath.join(root, "exports", `${sessionID}.json`), content);
      if (exitCode !== undefined) nodeFs.writeFileSync(nodePath.join(root, "exports", `${sessionID}.exit`), String(exitCode));
    },
    delay(ms) {
      process.env.FAKE_OPENCODE_DELAY = String(ms);
    },
    starts: () => calls().filter((c) => c.phase === "start"),
    ends: () => calls().filter((c) => c.phase === "end"),
    removeFromPath() {
      const empty = nodePath.join(root, "empty");
      nodeFs.mkdirSync(empty, { recursive: true });
      process.env.PATH = empty;
    },
  };
}
