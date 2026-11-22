/**
 * Exit safety in Runner mode: the standalone private server exits when its stdin
 * lease closes, with no chance for pending asynchronous writes to finish. A
 * Node child process plays the server: it hands the plugin the final step of a
 * session and exits one macrotask later. Every record of that step must already
 * be on disk, and nothing may be written to stdout or stderr.
 */

import { spawnSync } from "node:child_process";
import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { RUN_A } from "./events";
import { byEvent, invocationFolders, invocationStream, leftoverFiles, orchestratorStream, readText, LOGS } from "./log_reader";
import { useTestEnv } from "./harness";

const env = useTestEnv();
const HERE = nodePath.dirname(fileURLToPath(import.meta.url));
const PACKAGE_DIR = nodePath.resolve(HERE, "..");
const CHILD_SCRIPT = nodePath.join(HERE, "fixtures", "runner_exit_child.ts");
const VITE_NODE = nodePath.join(PACKAGE_DIR, "node_modules", "vite-node", "vite-node.mjs");
const SESSION = "ses_exit0000000000000000001";

function runChild(dir: string, role: "orchestrator" | "subagent") {
  const childEnv: NodeJS.ProcessEnv = { ...process.env, MOSAIC_ROLE: role, MOSAIC_RUN_ID: RUN_A };
  delete childEnv.MOSAIC_AGENT_INSTANCE_ID;
  if (role === "subagent") childEnv.MOSAIC_AGENT_INSTANCE_ID = "Worker#7";
  return spawnSync(process.execPath, [VITE_NODE, CHILD_SCRIPT, dir], { cwd: PACKAGE_DIR, env: childEnv, encoding: "utf-8", timeout: 80000 });
}

describe("a process that exits right after the final step", () => {
  it("leaves the orchestrator's last records on disk, ending with session_end", () => {
    const dir = env.workspace();

    const result = runChild(dir, "orchestrator");

    expect(result.status).toBe(0);
    const stream = orchestratorStream(dir, RUN_A);
    expect(stream.map((r) => r.event)).toEqual(["session_start", "turn", "turn", "usage_record", "session_end"]);
    expect(stream[2]).toMatchObject({ role: "assistant", content: '{"status_code":"SUCCESS"}' });
    expect(stream[3]).toMatchObject({
      record_id: "msg_exit_1",
      token_usage: { input_tokens: 120, output_tokens: 40, reasoning_tokens: 10, cache_read_tokens: 5, cache_creation_tokens: 2 },
    });
    expect(stream[4]).toMatchObject({ session_id: SESSION, reason: "session.step.ended" });
    expect(byEvent(stream, "run_start")).toEqual([]);
    expect(byEvent(stream, "run_end")).toEqual([]);
  }, 90000);

  it("leaves the subagent's invocation end and output file on disk", () => {
    const dir = env.workspace();

    const result = runChild(dir, "subagent");

    expect(result.status).toBe(0);
    const [folder] = invocationFolders(dir, RUN_A);
    expect(folder).toBeDefined();
    const records = invocationStream(dir, RUN_A, folder);
    expect(records.map((r) => r.event)).toEqual(["invocation_start", "turn", "turn", "usage_record", "invocation_end"]);
    expect(records[0]).toMatchObject({ agent_instance_id: "Worker#7", agent_type: "Worker", prompt: "Please do the exit-critical work." });
    expect(records[3]).toMatchObject({ source: "agent_transcript", agent_instance_id: "Worker#7" });
    expect(records[4]).toMatchObject({ agent_instance_id: "Worker#7", response: '{"status_code":"SUCCESS"}', status_code: "SUCCESS" });
    const output = readText(nodePath.join(dir, LOGS, RUN_A, folder, "02_output.md"));
    expect(output).toContain("SUCCESS");
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_A, "00_orchestrator_events.jsonl"))).toBe(false);
  }, 90000);

  it.each(["orchestrator", "subagent"] as const)("writes nothing to stdout or stderr and leaves no lock or temp file as %s", (role) => {
    const dir = env.workspace();

    const result = runChild(dir, role);

    expect(result.status).toBe(0);
    const events = role === "orchestrator" ? orchestratorStream(dir, RUN_A) : invocationStream(dir, RUN_A, invocationFolders(dir, RUN_A)[0]);
    expect(events.map((r) => r.event)).toContain(role === "orchestrator" ? "session_end" : "invocation_end");
    expect(result.stdout).toBe("");
    expect(result.stderr).toBe("");
    expect(leftoverFiles(dir)).toEqual([]);
  }, 90000);
});
