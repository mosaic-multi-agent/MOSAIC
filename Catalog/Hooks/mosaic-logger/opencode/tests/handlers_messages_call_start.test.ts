/**
 * handlers_messages_call_start.test.ts - the usage_record timestamp is the
 * message's start time (info.time.created, epoch milliseconds), driven through
 * the plugin's real event hook.
 *
 *   - a message with a usable time.created gets that instant as its timestamp
 *   - repeated observations keep one timestamp while the clock advances
 *   - an absent or unusable time.created falls back to the current time
 *   - the turn event keeps the observation's current time
 */

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import * as nodePath from "node:path";

import {
  sessionCreatedEvent,
  assistantMessageEntry,
  messageUpdatedEvent,
} from "./fixtures/opencode_api";
import type { OpenCodeEvent } from "../lib/handlers_session";
import {
  installRunnerTestEnv,
  makeSdk,
  startPlugin,
  userMessageUpdated,
  readJsonl,
  runDir,
  SESSION,
} from "./runner_mode_support";

const testEnv = installRunnerTestEnv();

const MSG = "msg-asst-start-1";

// Realistic epoch-millisecond start time: 2026-11-01T16:15:09.123Z.
const CREATED_MS = Date.UTC(2026, 10, 1, 16, 15, 9, 123);
const CREATED_ISO = "2026-11-01T16:15:09.123Z";

type Row = Record<string, unknown>;

function streamPath(): string {
  return nodePath.join(runDir(testEnv.dir, "unknown-run"), "00_orchestrator_events.jsonl");
}

/** An assistant message.updated observation whose info.time is exactly `time`. */
function observation(time: unknown, messageId = MSG): OpenCodeEvent {
  const entry = assistantMessageEntry("Answer.", { id: messageId, sessionID: SESSION });
  const info = { ...entry.info } as Record<string, unknown>;
  if (time === undefined) delete info["time"];
  else info["time"] = time;
  return messageUpdatedEvent({ info: info as never, parts: entry.parts as never });
}

async function startSession() {
  const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
  await hooks.event({ event: sessionCreatedEvent(SESSION) });
  await hooks.event({ event: userMessageUpdated(SESSION, "Do the work") });
  return hooks;
}

function usageRows(messageId = MSG): Row[] {
  return readJsonl(streamPath()).filter(
    (e) => e["event"] === "usage_record" && e["record_id"] === messageId,
  );
}

describe("usage_record timestamp is the message start time", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("uses time.created, not the observation time", async () => {
    const hooks = await startSession();
    vi.setSystemTime(new Date(CREATED_MS + 60_000));

    await hooks.event({ event: observation({ created: CREATED_MS }) });

    const rows = usageRows();
    expect(rows).toHaveLength(1);
    expect(rows[0]!["timestamp"]).toBe(CREATED_ISO);
  });

  it("keeps one timestamp across observations while the clock advances", async () => {
    const hooks = await startSession();

    vi.setSystemTime(new Date(CREATED_MS + 1_000));
    await hooks.event({ event: observation({ created: CREATED_MS }) });
    vi.setSystemTime(new Date(CREATED_MS + 30_000));
    await hooks.event({ event: observation({ created: CREATED_MS, completed: CREATED_MS + 29_000 }) });
    vi.setSystemTime(new Date(CREATED_MS + 5_400_000));
    await hooks.event({ event: observation({ created: CREATED_MS, completed: CREATED_MS + 29_000 }) });

    const rows = usageRows();
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(row["timestamp"]).toBe(CREATED_ISO);
    }
  });

  it("gives different messages their own start times", async () => {
    const hooks = await startSession();
    const otherMs = CREATED_MS + 120_000;
    vi.setSystemTime(new Date(otherMs + 10_000));

    await hooks.event({ event: observation({ created: CREATED_MS }, "msg-a") });
    await hooks.event({ event: observation({ created: otherMs }, "msg-b") });

    expect(usageRows("msg-a")[0]!["timestamp"]).toBe(CREATED_ISO);
    expect(usageRows("msg-b")[0]!["timestamp"]).toBe(new Date(otherMs).toISOString());
  });

  it("leaves the turn timestamp at the observation time", async () => {
    const hooks = await startSession();
    const now = CREATED_MS + 45_000;
    vi.setSystemTime(new Date(now));

    await hooks.event({ event: observation({ created: CREATED_MS }) });

    const turns = readJsonl(streamPath()).filter(
      (e) => e["event"] === "turn" && e["role"] === "assistant",
    );
    expect(turns).toHaveLength(1);
    expect(turns[0]!["timestamp"]).toBe(new Date(now).toISOString());
  });
});

describe("usage_record timestamp falls back to the current time", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  const NOW = Date.UTC(2026, 10, 1, 17, 0, 0, 456);
  const NOW_ISO = "2026-11-01T17:00:00.456Z";

  const CASES: Array<[string, unknown]> = [
    ["info.time is absent", undefined],
    ["time.created is absent", { completed: CREATED_MS }],
    ["time.created is zero", { created: 0 }],
    ["time.created is negative", { created: -5 }],
    ["time.created is not finite", { created: Number.POSITIVE_INFINITY }],
    ["time.created is NaN", { created: Number.NaN }],
    ["time.created is a numeric string", { created: String(CREATED_MS) }],
    ["time.created is an ISO string", { created: CREATED_ISO }],
    ["time.created is year 10000 or later", { created: 253402300800000 }],
  ];

  it.each(CASES)("uses the current time when %s", async (_name, time) => {
    const hooks = await startSession();
    vi.setSystemTime(new Date(NOW));

    await hooks.event({ event: observation(time) });

    const rows = usageRows();
    expect(rows).toHaveLength(1);
    expect(rows[0]!["timestamp"]).toBe(NOW_ISO);
  });

  it("accepts the last instant before year 10000", async () => {
    const hooks = await startSession();
    vi.setSystemTime(new Date(NOW));

    await hooks.event({ event: observation({ created: 253402300799999 }) });

    expect(usageRows()[0]!["timestamp"]).toBe("9999-12-31T23:59:59.999Z");
  });
});
