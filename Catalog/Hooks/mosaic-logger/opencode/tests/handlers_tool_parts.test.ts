/**
 * handlers_tool_parts.test.ts - Failed tool calls closed from tool part updates.
 *
 * A message.part.updated event whose tool part reaches state.status "error"
 * closes the matching open call with one tool_call_end (status "error", the
 * part's error text) on the stream that holds the call's tool_call_start.
 * Other part types and non-error states write nothing, and so does an error
 * part for a call whose start was never seen.
 */

import { describe, it, expect } from "vitest";

import { toolPartUpdatedEvent } from "./fixtures/opencode_api";
import {
  installTempDir,
  makeHarness,
  ORCH_SESSION,
  SUB_SESSION,
  RUN_ID,
  AGENT_INSTANCE_ID,
} from "./handlers_tool_parts_support";

const tmp = installTempDir();

describe("error tool part - closes an open call", () => {
  it("writes one tool_call_end with status error, the part's error text and the original call_id", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-bash-1");

    await h.part(
      toolPartUpdatedEvent({
        callID: "call-bash-1",
        tool: "bash",
        sessionID: ORCH_SESSION,
        status: "error",
        error: "command not found: foo",
      }),
    );

    const ends = h.ends();
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.call_id).toBe("call-bash-1");
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBe("command not found: foo");
    expect(ends[0]!.filePath).toBe(h.paths.orchestratorEvents(RUN_ID));
  });

  it("closes an MCP tool call the same way", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("github_create_issue", ORCH_SESSION, "call-mcp-1");

    await h.part(
      toolPartUpdatedEvent({
        callID: "call-mcp-1",
        tool: "github_create_issue",
        sessionID: ORCH_SESSION,
        status: "error",
        error: "MCP error -32602: Invalid params",
      }),
    );

    const ends = h.ends("call-mcp-1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBe("MCP error -32602: Invalid params");
  });

  it("closes an aborted call with the abort text copied verbatim", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-abort-1");

    await h.part(
      toolPartUpdatedEvent({
        callID: "call-abort-1",
        tool: "bash",
        sessionID: ORCH_SESSION,
        status: "error",
        error: "Tool execution aborted",
      }),
    );

    const ends = h.ends("call-abort-1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBe("Tool execution aborted");
  });

  it("closes every open call when several calls are aborted, one end per call_id", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-a");
    await h.before("read", ORCH_SESSION, "call-b");

    for (const id of ["call-a", "call-b"]) {
      await h.part(
        toolPartUpdatedEvent({
          callID: id,
          sessionID: ORCH_SESSION,
          status: "error",
          error: "Tool execution aborted",
        }),
      );
    }

    expect(h.ends("call-a")).toHaveLength(1);
    expect(h.ends("call-b")).toHaveLength(1);
    expect(h.ends()).toHaveLength(2);
  });

  it("writes the end to the subagent invocation stream when the start is there, even if the part names no session", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", SUB_SESSION, "call-sub-1");

    await h.part(toolPartUpdatedEvent({ callID: "call-sub-1", status: "error", error: "boom" }));

    const start = h.collected.find((e) => e.event.event === "tool_call_start");
    const ends = h.ends("call-sub-1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.filePath).toBe(start!.filePath);
    expect(ends[0]!.filePath).toBe(h.paths.invocationEvents(RUN_ID, AGENT_INSTANCE_ID));
  });

  it("carries no tool_output on the error end", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-noout");

    await h.part(toolPartUpdatedEvent({ callID: "call-noout", status: "error", error: "failed" }));

    expect(h.ends("call-noout")[0]!.event.tool_output).toBeUndefined();
  });

  it("omits the error field when the part's error is empty or not a string, but still ends with status error", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-empty-err");
    await h.before("bash", ORCH_SESSION, "call-obj-err");

    await h.part(toolPartUpdatedEvent({ callID: "call-empty-err", status: "error", error: "" }));
    await h.part(
      toolPartUpdatedEvent({ callID: "call-obj-err", status: "error", error: { code: 1 } }),
    );

    for (const id of ["call-empty-err", "call-obj-err"]) {
      const ends = h.ends(id);
      expect(ends).toHaveLength(1);
      expect(ends[0]!.event.status).toBe("error");
      expect(ends[0]!.event.error).toBeUndefined();
    }
  });
});

describe("tool part updates that are not failures", () => {
  it.each(["pending", "running", "completed", "something-else"])(
    "writes nothing for a tool part in state %s and leaves the call open",
    async (status) => {
      const h = makeHarness(tmp.dir());
      await h.before("bash", ORCH_SESSION, "call-open");

      await h.part(toolPartUpdatedEvent({ callID: "call-open", status }));
      expect(h.ends()).toHaveLength(0);

      // The call is still open: a later error part closes it.
      await h.part(toolPartUpdatedEvent({ callID: "call-open", status: "error", error: "late" }));
      expect(h.ends("call-open")).toHaveLength(1);
    },
  );

  it("writes nothing for an error state on a part that is not a tool part", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-text");
    const event = toolPartUpdatedEvent({ callID: "call-text", status: "error", error: "x" });
    (event.properties!["part"] as Record<string, unknown>)["type"] = "text";

    await h.part(event);

    expect(h.ends()).toHaveLength(0);
  });

  it("ignores events that carry no part or a malformed part without throwing", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-mal");

    await h.part({ type: "message.part.updated" });
    await h.part({ type: "message.part.updated", properties: {} });
    await h.part({ type: "message.part.updated", properties: { part: "nope" } });
    await h.part({ type: "message.part.updated", properties: { part: { type: "tool" } } });
    await h.part({
      type: "message.part.updated",
      properties: { part: { type: "tool", callID: "call-mal", state: "error" } },
    });

    expect(h.ends()).toHaveLength(0);
  });
});

describe("error tool part for a call that was never started", () => {
  it("writes nothing when the call_id was never registered", async () => {
    const h = makeHarness(tmp.dir());

    await h.part(
      toolPartUpdatedEvent({
        callID: "call-unknown",
        sessionID: SUB_SESSION,
        status: "error",
        error: "failed",
      }),
    );

    expect(h.collected).toHaveLength(0);
  });

  it("writes nothing when the part has no callID or an empty one", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "call-real");

    await h.part(toolPartUpdatedEvent({ status: "error", error: "failed" }));
    await h.part(toolPartUpdatedEvent({ callID: "", status: "error", error: "failed" }));

    expect(h.ends()).toHaveLength(0);
  });
});
