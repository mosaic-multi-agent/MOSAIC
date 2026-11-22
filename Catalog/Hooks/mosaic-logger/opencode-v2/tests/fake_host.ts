/**
 * fake_host.ts - a fake OpenCode V2 plugin context.
 *
 * The fake records every hook registration and event subscription the plugin
 * makes, lets a test push events into the live subscription, and fire the tool
 * hooks directly. emit() resolves once the plugin has finished processing the
 * pushed event, which is observable as the plugin pulling the next one.
 */

import type { V2Event, V2PluginContext, V2ToolHookEvent } from "../lib/host";

export interface SubscribeCall {
  types?: string[];
  signal?: AbortSignal;
}

export interface FakeHostOptions {
  /** Domains left out of the ctx entirely. */
  omit?: Array<"tool" | "event" | "permission" | "session">;
  /** How tool.hook registration fails: synchronously or by rejection. */
  toolHookFails?: "throw" | "reject";
  /** event.subscribe throws synchronously. */
  subscribeThrows?: boolean;
  /** Extra ctx members, e.g. throwing getters for client/$/directory. */
  extraCtx?: (ctx: Record<string, unknown>) => void;
}

type ToolCallback = (event: V2ToolHookEvent) => unknown;

interface Item {
  event: unknown;
  ack: () => void;
}

const EMIT_TIMEOUT_MS = 2000;

class Stream {
  queue: Item[] = [];
  waiter?: (result: IteratorResult<V2Event>) => void;
  rejecter?: (err: unknown) => void;
  inFlight?: Item;
  closed = false;
  failure?: unknown;

  constructor(readonly signal?: AbortSignal) {
    signal?.addEventListener("abort", () => this.close());
  }

  close(): void {
    this.closed = true;
    this.inFlight?.ack();
    this.inFlight = undefined;
    const waiter = this.waiter;
    this.waiter = undefined;
    waiter?.({ value: undefined, done: true });
  }

  fail(err: unknown): void {
    this.failure = err;
    this.inFlight?.ack();
    this.inFlight = undefined;
    const rejecter = this.rejecter;
    this.waiter = undefined;
    this.rejecter = undefined;
    rejecter?.(err);
  }

  next(): Promise<IteratorResult<V2Event>> {
    this.inFlight?.ack();
    this.inFlight = undefined;
    if (this.failure !== undefined) return Promise.reject(this.failure);
    if (this.closed) return Promise.resolve({ value: undefined, done: true });
    const item = this.queue.shift();
    if (item) {
      this.inFlight = item;
      return Promise.resolve({ value: item.event as V2Event, done: false });
    }
    return new Promise((resolve, reject) => {
      this.waiter = resolve;
      this.rejecter = reject;
    });
  }

  push(item: Item): void {
    const waiter = this.waiter;
    if (waiter) {
      this.waiter = undefined;
      this.rejecter = undefined;
      this.inFlight = item;
      waiter({ value: item.event as V2Event, done: false });
      return;
    }
    this.queue.push(item);
  }
}

export class FakeHost {
  readonly beforeHooks: ToolCallback[] = [];
  readonly afterHooks: ToolCallback[] = [];
  readonly otherHooks: Array<{ domain: string; name: string }> = [];
  readonly evaluateHooks: Array<(event: unknown) => unknown> = [];
  readonly compactionHooks: Array<(event: unknown) => unknown> = [];
  readonly subscribeCalls: SubscribeCall[] = [];
  readonly streams: Stream[] = [];
  readonly ctx: V2PluginContext;
  private counter = 0;

  constructor(
    readonly dir: string,
    private readonly options: FakeHostOptions = {},
  ) {
    this.ctx = this.buildCtx();
  }

