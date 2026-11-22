/**
 * Appends are atomic and locked: a burst of concurrent writes leaves every
 * JSONL line whole, and no lock sidecar or temp file is left behind.
 */

import { describe, expect, it } from "vitest";

import { ROOT, RUN_A, dispatchChild, execution, missionPrompt, userInbox, wakeUp, wrapWakeUp } from "./events";
import { allRecords, byEvent, leftoverFiles, orchestratorStream } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const BURST = 8;
const CHILD_RESPONSE = '{"status_code":"SUCCESS","status_message":"done"}';

function childId(n: number): string {
  return `ses_child000000000000d${String(n).padStart(4, "0")}`;
}

describe("write integrity", () => {
  it("leaves no lock sidecar or temp file after a session with dispatches has ended", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    await dispatchChild(host, { dir, parent: ROOT, child: childId(0), callId: "call_0", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    // The root only closes at a quiescent point, so the dispatched child ends first.
    await host.emit(wakeUp(ROOT, childId(0), "Worker", "completed", wrapWakeUp(childId(0), "completed", CHILD_RESPONSE)));
    await eventually(() => {
      expect(byEvent(allRecords(dir), "invocation_end")).toHaveLength(1);
    });
    await host.emit(execution(ROOT, "succeeded"));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "session_end")).toHaveLength(1);
    });
    await quiet();

    expect(leftoverFiles(dir)).toEqual([]);
  });

  it("keeps every line whole when many dispatches are recorded concurrently", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));

    await Promise.all(
      Array.from({ length: BURST }, (_, n) =>
        dispatchChild(host, {
          dir,
          parent: ROOT,
          child: childId(n + 1),
          callId: `call_${n + 1}`,
          agent: "Worker",
          prompt: missionPrompt(`Worker#${n + 1}`, RUN_A),
        }),
      ),
    );

    await eventually(() => {
      const records = allRecords(dir);
      expect(byEvent(records, "invocation_start")).toHaveLength(BURST);
    }, 5000);
    expect(leftoverFiles(dir)).toEqual([]);
  }, 15000);
});
