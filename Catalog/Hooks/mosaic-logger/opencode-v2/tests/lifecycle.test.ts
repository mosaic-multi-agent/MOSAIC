/**
 * Native session and run lifecycle for root sessions: session_start on the
 * first owned signal, run binding at the first MOSAIC dispatch, and
 * session_end / run_end at each quiescent point.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import {
  ROOT,
  RUN_A,
  RUN_B,
  beforePayload,
  created,
  dispatchChild,
  execution,
  located,
  missionPrompt,
  userInbox,
  wakeUp,
  wrapWakeUp,
} from "./events";
import { allRecords, byEvent, envelopeProblems, orchestratorStream, UNKNOWN_RUN, LOGS } from "./log_reader";
import { eventually, quiet, sleep, useTestEnv } from "./harness";

const env = useTestEnv();
const CHILD_1 = "ses_child00000000000000a01";
const CHILD_2 = "ses_child00000000000000a02";

async function dispatchOnly(host: import("./fake_host").FakeHost, callId: string, prompt: string): Promise<void> {
  await host.fireBefore(beforePayload(ROOT, callId, { agent: "Worker", description: "d", prompt }));
}

describe("session_start", () => {
  it("is written once to unknown-run on the first owned signal, with a valid envelope and no run id", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "first"));
    await host.emit(userInbox(dir, ROOT, "second"));

    await eventually(() => {
      const starts = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start");
      expect(starts).toHaveLength(1);
      expect(starts[0].session_id).toBe(ROOT);
      expect(starts[0].run_id).toBeUndefined();
      expect(envelopeProblems(starts[0])).toEqual([]);
    });
  });

  it("is never accompanied by a run_start without a run id", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "first"));
    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    expect(byEvent(allRecords(dir), "session_start")).toHaveLength(1);
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
  });
});

describe("run binding", () => {
  it("binds at the first dispatch carrying both ids: run_start then session_start go to the run's stream", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "start"));

    await dispatchOnly(host, "call_1", missionPrompt("Worker#1", RUN_A));

    await eventually(() => {
      const records = orchestratorStream(dir, RUN_A);
      expect(records.map((r) => r.event).filter((e) => e !== "tool_call_start" && e !== "tool_call_end")).toEqual(["run_start", "session_start"]);
      expect(records[0].run_id).toBe(RUN_A);
      expect(records[1].session_id).toBe(ROOT);
      expect(records.flatMap(envelopeProblems)).toEqual([]);
    });
  });

  it("carries adapter_version from the manifest's mosaic-logger hook entry", async () => {
    const dir = env.workspace();
    nodeFs.mkdirSync(nodePath.join(dir, ".mosaic"));
    nodeFs.writeFileSync(
      nodePath.join(dir, ".mosaic", "manifest.yaml"),
      "- ref:\n    kind: hook\n    key: mosaic-logger\n  version: 1.12.0\n",
    );
    const { host } = await env.start({ dir });
    await host.emit(userInbox(dir, ROOT, "start"));
    await sleep(300);

    await dispatchOnly(host, "call_1", missionPrompt("Worker#1", RUN_A));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "run_start")[0]?.adapter_version).toBe("1.12.0");
    });
  });

  it("omits adapter_version when there is no manifest", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "start"));

    await dispatchOnly(host, "call_1", missionPrompt("Worker#1", RUN_A));

    await eventually(() => {
      const [start] = byEvent(orchestratorStream(dir, RUN_A), "run_start");
      expect(start).toBeDefined();
      expect("adapter_version" in start).toBe(false);
    });
  });

  it("keeps the first binding: a later dispatch for another run does not rebind or write run_start", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "start"));
    await dispatchOnly(host, "call_1", missionPrompt("Worker#1", RUN_A));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "run_start")).toHaveLength(1);
    });

    await dispatchOnly(host, "call_2", missionPrompt("Worker#2", RUN_B));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, RUN_A), "run_end")).toHaveLength(1);
    });
    expect(byEvent(allRecords(dir), "run_start").map((r) => r.run_id)).toEqual([RUN_A]);
    expect(byEvent(allRecords(dir), "run_end").map((r) => r.run_id)).toEqual([RUN_A]);
    expect(byEvent(orchestratorStream(dir, RUN_B), "session_start")).toEqual([]);
  });

  it.each([
    ["a dispatch with a run id but no instance id", "run_id: " + RUN_A + "\n\nWork."],
    ["a dispatch with an instance id but no run id", missionPrompt("Worker#1", undefined)],
    ["a dispatch with neither", "Please do the work."],
  ])("does not bind on %s", async (_label, prompt) => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "start"));

    await dispatchOnly(host, "call_1", prompt);
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
    });
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
    expect(byEvent(allRecords(dir), "run_end")).toEqual([]);
  });

  it("is not caused by run ids in user text", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, `agent_instance_id: Worker#1\nrun_id: ${RUN_A}`));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
    });
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_A))).toBe(false);
  });

  it("is not caused by a re-call of a running child, even if its prompt carries ids", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "start"));

    await host.fireBefore(
      beforePayload(ROOT, "call_9", { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A), sessionID: CHILD_1 }),
    );
    await quiet();

    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    expect(byEvent(allRecords(dir), "run_start")).toEqual([]);
    expect(nodeFs.existsSync(nodePath.join(dir, LOGS, RUN_A))).toBe(false);
  });
});

describe("session_end and run_end at quiescent points", () => {
  it.each(["succeeded", "failed", "interrupted"] as const)(
    "a terminal %s execution event of an idle root writes session_end with the event type as reason",
    async (outcome) => {
      const { host, dir } = await env.start();
      await host.emit(userInbox(dir, ROOT, "go"));

      await host.emit(execution(ROOT, outcome));

      await eventually(() => {
        const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end");
        expect(ends).toHaveLength(1);
        expect(ends[0].session_id).toBe(ROOT);
        expect(ends[0].reason).toBe(`session.execution.${outcome}`);
        expect(envelopeProblems(ends[0])).toEqual([]);
      });
      expect(byEvent(allRecords(dir), "run_end")).toEqual([]);
    },
  );

  it("a bound root writes session_end and then run_end (without outcome) to the run's stream", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    await dispatchOnly(host, "call_1", missionPrompt("Worker#1", RUN_A));

    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const tail = orchestratorStream(dir, RUN_A).slice(-2);
      expect(tail.map((r) => r.event)).toEqual(["session_end", "run_end"]);
      expect(tail[0].reason).toBe("session.execution.succeeded");
      expect(tail[1].run_id).toBe(RUN_A);
      expect("outcome" in tail[1]).toBe(false);
    });
  });

  it("writes nothing for a second terminal event without activity in between", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));

    await host.emit(execution(ROOT, "succeeded"));
    await host.emit(execution(ROOT, "interrupted"));
    await quiet();

    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
  });

  it("writes one more pair per quiescent point, with a single session_start", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "round one"));
    await host.emit(execution(ROOT, "succeeded"));

    await host.emit(userInbox(dir, ROOT, "round two"));
    await host.emit(execution(ROOT, "failed"));

    await eventually(() => {
      const records = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(records, "session_start")).toHaveLength(1);
      expect(byEvent(records, "session_end").map((r) => r.reason)).toEqual([
        "session.execution.succeeded",
        "session.execution.failed",
      ]);
    });
  });

  it("session.deleted after activity writes the pair with reason session.deleted", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));

    await host.emit(located(dir, "session.deleted", { sessionID: ROOT }));

    await eventually(() => {
      const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end");
      expect(ends).toHaveLength(1);
      expect(ends[0].reason).toBe("session.deleted");
    });
  });

  it("session.deleted without activity since the last end writes nothing", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    await host.emit(execution(ROOT, "succeeded"));

    await host.emit(located(dir, "session.deleted", { sessionID: ROOT }));
    await quiet();

    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
  });
});

describe("root end with open children", () => {
  async function startWithOpenChild() {
    const running = await env.start();
    const { host, dir } = running;
    await host.emit(userInbox(dir, ROOT, "go"));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD_1, callId: "call_1", agent: "Worker", prompt: missionPrompt("Worker#1", RUN_A) });
    return running;
  }

  it("does not end the root while a child invocation is open", async () => {
    const { host, dir } = await startWithOpenChild();

    await host.emit(execution(ROOT, "succeeded"));
    await quiet();

    expect(byEvent(allRecords(dir), "run_start")).toHaveLength(1);
    expect(byEvent(allRecords(dir), "session_end")).toEqual([]);
    expect(byEvent(allRecords(dir), "run_end")).toEqual([]);
  });

  it("ends the root at the first terminal event after the child's invocation has ended", async () => {
    const { host, dir } = await startWithOpenChild();
    await host.emit(execution(ROOT, "succeeded"));
    await host.emit(wakeUp(ROOT, CHILD_1, "Worker", "completed", wrapWakeUp(CHILD_1, "completed", '{"status_code":"SUCCESS"}')));

    await host.emit(execution(ROOT, "started"));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const tail = orchestratorStream(dir, RUN_A).slice(-2);
      expect(tail.map((r) => r.event)).toEqual(["session_end", "run_end"]);
    });
  });

  it("a child's own terminal event never writes a session_end or run_end", async () => {
    const { host, dir } = await startWithOpenChild();

    await host.emit(execution(CHILD_1, "succeeded"));
    await quiet();

    expect(byEvent(allRecords(dir), "session_end")).toEqual([]);
    const started = byEvent(allRecords(dir), "session_start").map((r) => r.session_id);
    expect(started.length).toBeGreaterThan(0);
    expect(new Set(started)).toEqual(new Set([ROOT]));
  });

  it("a child created event never writes a session_start", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));

    await host.emit(created(dir, CHILD_2, ROOT, "Worker"));
    await quiet();

    expect(byEvent(allRecords(dir), "session_start").map((r) => r.session_id)).toEqual([ROOT]);
  });
});
