/**
 * invocations.ts - subagent invocations and their correlation with dispatches.
 *
 * A dispatch is a `subagent` execute.before from an owned session without a
 * usable input.sessionID. Its child session is found by the call id (the launch
 * result or a tool-progress event: the authoritative join) or, when a record of
 * a child is due before that, by the oldest unjoined dispatch of the same parent
 * and agent type (the fallback join). Binding is one-to-one and final: a join
 * that conflicts with an existing binding changes nothing.
 *
 * Each child has exactly one invocation. invocation_start is the first record of
 * its stream; invocation_end is written at the child's first wake-up whose state
 * is not "running".
 */

import {
  appendEvent,
  buildEvent,
  currentTimestamp,
  sanitizeComponent,
  UNKNOWN_RUN,
  writeTextAtomic,
} from "./core.js";
import { renderInput, renderOutput } from "./artifacts.js";
import { extractInstanceId, extractRunId, extractStatusCode, fallbackInstanceId } from "./identity.js";
import { strAt } from "./safe.js";
import {
  markChild,
  rememberDispatch,
  rootOf,
  type Dispatch,
  type InstanceState,
  type Invocation,
} from "./state.js";
import { requestRootTranscript } from "./transcript_queue.js";

const SUBAGENT_TAG_OPEN = /^<subagent(?:\s+(?:[^>"']|"[^"]*"|'[^']*')*)?>/;
const SUBAGENT_TAG_CLOSE = "</subagent>";

/** Capture a dispatch before its hook callback resolves. */
export function captureDispatch(
  state: InstanceState,
  parent: string,
  callId: string | undefined,
  input: unknown,
): void {
  const key = callId ?? `anonymous_${(state.anonymousCalls += 1)}`;
  if (state.dispatches.has(key)) return;
  const prompt = strAt(input, "prompt");
  rememberDispatch(state, {
    key,
    callId,
    parent,
    agent: strAt(input, "agent"),
    prompt,
    instanceId: extractInstanceId(prompt),
    runId: extractRunId(prompt),
    discarded: false,
  });
}

/** The launch result of a dispatch named no child or failed: it never produces an invocation. */
export function discardDispatch(state: InstanceState, callId: string): void {
  const dispatch = state.dispatches.get(callId);
  if (dispatch && dispatch.child === undefined) dispatch.discarded = true;
}

/**
 * Authoritative join of the dispatch with this call id to its child. A conflict
 * with an existing binding changes nothing. The child's invocation starts now.
 */
export function joinByCall(state: InstanceState, callId: string, parent: string, child: string): void {
  const dispatch = state.dispatches.get(callId);
  if (!dispatch || dispatch.parent !== parent || dispatch.discarded) return;
  if (dispatch.child !== undefined || state.dispatchByChild.has(child)) return;
  dispatch.child = child;
  state.dispatchByChild.set(child, dispatch);
  markChild(state, child, parent, dispatch.agent);
  ensureInvocation(state, child);
}

function takeFallbackDispatch(state: InstanceState, child: string): Dispatch | undefined {
  const session = state.sessions.get(child);
  if (!session || session.parent === undefined || session.agent === undefined) return undefined;
  for (const dispatch of state.dispatches.values()) {
    if (dispatch.child !== undefined || dispatch.discarded) continue;
    if (dispatch.parent !== session.parent || dispatch.agent !== session.agent) continue;
    dispatch.child = child;
    state.dispatchByChild.set(child, dispatch);
    return dispatch;
  }
  return undefined;
}

function invocationRun(state: InstanceState, parent: string, dispatch: Dispatch | undefined): string {
  if (state.mode?.runId !== undefined) return state.mode.runId;
  return dispatch?.runId ?? rootOf(state, parent)?.bound ?? UNKNOWN_RUN;
}

/** A folder name no other invocation of the run uses. */
function claimFolder(state: InstanceState, run: string, instanceId: string): string {
  const base = sanitizeComponent(instanceId);
  for (let n = 1; ; n += 1) {
    const name = n === 1 ? base : `${base}_${n}`;
    const key = `${run}/${name}`.toLowerCase();
    if (!state.claimedFolders.has(key)) {
      state.claimedFolders.add(key);
      return name;
    }
  }
}

/** Ask for a refresh of the orchestrator transcript of the root a child belongs to. */
function requestRootOfChild(state: InstanceState, child: string): void {
  const parent = state.sessions.get(child)?.parent;
  const root = parent === undefined ? undefined : rootOf(state, parent);
  if (root !== undefined) requestRootTranscript(state, root.id);
}

interface OpenParams {
  session: string;
  instanceId: string;
  agentType?: string;
  run: string;
  prompt?: string;
  spawnCallId?: string;
}

/** Register the invocation and write its first record and 01_input.md. */
function openInvocation(state: InstanceState, params: OpenParams): Invocation {
  const { session, instanceId, agentType, run, prompt } = params;
  const runId = run === UNKNOWN_RUN ? undefined : run;
  const folder = claimFolder(state, run, instanceId);
  const invocation: Invocation = { child: session, instanceId, agentType, run, folder, ended: false };
  state.invocations.set(session, invocation);

  const timestamp = currentTimestamp();
  appendEvent(
    state.paths.invocationEvents(run, folder),
    buildEvent(
      "invocation_start",
      { sessionId: session, runId, timestamp },
      {
        agent_instance_id: instanceId,
        agent_type: agentType,
        prompt,
        spawn_call_id: params.spawnCallId,
      },
    ),
  );
  writeTextAtomic(
    state.paths.invocationInput(run, folder),
    renderInput({ agentInstanceId: instanceId, agentType, sessionId: session, runId, timestamp, prompt }),
  );
  requestRootOfChild(state, session);
  return invocation;
}

function startInvocation(state: InstanceState, child: string, dispatch: Dispatch | undefined): Invocation | undefined {
  const session = state.sessions.get(child);
  if (!session || session.parent === undefined) return undefined;
  return openInvocation(state, {
    session: child,
    instanceId: dispatch?.instanceId ?? fallbackInstanceId(),
    agentType: dispatch?.agent ?? session.agent,
    run: invocationRun(state, session.parent, dispatch),
    prompt: dispatch?.prompt,
    spawnCallId: dispatch?.callId,
  });
}

/** The invocation of the Runner's primary session in the subagent role, with the Runner's identity. */
function startPrimaryInvocation(state: InstanceState, prompt: string | undefined): Invocation | undefined {
  const mode = state.mode;
  const primary = state.primary;
  if (mode?.role !== "subagent" || mode.agentInstanceId === undefined || primary === undefined) return undefined;
  return openInvocation(state, {
    session: primary,
    instanceId: mode.agentInstanceId,
    agentType: mode.agentType,
    run: mode.runId ?? UNKNOWN_RUN,
    prompt,
  });
}

/**
 * The invocation of a session, started now when it has none. A known child starts
 * with its joined dispatch, else by the fallback join, else without a dispatch. The
 * Runner's primary session in the subagent role starts with the Runner's identity
 * and the given prompt. Undefined for any other session.
 */
export function ensureInvocation(state: InstanceState, session: string, prompt?: string): Invocation | undefined {
  const existing = state.invocations.get(session);
  if (existing) return existing;
  if (state.mode?.role === "subagent" && state.primary === session) return startPrimaryInvocation(state, prompt);
  const dispatch = state.dispatchByChild.get(session) ?? takeFallbackDispatch(state, session);
  return startInvocation(state, session, dispatch);
}

/**
 * The text between the opening <subagent ...> tag and the final </subagent>,
 * without the one newline after the tag and the one before the closing tag.
 * The whole text when it does not have that form.
 */
export function wakeUpResponse(text: string): string {
  const open = SUBAGENT_TAG_OPEN.exec(text);
  const trimmed = text.trimEnd();
  if (!open || !trimmed.endsWith(SUBAGENT_TAG_CLOSE) || trimmed.length < open[0].length + SUBAGENT_TAG_CLOSE.length) {
    return text;
  }
  let body = trimmed.slice(open[0].length, trimmed.length - SUBAGENT_TAG_CLOSE.length);
  if (body.startsWith("\r\n")) body = body.slice(2);
  else if (body.startsWith("\n")) body = body.slice(1);
  if (body.endsWith("\r\n")) body = body.slice(0, -2);
  else if (body.endsWith("\n")) body = body.slice(0, -1);
  return body;
}

/**
 * End the child's invocation from its first non-running wake-up: invocation_end
 * and 02_output.md. Later wake-ups write nothing.
 */
export function endInvocation(state: InstanceState, child: string, wakeUpText: string | undefined): void {
  const invocation = ensureInvocation(state, child);
  if (!invocation || invocation.ended) return;
  invocation.ended = true;

  const response = wakeUpText === undefined ? undefined : wakeUpResponse(wakeUpText);
  const statusCode = extractStatusCode(response);
  const runId = invocation.run === UNKNOWN_RUN ? undefined : invocation.run;
  const timestamp = currentTimestamp();
  appendEvent(
    state.paths.invocationEvents(invocation.run, invocation.folder),
    buildEvent(
      "invocation_end",
      { sessionId: child, runId, timestamp },
      { agent_instance_id: invocation.instanceId, status_code: statusCode, response },
    ),
  );
  writeTextAtomic(
    state.paths.invocationOutput(invocation.run, invocation.folder),
    renderOutput({
      agentInstanceId: invocation.instanceId,
      agentType: invocation.agentType,
      sessionId: child,
      runId,
      timestamp,
      statusCode,
      response: response === "" ? undefined : response,
    }),
  );
  requestRootOfChild(state, child);
}
