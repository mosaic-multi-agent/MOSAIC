/**
 * Native-mode transcripts: when each export is requested, where the bytes go,
 * and what the sidecar says.
 */

import * as nodePath from "node:path";
import { describe, expect, it, vi } from "vitest";

import { RUN_A, execution, userInbox, wakeUp, wrapWakeUp } from "./events";
import { useFakeOpencode } from "./fake_opencode";
import { eventually, useTestEnv } from "./harness";
import { leftoverFiles, UNKNOWN_RUN } from "./log_reader";
import {
  CHILD,
  CHILD_EXPORT,
  PARENT,
  PARENT_EXPORT,
  SIDECAR_KEYS,
  complete,
  readBytes,
  readSidecar,
  runDir,
  settle,
  startAndDispatch,
  transcriptFiles,
} from "./transcript_support";

vi.setConfig({ testTimeout: 30000 });

const env = useTestEnv();
const fake = useFakeOpencode();

const CHILD_RAW = (dir: string): string => nodePath.join(runDir(dir), "Worker#1", "04_session.raw");
const CHILD_META = (dir: string): string => nodePath.join(runDir(dir), "Worker#1", "04_session.meta.json");
const ORCH_RAW = (dir: string): string => nodePath.join(runDir(dir), "00_orchestrator_session.raw");
const ORCH_META = (dir: string): string => nodePath.join(runDir(dir), "00_orchestrator_session.meta.json");

describe("child transcript", () => {
  it("is the export's bytes verbatim in 04_session.raw once the child's completion wake-up arrives", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);

    await complete(host);

    await eventually(() => expect(readBytes(CHILD_RAW(dir))?.equals(CHILD_EXPORT)).toBe(true), 8000);
  });

  it("has a V2 sidecar without a session id, written beside the transcript", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);

    await complete(host);

    await eventually(() => expect(readSidecar(CHILD_META(dir))).toBeDefined(), 8000);
    const meta = readSidecar(CHILD_META(dir)) as Record<string, unknown>;
    expect(Object.keys(meta)).toEqual(SIDECAR_KEYS);
    expect(meta).toMatchObject({
      harness: "opencode-v2",
      native_format: "opencode-v2-session-export-json",
      source_mechanism: "cli_session_export",
      byte_count: CHILD_EXPORT.length,
    });
    expect(String(meta.captured_at)).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
  });

  it("is not written before the child's completion", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);

    await host.emit(wakeUp(PARENT, CHILD, "Worker", "running", wrapWakeUp(CHILD, "running", "still going")));
    await settle(fake.starts, 1);

    expect(fake.starts().some((c) => c.args[2] === CHILD)).toBe(false);
    expect(readBytes(CHILD_RAW(dir))).toBeUndefined();
  });

  it("is refreshed by a later non-running wake-up for the same child", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await complete(host);
    await eventually(() => expect(readBytes(CHILD_RAW(dir))).toBeDefined(), 8000);
    const newer = Buffer.from(JSON.stringify({ info: { id: CHILD }, messages: [{ type: "user" }] }));
    fake.serve(CHILD, newer);

    await complete(host, CHILD, "second answer");

    await eventually(() => expect(readBytes(CHILD_RAW(dir))?.equals(newer)).toBe(true), 8000);
  });

  it.each(["failed", "cancelled"])("is refreshed by a later %s wake-up for the same child", async (state) => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await complete(host);
    await eventually(() => expect(readBytes(CHILD_RAW(dir))).toBeDefined(), 8000);
    const newer = Buffer.from(JSON.stringify({ info: { id: CHILD }, messages: [{ type: "idle" }] }));
    fake.serve(CHILD, newer);

    await host.emit(wakeUp(PARENT, CHILD, "Worker", state, wrapWakeUp(CHILD, state, "no answer")));

    await eventually(() => expect(readBytes(CHILD_RAW(dir))?.equals(newer)).toBe(true), 8000);
  });
});

