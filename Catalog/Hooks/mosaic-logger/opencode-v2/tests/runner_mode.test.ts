/**
 * Runner mode: the process's first root session is the Runner's invocation (subagent
 * role) or consultation (orchestrator role). The plugin never writes run_start,
 * run_end or a run binding, and writes the end records once, at the first end point.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import { RUN_A, RUN_B, ROOT, dispatchChild, execution, missionPrompt, userInbox, wakeUp, wrapWakeUp } from "./events";
import {
  allRecords,
  byEvent,
  envelopeProblems,
  invocationFolders,
  invocationOf,
  invocationStream,
  orchestratorStream,
  readText,
  UNKNOWN_RUN,
  LOGS,
} from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import type { FakeHost } from "./fake_host";
import { stepEnded, stepStarted, textEnded, tokens, toolBefore, toolAfter } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";
const OTHER_ROOT = "ses_root0000000000000000002";

function asRole(role: "orchestrator" | "subagent", runId: string | null = RUN_A): void {
  process.env.MOSAIC_ROLE = role;
  if (runId) process.env.MOSAIC_RUN_ID = runId;
  if (role === "subagent") process.env.MOSAIC_AGENT_INSTANCE_ID = "Worker#7";
}

async function finalStep(host: FakeHost, dir: string, session: string, text: string, finish = "stop") {
  await host.emit(stepStarted(dir, session, "msg_final"));
  await host.emit(textEnded(dir, session, "msg_final", text));
  await host.emit(stepEnded(dir, session, "msg_final", finish, tokens(10, 5)));
}

describe("orchestrator role", () => {
  it("writes session_start to the run's stream at the first signal, and never run_start or run_end", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "consult"));
    await finalStep(host, dir, ROOT, "answer");

    const stream = orchestratorStream(dir, RUN_A);
    expect(byEvent(stream, "session_start")).toHaveLength(1);
    expect(byEvent(stream, "session_start")[0].session_id).toBe(ROOT);
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
    expect(byEvent(allRecords(dir), "run_end")).toEqual([]);
    expect(stream.flatMap(envelopeProblems)).toEqual([]);
  });

  it("uses the unknown-run folder when the run id is absent", async () => {
    asRole("orchestrator", null);
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "consult"));

    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
  });

  it("writes the stop step's turn and usage record before session_end, with the step event as the reason", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await finalStep(host, dir, ROOT, "answer");

    const tail = orchestratorStream(dir, RUN_A).slice(-3);
    expect(tail.map((r) => r.event)).toEqual(["turn", "usage_record", "session_end"]);
    expect(tail[2]).toMatchObject({ session_id: ROOT, reason: "session.step.ended" });
  });

  it("ends at a terminal execution event when no stop step came", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await host.emit(execution(ROOT, "failed"));

    expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toMatchObject([{ session_id: ROOT, reason: "session.execution.failed" }]);
  });

  it("does not end at a step that is not a stop step", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await finalStep(host, dir, ROOT, "thinking", "tool-calls");
    await quiet();

    expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toEqual([]);
  });

  it("writes the end records exactly once per process", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));
    await finalStep(host, dir, ROOT, "answer");

    await host.emit(execution(ROOT, "succeeded"));
    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toHaveLength(1);
  });

  it("writes no second end after the primary became active again", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));
    await finalStep(host, dir, ROOT, "answer");

    await host.emit(userInbox(dir, ROOT, "one more question"));
    await host.emit(execution(ROOT, "succeeded"));
    await finalStep(host, dir, ROOT, "second answer");
    await quiet();

    expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toHaveLength(1);
  });

  it("keeps the end back while a child invocation is open, and ends at the next end point after it closed", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emit(execution(ROOT, "succeeded"));
    await quiet();
    expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toEqual([]);

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", "done")));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toHaveLength(1);
    });
  });

  it("writes nothing for a second root session of the process", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await host.emit(userInbox(dir, OTHER_ROOT, "another"));
    await host.emit(execution(OTHER_ROOT, "succeeded"));
    await quiet();

    expect(JSON.stringify(allRecords(dir))).not.toContain(OTHER_ROOT);
  });

  it("logs the primary's children as invocations of the run and never binds a run from a prompt", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_B) });

    await eventually(() => {
      expect(invocationOf(dir, CHILD)).toMatchObject({ run: RUN_A, start: { agent_instance_id: "Worker#1", spawn_call_id: "call_p" } });
    });
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_B))).toBe(false);
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
  });

  it("writes the primary's tool calls into the run's orchestrator stream", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));

    await host.fireBefore(toolBefore(ROOT, "call_r", "read", {}));
    await host.fireAfter(toolAfter(ROOT, "call_r", "read", "completed", { result: "ok" }));

    expect(byEvent(orchestratorStream(dir, RUN_A), "tool_call_end").map((r) => r.call_id)).toEqual(["call_r"]);
  });
});

describe("subagent role", () => {
  const folderOf = (dir: string) => invocationFolders(dir, RUN_A)[0];

  it("writes invocation_start once with the Runner's identity and the first user prompt, and 01_input.md", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "Do the assigned work."));
    await host.emit(userInbox(dir, ROOT, "A second user message."));

    expect(invocationFolders(dir, RUN_A)).toHaveLength(1);
    const records = invocationStream(dir, RUN_A, folderOf(dir));
    const starts = byEvent(records, "invocation_start");
    expect(starts).toHaveLength(1);
    expect(starts[0]).toMatchObject({ agent_instance_id: "Worker#7", agent_type: "Worker", prompt: "Do the assigned work.", session_id: ROOT });
    expect("spawn_call_id" in starts[0]).toBe(false);
    expect(records[0].event).toBe("invocation_start");
    const input = readText(nodePath.join(dir, LOGS, RUN_A, folderOf(dir), "01_input.md"));
    expect(input).toContain("Do the assigned work.");
    expect(input).toContain("opencode-v2");
  });

  it("writes invocation_start without a prompt when another record is due first", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();

    await host.fireBefore(toolBefore(ROOT, "call_r", "read", {}));

    const records = invocationStream(dir, RUN_A, folderOf(dir));
    expect(records.map((r) => r.event).slice(0, 2)).toEqual(["invocation_start", "tool_call_start"]);
    expect("prompt" in records[0]).toBe(false);
    expect(records[0].agent_instance_id).toBe("Worker#7");
  });

  it("writes the invocation end at the stop step with the last assistant turn as the response, then 02_output.md", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));
    await host.emit(stepStarted(dir, ROOT, "msg_1"));
    await host.emit(textEnded(dir, ROOT, "msg_1", "interim"));
    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));

    await finalStep(host, dir, ROOT, '{"status_code":"SUCCESS","result":"done"}');

    const records = invocationStream(dir, RUN_A, folderOf(dir));
    expect(records.slice(-3).map((r) => r.event)).toEqual(["turn", "usage_record", "invocation_end"]);
    const end = byEvent(records, "invocation_end")[0];
    expect(end).toMatchObject({ agent_instance_id: "Worker#7", response: '{"status_code":"SUCCESS","result":"done"}', status_code: "SUCCESS" });
    expect(byEvent(records, "usage_record").map((r) => r.source)).toEqual(["agent_transcript", "agent_transcript"]);
    const output = readText(nodePath.join(dir, LOGS, RUN_A, folderOf(dir), "02_output.md"));
    expect(output).toContain("SUCCESS");
  });

  it("writes an invocation_end without a response when the primary said nothing", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));

    await host.emit(execution(ROOT, "failed"));

    const [end] = byEvent(invocationStream(dir, RUN_A, folderOf(dir)), "invocation_end");
    expect(end).toBeDefined();
    expect("response" in end).toBe(false);
  });

  it("writes the invocation end exactly once per process", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));
    await finalStep(host, dir, ROOT, "answer");

    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    expect(byEvent(invocationStream(dir, RUN_A, folderOf(dir)), "invocation_end")).toHaveLength(1);
  });

  it("writes no second invocation end and no second output file write after the primary became active again", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));
    await finalStep(host, dir, ROOT, "answer");
    const outputPath = nodePath.join(dir, LOGS, RUN_A, folderOf(dir), "02_output.md");
    const firstOutput = readText(outputPath);

    await host.emit(userInbox(dir, ROOT, "follow-up"));
    await host.emit(execution(ROOT, "succeeded"));
    await finalStep(host, dir, ROOT, "second answer");
    await quiet();

    expect(byEvent(invocationStream(dir, RUN_A, folderOf(dir)), "invocation_end")).toHaveLength(1);
    expect(readText(outputPath)).toBe(firstOutput);
    expect(firstOutput).toContain("answer");
  });

  it("writes no orchestrator stream, session records, run records or run binding", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));
    await host.fireBefore(toolBefore(ROOT, "call_d", "subagent", { agent: "Helper", prompt: missionPrompt("Helper#1", RUN_B) }));
    await finalStep(host, dir, ROOT, "answer");

    const events = new Set(allRecords(dir).map((r) => r.event));
    for (const forbidden of ["session_start", "session_end", "run_start", "run_end"]) expect(events.has(forbidden)).toBe(false);
    expect(allRecords(dir).every((r) => r._stream !== "00_orchestrator_events.jsonl")).toBe(true);
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_B))).toBe(false);
  });

  it("holds the invocation end back while the primary's own child is open", async () => {
    asRole("subagent");
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "work"));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Helper", prompt: missionPrompt("Helper#1", RUN_A) });

    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    const own = invocationOf(dir, ROOT)?.records ?? [];
    expect(byEvent(own, "invocation_end")).toEqual([]);

    await host.emit(wakeUp(ROOT, CHILD, "Helper", "completed", wrapWakeUp(CHILD, "completed", "done")));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(invocationStream(dir, RUN_A, folderOf(dir)), "invocation_end")).toHaveLength(1);
    });
  });
});

describe("synchronous writes", () => {
  it("has every record of a hook or an event on disk when its processing resolves", async () => {
    asRole("orchestrator");
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "consult"));
    const afterEvent = orchestratorStream(dir, RUN_A).map((r) => r.event);
    await host.fireBefore(toolBefore(ROOT, "call_r", "read", {}));
    const afterHook = orchestratorStream(dir, RUN_A).map((r) => r.event);

    expect(afterEvent).toEqual(["session_start", "turn"]);
    expect(afterHook).toEqual(["session_start", "turn", "tool_call_start"]);
  });
});
