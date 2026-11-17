/**
 * The per-invocation input and output artifacts: 01_input.md is written with
 * invocation_start and 02_output.md with invocation_end, in the V1 markdown
 * layout with the Harness row opencode-v2.
 */

import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import { ROOT, RUN_A, dispatchChild, missionPrompt, userInbox, wakeUp, wrapWakeUp } from "./events";
import { LOGS, readText } from "./log_reader";
import { eventually, useTestEnv } from "./harness";

const env = useTestEnv();
const CHILD = "ses_child00000000000000d01";
const RESPONSE = '{"status_code":"SUCCESS","status_message":"all done"}';
const TABLE_TIMESTAMP = /\| Captured At \| \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z \|/;

async function runDispatch(prompt: string, instanceFolder: string) {
  const running = await env.start();
  const { host, dir } = running;
  await host.emit(userInbox(dir, ROOT, "go"));
  await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_a1", agent: "Worker", prompt });
  const folder = nodePath.join(dir, LOGS, RUN_A, instanceFolder);
  return { ...running, folder };
}

describe("01_input.md", () => {
  it("is written with the invocation start, in the V1 input layout with Harness opencode-v2", async () => {
    const prompt = missionPrompt("Worker#1", RUN_A, "Please do the thing.");
    const { folder } = await runDispatch(prompt, "Worker#1");

    await eventually(() => {
      const text = readText(nodePath.join(folder, "01_input.md"));
      expect(text).toBeDefined();
      expect(text).toContain("# Invocation Input: Worker#1\n");
      expect(text).toContain("| Field | Value |\n|---|---|\n");
      expect(text).toContain("| Agent Instance ID | Worker#1 |\n");
      expect(text).toContain("| Agent Type | Worker |\n");
      expect(text).toContain(`| Session ID | ${CHILD} |\n`);
      expect(text).toContain(`| Run ID | ${RUN_A} |\n`);
      expect(text).toContain("| Harness | opencode-v2 |\n");
      expect(text).toMatch(TABLE_TIMESTAMP);
      expect(text).toContain(`\n## Prompt\n\n${prompt}\n`);
    });
  });

  it("does not repeat the harness's child-prompt prefix in the prompt section", async () => {
    const { folder } = await runDispatch(missionPrompt("Worker#1", RUN_A), "Worker#1");

    await eventually(() => {
      const text = readText(nodePath.join(folder, "01_input.md")) ?? "";
      expect(text).toContain("## Prompt");
      expect(text).not.toContain("You are a subagent spawned by another session.");
    });
  });
});

describe("02_output.md", () => {
  it("is written with the invocation end, in the V1 output layout with Harness opencode-v2", async () => {
    const { host, folder } = await runDispatch(missionPrompt("Worker#1", RUN_A), "Worker#1");

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", RESPONSE)));

    await eventually(() => {
      const text = readText(nodePath.join(folder, "02_output.md"));
      expect(text).toBeDefined();
      expect(text).toContain("# Invocation Output: Worker#1\n");
      expect(text).toContain("| Agent Instance ID | Worker#1 |\n");
      expect(text).toContain("| Agent Type | Worker |\n");
      expect(text).toContain(`| Session ID | ${CHILD} |\n`);
      expect(text).toContain(`| Run ID | ${RUN_A} |\n`);
      expect(text).toContain("| Harness | opencode-v2 |\n");
      expect(text).toContain("| Status Code | SUCCESS |\n");
      expect(text).toMatch(TABLE_TIMESTAMP);
      expect(text).toContain(`\n## Response\n\n${RESPONSE}\n`);
    });
  });

  it("has no model or token rows, because the wake-up carries neither", async () => {
    const { host, folder } = await runDispatch(missionPrompt("Worker#1", RUN_A), "Worker#1");

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", RESPONSE)));

    await eventually(() => {
      const text = readText(nodePath.join(folder, "02_output.md")) ?? "";
      expect(text).toContain("## Response");
      expect(text).not.toContain("| Model |");
      expect(text).not.toContain("Tokens |");
    });
  });

  it("is not written before the invocation ends, and not rewritten by later wake-ups", async () => {
    const { host, folder } = await runDispatch(missionPrompt("Worker#1", RUN_A), "Worker#1");
    await host.emit(wakeUp(ROOT, CHILD, "Worker", "running", wrapWakeUp(CHILD, "running", "partial")));
    expect(readText(nodePath.join(folder, "02_output.md"))).toBeUndefined();

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", RESPONSE)));
    await eventually(() => {
      expect(readText(nodePath.join(folder, "02_output.md"))).toContain(RESPONSE);
    });
    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", "a later round")));
    await eventually(() => {
      expect(readText(nodePath.join(folder, "02_output.md"))).toContain(RESPONSE);
      expect(readText(nodePath.join(folder, "02_output.md"))).not.toContain("a later round");
    });
  });

  it("has no Response section when the wake-up carried no text", async () => {
    const { host, folder } = await runDispatch(missionPrompt("Worker#1", RUN_A), "Worker#1");

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", "")));

    await eventually(() => {
      const text = readText(nodePath.join(folder, "02_output.md"));
      expect(text).toBeDefined();
      expect(text).not.toContain("## Response");
    });
  });
});
