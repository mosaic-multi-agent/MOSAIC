/**
 * run_binding_sources.test.ts -- Which sources may bind a native OpenCode session to a run.
 *
 * A session is bound to a run only by a MOSAIC dispatch (a Task prompt that
 * carries an agent instance id as well as a run id), by Runner-mode
 * MOSAIC_RUN_ID, or by an existing binding. User prompts and assistant replies
 * never bind, whether the run id is bare or in `run_id: <id>` form.
 *
 * History part shape (unverified against the SDK, taken from the design): a
 * message entry carries `parts[]`; a Task tool part has `type: "tool"`,
 * `tool: "task"` and the dispatch prompt at `state.input.prompt`.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";

import { dispatchRunId, historyDispatchRunId } from "../lib/run_binding";
import {
  createSessionHandlers,
  type HandlerDependencies,
  type MessageEntry,
  type SdkClient,
} from "../lib/handlers_session";
import { createToolHandlers } from "../lib/handlers_tools";
import { createInvocationHandlers } from "../lib/handlers_invocation";
import { SessionCorrelationStore } from "../lib/correlation";
import { LogPaths, setDebugLogger } from "../lib/core";
import { makeSdk, makeDeps, type CollectedEvent } from "./handlers_invocation_support";
import {
  sessionCreatedEvent,
  taskDispatchPayload,
  userMessageEntry,
  assistantMessageEntry,
} from "./fixtures/opencode_api";
import {
  installRunnerTestEnv,
  setRunnerEnv,
  makeSdk as makeRunnerSdk,
  startPlugin,
  readJsonl,
  runDir,
  SESSION as RUNNER_SESSION,
  RUN_ID as RUNNER_RUN_ID,
} from "./runner_mode_support";

const RUN_A = "20260101T170000Z-a3f9";
const RUN_B = "20260202T080000Z-beef";
const ORCH = "sess-orch-001";
const SUB = "sess-sub-001";
const SUB_2 = "sess-sub-002";

/** A dispatch prompt with both an agent instance id and a run id. */
function dispatchPrompt(runId: string, instance = "Research#1"): string {
  return JSON.stringify({ agent_instance_id: instance, run_id: runId, task_description: "work" });
}

/** A history entry holding one Task tool part with the given prompt. */
function taskToolEntry(prompt: unknown, tool = "task", status = "completed"): MessageEntry {
  return {
    info: { id: "msg-task", role: "assistant", sessionID: "sess-fixture" },
    parts: [
      {
        id: "part-task",
        messageID: "msg-task",
        sessionID: "sess-fixture",
        type: "tool",
        tool,
        state: { status, input: { prompt } },
      },
    ],
  };
}

// ---------------------------------------------------------------------------
// dispatchRunId
// ---------------------------------------------------------------------------

describe("dispatchRunId", () => {
  it("returns the run id of a structured prompt carrying both ids", () => {
    expect(dispatchRunId(dispatchPrompt(RUN_A))).toBe(RUN_A);
  });

  it("returns the run id of a bare prompt carrying an instance name and a run id", () => {
    expect(dispatchRunId(`You are Research#4 working on ${RUN_B}`)).toBe(RUN_B);
  });

  it("returns undefined when the prompt carries a run id but no agent instance id", () => {
    expect(dispatchRunId(`run_id: ${RUN_A}`)).toBeUndefined();
    expect(dispatchRunId(`please look at ${RUN_A}`)).toBeUndefined();
  });

  it("returns undefined when the prompt carries an instance id but no run id", () => {
    expect(dispatchRunId('{"agent_instance_id": "Research#1"}')).toBeUndefined();
  });

  it("returns undefined for empty and non-string input without throwing", () => {
    for (const value of ["", null, undefined, 42, {}, [], true]) {
      expect(() => dispatchRunId(value)).not.toThrow();
      expect(dispatchRunId(value)).toBeUndefined();
    }
  });
});

// ---------------------------------------------------------------------------
// historyDispatchRunId
// ---------------------------------------------------------------------------

