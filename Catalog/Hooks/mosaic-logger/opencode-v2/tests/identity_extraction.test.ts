/**
 * Where a dispatch prompt's instance id and run id are read from (structured
 * key first, then the earliest bare match), and that every invocation folder
 * stays inside its run folder, one per invocation.
 */

import { describe, expect, it } from "vitest";

import { ROOT, RUN_A, RUN_B, dispatchChild, userInbox } from "./events";
import { createdFiles, invocationFolders, invocationOf, invocations, LOGS, type InvocationView } from "./log_reader";
import { eventually, useTestEnv } from "./harness";

const env = useTestEnv();
const C1 = "ses_child00000000000000c01";
const C2 = "ses_child00000000000000c02";

async function dispatchAndWait(prompt: string): Promise<{ inv: InvocationView; dir: string }> {
  const { host, dir } = await env.start();
  await host.emit(userInbox(dir, ROOT, "go"));
  await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt });
  let inv: InvocationView | undefined;
  await eventually(() => {
    inv = invocationOf(dir, C1);
    expect(inv?.start).toBeDefined();
  });
  return { inv: inv as InvocationView, dir };
}

describe("instance id and run id in a dispatch prompt", () => {
  it("accepts bare ids written in prose", async () => {
    const { inv } = await dispatchAndWait(`Please act as Worker#4 for ${RUN_A} and report back.`);

    expect(inv.start?.agent_instance_id).toBe("Worker#4");
    expect(inv.run).toBe(RUN_A);
    expect(inv.folder).toBe("Worker#4");
  });

  it("prefers the structured keys over a bare mention that comes earlier in the text", async () => {
    const prompt = `Earlier ${RUN_A} and Other#9 were mentioned.\nagent_instance_id: Worker#5\nrun_id: ${RUN_B}\n`;

    const { inv } = await dispatchAndWait(prompt);

    expect(inv.start?.agent_instance_id).toBe("Worker#5");
    expect(inv.run).toBe(RUN_B);
  });

  it("takes the earliest bare match when there are several and no structured key", async () => {
    const { inv } = await dispatchAndWait(`First Worker#6 then Other#7, run ${RUN_A} and later ${RUN_B}.`);

    expect(inv.start?.agent_instance_id).toBe("Worker#6");
    expect(inv.run).toBe(RUN_A);
  });

  it("does not accept a run id with upper-case hex, and files the invocation under unknown-run", async () => {
    const { inv } = await dispatchAndWait("agent_instance_id: Worker#8\nrun_id: 20261117T150000Z-ABCD\n");

    expect(inv.start?.agent_instance_id).toBe("Worker#8");
    expect(inv.run).toBe("unknown-run");
  });
});

describe("invocation folders", () => {
  it("keeps a folder name made of dots, dashes and underscores as it is, inside the run folder", async () => {
    const { inv, dir } = await dispatchAndWait(`agent_instance_id: Work.er-x_y#3\nrun_id: ${RUN_A}\n`);

    expect(inv.folder).toBe("Work.er-x_y#3");
    // The root's session_start is written before the dispatch binds it to the run, so the
    // unbound orchestrator stream is expected; nothing else may sit outside the run folder.
    const unboundStream = `${LOGS}/unknown-run/00_orchestrator_events.jsonl`;
    const outside = createdFiles(dir).filter((p) => !p.startsWith(`${LOGS}/${RUN_A}/`) && p !== unboundStream);
    expect(outside).toEqual([]);
  });

  it("gives two dispatches that name the same instance id different folders, both inside the run folder", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    const prompt = `agent_instance_id: Worker#1\nrun_id: ${RUN_A}\n`;

    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt });
    await dispatchChild(host, { dir, parent: ROOT, child: C2, callId: "call_2", agent: "Worker", prompt });

    await eventually(() => {
      const found = invocations(dir).filter((i) => i.start);
      expect(found.map((i) => i.start?.session_id).sort()).toEqual([C1, C2]);
      expect(new Set(found.map((i) => i.folder)).size).toBe(2);
    });
    expect(invocationFolders(dir, RUN_A)).toHaveLength(2);
    expect(createdFiles(dir).filter((p) => !p.startsWith(`${LOGS}/`))).toEqual([]);
  });
});
