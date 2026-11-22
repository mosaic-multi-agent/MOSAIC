/**
 * root_lifecycle.ts - session and run records of root sessions.
 *
 * session_start on the first owned signal, run binding at the first MOSAIC
 * dispatch (native mode), and session_end / run_end at each quiescent point:
 * a terminal event of the root while no child invocation is open. Child
 * sessions get invocation records instead and never reach this module.
 *
 * Under the Runner the Runner writes run_start and run_end, so this module
 * writes neither and a dispatch prompt never binds a run. The subagent role
 * writes nothing to an orchestrator stream.
 */

import { appendEvent, buildEvent, currentTimestamp, UNKNOWN_RUN } from "./core.js";
import { dispatchRunId } from "./identity.js";
import { endInvocation } from "./invocations.js";
import { requestRootTranscript } from "./transcript_queue.js";
import { claimPrimary, hasOpenChild, runOfRoot, type InstanceState, type Session } from "./state.js";

function writesRootRecords(state: InstanceState): boolean {
  return state.mode?.role !== "subagent";
}

function emitRoot(
  state: InstanceState,
  session: Session,
  event: string,
  fields: Record<string, unknown>,
  runOverride?: string,
): void {
  const run = runOverride ?? runOfRoot(state, session);
  const runId = run === UNKNOWN_RUN ? undefined : run;
  appendEvent(
    state.paths.orchestratorEvents(run),
    buildEvent(event, { sessionId: session.id, runId, timestamp: currentTimestamp() }, fields),
  );
}

/**
 * Note a signal naming an owned session. A root writes its session_start the
 * first time and counts the signal as activity; a child needs nothing here.
 */
export function touch(state: InstanceState, session: Session): void {
  if (session.parent !== undefined || !claimPrimary(state, session)) return;
  session.active = true;
  if (session.started) return;
  session.started = true;
  if (writesRootRecords(state)) emitRoot(state, session, "session_start", { session_id: session.id });
}

/**
 * Bind a root to the run named by a MOSAIC dispatch prompt. The first binding
 * wins. At binding, run_start and session_start go into the run's own stream.
 */
export function bindRun(state: InstanceState, session: Session, prompt: unknown): void {
  if (state.mode !== undefined || session.parent !== undefined || session.bound !== undefined) return;
  const run = dispatchRunId(prompt);
  if (run === undefined) return;
  session.bound = run;
  session.started = true;
  emitRoot(state, session, "run_start", { adapter_version: state.adapterVersion });
  emitRoot(state, session, "session_start", { session_id: session.id });
}

/**
 * A terminal event of a root: write the end pair when the root had activity
 * since its previous end and no child invocation is open.
 */
export function endIfQuiescent(state: InstanceState, session: Session, reason: string): void {
  if (session.parent !== undefined || !session.active) return;
  if (state.primaryEnded || hasOpenChild(state, session.id)) return;
  session.active = false;
  if (state.mode !== undefined) state.primaryEnded = true;
  if (!writesRootRecords(state)) {
    endInvocation(state, session.id, session.lastAssistant);
    return;
  }
  emitRoot(state, session, "session_end", { reason });
  if (state.mode === undefined && session.bound !== undefined) emitRoot(state, session, "run_end", {});
  // A deleted session can no longer be exported.
  if (reason !== "session.deleted") requestRootTranscript(state, session.id);
}
