/**
 * Tool call records: a start at every execute.before, exactly one end per call
 * from execute.after or a tool failure event, in the stream that holds the start.
 */

import { describe, expect, it } from "vitest";

import { RUN_A, ROOT, created, dispatchChild, missionPrompt, userInbox } from "./events";
import { PROBE_DIR_A, loadFixture, replay } from "./fixtures";
import { allRecords, byEvent, envelopeProblems, invocationOf, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import { toolAfter, toolBefore, toolFailed } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";
const FALLBACK_CALL = /^read_\d{8}T\d{6}Z-[0-9a-f]{4}$/;

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

const toolRecords = (dir: string) => allRecords(dir).filter((r) => r.event === "tool_call_start" || r.event === "tool_call_end");

describe("tool call start", () => {
  it("writes tool_call_start with the call id, tool name and input verbatim", async () => {
    const { host, dir } = await startRoot();

    await host.fireBefore(toolBefore(ROOT, "call_r1", "read", { filePath: "README.md" }));

    const [start] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_start");
    expect(start).toMatchObject({ call_id: "call_r1", tool_name: "read", tool_input: { filePath: "README.md" }, session_id: ROOT });
    expect(envelopeProblems(start)).toEqual([]);
  });

  it("keeps a string input and drops an input that is neither an object nor a string", async () => {
    const { host, dir } = await startRoot();

    await host.fireBefore(toolBefore(ROOT, "call_s", "shell", "ls -la"));
    await host.fireBefore(toolBefore(ROOT, "call_n", "shell", 42));

    const starts = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_start");
    expect(starts.find((r) => r.call_id === "call_s")?.tool_input).toBe("ls -la");
    expect(starts.find((r) => r.call_id === "call_n")).toBeDefined();
    expect("tool_input" in (starts.find((r) => r.call_id === "call_n") ?? {})).toBe(false);
  });

  it("generates a fallback call id when the hook carries none", async () => {
    const { host, dir } = await startRoot();

    await host.fireBefore(toolBefore(ROOT, undefined, "read", { filePath: "a" }));

    const [start] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_start");
    expect(start.call_id).toMatch(FALLBACK_CALL);
  });

  it.each([undefined, "", 7, null])("writes no record when the tool name is %j", async (tool) => {
    const { host, dir } = await startRoot();

    await host.fireBefore(toolBefore(ROOT, "call_x", tool, {}));
    await quiet();

    expect(toolRecords(dir)).toEqual([]);
  });
});

describe("tool call end", () => {
  it("writes a success end carrying the result verbatim", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_r1", "read", { filePath: "a" }));

    await host.fireAfter(toolAfter(ROOT, "call_r1", "read", "completed", { result: { output: "file text", metadata: { lines: 3 } } }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end).toMatchObject({ call_id: "call_r1", tool_name: "read", status: "success", tool_output: { output: "file text", metadata: { lines: 3 } } });
    expect(envelopeProblems(end)).toEqual([]);
  });

  it("writes an error end with the error message and no tool output", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_r1", "read", {}));

    await host.fireAfter(toolAfter(ROOT, "call_r1", "read", "error", { error: { _tag: "Tool.Error", message: "file not found" } }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end).toMatchObject({ call_id: "call_r1", status: "error", error: "file not found" });
    expect("tool_output" in end).toBe(false);
  });

  it("uses a string error as it is", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_r1", "read", {}));

    await host.fireAfter(toolAfter(ROOT, "call_r1", "read", "error", { error: "plain failure" }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end).toMatchObject({ status: "error", error: "plain failure" });
  });

  it("writes neither status nor error for any other status, keeping the result", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_r1", "read", {}));

    await host.fireAfter(toolAfter(ROOT, "call_r1", "read", "running", { result: "partial" }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end).toMatchObject({ call_id: "call_r1", tool_output: "partial" });
    expect("status" in end).toBe(false);
    expect("error" in end).toBe(false);
  });

  it("pairs an after without an id with the latest start that got a fallback call id", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, undefined, "read", {}));
    const [start] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_start");

    await host.fireAfter(toolAfter(ROOT, undefined, "read", "completed", { result: "ok" }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end.call_id).toBe(start.call_id);
    expect(end.status).toBe("success");
  });

  it("writes a start-less end to the session's stream with its own id", async () => {
    const { host, dir } = await startRoot();

    await host.fireAfter(toolAfter(ROOT, "call_orphan", "read", "completed", { result: "late" }));

    const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(ends).toHaveLength(1);
    expect(ends[0]).toMatchObject({ call_id: "call_orphan", tool_name: "read", status: "success" });
  });

  it("writes the end into the stream that holds the start, even after the root bound to a run", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_early", "read", {}));
    await host.fireBefore(toolBefore(ROOT, "call_d", "subagent", { agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) }));

    await host.fireAfter(toolAfter(ROOT, "call_early", "read", "completed", { result: "ok" }));

    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end").map((r) => r.call_id)).toEqual(["call_early"]);
    expect(byEvent(orchestratorStream(dir, RUN_A), "tool_call_end")).toEqual([]);
  });
});

