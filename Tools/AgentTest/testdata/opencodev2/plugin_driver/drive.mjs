// Test driver: loads a generated V2 interceptor plugin under a fake host
// context, replays a scripted scenario against the registered hooks and the
// event subscription, and writes a JSON result file. Usage:
//   node drive.mjs <pluginPath> <scenarioPath> <resultPath>
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { pathToFileURL } from "node:url";

const [pluginPath, scenarioPath, resultPath] = process.argv.slice(2);
const scenario = JSON.parse(readFileSync(scenarioPath, "utf8"));

// Anything the plugin writes to stdout or stderr is recorded, not printed.
const writes = [];
for (const stream of [process.stdout, process.stderr]) {
  stream.write = (chunk) => {
    writes.push(String(chunk));
    return true;
  };
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const logCount = () =>
  existsSync(scenario.logPath)
    ? readFileSync(scenario.logPath, "utf8").split("\n").filter(Boolean).length
    : 0;

const subs = [];
function makeSubscription(options) {
  const queue = [];
  let waiting = null;
  let done = false;
  const finish = () => {
    done = true;
    if (waiting) {
      const resolve = waiting;
      waiting = null;
      resolve({ value: undefined, done: true });
    }
  };
  subs.push({
    push(event) {
      if (done) return;
      if (waiting) {
        const resolve = waiting;
        waiting = null;
        resolve({ value: event, done: false });
      } else {
        queue.push(event);
      }
    },
    end: finish,
  });
  if (options && options.signal) {
    if (options.signal.aborted) finish();
    else options.signal.addEventListener("abort", finish);
  }
  return {
    [Symbol.asyncIterator]() {
      return {
        next() {
          if (queue.length) return Promise.resolve({ value: queue.shift(), done: false });
          if (done) return Promise.resolve({ value: undefined, done: true });
          return new Promise((resolve) => {
            waiting = resolve;
          });
        },
        return() {
          finish();
          return Promise.resolve({ value: undefined, done: true });
        },
      };
    },
  };
}

const hooks = {};
const ctx = {};
if (!scenario.noToolHook) {
  ctx.tool = {
    hook(name, callback) {
      if ((scenario.failHooks || []).includes(name)) throw new Error("registration failed");
      hooks[name] = callback;
      return {};
    },
  };
}
if (!scenario.noSubscribe) {
  ctx.event = { subscribe: (options) => makeSubscription(options) };
}

const result = { steps: [], hooks: [], writes, importError: "" };

async function waitFor(condition, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (!condition() && Date.now() < deadline) await sleep(20);
}

async function runStep(step, cleanup) {
  const before = logCount();
  const rec = { kind: step.kind, threw: null };
  if (step.kind === "before" || step.kind === "after") {
    const callback = hooks[step.kind === "before" ? "execute.before" : "execute.after"];
    try {
      if (callback) await callback(step.event);
    } catch (e) {
      rec.threw = e && e.message !== undefined ? String(e.message) : String(e);
    }
    rec.event = JSON.parse(JSON.stringify(step.event));
  } else if (step.kind === "event") {
    if (subs.length) subs[subs.length - 1].push(step.event);
  } else if (step.kind === "end") {
    const count = subs.length;
    subs[count - 1].end();
    await waitFor(() => subs.length > count, 5000);
  } else if (step.kind === "cleanup") {
    if (typeof cleanup === "function") await cleanup();
  }
  if (step.forwards > 0) await waitFor(() => logCount() >= before + step.forwards, 5000);
  await sleep(step.forwards > 0 ? 100 : 300);
  rec.forwarded = logCount() - before;
  rec.subscribeCount = subs.length;
  return rec;
}

try {
  let plugin;
  try {
    plugin = (await import(pathToFileURL(pluginPath).href)).default;
  } catch (e) {
    result.importError = String((e && e.code) || "") + " " + String((e && e.message) || e);
  }
  if (plugin) {
    result.id = plugin.id;
    let cleanup;
    try {
      cleanup = await plugin.setup(ctx);
    } catch (e) {
      result.setupError = String((e && e.message) || e);
    }
    if (!scenario.noSubscribe) await waitFor(() => subs.length > 0, 2000);
    await sleep(200);
    result.hooks = Object.keys(hooks);
    result.logAfterSetup = logCount();
    for (const step of scenario.steps) result.steps.push(await runStep(step, cleanup));
    result.subscribeCount = subs.length;
  }
} finally {
  writeFileSync(resultPath, JSON.stringify(result));
  process.exit(0);
}
