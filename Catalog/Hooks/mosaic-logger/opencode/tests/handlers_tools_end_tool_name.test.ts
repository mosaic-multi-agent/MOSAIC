/**
 * handlers_tools_end_tool_name.test.ts - Every tool_call_end write path carries
 * the tool name when the hook input, the dispatching tool or the failed tool part
 * has one, omits it otherwise, and tool_call_end events are stamped 1.6.0 while
 * other events keep the default version.
 */

import { describe, it, expect } from "vitest";

import { buildEvent } from "../lib/core";
import { schemaVersionFor } from "../lib/schema_versions";
import { toolPartUpdatedEvent } from "./fixtures/opencode_api";
import { installTempDir, makeHarness, ORCH_SESSION, SUB_SESSION } from "./handlers_tool_parts_support";

const tmp = installTempDir();
const ENVELOPE = { sessionId: ORCH_SESSION, runId: "20260101T170000Z-a3f9", timestamp: "2026-01-01T17:00:00.000Z" };

describe("tool_call_end schema version", () => {
  it("is 1.6.0 in the version table", () => {
    expect(schemaVersionFor("tool_call_end")).toBe("1.6.0");
  });

  it("is stamped on a built tool_call_end event", () => {
    expect(buildEvent("tool_call_end", ENVELOPE, {}).schema_version).toBe("1.6.0");
  });

  it.each(["tool_call_start", "notification", "session_start", "session_end", "subagent_start"])(
    "keeps the default version for a %s event",
    (name) => {
      expect(buildEvent(name, ENVELOPE, {}).schema_version).toBe("1.3.0");
    },
  );

  it("keeps compaction at 1.5.0", () => {
    expect(buildEvent("compaction", ENVELOPE, {}).schema_version).toBe("1.5.0");
  });
});

describe("tool_call_end tool_name - after-hook", () => {
  it("carries the tool name from the hook input", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "n1");

    await h.after("bash", ORCH_SESSION, "n1", { output: "ok" });

    expect(h.ends("n1")[0]!.event.tool_name).toBe("bash");
  });

  it("carries the tool name of an MCP tool", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("github_list_issues", ORCH_SESSION, "n2");

    await h.after("github_list_issues", ORCH_SESSION, "n2", { output: "[]" });

    expect(h.ends("n2")[0]!.event.tool_name).toBe("github_list_issues");
  });

  it("carries the tool name on a failed call closed by the after-hook", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("read", ORCH_SESSION, "n3");

    await h.after("read", ORCH_SESSION, "n3", { error: "no such file" });

    const end = h.ends("n3")[0]!;
    expect(end.event.status).toBe("error");
    expect(end.event.tool_name).toBe("read");
  });

  it("omits tool_name when the hook input carries no tool name", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "n4");

    await h.after("", ORCH_SESSION, "n4", { output: "ok" });

    const ends = h.ends("n4");
    expect(ends).toHaveLength(1);
    expect("tool_name" in ends[0]!.event).toBe(false);
  });
});

describe("tool_call_end tool_name - synthetic dispatch closure", () => {
  it("carries the name of the dispatching tool", async () => {
    const h = makeHarness(tmp.dir());
    await h.beforeTask(ORCH_SESSION, "d1");

    await h.closeDispatch(SUB_SESSION);

    const ends = h.ends("d1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.tool_name).toBe("task");
  });
});

describe("tool_call_end tool_name - failed tool part", () => {
  it("carries the tool name from the part", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "p1");

    await h.part(
      toolPartUpdatedEvent({ callID: "p1", tool: "bash", sessionID: ORCH_SESSION, status: "error", error: "exit 1" }),
    );

    const end = h.ends("p1")[0]!;
    expect(end.event.status).toBe("error");
    expect(end.event.tool_name).toBe("bash");
  });
});