describe("historyDispatchRunId", () => {
  it("returns the run id of a Task tool part whose prompt carries both ids", () => {
    const history = [userMessageEntry("go"), taskToolEntry(dispatchPrompt(RUN_A))];
    expect(historyDispatchRunId(history)).toBe(RUN_A);
  });

  it("ignores user text that quotes a run id, bare or structured", () => {
    expect(historyDispatchRunId([userMessageEntry(`run_id: ${RUN_A}`)])).toBeUndefined();
    expect(historyDispatchRunId([userMessageEntry(`look at ${RUN_A}`)])).toBeUndefined();
  });

  it("ignores assistant text that quotes a run id next to an agent instance name", () => {
    const text = `Dispatched Research#1 for run_id: ${RUN_A}`;
    expect(historyDispatchRunId([assistantMessageEntry(text)])).toBeUndefined();
  });

  it("ignores a Task tool part whose prompt has a run id but no agent instance id", () => {
    expect(historyDispatchRunId([taskToolEntry(`run_id: ${RUN_A}`)])).toBeUndefined();
  });

  it("ignores a tool part of another tool even when its prompt carries both ids", () => {
    expect(historyDispatchRunId([taskToolEntry(dispatchPrompt(RUN_A), "bash")])).toBeUndefined();
  });

  it("returns the run id of an in-flight or failed Task dispatch", () => {
    for (const status of ["pending", "running", "error"]) {
      const history = [taskToolEntry(dispatchPrompt(RUN_A), "task", status)];
      expect(historyDispatchRunId(history)).toBe(RUN_A);
    }
  });

  it("returns the last qualifying dispatch when the history holds several", () => {
    const history = [
      taskToolEntry(dispatchPrompt(RUN_A)),
      assistantMessageEntry(`earlier we used ${RUN_A}`),
      taskToolEntry(dispatchPrompt(RUN_B, "Review#2")),
      assistantMessageEntry(`and mentioned ${RUN_A} again`),
    ];
    expect(historyDispatchRunId(history)).toBe(RUN_B);
  });

  it("returns undefined for malformed input of any shape without throwing", () => {
    const malformed: unknown[] = [
      null,
      undefined,
      "text with " + RUN_A,
      42,
      {},
      [null],
      [{}],
      [{ parts: "x" }],
      [{ parts: [null] }],
      [{ parts: [{ type: "tool", tool: "task" }] }],
      [{ parts: [{ type: "tool", tool: "task", state: null }] }],
      [{ parts: [{ type: "tool", tool: "task", state: { input: null } }] }],
      [taskToolEntry(42)],
    ];
    for (const value of malformed) {
      expect(() => historyDispatchRunId(value)).not.toThrow();
      expect(historyDispatchRunId(value)).toBeUndefined();
    }
  });
});

// ---------------------------------------------------------------------------
// Orchestrator session.created history scan
// ---------------------------------------------------------------------------

describe("orchestrator session.created -- history scan", () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-rb-test-"));
    setDebugLogger(() => {});
  });

  afterEach(() => {
    nodeFs.rmSync(tmpDir, { recursive: true, force: true });
    setDebugLogger(() => {});
  });

  async function createWithHistory(history: unknown[]) {
    const store = new SessionCorrelationStore();
    const paths = new LogPaths(tmpDir);
    const collected: CollectedEvent[] = [];
    const deps: HandlerDependencies = makeDeps(store, paths, makeSdk(() => history), collected);
    const { handleEvent } = createSessionHandlers(deps);
    await handleEvent(sessionCreatedEvent(ORCH));
    return { store, paths, collected };
  }

  function expectNothingWrittenToRun(
    result: { paths: LogPaths; collected: CollectedEvent[] },
    runId: string,
  ): void {
    expect(result.collected.length).toBeGreaterThan(0);
    for (const e of result.collected) {
      expect(e.filePath).not.toBe(result.paths.orchestratorEvents(runId));
      expect(e.filePath).toBe(result.paths.orchestratorEvents("unknown-run"));
    }
  }

  it("does not bind a resumed session whose assistant reply quotes a run id", async () => {
    const result = await createWithHistory([
      userMessageEntry("continue"),
      assistantMessageEntry(`The earlier run was run_id: ${RUN_A}`),
    ]);

    expect(result.store.get(ORCH)?.runId).toBeUndefined();
    expectNothingWrittenToRun(result, RUN_A);
  });

  it("does not bind when an ordinary prompt quotes a bare run id", async () => {
    const result = await createWithHistory([userMessageEntry(`Look at logs of ${RUN_A}`)]);

    expect(result.store.get(ORCH)?.runId).toBeUndefined();
    expectNothingWrittenToRun(result, RUN_A);
  });

  it("does not bind when message text quotes a run id next to an agent instance name", async () => {
    const result = await createWithHistory([
      userMessageEntry(`Tell Research#1 about run_id: ${RUN_A}`),
      assistantMessageEntry(`agent_instance_id: Research#1, run_id: ${RUN_A}`),
    ]);

    expect(result.store.get(ORCH)?.runId).toBeUndefined();
    expectNothingWrittenToRun(result, RUN_A);
  });

  it("binds a resumed session from a Task tool-call input carrying run id and agent instance id", async () => {
    const result = await createWithHistory([
      userMessageEntry("go"),
      taskToolEntry(dispatchPrompt(RUN_A)),
    ]);

    expect(result.store.get(ORCH)?.runId).toBe(RUN_A);
    expect(result.collected.length).toBeGreaterThan(0);
    for (const e of result.collected) {
      expect(e.filePath).toBe(result.paths.orchestratorEvents(RUN_A));
    }
  });

  it("does not bind from a Task tool-call input that has a run id but no agent instance id", async () => {
    const result = await createWithHistory([taskToolEntry(`run_id: ${RUN_A}`)]);

    expect(result.store.get(ORCH)?.runId).toBeUndefined();
    expectNothingWrittenToRun(result, RUN_A);
  });

  it("binds to the last dispatch when the history holds several", async () => {
    const result = await createWithHistory([
      taskToolEntry(dispatchPrompt(RUN_A)),
      taskToolEntry(dispatchPrompt(RUN_B, "Review#2")),
      assistantMessageEntry(`recap of ${RUN_A}`),
    ]);

    expect(result.store.get(ORCH)?.runId).toBe(RUN_B);
  });
});

