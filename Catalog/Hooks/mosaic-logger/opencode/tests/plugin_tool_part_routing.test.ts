/**
 * plugin_tool_part_routing.test.ts - message.part.updated tool parts reach the
 * failed-call handling through the plugin's real event hook, in native mode and
 * in Runner mode (orchestrator and subagent roles).
 */

import { describe, it, expect } from "vitest";
import * as nodePath from "node:path";

import {
  sessionCreatedEvent,
  sessionIdleEvent,
  taskDispatchPayload,
  toolBeforePayload,
  toolAfterPayload,
  toolPartUpdatedEvent,
} from "./fixtures/opencode_api";
import {
  installRunnerTestEnv,
  setRunnerEnv,
  makeSdk,
  startPlugin,
  userMessageUpdated,
  readJsonl,
  runDir,
  RUN_ID,
  INSTANCE_ID,
  SESSION,
} from "./runner_mode_support";

const testEnv = installRunnerTestEnv();

function ends(events: Record<string, unknown>[], callId?: string): Record<string, unknown>[] {
  return events.filter(
    (e) => e["event"] === "tool_call_end" && (callId === undefined || e["call_id"] === callId),
  );
}

describe("native mode - tool part routing", () => {
  it("an error tool part delivered to the event hook closes the open call on the orchestrator stream", async () => {
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    const [bIn, bOut] = toolBeforePayload({ tool: "bash", sessionID: SESSION, callID: "n-1" });
    await hooks["tool.execute.before"](bIn, bOut);

    await hooks.event({
      event: toolPartUpdatedEvent({
        callID: "n-1",
        tool: "bash",
        sessionID: SESSION,
        status: "error",
        error: "command failed",
      }),
    });

    const stream = readJsonl(
      nodePath.join(runDir(testEnv.dir, "unknown-run"), "00_orchestrator_events.jsonl"),
    );
    const closed = ends(stream, "n-1");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("command failed");
  });

  it("an error part followed by the after-hook leaves one end on disk", async () => {
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    const [bIn, bOut] = toolBeforePayload({ tool: "bash", sessionID: SESSION, callID: "n-2" });
    await hooks["tool.execute.before"](bIn, bOut);

    await hooks.event({
      event: toolPartUpdatedEvent({ callID: "n-2", sessionID: SESSION, status: "error", error: "x" }),
    });
    const [aIn, aOut] = toolAfterPayload({ tool: "bash", sessionID: SESSION, callID: "n-2" });
    await hooks["tool.execute.after"](aIn, aOut);

    const stream = readJsonl(
      nodePath.join(runDir(testEnv.dir, "unknown-run"), "00_orchestrator_events.jsonl"),
    );
    // The error part arrived first, so its status and text win over the after-hook.
    const closed = ends(stream, "n-2");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("x");
  });

  it("a successful call closed by the after-hook is written with status success", async () => {
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    const [bIn, bOut] = toolBeforePayload({ tool: "bash", sessionID: SESSION, callID: "n-3" });
    await hooks["tool.execute.before"](bIn, bOut);
    const [aIn, aOut] = toolAfterPayload({
      tool: "bash",
      sessionID: SESSION,
      callID: "n-3",
      output: { output: "done" },
    });
    await hooks["tool.execute.after"](aIn, aOut);

    const stream = readJsonl(
      nodePath.join(runDir(testEnv.dir, "unknown-run"), "00_orchestrator_events.jsonl"),
    );
    expect(ends(stream, "n-3")[0]!["status"]).toBe("success");
  });
});