describe("tool failure events", () => {
  it("ends a started call with an error when the failure event names it", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_f", "shell", {}));

    await host.emit(toolFailed(dir, ROOT, "call_f", "Invalid arguments for tool shell"));

    const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(ends).toHaveLength(1);
    expect(ends[0]).toMatchObject({ call_id: "call_f", tool_name: "shell", status: "error" });
    expect(String(ends[0].error)).toContain("Invalid arguments");
  });

  it("writes exactly one end when both the after hook and the failure event report the call", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_f", "shell", {}));
    await host.fireAfter(toolAfter(ROOT, "call_f", "shell", "error", { error: { message: "first report" } }));

    await host.emit(toolFailed(dir, ROOT, "call_f", "second report"));
    await quiet();

    const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(ends).toHaveLength(1);
    expect(ends[0].error).toBe("first report");
  });

  it("writes nothing for a failure event whose call has no written start", async () => {
    const { host, dir } = await startRoot();

    await host.emit(toolFailed(dir, ROOT, "call_unknown", "nope"));
    await quiet();

    expect(toolRecords(dir)).toEqual([]);
  });
});

describe("calls inside child sessions and dispatch calls", () => {
  it("logs a child session's tool calls into the child's invocation stream", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.fireBefore(toolBefore(CHILD, "call_c1", "shell", { command: "echo hi" }));
    await host.fireAfter(toolAfter(CHILD, "call_c1", "shell", "completed", { result: { output: "hi" } }));

    await eventually(() => {
      const inv = invocationOf(dir, CHILD);
      expect(byEvent(inv?.records ?? [], "tool_call_start").map((r) => r.call_id)).toEqual(["call_c1"]);
      expect(byEvent(inv?.records ?? [], "tool_call_end").map((r) => r.call_id)).toEqual(["call_c1"]);
      expect(byEvent(inv?.records ?? [], "tool_call_start")[0].session_id).toBe(CHILD);
    });
    const rootCalls = byEvent(orchestratorStream(dir, RUN_A), "tool_call_start").map((r) => r.call_id);
    expect(rootCalls).not.toContain("call_c1");
  });

  it("writes a child's tool call after invocation_start when the child was never announced", async () => {
    const { host, dir } = await startRoot();
    await host.emit(created(dir, CHILD, ROOT, "Worker"));

    await host.fireBefore(toolBefore(CHILD, "call_c1", "read", {}));

    await eventually(() => {
      const inv = invocationOf(dir, CHILD);
      expect(inv?.records.map((r) => r.event).slice(0, 2)).toEqual(["invocation_start", "tool_call_start"]);
    });
  });

  it("logs a subagent dispatch in the bound run's stream after the run binding, ending at launch", async () => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "task", prompt: missionPrompt("Worker#1", RUN_A) };

    await host.fireBefore(toolBefore(ROOT, "call_d", "subagent", input));
    await host.fireAfter(
      toolAfter(ROOT, "call_d", "subagent", "completed", { input, result: { output: { sessionID: CHILD, status: "running" }, metadata: { sessionID: CHILD } } }),
    );

    const stream = orchestratorStream(dir, RUN_A);
    expect(stream.map((r) => r.event)).toEqual(["run_start", "session_start", "tool_call_start", "tool_call_end"]);
    expect(stream[2]).toMatchObject({ call_id: "call_d", tool_name: "subagent", tool_input: input });
    expect(stream[3]).toMatchObject({ call_id: "call_d", status: "success" });
    expect(stream[3].tool_output).toMatchObject({ output: { status: "running" } });
  });

  it("gives a re-call with a session id its own start and end and no second invocation", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    const input = { agent: "Worker", prompt: "continue", sessionID: CHILD };

    await host.fireBefore(toolBefore(ROOT, "call_re", "subagent", input));
    await host.fireAfter(toolAfter(ROOT, "call_re", "subagent", "completed", { input, result: { output: "ok" } }));

    const stream = orchestratorStream(dir, RUN_A);
    expect(byEvent(stream, "tool_call_start").map((r) => r.call_id)).toContain("call_re");
    expect(byEvent(stream, "tool_call_end").map((r) => r.call_id)).toContain("call_re");
    expect(byEvent(invocationOf(dir, CHILD)?.records ?? [], "invocation_start")).toHaveLength(1);
  });
});

describe("tool calls in the spike probe", () => {
  it("logs one start and one end per probe call, and one error end for the failed call", async () => {
    const { host, dir } = await env.start();
    const records = loadFixture("probe_proj_a_parallel_children.jsonl", { [PROBE_DIR_A]: dir });
    const hookIds = (name: string) =>
      new Set(records.filter((r) => r.hook === name).map((r) => String((r.e as { id: string }).id)));

    await replay(host, records);

    await eventually(() => {
      const all = allRecords(dir);
      const starts = byEvent(all, "tool_call_start").map((r) => r.call_id);
      const ends = byEvent(all, "tool_call_end").map((r) => r.call_id);
      expect(new Set(starts)).toEqual(hookIds("tool.execute.before"));
      expect(starts).toHaveLength(hookIds("tool.execute.before").size);
      expect(new Set(ends)).toEqual(hookIds("tool.execute.after"));
      expect(ends).toHaveLength(hookIds("tool.execute.after").size);
      expect(byEvent(all, "tool_call_end").filter((r) => r.status === "error")).toHaveLength(1);
      expect(all.flatMap((rec) => envelopeProblems(rec).map((p) => `${rec.event}: ${p}`))).toEqual([]);
    });
  });
});
