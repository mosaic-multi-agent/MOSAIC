/**
 * Transcript rules around the export: the orchestrator session guard, results
 * that are not written, one export at a time, cleanup, ids that cannot be
 * passed safely, and Runner mode.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { describe, expect, it, vi } from "vitest";

import { RUN_A, ROOT, execution, missionPrompt, userInbox } from "./events";
import { useFakeOpencode } from "./fake_opencode";
import { eventually, quiet, sleep, useTestEnv } from "./harness";
import { UNKNOWN_RUN, byEvent, invocationOf, type InvocationView } from "./log_reader";
import {
  CHILD,
  CHILD_EXPORT,
  PARENT,
  PARENT_EXPORT,
  complete,
  readBytes,
  readSidecar,
  runDir,
  settle,
  startAndDispatch,
  transcriptFiles,
} from "./transcript_support";
import { dispatchChild } from "./events";
import { stepEnded, stepStarted, textEnded, tokens } from "./step_events";

vi.setConfig({ testTimeout: 30000 });

const env = useTestEnv();
const fake = useFakeOpencode();

const ORCH_RAW = (dir: string): string => nodePath.join(runDir(dir), "00_orchestrator_session.raw");
const ORCH_META = (dir: string): string => nodePath.join(runDir(dir), "00_orchestrator_session.meta.json");
const SECOND_CHILD = "ses_child00000000000000b02";
const UNBOUND_ROOT_RAW = (dir: string): string =>
  nodePath.join(dir, "OrchestrationLogs", UNKNOWN_RUN, `00_orchestrator_session__opencode-v2__${PARENT}.raw`);

function seedOrchestratorTarget(dir: string, meta: Record<string, unknown>): void {
  nodeFs.mkdirSync(runDir(dir), { recursive: true });
  nodeFs.writeFileSync(ORCH_RAW(dir), "OLD-TRANSCRIPT");
  nodeFs.writeFileSync(ORCH_META(dir), JSON.stringify(meta));
}

describe("orchestrator session guard", () => {
  it("keeps an existing transcript whose sidecar names a different session, and still writes the child's", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const dir = env.workspace();
    const oldMeta = { harness: "opencode-v2", session_id: "ses_other", byte_count: 14 };
    seedOrchestratorTarget(dir, oldMeta);
    const { host } = await env.start({ dir });

    await startAndDispatch(host, dir);
    await complete(host);

    await eventually(() => expect(readBytes(nodePath.join(runDir(dir), "Worker#1", "04_session.raw"))).toBeDefined(), 8000);
    await settle(fake.starts, 3);
    expect(readBytes(ORCH_RAW(dir))?.toString("utf-8")).toBe("OLD-TRANSCRIPT");
    expect(readSidecar(ORCH_META(dir))).toEqual(oldMeta);
  });

  it("does not block on a sidecar that has no session id", async () => {
    fake.serve(PARENT, PARENT_EXPORT);
    const dir = env.workspace();
    seedOrchestratorTarget(dir, { harness: "opencode", native_format: "opencode-sdk-session-messages-json" });
    const { host } = await env.start({ dir });

    await startAndDispatch(host, dir);

    await eventually(() => expect(readBytes(ORCH_RAW(dir))?.equals(PARENT_EXPORT)).toBe(true), 8000);
    expect((readSidecar(ORCH_META(dir)) ?? {}).session_id).toBe(PARENT);
  });

  it("does not block on a sidecar that is not readable JSON", async () => {
    fake.serve(PARENT, PARENT_EXPORT);
    const dir = env.workspace();
    nodeFs.mkdirSync(runDir(dir), { recursive: true });
    nodeFs.writeFileSync(ORCH_META(dir), "{not json");
    const { host } = await env.start({ dir });

    await startAndDispatch(host, dir);

    await eventually(() => expect(readBytes(ORCH_RAW(dir))?.equals(PARENT_EXPORT)).toBe(true), 8000);
  });
});

describe("exports that write nothing", () => {
  async function dispatchAndComplete(): Promise<{ dir: string; inv: () => InvocationView | undefined }> {
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await complete(host);
    return { dir, inv: () => invocationOf(dir, CHILD) };
  }

  it("a document whose info.id is another session is not written", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, CHILD_EXPORT);
    const { dir } = await dispatchAndComplete();

    await settle(fake.starts, 3);

    expect(transcriptFiles(dir).filter((f) => f.includes("00_orchestrator"))).toEqual([]);
    expect(readBytes(nodePath.join(runDir(dir), "Worker#1", "04_session.raw"))?.equals(CHILD_EXPORT)).toBe(true);
  });

  it("a document that is not a JSON object with a messages array is not written", async () => {
    fake.serve(CHILD, "this is not json");
    fake.serve(PARENT, JSON.stringify({ info: { id: PARENT } }));
    const { dir } = await dispatchAndComplete();

    await settle(fake.starts, 3);

    expect(transcriptFiles(dir)).toEqual([]);
  });

  it("a non-zero exit writes nothing, even when a valid document was printed, and is not retried", async () => {
    fake.serve(CHILD, CHILD_EXPORT, 2);
    fake.serve(PARENT, PARENT_EXPORT, 2);
    const { dir } = await dispatchAndComplete();

    await settle(fake.starts, 3);

    expect(transcriptFiles(dir)).toEqual([]);
    expect(fake.starts().filter((c) => c.args[2] === CHILD)).toHaveLength(1);
  });

  it("an opencode command that is not on the PATH writes nothing and does not disturb the event streams", async () => {
    fake.removeFromPath();
    const { dir, inv } = await dispatchAndComplete();

    await eventually(() => expect(inv()?.end).toBeDefined());
    await quiet(800);

    expect(transcriptFiles(dir)).toEqual([]);
    expect(byEvent(inv()?.records ?? [], "invocation_end")).toHaveLength(1);
  });
});

describe("queue", () => {
  it("runs one export at a time", async () => {
    fake.delay(250);
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(SECOND_CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await dispatchChild(host, { dir, parent: PARENT, child: SECOND_CHILD, callId: "call_2", agent: "Worker", prompt: missionPrompt("Worker#2", RUN_A) });
    await complete(host);
    await complete(host, SECOND_CHILD);

    await settle(fake.starts, 4);

    await eventually(() => expect(fake.ends().length).toBe(fake.starts().length), 8000);
    const starts = fake.starts().map((c) => c.at);
    const ends = fake.ends().map((c) => c.at);
    for (let i = 1; i < starts.length; i++) expect(starts[i]).toBeGreaterThanOrEqual(ends[i - 1]);
  });

  it("queues a session that is already waiting only once, however often it is requested", async () => {
    const OTHER_ROOT = "ses_other_root0000000000c01";
    fake.delay(500);
    fake.serve(OTHER_ROOT, JSON.stringify({ info: { id: OTHER_ROOT }, messages: [] }));
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, OTHER_ROOT, "talk"));
    await host.emit(userInbox(dir, PARENT, "talk"));
    await host.emit(execution(OTHER_ROOT, "succeeded"));
    await eventually(() => expect(fake.starts()).toHaveLength(1), 8000);

    for (let i = 0; i < 4; i++) await host.emit(execution(PARENT, "succeeded"));

    await eventually(() => expect(fake.ends().length).toBeGreaterThanOrEqual(2), 8000);
    await quiet(1200);
    expect(fake.starts().filter((c) => c.args[2] === PARENT)).toHaveLength(1);
  });

  it("queues a session whose export is running once more, so its latest state is exported", async () => {
    fake.delay(500);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, PARENT, "talk"));
    await host.emit(execution(PARENT, "succeeded"));
    await eventually(() => expect(fake.starts()).toHaveLength(1), 8000);
    const newer = Buffer.from(JSON.stringify({ info: { id: PARENT }, messages: [{ type: "user" }, { type: "idle" }] }));
    fake.serve(PARENT, newer);

    await host.emit(execution(PARENT, "succeeded"));
    await host.emit(execution(PARENT, "succeeded"));

    await eventually(() => expect(fake.ends()).toHaveLength(2), 8000);
    await quiet(1200);
    expect(fake.starts().filter((c) => c.args[2] === PARENT)).toHaveLength(2);
    expect(fake.ends()).toHaveLength(2);
    expect(readBytes(UNBOUND_ROOT_RAW(dir))?.equals(newer)).toBe(true);
  });

  it("drops the running export at cleanup and writes nothing afterwards", async () => {
    fake.delay(900);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir, cleanup } = await env.start();
    await host.emit(userInbox(dir, PARENT, "talk"));
    await host.emit(execution(PARENT, "succeeded"));
    await eventually(() => expect(fake.starts()).toHaveLength(1), 8000);

    cleanup();
    await sleep(1800);

    expect(fake.ends()).toEqual([]);
    expect(transcriptFiles(dir)).toEqual([]);
  });
});

describe("session ids that cannot be passed safely", () => {
  const HOSTILE = "ses_a&b b";

  it("are passed as one argument, or not exported at all when a shell would be needed", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, HOSTILE, "talk"));

    await host.emit(execution(HOSTILE, "succeeded"));

    await quiet(1200);
    const calls = fake.starts();
    if (process.platform === "win32") {
      expect(calls).toEqual([]);
    } else {
      expect(calls).toHaveLength(1);
      expect(calls[0].args).toEqual(["session", "export", HOSTILE, "--standalone"]);
    }
  });
});

describe("Runner mode", () => {
  async function primaryWithChild(role: "orchestrator" | "subagent"): Promise<string> {
    process.env.MOSAIC_ROLE = role;
    process.env.MOSAIC_RUN_ID = RUN_A;
    if (role === "subagent") process.env.MOSAIC_AGENT_INSTANCE_ID = "Worker#7";
    fake.serve(ROOT, JSON.stringify({ info: { id: ROOT }, messages: [] }));
    fake.serve(CHILD, CHILD_EXPORT);
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "consult"));
    await dispatchChild(host, { dir, parent: ROOT, child: CHILD, callId: "call_r", agent: "Helper", prompt: missionPrompt("Helper#1", RUN_A) });
    await complete(host, CHILD);
    await host.emit(stepStarted(dir, ROOT, "msg_final"));
    await host.emit(textEnded(dir, ROOT, "msg_final", "answer"));
    await host.emit(stepEnded(dir, ROOT, "msg_final", "stop", tokens(10, 5)));
    await host.emit(execution(ROOT, "succeeded"));
    return dir;
  }

  it.each(["orchestrator", "subagent"] as const)("the %s role starts no export process and writes no transcript", async (role) => {
    const dir = await primaryWithChild(role);

    await quiet(1000);

    expect(fake.starts()).toEqual([]);
    expect(transcriptFiles(dir)).toEqual([]);
  });
});
