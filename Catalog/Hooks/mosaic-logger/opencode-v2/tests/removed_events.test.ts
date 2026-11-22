/**
 * The V1 events that V2 removed (message.*, session.updated, session.idle,
 * session.status) carry no meaning for the logger: delivered anyway, they
 * produce no record, whether or not the plugin filters its subscription.
 */

import { describe, expect, it } from "vitest";

import { ROOT, located, userInbox } from "./events";
import { allRecords, byEvent, orchestratorStream, UNKNOWN_RUN } from "./log_reader";
import { eventually, quiet, useTestEnv } from "./harness";

const env = useTestEnv();
const FRESH = "ses_root0000000000000000009";

function removedEvents(dir: string, session: string) {
  return [
    located(dir, "session.idle", { sessionID: session }),
    located(dir, "session.status", { sessionID: session, status: { type: "idle" } }),
    located(dir, "session.updated", { sessionID: session, info: { id: session, title: "t" } }),
    located(dir, "message.updated", { sessionID: session, info: { id: "msg_1", role: "assistant" } }),
    located(dir, "message.part.updated", { sessionID: session, part: { id: "prt_1", type: "text", text: "hi" } }),
  ];
}

describe("removed V1 events", () => {
  it("never end an owned root session, whatever it is doing", async () => {
    const { host, dir } = await env.start();
    await host.emit(userInbox(dir, ROOT, "go"));
    await eventually(() => {
      expect(byEvent(orchestratorStream(dir, UNKNOWN_RUN), "session_start")).toHaveLength(1);
    });
    const before = allRecords(dir).length;

    for (const event of removedEvents(dir, ROOT)) await host.emit(event);
    await quiet();

    expect(byEvent(allRecords(dir), "session_end")).toEqual([]);
    expect(byEvent(allRecords(dir), "run_end")).toEqual([]);
    expect(allRecords(dir)).toHaveLength(before);
  });

  it("never open a session of their own for a session the instance has not seen", async () => {
    const { host, dir } = await env.start();

    for (const event of removedEvents(dir, FRESH)) await host.emit(event);
    await quiet();

    expect(allRecords(dir)).toEqual([]);
  });
});
