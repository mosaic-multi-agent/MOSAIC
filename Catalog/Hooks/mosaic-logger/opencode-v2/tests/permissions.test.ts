/**
 * Permission prompts: only an ask evaluation writes a notification; an ask is
 * paired with its reply through the ephemeral asked/replied events.
 */

import { describe, expect, it } from "vitest";

import { RUN_A, ROOT, dispatchChild, execution, missionPrompt, userInbox } from "./events";
import { allRecords, byEvent, envelopeProblems, invocationOf, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import { evaluatePayload, permissionAsked, permissionReplied } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

const notifications = (dir: string) => byEvent(orchestratorStream(dir, UNKNOWN_RUN), "notification");

describe("permission hook registration", () => {
  it("registers the evaluate hook and the compaction session hook once each", async () => {
    const { host } = await env.start();

    expect(host.otherHooks).toContainEqual({ domain: "permission", name: "evaluate" });
    expect(host.otherHooks).toContainEqual({ domain: "session", name: "compaction" });
    expect(host.otherHooks.filter((h) => h.name === "evaluate")).toHaveLength(1);
    expect(host.otherHooks.filter((h) => h.name === "compaction")).toHaveLength(1);
  });

  it("still registers the tool hooks and subscribes when the permission and session domains are missing", async () => {
    const { host } = await env.start({ omit: ["permission", "session"] });

    expect(host.beforeHooks).toHaveLength(1);
    expect(host.subscribeCalls).toHaveLength(1);
  });
});

describe("ask evaluations", () => {
  it("writes one permission_prompt notification for an ask", async () => {
    const { host, dir } = await startRoot();

    await host.fireEvaluate(evaluatePayload(ROOT, "ask", { message: "Allow editing src/a.ts?" }));

    const found = notifications(dir);
    expect(found).toHaveLength(1);
    expect(found[0]).toMatchObject({
      session_id: ROOT,
      notification_type: "permission_prompt",
      message: "Allow editing src/a.ts?",
      requires_response: true,
      notification_id: "call_perm",
    });
    expect(envelopeProblems(found[0])).toEqual([]);
  });

  it("builds the message from the action and the joined resources when the event has none", async () => {
    const { host, dir } = await startRoot();

    await host.fireEvaluate(evaluatePayload(ROOT, "ask", { resources: ["a.ts", "b.ts"] }));

    expect(notifications(dir)[0].message).toBe("edit a.ts, b.ts");
  });

  it("omits the message and the notification id when neither can be derived", async () => {
    const { host, dir } = await startRoot();

    await host.fireEvaluate({ sessionID: ROOT, effect: "ask", source: {} });

    const [note] = notifications(dir);
    expect(note).toMatchObject({ notification_type: "permission_prompt", requires_response: true });
    expect("message" in note).toBe(false);
    expect("notification_id" in note).toBe(false);
  });

  it("writes a child session's ask into the child's invocation stream", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.fireEvaluate(evaluatePayload(CHILD, "ask"));

    await eventually(() => {
      expect(byEvent(invocationOf(dir, CHILD)?.records ?? [], "notification")).toHaveLength(1);
    });
    expect(byEvent(orchestratorStream(dir, RUN_A), "notification")).toEqual([]);
  });
});

describe("evaluations that need no human", () => {
  it.each(["allow", "deny", undefined, 5])("writes no record for effect %j", async (effect) => {
    const { host, dir } = await startRoot();

    await host.fireEvaluate(evaluatePayload(ROOT, effect as string));
    await quiet();

    expect(notifications(dir)).toEqual([]);
  });

  it("still makes the session owned, so its later events are logged", async () => {
    const { host, dir } = await env.start();

    await host.fireEvaluate(evaluatePayload(ROOT, "allow"));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const stream = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(stream, "session_start").map((r) => r.session_id)).toEqual([ROOT]);
      expect(byEvent(stream, "session_end")).toHaveLength(1);
    });
  });
});

describe("callback behaviour", () => {
  it("never changes the effect or the message of the event", async () => {
    const { host } = await startRoot();
    const event = evaluatePayload(ROOT, "ask", { message: "keep me" });
    const before = JSON.stringify(event);

    await host.fireEvaluate(event);

    expect(JSON.stringify(event)).toBe(before);
  });

  it.each([null, undefined, 7, "text", {}, { sessionID: 5, effect: {}, resources: "x" }, { effect: "ask" }])(
    "resolves normally for the payload %j",
    async (payload) => {
      const { host } = await startRoot();

      await expect(host.fireEvaluate(payload)).resolves.toBeUndefined();
    },
  );
});

describe("asked and replied pairing", () => {
  async function askedRoot() {
    const running = await startRoot();
    await running.host.fireEvaluate(evaluatePayload(ROOT, "ask"));
    return running;
  }

  it("writes a resolution notification with the linked id and the reply verbatim", async () => {
    const { host, dir } = await askedRoot();
    await host.emit(permissionAsked(dir, ROOT, "per_1", "call_perm"));

    await host.emit(permissionReplied(dir, ROOT, "per_1", "always"));

    const found = notifications(dir);
    expect(found).toHaveLength(2);
    expect(found[1]).toMatchObject({ notification_type: "permission_prompt", notification_id: "call_perm", resolution: "always" });
    expect(envelopeProblems(found[1])).toEqual([]);
  });

  it("writes nothing for the asked event itself", async () => {
    const { host, dir } = await askedRoot();

    await host.emit(permissionAsked(dir, ROOT, "per_1", "call_perm"));
    await quiet();

    expect(notifications(dir)).toHaveLength(1);
  });

  it("writes at most one resolution per notification id", async () => {
    const { host, dir } = await askedRoot();
    await host.emit(permissionAsked(dir, ROOT, "per_1", "call_perm"));
    await host.emit(permissionReplied(dir, ROOT, "per_1", "once"));

    await host.emit(permissionReplied(dir, ROOT, "per_1", "reject"));
    await quiet();

    expect(notifications(dir).filter((n) => "resolution" in n).map((n) => n.resolution)).toEqual(["once"]);
  });

  it("writes nothing for a reply whose request was never linked", async () => {
    const { host, dir } = await askedRoot();

    await host.emit(permissionReplied(dir, ROOT, "per_unknown", "once"));
    await quiet();

    expect(notifications(dir)).toHaveLength(1);
  });

  it("does not link an asked event whose call has no ask notification", async () => {
    const { host, dir } = await askedRoot();
    await host.emit(permissionAsked(dir, ROOT, "per_2", "call_other"));

    await host.emit(permissionReplied(dir, ROOT, "per_2", "once"));
    await quiet();

    expect(notifications(dir)).toHaveLength(1);
  });

  it("keeps an ask without any asked event a single, complete record", async () => {
    const { host, dir } = await askedRoot();
    await quiet();

    expect(allRecords(dir).filter((r) => r.event === "notification")).toHaveLength(1);
  });
});
