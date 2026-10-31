/**
 * handlers_messages_requested_tools.test.ts - usage_record requested_tools and
 * schema version, driven through the plugin's real event hook.
 *
 *   - requested_tools holds the tool parts of the assistant message with counts,
 *     whether they arrived on message.updated or on message.part.updated
 *   - a message with no tool parts carries an empty object
 *   - a later observation of a message never drops tools an earlier one carried
 *   - the same behaviour holds in native mode and in Runner mode (the native
 *     message handler under the synthetic runner-root registration)
 *   - usage_record is stamped 1.7.0; other events keep their own version
 */

import { describe, it, expect } from "vitest";
import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

import {
  sessionCreatedEvent,
  sessionDeletedEvent,
  sessionIdleEvent,
  assistantMessageEntry,
  messageUpdatedEvent,
} from "./fixtures/opencode_api";
import type { OpenCodeEvent } from "../lib/handlers_session";
import {
  installRunnerTestEnv,
  setRunnerEnv,
  makeSdk,
  startPlugin,
  userMessageUpdated,
  readJsonl,
  runDir,
  RUN_ID,
  INSTANCE_ID,
  SESSION,
} from "./runner_mode_support";

const testEnv = installRunnerTestEnv();

const MSG = "msg-asst-tools-1";

type Hooks = Awaited<ReturnType<typeof startPlugin>>;
type Row = Record<string, unknown>;

interface Mode {
  name: string;
  setup: () => void;
  stream: () => string;
  runId: string;
}

const MODES: Mode[] = [
  {
    name: "native mode",
    setup: () => {},
    runId: "unknown-run",
    stream: () => nodePath.join(runDir(testEnv.dir, "unknown-run"), "00_orchestrator_events.jsonl"),
  },
  {
    name: "Runner mode (subagent role, runner-root registration)",
    setup: () => setRunnerEnv({ role: "subagent", runId: RUN_ID, instanceId: INSTANCE_ID }),
    runId: RUN_ID,
    stream: () => nodePath.join(runDir(testEnv.dir, RUN_ID), INSTANCE_ID, "03_events.jsonl"),
  },
];

/** A tool part as OpenCode reports it on either event kind. */
function toolPart(id: string, tool: string, messageID = MSG): Record<string, unknown> {
  return {
    id,
    messageID,
    sessionID: SESSION,
    type: "tool",
    tool,
    callID: `call-${id}`,
    state: { status: "completed" },
  };
}

function textPart(id: string, text: string): Record<string, unknown> {
  return { id, messageID: MSG, sessionID: SESSION, type: "text", text };
}

function partUpdated(part: Record<string, unknown>): OpenCodeEvent {
  return { type: "message.part.updated", properties: { part } };
}

/** An assistant message.updated observation carrying exactly the given parts. */
function observation(parts: Record<string, unknown>[], messageId = MSG): OpenCodeEvent {
  const entry = assistantMessageEntry("Answer.", {
    id: messageId,
    sessionID: SESSION,
    time: { created: 1000 },
  });
  return messageUpdatedEvent({ info: entry.info, parts: parts as never });
}

async function startSession(mode: Mode): Promise<Hooks> {
  mode.setup();
  const hooks = await startPlugin(testEnv.dir, makeSdk(() => []));
  await hooks.event({ event: sessionCreatedEvent(SESSION) });
  await hooks.event({ event: userMessageUpdated(SESSION, "Do the work") });
  return hooks;
}

function usageRecords(mode: Mode, messageId = MSG): Row[] {
  return readJsonl(mode.stream()).filter(
    (e) => e["event"] === "usage_record" && e["record_id"] === messageId,
  );
}

function lastUsage(mode: Mode, messageId = MSG): Row {
  const rows = usageRecords(mode, messageId);
  expect(rows.length).toBeGreaterThan(0);
  return rows[rows.length - 1]!;
}