  private buildCtx(): V2PluginContext {
    const omit = new Set(this.options.omit ?? []);
    const ctx: Record<string, unknown> = { location: { directory: this.dir } };
    if (!omit.has("tool")) {
      ctx.tool = { hook: (name: string, cb: ToolCallback) => this.registerTool(name, cb) };
    }
    if (!omit.has("permission")) {
      ctx.permission = { hook: (name: string, cb?: (e: unknown) => unknown) => this.registerOther("permission", name, cb) };
    }
    if (!omit.has("session")) {
      ctx.session = { hook: (name: string, cb?: (e: unknown) => unknown) => this.registerOther("session", name, cb) };
    }
    if (!omit.has("event")) {
      ctx.event = { subscribe: (opts: SubscribeCall) => this.subscribe(opts) };
    }
    this.options.extraCtx?.(ctx);
    return ctx as V2PluginContext;
  }

  private registerOther(domain: string, name: string, cb?: (event: unknown) => unknown): undefined {
    this.otherHooks.push({ domain, name });
    if (cb && domain === "permission" && name === "evaluate") this.evaluateHooks.push(cb);
    if (cb && domain === "session" && name === "compaction") this.compactionHooks.push(cb);
    return undefined;
  }

  private registerTool(name: string, cb: ToolCallback): unknown {
    const fails = this.options.toolHookFails;
    if (name === "execute.before") this.beforeHooks.push(cb);
    if (name === "execute.after") this.afterHooks.push(cb);
    if (fails === "throw") throw new Error("registration failed");
    if (fails === "reject") return Promise.reject(new Error("registration rejected"));
    return undefined;
  }

  private subscribe(opts: SubscribeCall): AsyncIterable<V2Event> {
    this.subscribeCalls.push(opts);
    if (this.options.subscribeThrows) throw new Error("subscribe failed");
    const stream = new Stream(opts?.signal);
    this.streams.push(stream);
    return {
      [Symbol.asyncIterator]: () => ({
        next: () => stream.next(),
        return: () => {
          stream.close();
          return Promise.resolve({ value: undefined, done: true as const });
        },
      }),
    };
  }

  private get live(): Stream | undefined {
    return this.streams[this.streams.length - 1];
  }

  /** Push a raw event into the live subscription; resolves when it was processed. */
  async emitRaw(event: unknown): Promise<void> {
    const stream = this.live;
    if (!stream || stream.closed) return;
    let ack: () => void = () => {};
    const processed = new Promise<void>((resolve) => {
      ack = resolve;
    });
    stream.push({ event, ack });
    await Promise.race([processed, new Promise<void>((r) => setTimeout(r, EMIT_TIMEOUT_MS))]);
  }

  /** Push a V2 event with a generated id; resolves when it was processed. */
  async emit(event: Omit<V2Event, "id"> & { id?: string }): Promise<void> {
    this.counter += 1;
    await this.emitRaw({ id: `evt_${this.counter}`, created: Date.now(), ...event });
  }

  /** End the live subscription's iterator normally. */
  endStream(): void {
    this.live?.close();
  }

  /** Make the live subscription's iterator throw. */
  failStream(err: unknown = new Error("stream failed")): void {
    this.live?.fail(err);
  }

  /** Run every execute.before callback in order, awaiting each, rethrowing failures. */
  async fireBefore(event: unknown): Promise<void> {
    for (const cb of this.beforeHooks) await cb(event as V2ToolHookEvent);
  }

  /** Run every permission evaluate callback in order, awaiting each, rethrowing failures. */
  async fireEvaluate(event: unknown): Promise<void> {
    for (const cb of this.evaluateHooks) await cb(event);
  }

  /** Run every session compaction callback in order, awaiting each, rethrowing failures. */
  async fireCompaction(event: unknown): Promise<void> {
    for (const cb of this.compactionHooks) await cb(event);
  }

  /** Run every execute.after callback in order, awaiting each, rethrowing failures. */
  async fireAfter(event: unknown): Promise<void> {
    for (const cb of this.afterHooks) await cb(event as V2ToolHookEvent);
  }
}
