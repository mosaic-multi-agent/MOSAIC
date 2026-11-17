/**
 * Role detection: readRunnerMode resolves the Runner role from the environment,
 * and the plugin reads it once at setup. Without MOSAIC_ROLE the plugin logs
 * natively; under the Runner it never writes run_start or run_end and a
 * dispatch prompt never binds a run.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import { readRunnerMode } from "../lib/runner_mode";
import { ROOT, RUN_A, RUN_B, execution, missionPrompt, userInbox } from "./events";
import { allRecords, byEvent, LOGS, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();

describe("readRunnerMode", () => {
  it("is native (undefined) when MOSAIC_ROLE is absent", () => {
    expect(readRunnerMode({})).toBeUndefined();
  });

  it.each(["", "   ", "\t"])("is native when MOSAIC_ROLE is blank (%j)", (blank) => {
    expect(readRunnerMode({ MOSAIC_ROLE: blank })).toBeUndefined();
  });

  it("gives the subagent role with instance id and agent type from the text before the last #", () => {
    const mode = readRunnerMode({ MOSAIC_ROLE: "subagent", MOSAIC_AGENT_INSTANCE_ID: "Research#3", MOSAIC_RUN_ID: RUN_A });

    expect(mode).toEqual({ role: "subagent", agentInstanceId: "Research#3", agentType: "Research", runId: RUN_A });
  });

  it("matches the role trimmed and case-insensitively", () => {
    const mode = readRunnerMode({ MOSAIC_ROLE: "  SubAgent ", MOSAIC_AGENT_INSTANCE_ID: "Writer#1" });

    expect(mode?.role).toBe("subagent");
    expect(mode?.agentInstanceId).toBe("Writer#1");
  });

  it("takes the agent type from before the LAST #", () => {
    const mode = readRunnerMode({ MOSAIC_ROLE: "subagent", MOSAIC_AGENT_INSTANCE_ID: "Team#A#7" });

    expect(mode?.agentType).toBe("Team#A");
  });

  it.each(["#5", "NoHashHere"])("leaves agentType absent for instance id %j", (instanceId) => {
    const mode = readRunnerMode({ MOSAIC_ROLE: "subagent", MOSAIC_AGENT_INSTANCE_ID: instanceId });

    expect(mode?.role).toBe("subagent");
    expect(mode?.agentType).toBeUndefined();
  });

  it("falls back to the orchestrator role for subagent without a non-empty instance id", () => {
    expect(readRunnerMode({ MOSAIC_ROLE: "subagent" })?.role).toBe("orchestrator");
    expect(readRunnerMode({ MOSAIC_ROLE: "subagent", MOSAIC_AGENT_INSTANCE_ID: "" })?.role).toBe("orchestrator");
  });

  it.each(["orchestrator", "ORCHESTRATOR", "something-else"])("gives the orchestrator role for %j", (role) => {
    const mode = readRunnerMode({ MOSAIC_ROLE: role, MOSAIC_AGENT_INSTANCE_ID: "Research#3" });

    expect(mode?.role).toBe("orchestrator");
    expect(mode?.agentInstanceId).toBeUndefined();
  });

  it("sets runId only when MOSAIC_RUN_ID has the run id format", () => {
    expect(readRunnerMode({ MOSAIC_ROLE: "orchestrator", MOSAIC_RUN_ID: RUN_B })?.runId).toBe(RUN_B);
    expect(readRunnerMode({ MOSAIC_ROLE: "orchestrator", MOSAIC_RUN_ID: "not-a-run" })?.runId).toBeUndefined();
    expect(readRunnerMode({ MOSAIC_ROLE: "orchestrator", MOSAIC_RUN_ID: "20261117T150000Z-ABCD" })?.runId).toBeUndefined();
    expect(readRunnerMode({ MOSAIC_ROLE: "orchestrator" })?.runId).toBeUndefined();
  });

  it("reads process.env by default", () => {
    process.env.MOSAIC_ROLE = "orchestrator";
    process.env.MOSAIC_RUN_ID = RUN_A;

    expect(readRunnerMode()).toEqual({ role: "orchestrator", runId: RUN_A });
  });

  it("never throws, whatever the environment holds", () => {
    const hostile = new Proxy({}, { get: () => { throw new Error("hostile env"); } }) as Record<string, string>;

    expect(() => readRunnerMode(hostile)).not.toThrow();
    expect(() => readRunnerMode({ MOSAIC_ROLE: 5 as never })).not.toThrow();
  });
});

describe("plugin in native mode (no role environment)", () => {
  it("logs the run binding and session end with run_start and run_end", async () => {
    const { host, dir } = await env.start();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };

    await host.emit(userInbox(dir, ROOT, "start"));
    await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input });
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const records = orchestratorStream(dir, RUN_A);
      expect(byEvent(records, "run_start")).toHaveLength(1);
      expect(byEvent(records, "run_end")).toHaveLength(1);
    });
  });
});

describe("plugin under the Runner role environment", () => {
  function runnerEnv(role: string, extra: Record<string, string> = {}): void {
    process.env.MOSAIC_ROLE = role;
    process.env.MOSAIC_RUN_ID = RUN_A;
    for (const [name, value] of Object.entries(extra)) process.env[name] = value;
  }

  it.each([
    ["orchestrator", {}],
    ["subagent", { MOSAIC_AGENT_INSTANCE_ID: "Worker#1" }],
  ])("never writes run_start or run_end as the %s role", async (role, extra) => {
    runnerEnv(role, extra);
    const { host, dir } = await env.start();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#2", RUN_B) };

    await host.emit(userInbox(dir, ROOT, "start"));
    await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input });
    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    expect(host.subscribeCalls).toHaveLength(1);
    const all = allRecords(dir);
    expect(byEvent(all, "run_start")).toEqual([]);
    expect(byEvent(all, "run_end")).toEqual([]);
  });

  it("does not let a dispatch prompt bind a run: no folder is created for the prompt's run id", async () => {
    runnerEnv("orchestrator");
    const { host, dir } = await env.start();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#2", RUN_B) };

    await host.emit(userInbox(dir, ROOT, "start"));
    await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input });
    await quiet();

    expect(host.beforeHooks).toHaveLength(1);
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_B))).toBe(false);
    expect(orchestratorStream(dir, RUN_B)).toEqual([]);
  });

  it("reads the role once at setup: a later change of the environment does not switch modes", async () => {
    const { host, dir } = await env.start();
    runnerEnv("orchestrator");
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };

    await host.emit(userInbox(dir, ROOT, "start"));
    await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input });

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "run_start")).toHaveLength(1);
    });
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, UNKNOWN_RUN))).toBe(true);
  });
});