describe.each(MODES)("usage_record requested_tools - $name", (mode) => {
  it("counts the tool parts carried on message.updated by tool name", async () => {
    const hooks = await startSession(mode);

    await hooks.event({
      event: observation([
        toolPart("p1", "read_file"),
        toolPart("p2", "glob"),
        toolPart("p3", "read_file"),
        textPart("p4", "thinking out loud"),
      ]),
    });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ read_file: 2, glob: 1 });
  });

  it("counts tool parts that arrived on earlier message.part.updated events", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: partUpdated(toolPart("p1", "bash")) });
    await hooks.event({ event: partUpdated(toolPart("p2", "bash")) });
    await hooks.event({ event: partUpdated(toolPart("p3", "grep")) });
    await hooks.event({ event: observation([textPart("p4", "done")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ bash: 2, grep: 1 });
  });

  it("writes an empty requested_tools for a message with no tool parts", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([textPart("p1", "just text")]) });

    const record = lastUsage(mode);
    expect(record).toHaveProperty("requested_tools");
    expect(record["requested_tools"]).toEqual({});
  });

  it("counts a part once when it is updated repeatedly and also listed on message.updated", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: partUpdated({ ...toolPart("p1", "bash"), state: { status: "pending" } }) });
    await hooks.event({ event: partUpdated({ ...toolPart("p1", "bash"), state: { status: "running" } }) });
    await hooks.event({ event: partUpdated(toolPart("p1", "bash")) });
    await hooks.event({ event: observation([toolPart("p1", "bash")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ bash: 1 });
  });

  it("keeps tools of different messages apart", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: partUpdated(toolPart("a1", "read_file", "msg-a")) });
    await hooks.event({ event: partUpdated(toolPart("b1", "glob", "msg-b")) });
    await hooks.event({ event: observation([], "msg-a") });
    await hooks.event({ event: observation([], "msg-b") });

    expect(lastUsage(mode, "msg-a")["requested_tools"]).toEqual({ read_file: 1 });
    expect(lastUsage(mode, "msg-b")["requested_tools"]).toEqual({ glob: 1 });
  });
});

describe.each(MODES)("usage_record requested_tools never dropped by a later observation - $name", (mode) => {
  it("a later observation without parts keeps tools carried by an earlier message.updated", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "read_file"), toolPart("p2", "glob")]) });
    await hooks.event({ event: observation([]) });

    const rows = usageRecords(mode);
    expect(rows.length).toBeGreaterThanOrEqual(2);
    for (const row of rows) {
      expect(row["requested_tools"]).toEqual({ read_file: 1, glob: 1 });
    }
  });

  it("a later observation without tool parts keeps tools seen on message.part.updated", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: partUpdated(toolPart("p1", "bash")) });
    await hooks.event({ event: observation([toolPart("p1", "bash")]) });
    await hooks.event({ event: observation([textPart("p2", "final answer")]) });

    const rows = usageRecords(mode);
    expect(rows.length).toBeGreaterThanOrEqual(2);
    expect(rows[rows.length - 1]!["requested_tools"]).toEqual({ bash: 1 });
  });

  it("a later observation carrying a different tool part set still includes the earlier tools", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "read_file")]) });
    await hooks.event({ event: observation([toolPart("p2", "glob")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ read_file: 1, glob: 1 });
  });

  it("a tool part arriving between observations is included in the next observation", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "read_file")]) });
    await hooks.event({ event: partUpdated(toolPart("p2", "read_file")) });
    await hooks.event({ event: observation([]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ read_file: 2 });
  });
});