describe("Runner mode - tool part routing", () => {
  it("orchestrator role: an error tool part closes the open call on the orchestrator stream", async () => {
    setRunnerEnv({ role: "orchestrator", runId: RUN_ID });
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    await hooks.event({ event: userMessageUpdated(SESSION, "Plan the work") });
    const [bIn, bOut] = toolBeforePayload({ tool: "bash", sessionID: SESSION, callID: "r-1" });
    await hooks["tool.execute.before"](bIn, bOut);

    await hooks.event({
      event: toolPartUpdatedEvent({
        callID: "r-1",
        tool: "bash",
        sessionID: SESSION,
        status: "error",
        error: "Tool execution aborted",
      }),
    });

    const stream = readJsonl(
      nodePath.join(runDir(testEnv.dir, RUN_ID), "00_orchestrator_events.jsonl"),
    );
    const closed = ends(stream, "r-1");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("Tool execution aborted");
  });

  it("subagent role: an error tool part closes the open call on the invocation stream", async () => {
    setRunnerEnv({ role: "subagent", runId: RUN_ID, instanceId: INSTANCE_ID });
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    await hooks.event({ event: userMessageUpdated(SESSION, "Research the market") });
    const [bIn, bOut] = toolBeforePayload({ tool: "github_get_file", sessionID: SESSION, callID: "r-2" });
    await hooks["tool.execute.before"](bIn, bOut);

    await hooks.event({
      event: toolPartUpdatedEvent({
        callID: "r-2",
        tool: "github_get_file",
        sessionID: SESSION,
        status: "error",
        error: "MCP error: not found",
      }),
    });

    const stream = readJsonl(nodePath.join(runDir(testEnv.dir, RUN_ID), INSTANCE_ID, "03_events.jsonl"));
    const closed = ends(stream, "r-2");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("MCP error: not found");
  });

  it("subagent role: repeated error parts and a late after-hook leave exactly one end", async () => {
    setRunnerEnv({ role: "subagent", runId: RUN_ID, instanceId: INSTANCE_ID });
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    await hooks.event({ event: userMessageUpdated(SESSION, "Research the market") });
    const [bIn, bOut] = toolBeforePayload({ tool: "bash", sessionID: SESSION, callID: "r-3" });
    await hooks["tool.execute.before"](bIn, bOut);

    const part = toolPartUpdatedEvent({ callID: "r-3", sessionID: SESSION, status: "error", error: "e" });
    await hooks.event({ event: part });
    await hooks.event({ event: part });
    const [aIn, aOut] = toolAfterPayload({ tool: "bash", sessionID: SESSION, callID: "r-3" });
    await hooks["tool.execute.after"](aIn, aOut);

    const stream = readJsonl(nodePath.join(runDir(testEnv.dir, RUN_ID), INSTANCE_ID, "03_events.jsonl"));
    const closed = ends(stream, "r-3");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("e");
  });

  it("subagent role: a text part still reaches the final response accumulation", async () => {
    setRunnerEnv({ role: "subagent", runId: RUN_ID, instanceId: INSTANCE_ID });
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    await hooks.event({ event: userMessageUpdated(SESSION, "Research the market") });
    await hooks.event({
      event: {
        type: "message.updated",
        properties: { info: { id: "msg-asst-text", sessionID: SESSION, role: "assistant", time: { created: 1000 } } },
      },
    });

    await hooks.event({
      event: {
        type: "message.part.updated",
        properties: {
          part: { id: "p-text", messageID: "msg-asst-text", sessionID: SESSION, type: "text", text: "Streamed final answer" },
        },
      },
    });
    void hooks.event({ event: sessionIdleEvent(SESSION) });

    const stream = readJsonl(nodePath.join(runDir(testEnv.dir, RUN_ID), INSTANCE_ID, "03_events.jsonl"));
    const end = stream.find((e) => e["event"] === "invocation_end");
    expect(end?.["response"]).toBe("Streamed final answer");
  });

  it("orchestrator role: an empty-result task after-hook then an error part ends the call with status error", async () => {
    setRunnerEnv({ role: "orchestrator", runId: RUN_ID });
    const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
    await hooks.event({ event: sessionCreatedEvent(SESSION) });
    await hooks.event({ event: userMessageUpdated(SESSION, "Plan the work") });
    const [bIn, bOut] = taskDispatchPayload({ sessionID: SESSION, callID: "r-4", runId: RUN_ID });
    await hooks["tool.execute.before"](bIn, bOut);
    const [aIn, aOut] = toolAfterPayload({ tool: "task", sessionID: SESSION, callID: "r-4" });
    await hooks["tool.execute.after"](aIn, aOut);

    await hooks.event({
      event: toolPartUpdatedEvent({
        callID: "r-4",
        tool: "task",
        sessionID: SESSION,
        status: "error",
        error: "subagent crashed",
      }),
    });

    const stream = readJsonl(
      nodePath.join(runDir(testEnv.dir, RUN_ID), "00_orchestrator_events.jsonl"),
    );
    const closed = ends(stream, "r-4");
    expect(closed).toHaveLength(1);
    expect(closed[0]!["status"]).toBe("error");
    expect(closed[0]!["error"]).toBe("subagent crashed");
  });
});
