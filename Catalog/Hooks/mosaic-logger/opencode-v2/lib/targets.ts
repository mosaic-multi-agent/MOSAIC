/**
 * targets.ts - where the records of a session go, and how one is written there.
 *
 * A child session writes into its invocation's stream. A root writes into the
 * orchestrator stream of its run: the run a dispatch bound it to, else the
 * Runner's run, else unknown-run. Under the Runner the primary session in the
 * subagent role is itself an invocation, so its records go to an invocation
 * stream; a root that is not the primary has no target and writes nothing.
 */

import { appendEvent, buildEvent, currentTimestamp, UNKNOWN_RUN } from "./core.js";
import { ensureInvocation } from "./invocations.js";
import { claimPrimary, runOfRoot, type InstanceState, type Session, type Target } from "./state.js";

/**
 * The stream target of a session, or undefined when it is not logged. For the
 * primary session of the subagent role, prompt is the first user prompt when the
 * invocation has to start now.
 */
export function targetFor(state: InstanceState, session: Session, prompt?: string): Target | undefined {
  const needsInvocation = session.parent !== undefined || state.mode?.role === "subagent";
  if (session.parent === undefined && !claimPrimary(state, session)) return undefined;

  if (needsInvocation) {
    const invocation = ensureInvocation(state, session.id, prompt);
    if (!invocation) return undefined;
    return {
      file: state.paths.invocationEvents(invocation.run, invocation.folder),
      sessionId: session.id,
      runId: invocation.run === UNKNOWN_RUN ? undefined : invocation.run,
      instanceId: invocation.instanceId,
      source: "agent_transcript",
    };
  }

  const run = runOfRoot(state, session);
  return {
    file: state.paths.orchestratorEvents(run),
    sessionId: session.id,
    runId: run === UNKNOWN_RUN ? undefined : run,
    source: "orchestrator_transcript",
  };
}

/**
 * Write one record to the target. Fields are pruned; raw fields are added as they
 * are, for values the pruning would drop but the format requires (an empty
 * requested_tools object). timestamp defaults to now.
 */
export function emit(
  target: Target,
  event: string,
  fields: Record<string, unknown>,
  options: { timestamp?: string; raw?: Record<string, unknown> } = {},
): void {
  const record = buildEvent(
    event,
    { sessionId: target.sessionId, runId: target.runId, timestamp: options.timestamp ?? currentTimestamp() },
    fields,
  );
  if (options.raw) Object.assign(record, options.raw);
  appendEvent(target.file, record);
}