describe.each(MODES)("usage_record requested_tools part identity - $name", (mode) => {
  it("counts a part once by callID even when its id differs across feeds", async () => {
    const hooks = await startSession(mode);
    const viaPart = { ...toolPart("p1", "bash"), callID: "call-shared" };
    const viaMessage = { ...toolPart("other-id", "bash"), callID: "call-shared" };

    await hooks.event({ event: partUpdated(viaPart) });
    await hooks.event({ event: observation([viaMessage]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ bash: 1 });
  });

  it("counts a part that has only an id, once across feeds", async () => {
    const hooks = await startSession(mode);
    const { callID: _unused, ...idOnly } = toolPart("p1", "glob") as Record<string, unknown>;

    await hooks.event({ event: partUpdated(idOnly) });
    await hooks.event({ event: observation([idOnly]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ glob: 1 });
  });

  it("counts two parts with distinct callIDs but the same id separately", async () => {
    const hooks = await startSession(mode);

    await hooks.event({
      event: observation([
        { ...toolPart("same", "bash"), callID: "call-a" },
        { ...toolPart("same", "bash"), callID: "call-b" },
      ]),
    });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ bash: 2 });
  });

  it("does not count a tool part without a tool name", async () => {
    const hooks = await startSession(mode);
    const { tool: _unused, ...nameless } = toolPart("p1", "bash") as Record<string, unknown>;

    await hooks.event({ event: partUpdated(nameless) });
    await hooks.event({ event: observation([nameless, toolPart("p2", "grep")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ grep: 1 });
  });

  it("does not count a tool part with neither callID nor id", async () => {
    const hooks = await startSession(mode);
    const { callID: _c, id: _i, ...anonymous } = toolPart("p1", "bash") as Record<string, unknown>;

    await hooks.event({ event: partUpdated(anonymous) });
    await hooks.event({ event: observation([anonymous, toolPart("p2", "grep")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ grep: 1 });
  });

  it("counts a part whose status is error", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: partUpdated({ ...toolPart("p1", "bash"), state: { status: "error" } }) });
    await hooks.event({ event: observation([]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ bash: 1 });
  });
});

describe.each(MODES)("usage_record requested_tools message without id - $name", (mode) => {
  it("carries no requested_tools when the assistant message has no info.id", async () => {
    const hooks = await startSession(mode);
    const entry = assistantMessageEntry("Answer.", { sessionID: SESSION, time: { created: 1000 } });
    const info = { ...entry.info } as Record<string, unknown>;
    delete info["id"];

    await hooks.event({
      event: messageUpdatedEvent({
        info: info as never,
        parts: [toolPart("p1", "bash")] as never,
      }),
    });

    const rows = readJsonl(mode.stream()).filter((e) => e["event"] === "usage_record");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(row).not.toHaveProperty("requested_tools");
    }
  });
});

describe.each(MODES)("usage_record requested_tools session release - $name", (mode) => {
  it("keeps earlier tools for a later observation after session.idle", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "read_file")]) });
    await hooks.event({ event: sessionIdleEvent(SESSION) });
    await hooks.event({ event: observation([toolPart("p2", "glob")]) });

    expect(lastUsage(mode)["requested_tools"]).toEqual({ read_file: 1, glob: 1 });
  });

  it("forgets earlier tools of a top-level session after session.deleted", async () => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "read_file")]) });
    const before = usageRecords(mode).length;
    await hooks.event({ event: sessionDeletedEvent(SESSION) });
    await hooks.event({ event: observation([toolPart("p2", "glob")]) });

    expect(usageRecords(mode).length).toBeGreaterThan(before);
    expect(lastUsage(mode)["requested_tools"]).toEqual({ glob: 1 });
  });

  it("forgets earlier tools of a subagent session after session.deleted", async () => {
    const hooks = await startSession(mode);
    const SUB = "sess-sub-tools";
    const subObservation = (parts: Record<string, unknown>[]): OpenCodeEvent => {
      const entry = assistantMessageEntry("Answer.", { id: MSG, sessionID: SUB, time: { created: 1000 } });
      return messageUpdatedEvent({ info: entry.info, parts: parts as never });
    };
    await hooks.event({ event: sessionCreatedEvent(SUB, SESSION) });

    // A subagent session writes to its own invocation stream (a generated
    // agent_* directory), not the mode stream: read every invocation stream
    // of the run except the one the mode stream itself points at.
    const subRows = (): Row[] => {
      const dir = runDir(testEnv.dir, mode.runId);
      if (!nodeFs.existsSync(dir)) return [];
      return nodeFs
        .readdirSync(dir, { withFileTypes: true })
        .filter((d) => d.isDirectory())
        .map((d) => nodePath.join(dir, d.name, "03_events.jsonl"))
        .filter((p) => p !== mode.stream() && nodeFs.existsSync(p))
        .flatMap((p) => readJsonl(p))
        .filter((e) => e["event"] === "usage_record" && e["record_id"] === MSG);
    };

    await hooks.event({ event: subObservation([{ ...toolPart("p1", "read_file"), sessionID: SUB }]) });
    const before = subRows().length;
    expect(before).toBeGreaterThan(0);
    await hooks.event({ event: sessionDeletedEvent(SUB) });
    await hooks.event({ event: subObservation([{ ...toolPart("p2", "glob"), sessionID: SUB }]) });

    const rows = subRows();
    expect(rows.length).toBeGreaterThan(before);
    expect(rows[rows.length - 1]!["requested_tools"]).toEqual({ glob: 1 });
  });
});

describe("usage_record schema version", () => {
  it.each(MODES)("is stamped 1.7.0 - $name", async (mode) => {
    const hooks = await startSession(mode);

    await hooks.event({ event: observation([toolPart("p1", "bash")]) });

    expect(lastUsage(mode)["schema_version"]).toBe("1.7.0");
  });

  it("does not move other events to 1.7.0", async () => {
    const hooks = await startSession(MODES[0]!);

    await hooks.event({ event: observation([toolPart("p1", "bash")]) });

    const turns = readJsonl(MODES[0]!.stream()).filter((e) => e["event"] === "turn");
    expect(turns.length).toBeGreaterThan(0);
    for (const turn of turns) {
      expect(turn["schema_version"]).not.toBe("1.7.0");
    }
  });
});
