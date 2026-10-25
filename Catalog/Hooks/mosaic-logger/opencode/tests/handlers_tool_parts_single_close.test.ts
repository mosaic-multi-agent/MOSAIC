/**
 * handlers_tool_parts_single_close.test.ts - Exactly one tool_call_end per
 * call_id, whatever order the after-hook, error tool parts and the dispatch
 * closure arrive in, and however often they repeat. Includes the failed-subtask
 * order: the after-hook fires first with an empty or undefined result, then the
 * error part arrives; the call must end with status error.
 */

import { describe, it, expect } from "vitest";

import { toolPartUpdatedEvent } from "./fixtures/opencode_api";
import { installTempDir, makeHarness, ORCH_SESSION, SUB_SESSION } from "./handlers_tool_parts_support";

const tmp = installTempDir();

function errorPart(callID: string, error = "tool failed") {
  return toolPartUpdatedEvent({ callID, sessionID: ORCH_SESSION, status: "error", error });
}

describe("single close - regular tool calls", () => {
  it("after-hook then error part leaves exactly one end, the one from the after-hook", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "c1");

    await h.after("bash", ORCH_SESSION, "c1", { output: "ok" });
    await h.part(errorPart("c1"));

    const ends = h.ends("c1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("success");
  });

  it("error part then after-hook leaves exactly one end, the error one", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "c2");

    await h.part(errorPart("c2", "exit 1"));
    await h.after("bash", ORCH_SESSION, "c2", { output: "ok" });

    const ends = h.ends("c2");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBe("exit 1");
  });

  it("repeated error parts leave exactly one end", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "c3");

    await h.part(errorPart("c3"));
    await h.part(errorPart("c3"));
    await h.part(errorPart("c3", "different text"));

    const ends = h.ends("c3");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.error).toBe("tool failed");
  });

  it("an error part for one call does not close another open call", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "c4a");
    await h.before("bash", ORCH_SESSION, "c4b");

    await h.part(errorPart("c4a"));
    await h.after("bash", ORCH_SESSION, "c4b", { output: "ok" });

    expect(h.ends("c4a")).toHaveLength(1);
    expect(h.ends("c4b")).toHaveLength(1);
    expect(h.ends("c4b")[0]!.event.status).toBe("success");
  });
});

describe("single close - failed subtask (task call)", () => {
  it("after-hook with an empty result then an error part ends with status error and the part's text", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t1");

    await h.after("task", ORCH_SESSION, "t1", {});
    await h.part(errorPart("t1", "subagent crashed"));

    const ends = h.ends("t1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBe("subagent crashed");
  });

  it("after-hook with an undefined result then an error part ends with status error", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t2");

    await h.afterNoOutput("task", ORCH_SESSION, "t2");
    await h.part(errorPart("t2", "subagent crashed"));

    const ends = h.ends("t2");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
  });

  it("after-hook with an empty-string result then an error part ends with status error", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t3");

    await h.after("task", ORCH_SESSION, "t3", { output: "" });
    await h.part(errorPart("t3", "subagent crashed"));

    const ends = h.ends("t3");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
  });

  it("never records a failed subtask as success, even before the error part arrives", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t4");

    await h.after("task", ORCH_SESSION, "t4", {});

    expect(h.ends("t4").filter((e) => e.event.status === "success")).toHaveLength(0);
  });

  it("an empty-result after-hook then the dispatch closure ends once with status error and no error text", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t5");

    await h.after("task", ORCH_SESSION, "t5", {});
    await h.closeDispatch(SUB_SESSION);

    const ends = h.ends("t5");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
    expect(ends[0]!.event.error).toBeUndefined();
  });

  it("after-hook, error part and dispatch closure together leave exactly one end", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t6");

    await h.after("task", ORCH_SESSION, "t6", {});
    await h.part(errorPart("t6", "subagent crashed"));
    await h.closeDispatch(SUB_SESSION);
    await h.part(errorPart("t6", "subagent crashed"));

    const ends = h.ends("t6");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
  });

  it("error part before the after-hook ends once with status error", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t7");

    await h.part(errorPart("t7", "subagent crashed"));
    await h.after("task", ORCH_SESSION, "t7", {});

    const ends = h.ends("t7");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("error");
  });

  it("a task call with a real result closes as success and a later error part adds nothing", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t8");

    await h.after("task", ORCH_SESSION, "t8", { output: "subagent final answer" });
    await h.part(errorPart("t8"));

    const ends = h.ends("t8");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("success");
  });

  it("the dispatch closure alone, with no failure signal, still writes no status", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t9");

    await h.closeDispatch(SUB_SESSION);

    const ends = h.ends("t9");
    expect(ends).toHaveLength(1);
    expect("status" in ends[0]!.event).toBe(false);
  });

  it("dispatch closure then an error part leaves exactly one end", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "t10");

    await h.closeDispatch(SUB_SESSION);
    await h.part(errorPart("t10"));

    expect(h.ends("t10")).toHaveLength(1);
  });
});
