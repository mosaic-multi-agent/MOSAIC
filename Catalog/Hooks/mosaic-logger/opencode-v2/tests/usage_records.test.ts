/**
 * Usage records: one per step from session.step.ended (or a failed step that
 * carries tokens), mapped per the format, never from the cumulative event and
 * never with a cost.
 */

import { describe, expect, it } from "vitest";

import { RUN_A, ROOT, dispatchChild, missionPrompt, userInbox } from "./events";
import { PROBE_DIR_A, PROBE_ALPHA, loadFixture, replay } from "./fixtures";
import { allRecords, byEvent, envelopeProblems, invocationOf, orchestratorStream, UNKNOWN_RUN, type LogRecord } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import { compactionEnded, stepEnded, stepFailed, stepStarted, tokens, toolInputStarted, usageUpdated } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

const usageOf = (records: LogRecord[]) => byEvent(records, "usage_record");

function keysDeep(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap(keysDeep);
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([key, item]) => [key, ...keysDeep(item)]);
  }
  return [];
}

describe("step usage records", () => {
  it("writes one record per ended step with the mapped tokens, model, provider and requested tools", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepStarted(dir, ROOT, "msg_1", { id: "gpt-6-luna", providerID: "openai" }));
    await host.emit(toolInputStarted(dir, ROOT, "msg_1", "call_a", "read"));
    await host.emit(toolInputStarted(dir, ROOT, "msg_1", "call_b", "read"));
    await host.emit(toolInputStarted(dir, ROOT, "msg_1", "call_c", "shell"));
    await host.emit(toolInputStarted(dir, ROOT, "msg_other", "call_z", "write"));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1000, 40, 10, 200, 30)));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec).toMatchObject({
      session_id: ROOT,
      record_id: "msg_1",
      record_index: 0,
      source: "orchestrator_transcript",
      model: "gpt-6-luna",
      provider: "openai",
      token_usage: { input_tokens: 1000, output_tokens: 50, reasoning_tokens: 10, cache_read_tokens: 200, cache_creation_tokens: 30 },
      requested_tools: { read: 2, shell: 1 },
    });
    expect("call_kind" in rec).toBe(false);
    expect("agent_instance_id" in rec).toBe(false);
    expect(envelopeProblems(rec)).toEqual([]);
  });

  it("writes an empty requested_tools object when the step requested none", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepStarted(dir, ROOT, "msg_1"));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(5, 5)));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec.requested_tools).toEqual({});
  });

  it("numbers the records by distinct record id in the order they were written", async () => {
    const { host, dir } = await startRoot();

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));
    await host.emit(stepEnded(dir, ROOT, "msg_2", "tool-calls", tokens(2, 2)));
    await host.emit(stepEnded(dir, ROOT, "msg_3", "stop", tokens(3, 3)));

    const recs = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(recs.map((r) => [r.record_id, r.record_index])).toEqual([["msg_1", 0], ["msg_2", 1], ["msg_3", 2]]);
  });

  it("keeps the index and the stream of a record id that is observed again", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));
    await host.emit(stepEnded(dir, ROOT, "msg_2", "tool-calls", tokens(2, 2)));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));

    const again = usageOf(allRecords(dir)).filter((r) => r.record_id === "msg_1");
    expect(new Set(again.map((r) => r.record_index))).toEqual(new Set([0]));
    expect(new Set(again.map((r) => r._stream)).size).toBe(1);
  });

  it("writes a record id observed again after the run binding to the stream of its first observation", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(1, 1)));

    const first = usageOf(orchestratorStream(dir, UNKNOWN_RUN)).filter((r) => r.record_id === "msg_1");
    const bound = usageOf(orchestratorStream(dir, RUN_A)).filter((r) => r.record_id === "msg_1");
    expect(first.length).toBeGreaterThanOrEqual(1);
    expect(bound).toEqual([]);
    expect(new Set(first.map((r) => r.record_index))).toEqual(new Set([0]));
  });

  it("writes a compaction record id observed again after the run binding to the stream of its first observation", async () => {
    const { host, dir } = await startRoot();
    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto", tokens(9, 1)) });
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto", tokens(9, 1)) });

    const first = usageOf(orchestratorStream(dir, UNKNOWN_RUN)).filter((r) => r.record_id === "compaction:evt_c1");
    const bound = usageOf(orchestratorStream(dir, RUN_A)).filter((r) => r.record_id === "compaction:evt_c1");
    expect(first.length).toBeGreaterThanOrEqual(1);
    expect(bound).toEqual([]);
    expect(new Set(first.map((r) => r.record_index))).toEqual(new Set([0]));
  });

  it("stamps the step's start time as the record timestamp", async () => {
    const { host, dir } = await startRoot();
    const started = 1794925254870;
    await host.emit(stepStarted(dir, ROOT, "msg_1", undefined, started));

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec.timestamp).toBe(new Date(started).toISOString());
  });

  it("writes the record without model and provider when no step start was seen", async () => {
    const { host, dir } = await startRoot();

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", tokens(1, 1)));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec.record_id).toBe("msg_1");
    expect("model" in rec).toBe(false);
    expect("provider" in rec).toBe(false);
  });

  it("writes a record without token_usage when the step reported no usable tokens", async () => {
    const { host, dir } = await startRoot();

    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", undefined));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec).toMatchObject({ record_id: "msg_1" });
    expect("token_usage" in rec).toBe(false);
  });

  it("falls back to the session and event id for the record id, and writes nothing without either", async () => {
    const { host, dir } = await startRoot();
    const ended = stepEnded(dir, ROOT, "msg_x", "stop", tokens(1, 1));

    await host.emitRaw({ id: "evt_77", ...ended, data: { ...ended.data, assistantMessageID: undefined } });
    await host.emitRaw({ ...ended, data: { ...ended.data, assistantMessageID: undefined } });

    const recs = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(recs.map((r) => r.record_id)).toEqual([`${ROOT}:evt_77`]);
  });
});

