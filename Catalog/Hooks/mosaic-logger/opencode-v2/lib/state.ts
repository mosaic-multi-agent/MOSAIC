/**
 * state.ts - everything one plugin instance knows.
 *
 * All state belongs to the object createState returns; there is no module-level
 * mutable state, so two instances in one process never see each other's
 * sessions, bindings or invocations.
 */

import { LogPaths, UNKNOWN_RUN } from "./core.js";
import type { RunnerMode } from "./runner_mode.js";
import type { TranscriptExporter } from "./transcript_queue.js";

/** One subagent dispatch seen at its execute.before hook. */
export interface Dispatch {
  /** Lookup key: the tool call id, or a generated key when the host sent none. */
  key: string;
  callId?: string;
  parent: string;
  agent?: string;
  prompt?: string;
  instanceId?: string;
  runId?: string;
  /** The child this dispatch is joined to; set once and never changed. */
  child?: string;
  discarded: boolean;
}

/** The one invocation of a child session. */
export interface Invocation {
  child: string;
  instanceId: string;
  agentType?: string;
  /** Run folder name: the run id, or unknown-run. */
  run: string;
  folder: string;
  ended: boolean;
}

export interface Session {
  id: string;
  /** Known parent session (an owned session named it); undefined for a root. */
  parent?: string;
  agent?: string;
  /** Run a root is bound to by a MOSAIC dispatch (native mode). */
  bound?: string;
  /** session_start was written for this root. */
  started: boolean;
  /** Activity since the root's previous session_end. */
  active: boolean;
  /** Text of the last assistant turn written for the session. */
  lastAssistant?: string;
}

/** Where the records of one actor go, and how they are attributed. */
export interface Target {
  file: string;
  sessionId: string;
  /** Run id for the envelope; absent in the unknown-run folder. */
  runId?: string;
  /** Present on invocation streams only. */
  instanceId?: string;
  source: "orchestrator_transcript" | "agent_transcript";
}

/** A tool call whose start was written (the end goes to the same stream). */
export interface CallRecord {
  callId: string;
  tool: string;
  target: Target;
  /** The call id was generated because the host sent none. */
  generated: boolean;
  ended: boolean;
}

/** What the step events tell about one assistant message before it ends. */
export interface StepInfo {
  model?: string;
  provider?: string;
  /** Epoch milliseconds of the step start. */
  started?: number;
  tools: Map<string, number>;
  toolIds: Set<string>;
  texts: Array<{ ordinal: number; seq: number; text: string }>;
}

/** A permission prompt written as a notification and not yet resolved. */
export interface Ask {
  target: Target;
  resolved: boolean;
}

/** Record ids written for one session, and the next record index. */
export interface UsageLedger {
  next: number;
  ids: Set<string>;
}

export interface InstanceState {
  readonly dir: string;
  readonly paths: LogPaths;
  readonly mode: RunnerMode | undefined;
  adapterVersion: string | undefined;
  readonly sessions: Map<string, Session>;
  readonly dispatches: Map<string, Dispatch>;
  readonly dispatchByChild: Map<string, Dispatch>;
  readonly invocations: Map<string, Invocation>;
  readonly claimedFolders: Set<string>;
  anonymousCalls: number;
  /** Runner mode: the process's first root session. */
  primary: string | undefined;
  /** Runner mode: the primary's end records were written. */
  primaryEnded: boolean;
  readonly calls: Map<string, CallRecord>;
  /** Per session, the keys of calls with a generated id, in start order. */
  readonly generatedCalls: Map<string, string[]>;
  readonly steps: Map<string, StepInfo>;
  readonly turnsDone: Set<string>;
  readonly inboxSeen: Set<string>;
  readonly usage: Map<string, UsageLedger>;
  readonly compactionsSeen: Set<string>;
  readonly asks: Map<string, Ask>;
  /** Permission request id to the notification id of its ask. */
  readonly askRequests: Map<string, string>;
  /** Native mode: the transcript export queue, set by the plugin. Absent under the Runner. */
  exporter: TranscriptExporter | undefined;
}

