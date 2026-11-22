/**
 * Replays of the sanitized spike probe logs: what the instance of project A
 * writes for two parallel subagents, and what the instance of project B writes
 * when events of both projects are delivered to it.
 */

import { describe, expect, it } from "vitest";

import {
  PROBE_ALPHA,
  PROBE_B_FOREIGN,
  PROBE_B_OWN,
  PROBE_BETA,
  PROBE_CALL_ALPHA,
  PROBE_CALL_BETA,
  PROBE_DIR_A,
  PROBE_DIR_B,
  PROBE_ROOT,
  loadFixture,
  replay,
} from "./fixtures";
import { allRecords, byEvent, envelopeProblems, invocationOf, invocations, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, useTestEnv } from "./harness";

const env = useTestEnv();
const FALLBACK_ID = /^agent_\d{8}T\d{6}Z-[0-9a-f]{4}$/;

describe("project A: a root session with two parallel subagents", () => {
  async function replayA() {
    const running = await env.start();
    const records = loadFixture("probe_proj_a_parallel_children.jsonl", { [PROBE_DIR_A]: running.dir });
    await replay(running.host, records);
    return running;
  }

  it("logs one root start and one root end once both children have finished", async () => {
    const { dir } = await replayA();

    await eventually(() => {
      const stream = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(stream, "session_start").map((r) => r.session_id)).toEqual([PROBE_ROOT]);
      const ends = byEvent(stream, "session_end");
      expect(ends).toHaveLength(1);
      expect(ends[0]).toMatchObject({ session_id: PROBE_ROOT, reason: "session.execution.succeeded" });
      expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
    });
  });

  it("logs one invocation per child, joined to its own dispatch by call id", async () => {
    const { dir } = await replayA();

    await eventually(() => {
      expect(invocations(dir)).toHaveLength(2);
      const alpha = invocationOf(dir, PROBE_ALPHA);
      const beta = invocationOf(dir, PROBE_BETA);
      expect(alpha?.start).toMatchObject({ agent_type: "spike-child", spawn_call_id: PROBE_CALL_ALPHA });
      expect(alpha?.start?.prompt).toContain("Your name is ALPHA.");
      expect(beta?.start).toMatchObject({ agent_type: "spike-child", spawn_call_id: PROBE_CALL_BETA });
      expect(beta?.start?.prompt).toContain("Your name is BETA.");
      expect(alpha?.start?.agent_instance_id).toMatch(FALLBACK_ID);
      expect(beta?.start?.agent_instance_id).toMatch(FALLBACK_ID);
      expect(alpha?.folder).not.toBe(beta?.folder);
    });
  });

  it("ends each invocation at its first completed wake-up and ignores the re-calls and later wake-ups", async () => {
    const { dir } = await replayA();

    await eventually(() => {
      for (const [child, name] of [[PROBE_ALPHA, "ALPHA"], [PROBE_BETA, "BETA"]] as const) {
        const inv = invocationOf(dir, child);
        expect(inv?.records.map((r) => r.event).filter((e) => e === "invocation_start" || e === "invocation_end")).toEqual(["invocation_start", "invocation_end"]);
        expect(inv?.end?.status_code).toBe("SUCCESS");
        expect(JSON.parse(String(inv?.end?.response))).toMatchObject({ status_code: "SUCCESS", child: name });
        expect(inv?.end?.agent_instance_id).toBe(inv?.start?.agent_instance_id);
      }
    });
  });

  it("writes only valid opencode-v2 records, and nothing for the children as root sessions", async () => {
    const { dir } = await replayA();

    await eventually(() => {
      const all = allRecords(dir);
      expect(all.length).toBeGreaterThan(0);
      expect(all.flatMap((rec) => envelopeProblems(rec).map((p) => `${rec.event}: ${p}`))).toEqual([]);
      const rootEvents = all.filter((r) => ["session_start", "session_end"].includes(r.event));
      expect(new Set(rootEvents.map((r) => r.session_id))).toEqual(new Set([PROBE_ROOT]));
    });
  });
});

describe("project B: hears events of project A on the shared service", () => {
  async function replayB() {
    const dirA = env.workspace();
    const running = await env.start();
    const records = loadFixture("probe_proj_b_cross_project.jsonl", { [PROBE_DIR_A]: dirA, [PROBE_DIR_B]: running.dir });
    await replay(running.host, records);
    return { ...running, dirA };
  }

  it("logs its own root session without ever seeing a session.created for it", async () => {
    const { dir } = await replayB();

    await eventually(() => {
      const stream = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(stream, "session_start").map((r) => r.session_id)).toEqual([PROBE_B_OWN]);
      expect(byEvent(stream, "session_end").map((r) => r.session_id)).toEqual([PROBE_B_OWN]);
    });
  });

  it("writes nothing for project A's session, located events or location-less ones", async () => {
    const { dir } = await replayB();

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    expect(JSON.stringify(allRecords(dir))).not.toContain(PROBE_B_FOREIGN);
    expect(invocations(dir)).toEqual([]);
  });
});
