/**
 * plugin.ts - entry of the mosaic-logger OpenCode V2 variant.
 *
 * A plain {id, setup} default export. Only top-level files in .opencode/plugins/
 * load as plugins, so every helper lives under lib/. The plugin imports no npm
 * package. It never writes to stdout or stderr (under --standalone stdout is the
 * server's own channel), never blocks setup with file I/O, and never lets a
 * failure reach the host: a throwing hook would fail the V2 session.
 *
 * V2 `ctx` has no client, no `$` and no directory member; the project directory is
 * ctx.location.directory.
 */

import { readManifestAdapterVersion } from "./lib/manifest_version.js";
import { processEvent } from "./lib/event_router.js";
import { onCompactionHook, onToolAfter, onToolBefore } from "./lib/hooks.js";
import { onPermissionEvaluate } from "./lib/permission_records.js";
import type { V2PluginContext } from "./lib/host.js";
import { readRunnerMode } from "./lib/runner_mode.js";
import { get, str } from "./lib/safe.js";
import { createState, type InstanceState } from "./lib/state.js";
import { createExporter, type TranscriptExporter } from "./lib/transcript_queue.js";

const RESUBSCRIBE_DELAY_MS = 1000;

type HookRegistrar = (name: string, cb: (event: unknown) => unknown) => unknown;

/** The project directory, or undefined when ctx does not carry a usable one. */
function projectDirectory(ctx: unknown): string | undefined {
  try {
    return str(get(get(ctx, "location"), "directory"));
  } catch {
    return undefined;
  }
}

/** Register one hook of a ctx domain; a missing domain or a failing registration is skipped. */
async function registerHook(
  ctx: unknown,
  domainName: "tool" | "permission" | "session",
  name: string,
  callback: (event: unknown) => void,
): Promise<void> {
  try {
    const domain = get(ctx, domainName);
    const hook = get(domain, "hook");
    if (typeof hook !== "function") return;
    await (hook as HookRegistrar).call(domain, name, callback);
  } catch {
    // the other registrations proceed
  }
}

/** Wait for the delay or for the signal to abort, whichever comes first. */
function pause(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(done, ms);
    (timer as { unref?: () => void }).unref?.();
    function done(): void {
      clearTimeout(timer);
      signal.removeEventListener("abort", done);
      resolve();
    }
    signal.addEventListener("abort", done, { once: true });
  });
}

/**
 * Consume the event subscription one event at a time, in delivery order. When the
 * iterator ends or throws and the signal is not aborted, subscribe again no more
 * often than once per second. Events delivered while unsubscribed are lost.
 */
async function subscriptionLoop(ctx: unknown, state: InstanceState, signal: AbortSignal): Promise<void> {
  const event = get(ctx, "event");
  const subscribe = get(event, "subscribe");
  if (typeof subscribe !== "function") return;
  while (!signal.aborted) {
    try {
      const stream = (subscribe as (options: { signal: AbortSignal }) => AsyncIterable<unknown>).call(event, { signal });
      for await (const item of stream) {
        if (signal.aborted) break;
        processEvent(state, item);
      }
    } catch {
      // fall through to the next subscription
    }
    if (signal.aborted) return;
    await pause(RESUBSCRIBE_DELAY_MS, signal);
  }
}

const plugin = {
  id: "mosaic-logger" as const,

  async setup(ctx: V2PluginContext): Promise<() => void> {
    const controller = new AbortController();
    let cleaned = false;
    const cleanup = (): void => {
      if (cleaned) return;
      cleaned = true;
      try {
        controller.abort();
      } catch {
        // cleanup never throws
      }
      try {
        exporter?.stop();
      } catch {
        // cleanup never throws
      }
    };
    let exporter: TranscriptExporter | undefined;

    try {
      const dir = projectDirectory(ctx);
      if (dir === undefined) return cleanup;

      const state = createState(dir, readRunnerMode());
      if (state.mode === undefined) {
        exporter = createExporter(state);
        state.exporter = exporter;
      }
      // Not awaited: a slow disk must never delay activation.
      void readManifestAdapterVersion(dir).then((version) => {
        state.adapterVersion = version;
      });

      await registerHook(ctx, "tool", "execute.before", (event) => onToolBefore(state, event));
      await registerHook(ctx, "tool", "execute.after", (event) => onToolAfter(state, event));
      await registerHook(ctx, "permission", "evaluate", (event) => onPermissionEvaluate(state, event));
      await registerHook(ctx, "session", "compaction", (event) => onCompactionHook(state, event));
      void subscriptionLoop(ctx, state, controller.signal).catch(() => undefined);
    } catch {
      // setup never rejects
    }
    return cleanup;
  },
};

export default plugin;
