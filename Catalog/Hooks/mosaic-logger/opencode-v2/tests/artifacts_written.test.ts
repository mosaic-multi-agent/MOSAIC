/**
 * Artifacts written: the V2 file-writing tools (write, edit, patch) are logged
 * as ordinary tool calls with their input and output verbatim. Nothing else
 * is written for them, not even for the edit permission metadata.
 */

import { describe, expect, it } from "vitest";

import { ROOT, userInbox } from "./events";
import { allRecords, byEvent, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { useTestEnv } from "./harness";
import { evaluatePayload, toolAfter, toolBefore } from "./step_events";

const env = useTestEnv();

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

const WRITES: Array<[string, Record<string, unknown>, Record<string, unknown>]> = [
  ["write", { filePath: "src/new.ts", content: "export const a = 1;\n" }, { output: "wrote src/new.ts" }],
  ["edit", { filePath: "src/a.ts", oldString: "a", newString: "b" }, { output: "edited", metadata: { diff: "-a\n+b" } }],
  ["patch", { patchText: "*** Begin Patch\n*** Update File: src/a.ts\n*** End Patch" }, { output: "patched 1 file" }],
];

describe("file-writing tool calls", () => {
  it.each(WRITES)("logs %s with its input and result verbatim", async (tool, input, result) => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, `call_${tool}`, tool, input));

    await host.fireAfter(toolAfter(ROOT, `call_${tool}`, tool, "completed", { input, result }));

    const stream = orchestratorStream(dir, UNKNOWN_RUN);
    expect(byEvent(stream, "tool_call_start")[0]).toMatchObject({ call_id: `call_${tool}`, tool_name: tool, tool_input: input });
    expect(byEvent(stream, "tool_call_end")[0]).toMatchObject({ call_id: `call_${tool}`, tool_name: tool, status: "success", tool_output: result });
  });

  it.each(WRITES)("writes no record besides the call's start and end for %s", async (tool, input, result) => {
    const { host, dir } = await startRoot();

    await host.fireBefore(toolBefore(ROOT, `call_${tool}`, tool, input));
    await host.fireAfter(toolAfter(ROOT, `call_${tool}`, tool, "completed", { input, result }));

    const events = allRecords(dir).map((r) => r.event);
    expect(events).toEqual(["session_start", "turn", "tool_call_start", "tool_call_end"]);
  });

  it("writes no artifact record for the edit permission metadata of an allowed edit", async () => {
    const { host, dir } = await startRoot();

    await host.fireEvaluate(
      evaluatePayload(ROOT, "allow", { metadata: { filepath: "src/a.ts", diff: "-a\n+b" } }),
    );

    expect(allRecords(dir).map((r) => r.event)).toEqual(["session_start", "turn"]);
  });

  it("logs a failed write as an error end", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(toolBefore(ROOT, "call_w", "write", { filePath: "/denied.txt", content: "x" }));

    await host.fireAfter(toolAfter(ROOT, "call_w", "write", "error", { error: { message: "permission denied" } }));

    const [end] = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "tool_call_end");
    expect(end).toMatchObject({ call_id: "call_w", tool_name: "write", status: "error", error: "permission denied" });
  });
});
