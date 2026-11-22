/**
 * transcript_process.ts - runs one export command as a child process.
 *
 * stdin is closed (never a TTY), stderr is discarded, and the host's stdio is
 * never passed on. The session id is one argument and never part of a shell line:
 * on Windows the command can resolve to an npm .cmd shim that only a shell starts,
 * so there an export runs only for an id made of letters, digits, `_` and `-`.
 */

import { spawn, spawnSync, type ChildProcess } from "node:child_process";

import type { ExportCommand } from "./transcript.js";

const WINDOWS = process.platform === "win32";
const SHELL_SAFE_ID = /^[A-Za-z0-9_-]+$/;
const WINDOWS_RELEASE_WAIT_MS = 150;

/** One running export: its collected stdout (undefined unless it exited 0) and a way to end it. */
export interface RunningExport {
  result: Promise<Buffer | undefined>;
  kill(): void;
}

/** Whether an export for this session id can be started safely on this platform. */
export function canExport(sessionID: string): boolean {
  return !WINDOWS || SHELL_SAFE_ID.test(sessionID);
}

/** End the child and everything it started. Never throws. */
function killTree(child: ChildProcess): void {
  try {
    const pid = child.pid;
    if (pid === undefined) return;
    if (WINDOWS) {
      spawnSync("taskkill", ["/pid", String(pid), "/T", "/F"], { stdio: "ignore", windowsHide: true });
      // Windows releases the dead process's working directory a moment later; wait
      // briefly so the folder can be removed right after cleanup.
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, WINDOWS_RELEASE_WAIT_MS);
    } else {
      process.kill(-pid, "SIGKILL");
    }
  } catch {
    try {
      child.kill("SIGKILL");
    } catch {
      // nothing left to end
    }
  }
}

function launch(command: ExportCommand): ChildProcess {
  const stdio: ["ignore", "pipe", "ignore"] = ["ignore", "pipe", "ignore"];
  if (WINDOWS) {
    const line = [command.command, ...command.args].join(" ");
    return spawn(line, { cwd: command.cwd, env: command.env, stdio, shell: true, windowsHide: true });
  }
  return spawn(command.command, command.args, { cwd: command.cwd, env: command.env, stdio, detached: true });
}

/** Start the command. The result is the stdout of a run that exited 0, else undefined. Never throws. */
export function startExport(command: ExportCommand, timeoutMs: number): RunningExport {
  let child: ChildProcess | undefined;
  let killed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const kill = (): void => {
    killed = true;
    if (child) killTree(child);
  };

  const result = new Promise<Buffer | undefined>((resolve) => {
    let settled = false;
    const finish = (value: Buffer | undefined): void => {
      if (settled) return;
      settled = true;
      if (timer !== undefined) clearTimeout(timer);
      resolve(value);
    };
    timer = setTimeout(() => {
      kill();
      finish(undefined);
    }, timeoutMs);
    (timer as { unref?: () => void }).unref?.();

    try {
      const chunks: Buffer[] = [];
      child = launch(command);
      child.stdout?.on("data", (chunk: Buffer) => chunks.push(chunk));
      child.stdout?.on("error", () => undefined);
      child.on("error", () => finish(undefined));
      child.on("close", (code) => finish(code === 0 && !killed ? Buffer.concat(chunks) : undefined));
    } catch {
      finish(undefined);
    }
  });

  return { result, kill };
}
