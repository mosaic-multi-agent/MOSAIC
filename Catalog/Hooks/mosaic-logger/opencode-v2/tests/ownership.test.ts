/**
 * Ownership filtering: an instance logs only sessions belonging to its own
 * project, including events that carry no location, and never depends on
 * seeing its own session.created.
 */

import * as nodePath from "node:path";
import { describe, expect, it } from "vitest";

import {
  CHILD_PREFIX,
  ROOT,
  created,
  execution,
  located,
  missionPrompt,
  userInbox,
  wakeUp,
  wrapWakeUp,
  RUN_A,
} from "./events";
import { allRecords, byEvent, invocations, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const OTHER_SESSION = "ses_other000000000000000002";

function recordsMentioning(dir: string, sessionId: string) {
  return allRecords(dir).filter((rec) => JSON.stringify(rec).includes(sessionId));
}

describe("own-project and foreign events", () => {
  it("logs a session whose events are located in the instance's directory", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "hello"));

    await eventually(() => {
      const starts = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start");
      expect(starts).toHaveLength(1);
      expect(starts[0].session_id).toBe(ROOT);
    });
  });

  it("ignores a session whose events are located in another project", async () => {
    const { host, dir } = await env.start();
    const foreignDir = env.workspace();

    await host.emit(userInbox(foreignDir, OTHER_SESSION, "someone else's prompt"));
    await host.emit(created(foreignDir, OTHER_SESSION, undefined, "Agent"));
    await host.emit(execution(OTHER_SESSION, "succeeded"));
    await host.emit(userInbox(dir, ROOT, "mine"));
    await quiet();

    expect(recordsMentioning(dir, OTHER_SESSION)).toEqual([]);
    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
  });

  it("drops a location-less event of a session it does not own, and does not buffer it for later", async () => {
    const { host, dir } = await env.start();

    await host.emit(execution(ROOT, "succeeded"));
    await host.emit(userInbox(dir, ROOT, "now the session is known"));
    await quiet();

    const records = orchestratorStream(dir, UNKNOWN_RUN);
    expect(byEvent(records, "session_start")).toHaveLength(1);
    expect(byEvent(records, "session_end")).toEqual([]);
  });

  it("attributes a location-less terminal event to a session that became owned through a located event", async () => {
    const { host, dir } = await env.start();

    await host.emit(userInbox(dir, ROOT, "hello"));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const ends = byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end");
      expect(ends).toHaveLength(1);
      expect(ends[0].session_id).toBe(ROOT);
    });
  });

  it("drops a synthetic wake-up addressed to a parent it does not own", async () => {
    const { host, dir } = await env.start();

    await host.emit(wakeUp(OTHER_SESSION, "ses_child_x", "Agent", "completed", wrapWakeUp("ses_child_x", "completed", "{}")));
    await host.emit(userInbox(dir, ROOT, "mine"));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    await quiet();

    expect(allRecords(dir).filter((r) => r.event !== "session_start" && !(r.event === "turn" && r.session_id === ROOT))).toEqual([]);
    expect(invocations(dir)).toEqual([]);
  });
});

describe("what makes a session owned", () => {
  it("a tool hook carrying the session id makes it owned, so its later location-less events count", async () => {
    const { host, dir } = await env.start();

    await host.fireBefore({ tool: "read", sessionID: ROOT, id: "call_r", input: { path: "README.md" } });
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const records = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(records, "session_start")).toHaveLength(1);
      expect(byEvent(records, "session_end")).toHaveLength(1);
    });
  });

  it("a session.created whose data.location is this directory makes the session owned even without an envelope location", async () => {
    const { host, dir } = await env.start();

    await host.emit({ type: "session.created", data: { sessionID: ROOT, location: { directory: dir }, agent: "A" } });
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
    });
  });

  it("a session.created located in another directory does not make the session owned", async () => {
    const { host, dir } = await env.start();
    const foreignDir = env.workspace();

    await host.emit({ type: "session.created", location: { directory: foreignDir }, data: { sessionID: OTHER_SESSION, location: { directory: foreignDir } } });
    await host.emit(execution(OTHER_SESSION, "succeeded"));
    await host.emit(userInbox(dir, ROOT, "mine"));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    await quiet();

    expect(recordsMentioning(dir, OTHER_SESSION)).toEqual([]);
  });

  it("a root whose session.created is never seen is logged from its first owned signal", async () => {
    const { host, dir } = await env.start();

    await host.fireBefore({ tool: "read", sessionID: ROOT, id: "call_r", input: {} });
    await host.emit(userInbox(dir, ROOT, "no created event ever arrives"));
    await host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      const records = orchestratorStream(dir, UNKNOWN_RUN);
      expect(byEvent(records, "session_start")).toHaveLength(1);
      expect(byEvent(records, "session_end")).toHaveLength(1);
    });
  });

  it("a child created by an owned parent is owned even when its created event carries no location", async () => {
    const { host, dir } = await env.start();
    const child = "ses_child00000000000000003";
    await host.emit(userInbox(dir, ROOT, "start"));

    await host.emit({ type: "session.created", data: { sessionID: child, parentID: ROOT, agent: "Worker" } });
    await host.emit(wakeUp(ROOT, child, "Worker", "completed", wrapWakeUp(child, "completed", "done")));

    await eventually(() => {
      const [only] = invocations(dir);
      expect(only?.start?.session_id).toBe(child);
      expect(only?.end?.agent_instance_id).toBe(only?.start?.agent_instance_id);
    });
  });

  it("a child created by a parent the instance does not own is not owned", async () => {
    const { host, dir } = await env.start();
    const child = "ses_child00000000000000004";

    await host.emit({ type: "session.created", data: { sessionID: child, parentID: OTHER_SESSION, agent: "Worker" } });
    await host.emit(execution(child, "succeeded"));
    await host.emit(wakeUp(OTHER_SESSION, child, "Worker", "completed", wrapWakeUp(child, "completed", "done")));
    await host.emit(userInbox(dir, ROOT, "mine"));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    await quiet();

    expect(allRecords(dir).filter((r) => r.event !== "session_start" && !(r.event === "turn" && r.session_id === ROOT))).toEqual([]);
    expect(recordsMentioning(dir, child)).toEqual([]);
  });

  it("a dispatch result owned by an owned parent names the child as owned", async () => {
    const { host, dir } = await env.start();
    const child = "ses_child00000000000000005";
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A, "task") };
    await host.emit(userInbox(dir, ROOT, "start"));

    await host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_w", input });
    await host.fireAfter({ tool: "subagent", sessionID: ROOT, id: "call_w", input, status: "completed", result: { metadata: { sessionID: child, status: "running" } } });
    await host.emit(wakeUp(ROOT, child, "Worker", "completed", wrapWakeUp(child, "completed", "done")));

    await eventually(() => {
      const [only] = invocations(dir);
      expect(only?.start?.session_id).toBe(child);
      expect(only?.end).toBeDefined();
    });
  });

  it("a child session's own events never open a root session of their own", async () => {
    const { host, dir } = await env.start();
    const child = "ses_child00000000000000006";
    await host.emit(userInbox(dir, ROOT, "start"));
    await host.emit(created(dir, child, ROOT, "Worker"));
    await host.emit(userInbox(dir, child, `${CHILD_PREFIX}task text`));
    await host.emit(wakeUp(ROOT, child, "Worker", "completed", wrapWakeUp(child, "completed", "done")));

    await eventually(() => {
      expect(invocations(dir)).toHaveLength(1);
    });
    expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
  });
});

