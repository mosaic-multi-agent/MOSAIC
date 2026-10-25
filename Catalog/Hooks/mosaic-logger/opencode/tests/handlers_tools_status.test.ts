/**
 * handlers_tools_status.test.ts - Every successful call closed by the
 * after-hook carries status "success"; the status is never omitted.
 */

import { describe, it, expect } from "vitest";

import { deriveToolOutcome } from "../lib/handlers_tools";
import { toolAfterPayload } from "./fixtures/opencode_api";
import { installTempDir, makeHarness, ORCH_SESSION } from "./handlers_tool_parts_support";

const tmp = installTempDir();

describe("after-hook - successful calls carry status success", () => {
  it.each([
    ["an empty output object", {}],
    ["a plain text output", { output: "listing of files" }],
    ["title and metadata without a status signal", { title: "Ran bash", metadata: { exit: 0 } }],
  ])("writes status success for %s", async (_label, output) => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "s1");

    await h.after("bash", ORCH_SESSION, "s1", output as Record<string, unknown>);

    const ends = h.ends("s1");
    expect(ends).toHaveLength(1);
    expect(ends[0]!.event.status).toBe("success");
    expect(ends[0]!.event.error).toBeUndefined();
  });

  // Support coverage only: the undefined-payload fallback already exists; the
  // deriveToolOutcome unit tests below carry the new-behavior evidence.
  it("writes status success when the after-hook gets no output argument", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("read", ORCH_SESSION, "s2");

    await h.afterNoOutput("read", ORCH_SESSION, "s2");

    expect(h.ends("s2")[0]!.event.status).toBe("success");
  });

  it("writes status success for an MCP tool call", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("github_list_issues", ORCH_SESSION, "s3");

    await h.after("github_list_issues", ORCH_SESSION, "s3", { output: "[]" });

    expect(h.ends("s3")[0]!.event.status).toBe("success");
  });

  it("writes status success for a non-task call whose result is empty", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("write", ORCH_SESSION, "s4");

    await h.after("write", ORCH_SESSION, "s4", { output: "" });

    expect(h.ends("s4")[0]!.event.status).toBe("success");
  });

  it("still writes status error with the error text for an explicit error signal", async () => {
    const h = makeHarness(tmp.dir());
    await h.before("bash", ORCH_SESSION, "s5");

    await h.after("bash", ORCH_SESSION, "s5", { error: "exit code 2" });

    const end = h.ends("s5")[0]!;
    expect(end.event.status).toBe("error");
    expect(end.event.error).toBe("exit code 2");
  });
});

describe("deriveToolOutcome - status is never omitted", () => {
  const [input] = toolAfterPayload({ tool: "bash", sessionID: ORCH_SESSION });

  it("returns success for an undefined payload", () => {
    expect(deriveToolOutcome(input, undefined).status).toBe("success");
  });

  it("returns success for an empty payload", () => {
    expect(deriveToolOutcome(input, {}).status).toBe("success");
  });

  it("returns success when the error field is an empty string", () => {
    const outcome = deriveToolOutcome(input, { error: "" });
    expect(outcome.status).toBe("success");
    expect(outcome.error).toBeUndefined();
  });

  it("returns error without error text when only status is error", () => {
    const outcome = deriveToolOutcome(input, { status: "error" });
    expect(outcome.status).toBe("error");
    expect(outcome.error).toBeUndefined();
  });
});
