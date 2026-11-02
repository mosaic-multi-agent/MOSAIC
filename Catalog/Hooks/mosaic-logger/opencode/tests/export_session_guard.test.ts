/**
 * export_session_guard.test.ts -- A run's shared orchestrator transcript is
 * never replaced by a transcript of a different session.
 *
 * The sidecar next to 00_orchestrator_session.raw records the exporting
 * session. A target whose sidecar names another session is kept untouched;
 * the same session's re-export replaces it; a target with no sidecar, or a
 * sidecar without a usable session id, is unowned and replaced.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";

import { exportSession, exportOrchestratorTranscript } from "../lib/export";
import { LogPaths } from "../lib/core";
import { makeSdkClient } from "./export_support";

const RUN_ID = "20260101T170000Z-a3f9";
const SESS_A = "sess-orch-A";
const SESS_B = "sess-orch-B";
const MESSAGES_A = [{ label: "transcript of A" }];
const MESSAGES_A_LATER = [{ label: "transcript of A" }, { label: "A grew" }];
const MESSAGES_B = [{ label: "transcript of B" }];

let tempDir: string;
let paths: LogPaths;

beforeEach(() => {
  tempDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-export-guard-"));
  paths = new LogPaths(tempDir);
});

afterEach(() => {
  nodeFs.rmSync(tempDir, { recursive: true, force: true });
});

function exportFor(sessionId: string, messages: unknown) {
  return exportOrchestratorTranscript(makeSdkClient({ messagesResult: messages }), sessionId, paths, RUN_ID);
}

function readRaw(): unknown {
  return JSON.parse(nodeFs.readFileSync(paths.orchestratorRaw(RUN_ID), "utf-8"));
}

function readMeta(): Record<string, unknown> {
  return JSON.parse(nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID), "utf-8"));
}

/** Seed a target .raw and sidecar text (sidecar skipped when undefined). */
function seedTarget(rawText: string, metaText: string | undefined): void {
  nodeFs.mkdirSync(nodePath.dirname(paths.orchestratorRaw(RUN_ID)), { recursive: true });
  nodeFs.writeFileSync(paths.orchestratorRaw(RUN_ID), rawText);
  if (metaText !== undefined) nodeFs.writeFileSync(paths.orchestratorRawMeta(RUN_ID), metaText);
}

const SEEDED_RAW = JSON.stringify([{ label: "seeded" }]);

function sidecarText(extra: Record<string, unknown>): string {
  return JSON.stringify({
    harness: "opencode",
    native_format: "opencode-sdk-session-messages-json",
    source_mechanism: "sdk_client_session_messages",
    captured_at: "2026-01-01T17:00:00.000Z",
    byte_count: SEEDED_RAW.length,
    ...extra,
  });
}

describe("orchestrator transcript export -- ownership", () => {
  it("records the exporting session in the sidecar", async () => {
    const result = await exportFor(SESS_A, MESSAGES_A);

    expect(result.ok).toBe(true);
    expect(readMeta()["session_id"]).toBe(SESS_A);
  });

  it("does not replace the first session's transcript or sidecar with a second session's", async () => {
    await exportFor(SESS_A, MESSAGES_A);
    const rawBefore = nodeFs.readFileSync(paths.orchestratorRaw(RUN_ID));
    const metaBefore = nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID));

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(false);
    expect(nodeFs.readFileSync(paths.orchestratorRaw(RUN_ID)).equals(rawBefore)).toBe(true);
    expect(nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID)).equals(metaBefore)).toBe(true);
  });

  it("still replaces transcript and sidecar when the same session re-exports", async () => {
    await exportFor(SESS_A, MESSAGES_A);

    const result = await exportFor(SESS_A, MESSAGES_A_LATER);

    expect(result.ok).toBe(true);
    expect(readRaw()).toEqual(MESSAGES_A_LATER);
    expect(readMeta()["session_id"]).toBe(SESS_A);
  });

  it("keeps a pre-seeded foreign-owned target untouched", async () => {
    seedTarget(SEEDED_RAW, sidecarText({ session_id: SESS_A }));
    const metaBefore = nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID));

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(false);
    expect(readRaw()).toEqual([{ label: "seeded" }]);
    expect(nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID)).equals(metaBefore)).toBe(true);
  });

  it("replaces a pre-seeded target owned by the same session", async () => {
    seedTarget(SEEDED_RAW, sidecarText({ session_id: SESS_A }));

    const result = await exportFor(SESS_A, MESSAGES_A);

    expect(result.ok).toBe(true);
    expect(readRaw()).toEqual(MESSAGES_A);
  });
});

