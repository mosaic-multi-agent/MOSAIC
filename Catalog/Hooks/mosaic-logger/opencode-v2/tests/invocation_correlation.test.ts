/**
 * Invocation correlation: re-calls of a running child, parallel children of
 * one parent, and the one-to-one, final binding between dispatches and children.
 */

import { describe, expect, it } from "vitest";

import {
  ROOT,
  RUN_A,
  RUN_B,
  afterPayload,
  beforePayload,
  created,
  dispatchChild,
  missionPrompt,
  progress,
  userInbox,
  wakeUp,
  wrapWakeUp,
} from "./events";
import { byEvent, invocationOf, invocations, orchestratorStream, type InvocationView } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const C1 = "ses_child00000000000000c01";
const C2 = "ses_child00000000000000c02";
const C3 = "ses_child00000000000000c03";
const done = (child: string, text: string) => wakeUp(ROOT, child, "Worker", "completed", wrapWakeUp(child, "completed", text));

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

async function waitEnded(dir: string, child: string): Promise<InvocationView> {
  let found: InvocationView | undefined;
  await eventually(() => {
    found = invocationOf(dir, child);
    expect(found?.end).toBeDefined();
  });
  return found as InvocationView;
}

describe("re-calling a running child with its session id", () => {
  async function withRunningChild() {
    const running = await startRoot();
    await dispatchChild(running.host, { dir: running.dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    return running;
  }

  it("continues the same invocation: no second start, no new folder, no new run", async () => {
    const { host, dir } = await withRunningChild();
    const recallInput = { agent: "Worker", description: "wait", prompt: missionPrompt("Other#9", RUN_B), sessionID: C1 };

    await host.fireBefore(beforePayload(ROOT, "call_2", recallInput));
    await host.fireAfter(afterPayload(ROOT, "call_2", recallInput, C1));
    await host.emit(progress(dir, ROOT, "call_2", C1));
    await host.emit(done(C1, '{"status_code":"SUCCESS"}'));

    const inv = await waitEnded(dir, C1);
    expect(invocations(dir)).toHaveLength(1);
    expect(inv.folder).toBe("Worker#1");
    expect(byEvent(inv.records, "invocation_start")).toHaveLength(1);
    expect(byEvent(inv.records, "invocation_end")).toHaveLength(1);
    expect(inv.end?.agent_instance_id).toBe("Worker#1");
    expect(orchestratorStream(dir, RUN_B)).toEqual([]);
  });

  it("does not let the re-call prompt's identity create an invocation or bind a run", async () => {
    const { host, dir } = await startRoot();
    const recallInput = { agent: "Worker", description: "wait", prompt: missionPrompt("Other#9", RUN_B), sessionID: C1 };

    await host.fireBefore(beforePayload(ROOT, "call_2", recallInput));
    await host.fireAfter(afterPayload(ROOT, "call_2", recallInput, C1));
    await quiet();

    expect(host.beforeHooks).toHaveLength(1);
    expect(invocations(dir)).toEqual([]);
    expect(byEvent(orchestratorStream(dir, RUN_B), "run_start")).toEqual([]);
  });

  it("does not start a second invocation when session.created repeats for the child", async () => {
    const { host, dir } = await withRunningChild();

    await host.emit(created(dir, C1, ROOT, "Worker"));
    await host.emit(created(dir, C1, ROOT, "Worker"));
    await host.emit(done(C1, "finished"));

    const inv = await waitEnded(dir, C1);
    expect(invocations(dir)).toHaveLength(1);
    expect(byEvent(inv.records, "invocation_start")).toHaveLength(1);
  });

  it.each([
    ["an empty string", ""],
    ["a number", 5],
    ["null", null],
  ])("treats a sessionID of %s as a new dispatch", async (_label, sessionID) => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#4", RUN_A), sessionID };

    await host.fireBefore(beforePayload(ROOT, "call_4", input as never));
    await host.fireAfter(afterPayload(ROOT, "call_4", input as never, C2));
    await host.emit(done(C2, "finished"));

    const inv = await waitEnded(dir, C2);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#4", spawn_call_id: "call_4" });
  });
});

describe("parallel children of one parent", () => {
  it("keeps each child's own identity, prompt, call id and folder, whatever order they report in", async () => {
    const { host, dir } = await startRoot();
    const inputs = [
      { call: "call_p1", child: C1, id: "Worker#1" },
      { call: "call_p2", child: C2, id: "Worker#2" },
      { call: "call_p3", child: C3, id: "Worker#3" },
    ];
    for (const p of inputs) {
      const input = { agent: "Worker", description: p.id, prompt: missionPrompt(p.id, RUN_A, `task ${p.id}`) };
      await host.fireBefore(beforePayload(ROOT, p.call, input));
    }
    for (const p of [...inputs].reverse()) {
      const input = { agent: "Worker", description: p.id, prompt: missionPrompt(p.id, RUN_A, `task ${p.id}`) };
      await host.fireAfter(afterPayload(ROOT, p.call, input, p.child));
      await host.emit(created(dir, p.child, ROOT, "Worker"));
    }

    await host.emit(done(C2, "result two"));
    await host.emit(done(C3, "result three"));
    await host.emit(done(C1, "result one"));

    await eventually(() => {
      const all = invocations(dir);
      expect(all).toHaveLength(3);
      expect(new Set(all.map((i) => i.folder)).size).toBe(3);
      for (const p of inputs) {
        const inv = invocationOf(dir, p.child);
        expect(inv?.start).toMatchObject({ agent_instance_id: p.id, spawn_call_id: p.call });
        expect(inv?.start?.prompt).toContain(`task ${p.id}`);
        expect(inv?.end?.agent_instance_id).toBe(p.id);
      }
      expect(invocationOf(dir, C2)?.end?.response).toBe("result two");
      expect(invocationOf(dir, C1)?.end?.response).toBe("result one");
    });
  });

  it("writes each invocation's records into its own stream only", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_p1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    await dispatchChild(host, { dir, parent: ROOT, child: C2, callId: "call_p2", agent: "Worker", prompt: missionPrompt("Worker#2", RUN_A) });

    await host.emit(done(C1, "one"));
    await host.emit(done(C2, "two"));

    await eventually(() => {
      for (const inv of invocations(dir)) {
        const sessions = new Set(inv.records.map((r) => r.session_id));
        expect(sessions.size).toBe(1);
        expect(inv.records.map((r) => r.event).filter((e) => e === "invocation_start" || e === "invocation_end")).toEqual(["invocation_start", "invocation_end"]);
      }
      expect(invocations(dir)).toHaveLength(2);
    });
  });
});

