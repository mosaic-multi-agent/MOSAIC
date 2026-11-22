/**
 * A subagent dispatched by an owned child session is captured like any other
 * dispatch: the grandchild gets its own invocation, correlated by its own call.
 */

import { describe, expect, it } from "vitest";

import { ROOT, RUN_A, dispatchChild, missionPrompt, userInbox, wakeUp, wrapWakeUp } from "./events";
import { invocationOf, invocations } from "./log_reader";
import { eventually, useTestEnv } from "./harness";

const env = useTestEnv();
const CHILD = "ses_child00000000000000e01";
const GRANDCHILD = "ses_child00000000000000e02";

describe("nested dispatch", () => {
  it("records a dispatch made from a child session as its own invocation, then ends it from its wake-up", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_c", agent: "Lead", prompt: missionPrompt("Lead#1", RUN_A) });

    await dispatchChild(host, { dir, parent: CHILD, child: GRANDCHILD, callId: "call_g", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    await host.emit(wakeUp(CHILD, GRANDCHILD, "Worker", "completed", wrapWakeUp(GRANDCHILD, "completed", '{"status_code":"SUCCESS"}')));

    await eventually(() => {
      const inv = invocationOf(dir, GRANDCHILD);
      expect(inv?.start).toMatchObject({ agent_type: "Worker", agent_instance_id: "Worker#1", spawn_call_id: "call_g" });
      expect(inv?.end?.agent_instance_id).toBe("Worker#1");
    });
    expect(invocations(dir).filter((i) => i.start)).toHaveLength(2);
  });
});