describe("orchestrator transcript export -- unowned targets are replaced", () => {
  it("replaces a .raw that has no sidecar and records the new owner", async () => {
    seedTarget(SEEDED_RAW, undefined);

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(true);
    expect(readRaw()).toEqual(MESSAGES_B);
    expect(readMeta()["session_id"]).toBe(SESS_B);
  });

  it("replaces a target whose sidecar has no session id and records the new owner", async () => {
    seedTarget(SEEDED_RAW, sidecarText({}));

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(true);
    expect(readRaw()).toEqual(MESSAGES_B);
    expect(readMeta()["session_id"]).toBe(SESS_B);
  });

  it("replaces a target whose sidecar session id is an empty string", async () => {
    seedTarget(SEEDED_RAW, sidecarText({ session_id: "" }));

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(true);
    expect(readMeta()["session_id"]).toBe(SESS_B);
  });

  it("replaces a target whose sidecar is not valid JSON", async () => {
    seedTarget(SEEDED_RAW, "{ not json");

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(true);
    expect(readRaw()).toEqual(MESSAGES_B);
    expect(readMeta()["session_id"]).toBe(SESS_B);
  });

  it("replaces a target whose sidecar is JSON but not an object", async () => {
    seedTarget(SEEDED_RAW, JSON.stringify(["session_id", SESS_A]));

    const result = await exportFor(SESS_B, MESSAGES_B);

    expect(result.ok).toBe(true);
    expect(readMeta()["session_id"]).toBe(SESS_B);
  });
});

describe("orchestrator transcript export -- guard ordering and scope", () => {
  it("keeps a foreign-owned target untouched when the foreign session has no messages", async () => {
    seedTarget(SEEDED_RAW, sidecarText({ session_id: SESS_A }));
    const metaBefore = nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID));

    await exportFor(SESS_B, []);

    expect(readRaw()).toEqual([{ label: "seeded" }]);
    expect(nodeFs.readFileSync(paths.orchestratorRawMeta(RUN_ID)).equals(metaBefore)).toBe(true);
  });

  it("reports the source failure, not an ownership refusal, when the SDK call fails", async () => {
    seedTarget(SEEDED_RAW, sidecarText({ session_id: SESS_A }));
    const client = makeSdkClient({ messagesThrows: new Error("SDK down") });

    const result = await exportOrchestratorTranscript(client, SESS_B, paths, RUN_ID);

    expect(result.ok).toBe(false);
    expect(result.reason).toContain("SDK down");
  });

  it("does not guard exports that carry no owner, and writes no session id for them", async () => {
    const target = nodePath.join(tempDir, "04_session.raw");
    nodeFs.writeFileSync(target, SEEDED_RAW);
    nodeFs.writeFileSync(
      nodePath.join(tempDir, "04_session.meta.json"),
      sidecarText({ session_id: SESS_A }),
    );

    const result = await exportSession(makeSdkClient({ messagesResult: MESSAGES_B }), SESS_B, target);

    expect(result.ok).toBe(true);
    expect(JSON.parse(nodeFs.readFileSync(target, "utf-8"))).toEqual(MESSAGES_B);
    const meta = JSON.parse(nodeFs.readFileSync(nodePath.join(tempDir, "04_session.meta.json"), "utf-8"));
    expect(meta).not.toHaveProperty("session_id");
  });
});
