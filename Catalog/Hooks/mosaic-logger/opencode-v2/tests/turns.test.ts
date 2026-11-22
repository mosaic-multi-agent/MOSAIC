/**
 * Turns: user turns from user inbox items, assistant turns from the text of
 * each step. Synthetic wake-ups and reasoning text are never turns.
 */

import { describe, expect, it } from "vitest";

import { RUN_A, ROOT, dispatchChild, located, missionPrompt, wakeUp, wrapWakeUp, CHILD_PREFIX } from "./events";
import { allRecords, byEvent, envelopeProblems, invocationOf, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import { reasoningEnded, stepEnded, stepFailed, stepStarted, textEnded, tokens } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";

const turnsOf = (dir: string) => byEvent(orchestratorStream(dir, UNKNOWN_RUN), "turn");

function userItem(dir: string, session: string, inboxID: string, text: unknown, type = "user") {
  return located(dir, "session.inbox.enqueued", { sessionID: session, inboxID, item: { type, payload: { text }, delivery: "steer" } });
}

describe("user turns", () => {
  it("writes the text of a user inbox item verbatim", async () => {
    const { host, dir } = await env.start();

    await host.emit(userItem(dir, ROOT, "msg_u1", "  Please fix the build.\nThanks  "));

    const [turn] = turnsOf(dir);
    expect(turn).toMatchObject({ role: "user", content: "  Please fix the build.\nThanks  ", session_id: ROOT });
    expect(envelopeProblems(turn)).toEqual([]);
  });

  it("writes one turn per inbox id, however often the item is delivered", async () => {
    const { host, dir } = await env.start();

    await host.emit(userItem(dir, ROOT, "msg_u1", "same"));
    await host.emit(userItem(dir, ROOT, "msg_u1", "same"));
    await host.emit(userItem(dir, ROOT, "msg_u2", "next"));

    expect(turnsOf(dir).map((t) => t.content)).toEqual(["same", "next"]);
  });

  it.each([["empty", ""], ["not a string", 12]])("writes no turn when the text is %s", async (_name, text) => {
    const { host, dir } = await env.start();

    await host.emit(userItem(dir, ROOT, "msg_u1", text));
    await quiet();

    expect(turnsOf(dir)).toEqual([]);
  });

  it("writes no turn for a synthetic subagent wake-up", async () => {
    const { host, dir } = await env.start();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emit(wakeUp(ROOT, CHILD, "Worker", "completed", wrapWakeUp(CHILD, "completed", "done")));

    await eventually(() => {
      expect(byEvent(invocationOf(dir, CHILD)?.records ?? [], "invocation_end")).toHaveLength(1);
    });
    expect(byEvent(orchestratorStream(dir, RUN_A), "turn")).toEqual([]);
  });

  it("writes a child's user turn into the child's invocation stream", async () => {
    const { host, dir } = await env.start();

    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await eventually(() => {
      const turns = byEvent(invocationOf(dir, CHILD)?.records ?? [], "turn");
      expect(turns).toHaveLength(1);
      expect(turns[0]).toMatchObject({ role: "user", session_id: CHILD });
      expect(String(turns[0].content)).toContain(CHILD_PREFIX.trim());
    });
  });
});

describe("assistant turns", () => {
  async function started() {
    const running = await env.start();
    await running.host.emit(userItem(running.dir, ROOT, "msg_u1", "go"));
    return running;
  }

  it("writes the step's text with the step's model and provider, and no token usage", async () => {
    const { host, dir } = await started();
    await host.emit(stepStarted(dir, ROOT, "msg_1", { id: "gpt-6-luna", providerID: "openai" }));
    await host.emit(textEnded(dir, ROOT, "msg_1", "All done."));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(5, 5)));

    const turns = turnsOf(dir).filter((t) => t.role === "assistant");
    expect(turns).toHaveLength(1);
    expect(turns[0]).toMatchObject({ role: "assistant", content: "All done.", model: "gpt-6-luna", provider: "openai" });
    expect("token_usage" in turns[0]).toBe(false);
    expect(envelopeProblems(turns[0])).toEqual([]);
  });

  it("joins several texts by ordinal with a newline, and by arrival order for equal ordinals", async () => {
    const { host, dir } = await started();
    await host.emit(textEnded(dir, ROOT, "msg_1", "second", 1));
    await host.emit(textEnded(dir, ROOT, "msg_1", "first", 0));
    await host.emit(textEnded(dir, ROOT, "msg_2", "alpha", 0));
    await host.emit(textEnded(dir, ROOT, "msg_2", "beta", 0));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));
    await host.emit(stepEnded(dir, ROOT, "msg_2", "stop", tokens(1, 1)));

    expect(turnsOf(dir).filter((t) => t.role === "assistant").map((t) => t.content)).toEqual(["first\nsecond", "alpha\nbeta"]);
  });

  it("skips empty text values and writes no turn when no text is left", async () => {
    const { host, dir } = await started();
    await host.emit(textEnded(dir, ROOT, "msg_1", "", 0));
    await host.emit(textEnded(dir, ROOT, "msg_2", "kept", 0));
    await host.emit(textEnded(dir, ROOT, "msg_2", "", 1));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));
    await host.emit(stepEnded(dir, ROOT, "msg_2", "stop", tokens(1, 1)));

    expect(turnsOf(dir).filter((t) => t.role === "assistant").map((t) => t.content)).toEqual(["kept"]);
  });

  it("writes at most one turn per assistant message", async () => {
    const { host, dir } = await started();
    await host.emit(textEnded(dir, ROOT, "msg_1", "once"));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));
    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));

    expect(turnsOf(dir).filter((t) => t.role === "assistant")).toHaveLength(1);
  });

  it("writes the turn of a failed step too", async () => {
    const { host, dir } = await started();
    await host.emit(textEnded(dir, ROOT, "msg_f", "partial answer"));

    await host.emit(stepFailed(dir, ROOT, "msg_f"));

    expect(turnsOf(dir).filter((t) => t.role === "assistant").map((t) => t.content)).toEqual(["partial answer"]);
  });

  it("never logs reasoning text", async () => {
    const { host, dir } = await started();
    await host.emit(reasoningEnded(dir, ROOT, "msg_1", "secret chain of thought"));
    await host.emit(textEnded(dir, ROOT, "msg_1", "visible"));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));

    expect(JSON.stringify(allRecords(dir))).not.toContain("secret chain of thought");
    expect(turnsOf(dir).filter((t) => t.role === "assistant").map((t) => t.content)).toEqual(["visible"]);
  });

  it("writes the step's turn before its usage record", async () => {
    const { host, dir } = await started();
    await host.emit(textEnded(dir, ROOT, "msg_1", "answer"));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));

    const events = orchestratorStream(dir, UNKNOWN_RUN).map((r) => r.event);
    const turnAt = events.lastIndexOf("turn");
    const usageAt = events.indexOf("usage_record");
    expect(usageAt).toBeGreaterThan(turnAt);
  });
});