describe("orchestrator transcript", () => {
  it("is exported when a child starts: the bound run's 00_orchestrator_session.raw holds the parent export verbatim", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();

    await startAndDispatch(host, dir);

    await eventually(() => expect(readBytes(ORCH_RAW(dir))?.equals(PARENT_EXPORT)).toBe(true), 8000);
  });

  it("has a sidecar that names the exported session", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();

    await startAndDispatch(host, dir);

    await eventually(() => expect(readSidecar(ORCH_META(dir))).toBeDefined(), 8000);
    const meta = readSidecar(ORCH_META(dir)) as Record<string, unknown>;
    expect(Object.keys(meta)).toEqual([...SIDECAR_KEYS, "session_id"]);
    expect(meta).toMatchObject({
      harness: "opencode-v2",
      native_format: "opencode-v2-session-export-json",
      source_mechanism: "cli_session_export",
      byte_count: PARENT_EXPORT.length,
      session_id: PARENT,
    });
  });

  it("is refreshed at the root's session_end, after the last child ended", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await complete(host);
    await settle(fake.starts, 3);
    const later = Buffer.from(JSON.stringify({ info: { id: PARENT }, messages: [{ type: "user" }, { type: "assistant" }] }));
    fake.serve(PARENT, later);

    await host.emit(execution(PARENT, "succeeded"));

    await eventually(() => expect(readBytes(ORCH_RAW(dir))?.equals(later)).toBe(true), 8000);
    const meta = readSidecar(ORCH_META(dir)) as Record<string, unknown>;
    expect(meta.byte_count).toBe(later.length);
  });

  it("goes to a session-scoped name in unknown-run for a root that never bound a run", async () => {
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, PARENT, "just talking"));

    await host.emit(execution(PARENT, "succeeded"));

    const stem = nodePath.join(dir, "OrchestrationLogs", UNKNOWN_RUN, `00_orchestrator_session__opencode-v2__${PARENT}`);
    await eventually(() => expect(readBytes(`${stem}.raw`)?.equals(PARENT_EXPORT)).toBe(true), 8000);
    await eventually(() => expect((readSidecar(`${stem}.meta.json`) ?? {}).session_id).toBe(PARENT), 8000);
  });

  it("leaves no temporary or lock files, and each sidecar's byte_count matches its .raw", async () => {
    fake.serve(CHILD, CHILD_EXPORT);
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await startAndDispatch(host, dir);
    await complete(host);

    await settle(fake.starts, 3);

    expect(transcriptFiles(dir)).toEqual([
      `${RUN_A}/00_orchestrator_session.meta.json`,
      `${RUN_A}/00_orchestrator_session.raw`,
      `${RUN_A}/Worker#1/04_session.meta.json`,
      `${RUN_A}/Worker#1/04_session.raw`,
    ]);
    expect(leftoverFiles(dir)).toEqual([]);
    expect(readSidecar(CHILD_META(dir))?.byte_count).toBe(CHILD_EXPORT.length);
    expect(readSidecar(ORCH_META(dir))?.byte_count).toBe(PARENT_EXPORT.length);
  });
});

describe("export command as run by the plugin", () => {
  it("passes the fixed arguments, the instance directory as cwd and PWD, and the rest of the environment", async () => {
    fake.serve(PARENT, PARENT_EXPORT);
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, PARENT, "talk"));

    await host.emit(execution(PARENT, "succeeded"));

    await settle(fake.starts, 1);
    const call = fake.starts()[0];
    expect(call.args).toEqual(["session", "export", PARENT, "--standalone"]);
    expect(call.cwd).toBe(dir);
    expect(call.pwd).toBe(dir);
    expect(call.mosaicRole).toBeNull();
    expect(call.keepMarker).toBe("kept-through-export");
    expect(call.stdinIsTTY).toBe(false);
  });
});