const MAX_REMEMBERED_DISPATCHES = 2000;
const MAX_REMEMBERED_ITEMS = 5000;
const MAX_PARENT_DEPTH = 32;

export function createState(dir: string, mode: RunnerMode | undefined): InstanceState {
  return {
    dir,
    paths: new LogPaths(dir),
    mode,
    adapterVersion: undefined,
    sessions: new Map(),
    dispatches: new Map(),
    dispatchByChild: new Map(),
    invocations: new Map(),
    claimedFolders: new Set(),
    anonymousCalls: 0,
    primary: undefined,
    primaryEnded: false,
    calls: new Map(),
    generatedCalls: new Map(),
    steps: new Map(),
    turnsDone: new Set(),
    inboxSeen: new Set(),
    usage: new Map(),
    compactionsSeen: new Set(),
    asks: new Map(),
    askRequests: new Map(),
    exporter: undefined,
  };
}

/** Set a map entry, forgetting the oldest entries beyond a bound. */
export function remember<K, V>(map: Map<K, V>, key: K, value: V): void {
  map.set(key, value);
  if (map.size <= MAX_REMEMBERED_ITEMS) return;
  const oldest = map.keys().next();
  if (!oldest.done) map.delete(oldest.value);
}

/** Add a set member, forgetting the oldest members beyond a bound. */
export function rememberKey(set: Set<string>, key: string): void {
  set.add(key);
  if (set.size <= MAX_REMEMBERED_ITEMS) return;
  const oldest = set.values().next();
  if (!oldest.done) set.delete(oldest.value);
}

/**
 * Whether the session's records are logged by this instance. Natively every owned
 * session is. Under the Runner the process's first root is the primary: it is
 * logged, its children are, and any other root is not.
 */
export function claimPrimary(state: InstanceState, session: Session): boolean {
  if (state.mode === undefined || session.parent !== undefined) return true;
  if (state.primary === undefined) state.primary = session.id;
  return state.primary === session.id;
}

/** The session record, created when the session becomes owned. */
export function ensureSession(state: InstanceState, id: string): Session {
  let session = state.sessions.get(id);
  if (!session) {
    session = { id, started: false, active: false };
    state.sessions.set(id, session);
  }
  return session;
}

/** Record that parent named id as its child (ownership by an owned parent). */
export function markChild(state: InstanceState, id: string, parent: string, agent?: string): Session {
  const session = ensureSession(state, id);
  if (session.parent === undefined && id !== parent) session.parent = parent;
  if (session.agent === undefined && agent !== undefined) session.agent = agent;
  return session;
}

/** The root of a session's chain of parents (the session itself when it is a root). */
export function rootOf(state: InstanceState, id: string): Session | undefined {
  let current = state.sessions.get(id);
  for (let depth = 0; current && depth < MAX_PARENT_DEPTH; depth += 1) {
    if (current.parent === undefined) return current;
    current = state.sessions.get(current.parent);
  }
  return current;
}

/** True when a known child of the session has not ended its invocation. */
export function hasOpenChild(state: InstanceState, id: string): boolean {
  for (const session of state.sessions.values()) {
    if (session.parent !== id) continue;
    const invocation = state.invocations.get(session.id);
    if (invocation === undefined || !invocation.ended) return true;
  }
  return false;
}

/** Run folder of a root session: its binding, else the Runner's run, else unknown-run. */
export function runOfRoot(state: InstanceState, session: Session | undefined): string {
  return session?.bound ?? state.mode?.runId ?? UNKNOWN_RUN;
}

/** Remember a dispatch, forgetting the oldest ones beyond a bound. */
export function rememberDispatch(state: InstanceState, dispatch: Dispatch): void {
  state.dispatches.set(dispatch.key, dispatch);
  if (state.dispatches.size <= MAX_REMEMBERED_DISPATCHES) return;
  const oldest = state.dispatches.keys().next();
  if (!oldest.done) state.dispatches.delete(oldest.value);
}