describe("directory comparison", () => {
  it("treats a trailing separator as the same directory", async () => {
    const { host, dir } = await env.start();

    await host.emit(located(dir + nodePath.sep, "session.inbox.enqueued", { sessionID: ROOT, inboxID: "m1", item: { type: "user", payload: { text: "hi" } } }));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });

  it.skipIf(process.platform !== "win32")("ignores letter case in directory names on Windows", async () => {
    const { host, dir } = await env.start();

    await host.emit(located(dir.toUpperCase(), "session.inbox.enqueued", { sessionID: ROOT, inboxID: "m1", item: { type: "user", payload: { text: "hi" } } }));

    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });

  it.skipIf(process.platform === "win32")("treats directories that differ only in letter case as different elsewhere", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, "ses_known0000000000000000007", "x"));

    await host.emit(located(dir.toUpperCase(), "session.inbox.enqueued", { sessionID: OTHER_SESSION, inboxID: "m1", item: { type: "user", payload: { text: "hi" } } }));
    await quiet();

    expect(recordsMentioning(dir, OTHER_SESSION)).toEqual([]);
  });
});

describe("two instances in one process", () => {
  it("each logs only its own project even when both hear the other's events", async () => {
    const a = await env.start();
    const b = await env.start();
    const sessionA = "ses_aaaa00000000000000000008";
    const sessionB = "ses_bbbb00000000000000000009";

    for (const host of [a.host, b.host]) {
      await host.emit(userInbox(a.dir, sessionA, "prompt in A"));
      await host.emit(userInbox(b.dir, sessionB, "prompt in B"));
      await host.emit(execution(sessionA, "succeeded"));
      await host.emit(execution(sessionB, "succeeded"));
    }

    await eventually(() => {
      expect(byEvent(orchestratorStream(a.dir, UNKNOWN_RUN), "session_end").map((r) => r.session_id)).toEqual([sessionA]);
      expect(byEvent(orchestratorStream(b.dir, UNKNOWN_RUN), "session_end").map((r) => r.session_id)).toEqual([sessionB]);
    });
    expect(recordsMentioning(a.dir, sessionB)).toEqual([]);
    expect(recordsMentioning(b.dir, sessionA)).toEqual([]);
  });

  it("does not share run bindings or invocations between instances", async () => {
    const a = await env.start();
    const b = await env.start();
    const input = { agent: "Worker", description: "d", prompt: missionPrompt("Worker#1", RUN_A) };

    await a.host.emit(userInbox(a.dir, ROOT, "start A"));
    await b.host.emit(userInbox(b.dir, ROOT, "start B"));
    await a.host.fireBefore({ tool: "subagent", sessionID: ROOT, id: "call_1", input });
    await b.host.emit(execution(ROOT, "succeeded"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(a.dir, RUN_A), "run_start")).toHaveLength(1);
    });
    await quiet();
    expect(orchestratorStream(b.dir, RUN_A)).toEqual([]);
    expect(byEvent(orchestratorStream(b.dir, UNKNOWN_RUN), "run_end")).toEqual([]);
    expect(byEvent(orchestratorStream(b.dir, UNKNOWN_RUN), "session_end")).toHaveLength(1);
  });

  it("stopping one instance leaves the other logging", async () => {
    const a = await env.start();
    const b = await env.start();

    a.cleanup();
    await b.host.emit(userInbox(b.dir, ROOT, "still here"));

    await eventually(() => {
      expect(byEvent(orchestratorStream(b.dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
  });
});