describe("failed steps", () => {
  it("writes a usage record for a failed step that carries tokens", async () => {
    const { host, dir } = await startRoot();

    await host.emit(stepFailed(dir, ROOT, "msg_f", tokens(50, 5)));

    const [rec] = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(rec).toMatchObject({ record_id: "msg_f", token_usage: { input_tokens: 50, output_tokens: 5 } });
  });

  it.each([undefined, {}, { cost: 2 }])("writes no record for a failed step whose tokens are %j", async (value) => {
    const { host, dir } = await startRoot();

    await host.emit(stepFailed(dir, ROOT, "msg_f", value));
    await quiet();

    expect(usageOf(allRecords(dir))).toEqual([]);
  });
});

describe("one source per record", () => {
  it("never writes a record from the cumulative usage event, alone or beside the step", async () => {
    const { host, dir } = await startRoot();

    await host.emit(usageUpdated(ROOT, tokens(536, 10)));
    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(536, 10)));
    await host.emit(usageUpdated(ROOT, tokens(1500, 30)));
    await host.emit(stepEnded(dir, ROOT, "msg_2", "stop", tokens(964, 20)));

    const recs = usageOf(orchestratorStream(dir, UNKNOWN_RUN));
    expect(recs.map((r) => r.token_usage.input_tokens)).toEqual([536, 964]);
  });

  it("writes nothing for a cumulative event alone", async () => {
    const { host, dir } = await startRoot();

    await host.emit(usageUpdated(ROOT, tokens(536, 10)));
    await quiet();

    expect(usageOf(allRecords(dir))).toEqual([]);
  });

  it("writes nothing for step events of a session in another directory or of an unowned session without a location", async () => {
    const { host, dir } = await startRoot();
    const foreign = "ses_foreign0000000000000001";
    const unowned = "ses_unowned0000000000000001";

    await host.emit(stepEnded(env.workspace(), foreign, "msg_f1", "stop", tokens(9, 9)));
    await host.emit({ type: "session.step.ended", data: { sessionID: unowned, assistantMessageID: "msg_f2", finish: "stop", tokens: tokens(9, 9) } });
    await quiet();

    expect(usageOf(allRecords(dir))).toEqual([]);
    expect(JSON.stringify(allRecords(dir))).not.toContain(foreign);
    expect(JSON.stringify(allRecords(dir))).not.toContain(unowned);
  });
});

describe("no cost", () => {
  it("never writes a cost field anywhere, whatever V2 reports", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepStarted(dir, ROOT, "msg_1"));
    await host.emit(usageUpdated(ROOT, { ...tokens(1, 1), cost: 9 }));
    await host.emit(stepEnded(dir, ROOT, "msg_1", "stop", { ...tokens(1, 1), cost: 7 }, { cost: 3.2 }));

    await quiet();

    const keys = allRecords(dir).flatMap((r) => keysDeep(r));
    expect(keys.filter((k) => /cost/i.test(k.replace(/cache_creation_tokens/g, "")))).toEqual([]);
    expect(usageOf(allRecords(dir))).toHaveLength(1);
  });
});

describe("child sessions", () => {
  it("writes a child's record into its invocation stream as agent_transcript with the instance id", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    await host.emit(stepStarted(dir, CHILD, "msg_c1"));

    await host.emit(stepEnded(dir, CHILD, "msg_c1", "stop", tokens(11, 3)));

    await eventually(() => {
      const inv = invocationOf(dir, CHILD);
      const [rec] = usageOf(inv?.records ?? []);
      expect(rec).toMatchObject({ source: "agent_transcript", agent_instance_id: "Worker#1", record_id: "msg_c1", record_index: 0, session_id: CHILD });
    });
    expect(usageOf(orchestratorStream(dir, RUN_A))).toEqual([]);
  });

  it("numbers a child's records independently of the root's", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    await host.emit(stepEnded(dir, ROOT, "msg_r1", "tool-calls", tokens(1, 1)));

    await host.emit(stepEnded(dir, CHILD, "msg_c1", "stop", tokens(2, 2)));

    await eventually(() => {
      expect(usageOf(invocationOf(dir, CHILD)?.records ?? [])[0]?.record_index).toBe(0);
    });
  });
});

describe("usage in the spike probe", () => {
  it("writes one record per probe step and sums to the per-step tokens, not the cumulative ones", async () => {
    const { host, dir } = await env.start();
    const records = loadFixture("probe_proj_a_parallel_children.jsonl", { [PROBE_DIR_A]: dir });
    const steps = records.filter((r) => r.type === "session.step.ended").map((r) => r.data as { assistantMessageID: string; tokens: { input: number } });

    await replay(host, records);

    await eventually(() => {
      const recs = usageOf(allRecords(dir));
      expect(new Set(recs.map((r) => r.record_id))).toEqual(new Set(steps.map((s) => s.assistantMessageID)));
      expect(recs).toHaveLength(new Set(steps.map((s) => s.assistantMessageID)).size);
      expect(recs.reduce((sum, r) => sum + r.token_usage.input_tokens, 0)).toBe(steps.reduce((sum, s) => sum + s.tokens.input, 0));
      expect(recs.every((r) => r.model === "gpt-6-luna" && r.provider === "openai")).toBe(true);
      expect(recs.flatMap((r) => envelopeProblems(r))).toEqual([]);
    });
    expect(invocationOf(dir, PROBE_ALPHA)).toBeDefined();
  });
});
