/**
 * Subagent invocations in native mode: invocation_start from a dispatch joined
 * to its child, invocation_end from the child's first non-running wake-up, and
 * the identity, run and fallback rules around them.
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
import { byEvent, envelopeProblems, invocationOf, invocations, type InvocationView, type LogRecord } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const C1 = "ses_child00000000000000b01";
const C2 = "ses_child00000000000000b02";
const FALLBACK_ID = /^agent_\d{8}T\d{6}Z-[0-9a-f]{4}$/;
const OK = '{"status_code":"SUCCESS","status_message":"done"}';

async function waitInvocation(dir: string, child: string, until: (inv: InvocationView) => boolean = (i) => !!i.start): Promise<InvocationView> {
  let found: InvocationView | undefined;
  await eventually(() => {
    found = invocationOf(dir, child);
    expect(found).toBeDefined();
    expect(until(found as InvocationView)).toBe(true);
  });
  return found as InvocationView;
}

function lifecycle(records: LogRecord[]): string[] {
  return records.map((r) => r.event).filter((e) => e === "invocation_start" || e === "invocation_end");
}

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

describe("invocation_start from a dispatch", () => {
  it("takes identity, agent type, prompt and spawn call id from the dispatch, in the run's invocation folder", async () => {
    const { host, dir } = await startRoot();
    const prompt = missionPrompt("Worker#1", RUN_A, "Do the thing.");

    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_d1", agent: "Worker", prompt });

    const inv = await waitInvocation(dir, C1);
    expect(inv.run).toBe(RUN_A);
    expect(inv.folder).toBe("Worker#1");
    expect(inv.start).toMatchObject({
      event: "invocation_start",
      schema_version: "1.4.0",
      harness: "opencode-v2",
      session_id: C1,
      run_id: RUN_A,
      agent_instance_id: "Worker#1",
      agent_type: "Worker",
      prompt,
      spawn_call_id: "call_d1",
    });
    expect(envelopeProblems(inv.start as LogRecord)).toEqual([]);
  });

  it("is written exactly once and is the first record of the invocation's stream", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_d1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emit(created(dir, C1, ROOT, "Worker"));
    await host.emit(progress(dir, ROOT, "call_d1", C1));
    await host.emit(wakeUp(ROOT, C1, "Worker", "running", wrapWakeUp(C1, "running", "")));

    const inv = await waitInvocation(dir, C1);
    await quiet();
    const again = invocationOf(dir, C1) as InvocationView;
    expect(again.records[0].event).toBe("invocation_start");
    expect(lifecycle(again.records)).toEqual(["invocation_start"]);
    expect(inv.folder).toBe("Worker#1");
  });

  it("strips the harness's child-prompt prefix: the logged prompt is the dispatch prompt verbatim", async () => {
    const { host, dir } = await startRoot();
    const prompt = missionPrompt("Worker#1", RUN_A, "Line one\nLine two");

    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_d1", agent: "Worker", prompt });

    const inv = await waitInvocation(dir, C1);
    expect(inv.start?.prompt).toBe(prompt);
    expect(String(inv.start?.prompt)).not.toContain("You are a subagent spawned by another session.");
  });

  it("uses a fallback instance id, but keeps the dispatch's agent type, prompt and call id, when the prompt has no identity", async () => {
    const { host, dir } = await startRoot();

    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_d1", agent: "Worker", prompt: "Plain task with no ids." });

    const inv = await waitInvocation(dir, C1);
    expect(inv.start?.agent_instance_id).toMatch(FALLBACK_ID);
    expect(inv.start).toMatchObject({ agent_type: "Worker", prompt: "Plain task with no ids.", spawn_call_id: "call_d1" });
    expect(inv.run).toBe("unknown-run");
  });
});

describe("which run an invocation belongs to", () => {
  it("uses the root's binding when the dispatch prompt names no run", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await dispatchChild(host, { dir, parent: ROOT, child: C2, callId: "call_2", agent: "Worker", prompt: missionPrompt("Worker#2", undefined) });

    const inv = await waitInvocation(dir, C2);
    expect(inv.run).toBe(RUN_A);
    expect(inv.folder).toBe("Worker#2");
  });

  it("uses the dispatch prompt's own run id over the root's binding", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await dispatchChild(host, { dir, parent: ROOT, child: C2, callId: "call_2", agent: "Worker", prompt: missionPrompt("Worker#2", RUN_B) });

    const inv = await waitInvocation(dir, C2);
    expect(inv.run).toBe(RUN_B);
    expect(inv.start?.run_id).toBe(RUN_B);
  });

  it("falls back to unknown-run for an unbound root and a prompt without a run id", async () => {
    const { host, dir } = await startRoot();

    await dispatchChild(host, { dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", undefined) });

    const inv = await waitInvocation(dir, C1);
    expect(inv.run).toBe("unknown-run");
    expect(inv.folder).toBe("Worker#1");
    expect(inv.start?.run_id).toBeUndefined();
  });
});

describe("invocation_end from the wake-up", () => {
  async function runningChild() {
    const running = await startRoot();
    await dispatchChild(running.host, { dir: running.dir, parent: ROOT, child: C1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    return running;
  }

  it("writes the child's final text as the response, with the protocol status code and the start's identity", async () => {
    const { host, dir } = await runningChild();

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.end).toMatchObject({
      event: "invocation_end",
      schema_version: "1.3.0",
      harness: "opencode-v2",
      session_id: C1,
      run_id: RUN_A,
      agent_instance_id: "Worker#1",
      response: OK,
      status_code: "SUCCESS",
    });
    expect(inv.end).not.toHaveProperty("model");
    expect(inv.end).not.toHaveProperty("token_usage");
    expect(envelopeProblems(inv.end as LogRecord)).toEqual([]);
    expect(lifecycle(inv.records)).toEqual(["invocation_start", "invocation_end"]);
  });

  it("removes only the one newline after the opening tag and the one before the closing tag", async () => {
    const { host, dir } = await runningChild();

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", `<subagent sessionID="${C1}" state="completed">\n\nbody line\n\n</subagent>`));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.end?.response).toBe("\nbody line\n");
  });

  it("uses the whole text verbatim when it does not have the subagent tag form", async () => {
    const { host, dir } = await runningChild();

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", "plain text without the wrapper"));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.end?.response).toBe("plain text without the wrapper");
  });

  it("omits the response when the wrapped body is empty, and the status code when the response has none", async () => {
    const { host, dir } = await runningChild();

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", "")));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.end).not.toHaveProperty("response");
    expect(inv.end).not.toHaveProperty("status_code");
  });

  it("does not end the invocation for a running wake-up, but does for the first later non-running one", async () => {
    const { host, dir } = await runningChild();

    await host.emit(wakeUp(ROOT, C1, "Worker", "running", wrapWakeUp(C1, "running", "still going")));
    await quiet();
    const whileRunning = invocationOf(dir, C1) as InvocationView;
    await host.emit(wakeUp(ROOT, C1, "Worker", "failed", wrapWakeUp(C1, "failed", "it broke")));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(whileRunning.end).toBeUndefined();
    expect(inv.end?.response).toBe("it broke");
  });

  it("writes nothing for later wake-ups of the same child", async () => {
    const { host, dir } = await runningChild();
    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", OK)));
    await waitInvocation(dir, C1, (i) => !!i.end);

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", '{"status_code":"FAILED"}')));
    await quiet();

    const inv = invocationOf(dir, C1) as InvocationView;
    expect(byEvent(inv.records, "invocation_end")).toHaveLength(1);
    expect(inv.end?.response).toBe(OK);
  });

  it("ignores the child's session.deleted: it ends nothing", async () => {
    const { host, dir } = await runningChild();

    await waitInvocation(dir, C1);
    await host.emit({ type: "session.deleted", location: { directory: dir }, data: { sessionID: C1 } });
    await quiet();

    expect(invocationOf(dir, C1)?.end).toBeUndefined();
  });
});

describe("joining a child to its dispatch", () => {
  it("writes invocation_start before invocation_end when the wake-up is the first record due", async () => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_1", input));
    await host.fireAfter(afterPayload(ROOT, "call_1", input, C1));

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(lifecycle(inv.records)).toEqual(["invocation_start", "invocation_end"]);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_1" });
  });

  it("joins through result.output.sessionID when the result has no metadata", async () => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_1", input));
    await host.fireAfter({ tool: "subagent", sessionID: ROOT, id: "call_1", input, status: "completed", result: { output: { sessionID: C1, status: "running" } } });

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_1" });
  });

  it("joins through a session.tool.progress event when the launch result never named the child", async () => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_1", input));
    await host.emit(progress(dir, ROOT, "call_1", C1));

    await host.emit(wakeUp(ROOT, C1, "Worker", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.start).toMatchObject({ agent_instance_id: "Worker#1", spawn_call_id: "call_1" });
  });

  it.each([
    ["an error status", (input: object) => afterPayload(ROOT, "call_1", input as never, undefined, "error")],
    ["a completed result that names no child", (input: object) => afterPayload(ROOT, "call_1", input as never, undefined)],
  ])("discards the dispatch after %s: no invocation results and a later child does not take it", async (_label, makeAfter) => {
    const { host, dir } = await startRoot();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };
    await host.fireBefore(beforePayload(ROOT, "call_1", input));
    await host.fireAfter(makeAfter(input));
    await quiet();
    expect(invocations(dir)).toEqual([]);

    await host.emit(created(dir, C2, ROOT, "Worker"));
    await host.emit(wakeUp(ROOT, C2, "Worker", "completed", wrapWakeUp(C2, "completed", OK)));

    const inv = await waitInvocation(dir, C2, (i) => !!i.end);
    expect(inv.start?.agent_instance_id).toMatch(FALLBACK_ID);
    expect(inv.start).not.toHaveProperty("spawn_call_id");
    expect(inv.start).not.toHaveProperty("prompt");
  });
});

describe("a child without a dispatch", () => {
  it("gets a fallback instance id and the agent type from session.created, with no prompt or call id", async () => {
    const { host, dir } = await startRoot();

    await host.emit(created(dir, C1, ROOT, "Reviewer"));
    await host.emit(wakeUp(ROOT, C1, "Reviewer", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.start?.agent_instance_id).toMatch(FALLBACK_ID);
    expect(inv.start?.agent_type).toBe("Reviewer");
    expect(inv.start).not.toHaveProperty("prompt");
    expect(inv.start).not.toHaveProperty("spawn_call_id");
    expect(inv.end?.agent_instance_id).toBe(inv.start?.agent_instance_id);
    expect(lifecycle(inv.records)).toEqual(["invocation_start", "invocation_end"]);
  });

  it("is known from a wake-up alone, taking the agent type from its metadata", async () => {
    const { host, dir } = await startRoot();

    await host.emit(wakeUp(ROOT, C1, "Auditor", "completed", wrapWakeUp(C1, "completed", OK)));

    const inv = await waitInvocation(dir, C1, (i) => !!i.end);
    expect(inv.start?.agent_type).toBe("Auditor");
    expect(inv.start?.agent_instance_id).toMatch(FALLBACK_ID);
    expect(inv.run).toBe("unknown-run");
  });

  it("never shares a folder with another unresolved invocation", async () => {
    const { host, dir } = await startRoot();

    await host.emit(created(dir, C1, ROOT, "Reviewer"));
    await host.emit(created(dir, C2, ROOT, "Reviewer"));
    await host.emit(wakeUp(ROOT, C1, "Reviewer", "completed", wrapWakeUp(C1, "completed", "one")));
    await host.emit(wakeUp(ROOT, C2, "Reviewer", "completed", wrapWakeUp(C2, "completed", "two")));

    await eventually(() => {
      const all = invocations(dir);
      expect(all).toHaveLength(2);
      expect(new Set(all.map((i) => i.folder)).size).toBe(2);
      expect(all.map((i) => i.end?.response).sort()).toEqual(["one", "two"]);
    });
  });
});
