/**
 * Compactions: one self-contained compaction event and one compaction usage
 * record per ended compaction. Started, delta and failed compactions write
 * nothing; the compaction hook is an ownership signal only.
 *
 * The payload shapes come from source-reading research, not from the spike.
 */

import { describe, expect, it } from "vitest";

import { RUN_A, ROOT, dispatchChild, execution, missionPrompt, userInbox } from "./events";
import { allRecords, byEvent, envelopeProblems, invocationOf, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";
import { compactionEnded, compactionOther, stepEnded, tokens } from "./step_events";

const env = useTestEnv();
const CHILD = "ses_child0000000000000000001";

async function startRoot() {
  const running = await env.start();
  await running.host.emit(userInbox(running.dir, ROOT, "go"));
  return running;
}

const stream = (dir: string) => orchestratorStream(dir, UNKNOWN_RUN);

describe("ended compactions", () => {
  it("writes a compaction event with the trigger and then a compaction usage record", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto", tokens(900, 100, 20, 50, 5), "gpt-6-luna") });

    const records = stream(dir).filter((r) => r.event === "compaction" || r.event === "usage_record");
    expect(records.map((r) => r.event)).toEqual(["compaction", "usage_record"]);
    expect(records[0]).toMatchObject({ session_id: ROOT, trigger: "auto" });
    for (const absent of ["phase", "tokens_before", "tokens_after"]) expect(absent in records[0]).toBe(false);
    expect(records[1]).toMatchObject({
      call_kind: "compaction",
      record_id: "compaction:evt_c1",
      source: "orchestrator_transcript",
      model: "gpt-6-luna",
      token_usage: { input_tokens: 900, output_tokens: 120, reasoning_tokens: 20, cache_read_tokens: 50, cache_creation_tokens: 5 },
    });
    expect("requested_tools" in records[1]).toBe(false);
    expect(records.flatMap(envelopeProblems)).toEqual([]);
  });

  it("takes model and provider from a model object", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "manual", tokens(1, 1), { id: "gpt-6-luna", providerID: "openai" }) });

    const rec = byEvent(stream(dir), "usage_record")[0];
    expect(rec).toMatchObject({ model: "gpt-6-luna", provider: "openai" });
    expect(byEvent(stream(dir), "compaction")[0].trigger).toBe("manual");
  });

  it("writes the compaction without a trigger when the reason is neither auto nor manual", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "overflow", tokens(1, 1)) });

    const [compaction] = byEvent(stream(dir), "compaction");
    expect(compaction).toBeDefined();
    expect("trigger" in compaction).toBe(false);
  });

  it("writes only the compaction event when the report carries no tokens", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto") });

    expect(byEvent(stream(dir), "compaction")).toHaveLength(1);
    expect(byEvent(stream(dir), "usage_record")).toEqual([]);
  });

  it("writes no usage record when the event has no id of its own", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw(compactionEnded(dir, ROOT, "auto", tokens(1, 1)));

    expect(byEvent(stream(dir), "compaction")).toHaveLength(1);
    expect(byEvent(stream(dir), "usage_record")).toEqual([]);
  });

  it("continues the record numbering of the session's step usage records", async () => {
    const { host, dir } = await startRoot();
    await host.emit(stepEnded(dir, ROOT, "msg_1", "tool-calls", tokens(10, 1)));

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto", tokens(5, 5)) });

    expect(byEvent(stream(dir), "usage_record").map((r) => [r.record_id, r.record_index])).toEqual([["msg_1", 0], ["compaction:evt_c1", 1]]);
  });

  it("never writes a cost field", async () => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_c1", ...compactionEnded(dir, ROOT, "auto", { ...tokens(5, 5), cost: 4 }) });

    expect(JSON.stringify(allRecords(dir))).not.toMatch(/"cost"/);
  });

  it("writes a child's compaction into its invocation stream with the instance id", async () => {
    const { host, dir } = await startRoot();
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_p", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });

    await host.emitRaw({ id: "evt_c9", ...compactionEnded(dir, CHILD, "auto", tokens(3, 3)) });

    await eventually(() => {
      const records = invocationOf(dir, CHILD)?.records ?? [];
      expect(byEvent(records, "compaction")).toHaveLength(1);
      expect(byEvent(records, "usage_record")[0]).toMatchObject({ call_kind: "compaction", source: "agent_transcript", agent_instance_id: "Worker#1" });
    });
    expect(byEvent(orchestratorStream(dir, RUN_A), "compaction")).toEqual([]);
  });
});

describe("compactions that did not end", () => {
  it.each(["started", "delta", "failed"] as const)("writes nothing for a %s compaction", async (kind) => {
    const { host, dir } = await startRoot();

    await host.emitRaw({ id: "evt_x", ...compactionOther(dir, ROOT, kind) });
    await quiet();

    expect(byEvent(allRecords(dir), "compaction")).toEqual([]);
    expect(byEvent(allRecords(dir), "usage_record")).toEqual([]);
  });
});

describe("compaction hook", () => {
  it("writes nothing and never sets a result", async () => {
    const { host, dir } = await startRoot();
    const event: Record<string, unknown> = { sessionID: ROOT, reason: "auto" };
    const before = allRecords(dir).length;

    await host.fireCompaction(event);
    await quiet();

    expect("result" in event).toBe(false);
    expect(allRecords(dir)).toHaveLength(before);
  });

  it("makes its session owned, so its later events are logged", async () => {
    const { host, dir } = await env.start();

    await host.fireCompaction({ sessionID: ROOT });
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(stream(dir), "session_end").map((r) => r.session_id)).toEqual([ROOT]);
    });
  });

  it.each([null, undefined, 3, "x", {}, { sessionID: 4 }])("resolves normally for the payload %j", async (payload) => {
    const { host } = await startRoot();

    await expect(host.fireCompaction(payload)).resolves.toBeUndefined();
  });
});
