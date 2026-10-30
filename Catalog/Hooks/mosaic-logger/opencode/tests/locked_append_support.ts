/**
 * Helpers for the locked-append tests: temp folders, very long lines, lock files held by
 * other writers, child-process writers and a Python interpreter probe.
 *
 * A line carries 100000 characters, more than the OS write buffer, so an unlocked append
 * from several writers is able to interleave.
 */

import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { fileURLToPath } from "node:url";

export const PAD = 100_000;

const HERE = nodePath.dirname(fileURLToPath(import.meta.url));
const CHILD = nodePath.join(HERE, "fixtures", "locked_append_child.mjs");
export const PY_WRITER = nodePath.join(HERE, "fixtures", "locked_append_py_writer.py");
export const PY_HOLDER = nodePath.join(HERE, "fixtures", "locked_append_py_holder.py");
/** Folder holding the Python locked-append module the OpenCode writer must interoperate with. */
export const PY_MODULE_DIR = nodePath.resolve(HERE, "..", "..", "copilot-shared");

export function makeTempDir(): string {
  return nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "locked-append-"));
}

/** One JSON line (newline included) with a long pad. */
export function longLine(tag: string, seq: number, pad: number = PAD): string {
  return JSON.stringify({ event: "probe", writer: tag, seq, pad: "p".repeat(pad) }) + "\n";
}

export interface LogReading {
  lines: Array<Record<string, unknown>>;
  /** Unparseable or unterminated pieces. */
  torn: number;
  /** Empty lines. */
  blank: number;
}

export function readLog(file: string): LogReading {
  const text = nodeFs.existsSync(file) ? nodeFs.readFileSync(file, "utf-8") : "";
  const reading: LogReading = { lines: [], torn: 0, blank: 0 };
  if (text === "") return reading;
  const pieces = text.split("\n");
  const last = pieces.pop() as string;
  if (last !== "") reading.torn++;
  for (const piece of pieces) {
    if (piece === "") {
      reading.blank++;
      continue;
    }
    try {
      reading.lines.push(JSON.parse(piece) as Record<string, unknown>);
    } catch {
      reading.torn++;
    }
  }
  return reading;
}

/** Sequence numbers written by one writer tag, in file order. */
export function seqsOf(reading: LogReading, tag: string): number[] {
  return reading.lines.filter((l) => l.writer === tag).map((l) => l.seq as number);
}

export function range(count: number): number[] {
  return Array.from({ length: count }, (_, i) => i);
}

/** Create the lock file as another live writer would have; ageMs back-dates its mtime. */
export function holdLock(lock: string, ageMs: number = 0): void {
  nodeFs.mkdirSync(nodePath.dirname(lock), { recursive: true });
  nodeFs.writeFileSync(lock, "");
  if (ageMs > 0) {
    const stamp = (Date.now() - ageMs) / 1000;
    nodeFs.utimesSync(lock, stamp, stamp);
  }
}

/** Delete path after ms from a separate process, so it also runs while this process is blocked. */
export function releaseAfter(path: string, ms: number): ChildProcess {
  const code =
    "setTimeout(() => { try { require('fs').unlinkSync(process.argv[1]); } catch {} }, " +
    ms +
    ");";
  const child = spawn(process.execPath, ["-e", code, path], { stdio: "ignore" });
  child.unref();
  return child;
}

export interface ChildResult {
  code: number | null;
  stderr: string;
}

function collect(child: ChildProcess): Promise<ChildResult> {
  return new Promise((resolve) => {
    let stderr = "";
    child.stderr?.on("data", (chunk) => (stderr += String(chunk)));
    child.on("close", (code) => resolve({ code, stderr }));
  });
}

/** Run the module-under-test writer in a separate node process. */
export function runNodeWriter(
  mode: "sync" | "async",
  target: string,
  tag: string,
  count: number,
  pad: number = PAD,
): Promise<ChildResult> {
  const args = [
    "--experimental-strip-types",
    "--no-warnings",
    CHILD,
    mode,
    target,
    tag,
    String(count),
    String(pad),
  ];
  return collect(spawn(process.execPath, args, { stdio: ["ignore", "ignore", "pipe"] }));
}

/** Python command for this platform, or null when no working interpreter is present. */
export function findPython(): string | null {
  const command = process.platform === "win32" ? "py" : "python3";
  try {
    const probe = spawnSync(command, ["-c", "import sys; sys.exit(0)"], { stdio: "ignore" });
    return probe.status === 0 ? command : null;
  } catch {
    return null;
  }
}

/** Run the Python append writer fixture against target. */
export function runPythonWriter(
  python: string,
  target: string,
  tag: string,
  count: number,
  pad: number = PAD,
): Promise<ChildResult> {
  const args = [PY_WRITER, PY_MODULE_DIR, target, tag, String(count), String(pad)];
  return collect(spawn(python, args, { stdio: ["ignore", "ignore", "pipe"] }));
}

export interface Holder {
  child: ChildProcess;
  done: Promise<ChildResult>;
}

/** Start the Python lock holder; resolves once it printed "held". */
export function startPythonHolder(
  python: string,
  target: string,
  holdSeconds: number,
): Promise<Holder> {
  const child = spawn(python, [PY_HOLDER, PY_MODULE_DIR, target, String(holdSeconds)], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  const done = collect(child);
  return new Promise((resolve, reject) => {
    let out = "";
    child.stdout?.on("data", (chunk) => {
      out += String(chunk);
      if (out.includes("held")) resolve({ child, done });
    });
    child.on("close", () => reject(new Error("python holder ended before holding the lock")));
  });
}

/** The lock file the log format names for target (<dir>/.<name>.lock), independent of the module under test. */
export function lockFileOf(target: string): string {
  return nodePath.join(nodePath.dirname(target), "." + nodePath.basename(target) + ".lock");
}

/** The lock file the log format names for target (<dir>/.<name>.lock), independent of the module under test. */
export function lockFileOf(target: string): string {
  return nodePath.join(nodePath.dirname(target), "." + nodePath.basename(target) + ".lock");
}