describe("binding between a dispatch and a child is one-to-one and final", () => {
  it("keeps a fallback-joined child's dispatch when a later authoritative join disagrees", async () => {
    const { host, dir } = await startRoot();
    const x1 = { agent: "Worker", description: "one", prompt: missionPrompt("Worker#1", RUN_A) };
    const x2 = { agent: "Worker", description: "two", prompt: missionPrompt("Worker#2", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_x1", x1));
    await host.fireBefore(beforePayload(ROOT, "call_x2", x2));

    // C reports first; the oldest unjoined Worker dispatch (x1) is taken by fallback.
    await host.emit(created(dir, C1, ROOT, "Worker"));
    await host.emit(done(C1, "from c1"));
    const first = await waitEnded(dir, C1);
    expect(first.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_x1" });

    // The authoritative result now says C1 belongs to x2: nothing changes.
    await host.fireAfter(afterPayload(ROOT, "call_x2", x2, C1));
    await host.fireAfter(afterPayload(ROOT, "call_x1", x1, C2));
    await host.emit(created(dir, C2, ROOT, "Worker"));
    await host.emit(done(C2, "from c2"));

    const second = await waitEnded(dir, C2);
    const firstAgain = invocationOf(dir, C1) as InvocationView;
    expect(firstAgain.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_x1" });
    expect(firstAgain.folder).toBe("Worker#1");
    expect(second.start).toMatchObject({ agent_instance_id: "Worker#2", spawn_call_id: "call_x2" });
    expect(byEvent(firstAgain.records, "invocation_start")).toHaveLength(1);
    expect(invocations(dir)).toHaveLength(2);
  });

  it("does not hand a dispatch that an authoritative join already claimed to another child's fallback", async () => {
    const { host, dir } = await startRoot();
    const x1 = { agent: "Worker", description: "one", prompt: missionPrompt("Worker#1", RUN_A) };
    const x2 = { agent: "Worker", description: "two", prompt: missionPrompt("Worker#2", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_x1", x1));
    await host.fireBefore(beforePayload(ROOT, "call_x2", x2));
    await host.fireAfter(afterPayload(ROOT, "call_x1", x1, C1));

    await host.emit(created(dir, C2, ROOT, "Worker"));
    await host.emit(done(C2, "from c2"));

    const inv = await waitEnded(dir, C2);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#2", spawn_call_id: "call_x2" });
  });

  it("takes the oldest unjoined dispatch of the same agent type, not one of another type", async () => {
    const { host, dir } = await startRoot();
    await host.fireBefore(beforePayload(ROOT, "call_o", { agent: "Other", description: "o", prompt: missionPrompt("Other#1", RUN_A) }));
    await host.fireBefore(beforePayload(ROOT, "call_w", { agent: "Worker", description: "w", prompt: missionPrompt("Worker#1", RUN_A) }));

    await host.emit(created(dir, C1, ROOT, "Worker"));
    await host.emit(done(C1, "from worker"));

    const inv = await waitEnded(dir, C1);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_w" });
  });
});