// ---------------------------------------------------------------------------
// Dispatch sources require an agent instance id
// ---------------------------------------------------------------------------

describe("dispatch sources", () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-rb-dispatch-"));
    setDebugLogger(() => {});
  });

  afterEach(() => {
    nodeFs.rmSync(tmpDir, { recursive: true, force: true });
    setDebugLogger(() => {});
  });

  function makeFixture(getMessages: (sessionId: string) => unknown = () => []) {
    const store = new SessionCorrelationStore();
    const paths = new LogPaths(tmpDir);
    const collected: CollectedEvent[] = [];
    const sdk: SdkClient = makeSdk(getMessages);
    const deps = makeDeps(store, paths, sdk, collected);
    store.register(ORCH, {});
    return {
      store,
      paths,
      collected,
      tools: createToolHandlers(deps),
      invocations: createInvocationHandlers(deps),
    };
  }

  function taskPayloadWithPrompt(prompt: string) {
    const [input, output] = taskDispatchPayload({ sessionID: ORCH });
    output.args["prompt"] = prompt;
    return [input, output] as const;
  }

  it("Task prompt with run id and agent instance id queues a dispatch carrying the run id", async () => {
    const f = makeFixture();
    const [input, output] = taskDispatchPayload({
      sessionID: ORCH,
      agentInstanceId: "Reviewer#2",
      runId: RUN_B,
    });

    await f.tools.handleToolBefore(input, output);

    expect(f.store.claimPendingDispatch(ORCH)?.runId).toBe(RUN_B);
  });

  it("Task prompt with a run id but no agent instance id queues a dispatch without a run id", async () => {
    const f = makeFixture();
    const [input, output] = taskPayloadWithPrompt(`Investigate run_id: ${RUN_A} please`);

    await f.tools.handleToolBefore(input, output);

    expect(f.store.claimPendingDispatch(ORCH)?.runId).toBeUndefined();
  });

  it("subagent first user text with run id and agent instance id binds the chain", async () => {
    const f = makeFixture(() => [userMessageEntry(dispatchPrompt(RUN_A))]);
    f.store.register(SUB, { parentId: ORCH });

    await f.invocations.handleInvocationStart(SUB, ORCH);

    expect(f.store.get(SUB)?.runId).toBe(RUN_A);
    expect(f.store.getRunId(ORCH)).toBe(RUN_A);
  });

  it("subagent first user text with a run id but no agent instance id does not bind the chain", async () => {
    const f = makeFixture(() => [userMessageEntry(`Review the logs of run_id: ${RUN_A}`)]);
    f.store.register(SUB, { parentId: ORCH });

    await f.invocations.handleInvocationStart(SUB, ORCH);

    expect(f.store.get(SUB)?.runId).toBeUndefined();
    expect(f.store.getRunId(ORCH)).toBeUndefined();
    const sinks = f.collected.map((e) => e.filePath);
    expect(sinks.some((p) => p.includes(RUN_A))).toBe(false);
  });

  it("keeps the first binding: a dispatch for another run does not switch a bound orchestrator", async () => {
    const f = makeFixture();
    f.store.register(SUB, { parentId: ORCH });
    f.store.register(SUB_2, { parentId: ORCH });

    const [inA, outA] = taskDispatchPayload({ sessionID: ORCH, runId: RUN_A, agentInstanceId: "A#1" });
    await f.tools.handleToolBefore(inA, outA);
    await f.invocations.handleInvocationStart(SUB, ORCH);

    const [inB, outB] = taskDispatchPayload({ sessionID: ORCH, runId: RUN_B, agentInstanceId: "B#1" });
    await f.tools.handleToolBefore(inB, outB);
    await f.invocations.handleInvocationStart(SUB_2, ORCH);

    expect(f.store.getRunId(ORCH)).toBe(RUN_A);
  });
});

// ---------------------------------------------------------------------------
// Runner mode still binds
// ---------------------------------------------------------------------------

describe("Runner mode", () => {
  const testEnv = installRunnerTestEnv();

  it("binds the orchestrator session to MOSAIC_RUN_ID even when its history quotes another run id", async () => {
    setRunnerEnv({ role: "orchestrator", runId: RUNNER_RUN_ID });
    const history = [assistantMessageEntry(`earlier: run_id: ${RUN_A}`)];
    const hooks = await startPlugin(testEnv.dir, makeRunnerSdk(() => history));

    await hooks.event({ event: sessionCreatedEvent(RUNNER_SESSION) });

    const own = readJsonl(nodePath.join(runDir(testEnv.dir, RUNNER_RUN_ID), "00_orchestrator_events.jsonl"));
    expect(own.length).toBeGreaterThan(0);
    for (const e of own) expect(e["run_id"]).toBe(RUNNER_RUN_ID);
    expect(nodeFs.existsSync(runDir(testEnv.dir, RUN_A))).toBe(false);
  });
});
