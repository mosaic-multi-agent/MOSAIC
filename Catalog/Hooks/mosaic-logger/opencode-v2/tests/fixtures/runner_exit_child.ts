/**
 * runner_exit_child.ts - a stand-in for the private standalone server process.
 *
 * Usage: runner_exit_child.ts <workspace dir> [with-child-dispatch]
 *
 * It loads the plugin, delivers one root session's events and ends the process
 * with process.exit one macrotask after the last event was handed to the plugin,
 * the way the standalone server exits when its stdin lease closes. Pending
 * asynchronous file operations are lost at that point, so only records that were
 * written synchronously while the event was processed survive.
 *
 * It writes nothing to stdout or stderr: under --standalone stdout is the
 * serve --stdio channel.
 */

import plugin from "../../plugin";

const dir = process.argv[2];
const SESSION = "ses_exit0000000000000000001";
const MESSAGE = "msg_exit_1";

const located = (type: string, data: Record<string, unknown>) => ({ id: `evt_${type}`, type, location: { directory: dir }, data });

const events = [
  located("session.inbox.enqueued", {
    sessionID: SESSION,
    inboxID: "inbox_1",
    item: { type: "user", payload: { text: "Please do the exit-critical work." }, delivery: "steer" },
  }),
  located("session.step.started", {
    sessionID: SESSION,
    assistantMessageID: MESSAGE,
    model: { id: "gpt-6-luna", providerID: "openai" },
    started: 1794925254870,
  }),
  located("session.text.ended", { sessionID: SESSION, assistantMessageID: MESSAGE, ordinal: 0, text: '{"status_code":"SUCCESS"}' }),
  located("session.step.ended", {
    sessionID: SESSION,
    assistantMessageID: MESSAGE,
    finish: "stop",
    cost: 1,
    tokens: { input: 120, output: 30, reasoning: 10, cache: { read: 5, write: 2 } },
  }),
];

let index = 0;
const iterable = {
  [Symbol.asyncIterator]: () => ({
    next: async (): Promise<IteratorResult<unknown>> => {
      if (index >= events.length) return new Promise<IteratorResult<unknown>>(() => {});
      const value = events[index];
      index += 1;
      if (index === events.length) setImmediate(() => process.exit(0));
      return { value, done: false };
    },
    return: async (): Promise<IteratorResult<unknown>> => ({ value: undefined, done: true }),
  }),
};

const ctx = {
  location: { directory: dir },
  tool: { hook: () => undefined },
  permission: { hook: () => undefined },
  session: { hook: () => undefined },
  event: { subscribe: () => iterable },
};

void plugin.setup(ctx as never);
