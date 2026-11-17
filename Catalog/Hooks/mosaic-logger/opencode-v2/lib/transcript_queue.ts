/**
 * transcript_queue.ts - the native-mode export queue of one plugin instance.
 *
 * Requests are made after the trigger's records are written and never make the
 * caller wait. Exports run one at a time in request order. A session already
 * waiting is not queued again; a session whose export is running is queued once
 * more. A failed export writes nothing and is not retried. Cleanup ends the
 * running export and drops the waiting ones.
 */

import type { InstanceState } from "./state.js";
import { buildExportCommand } from "./transcript.js";
import { canExport, startExport, type RunningExport } from "./transcript_process.js";
import { isExportOf, resolveTarget, writeTranscript, type TranscriptKind } from "./transcript_write.js";

const EXPORT_TIMEOUT_MS = 120_000;

export interface TranscriptExporter {
  requestChild(id: string): void;
  requestRoot(id: string): void;
  stop(): void;
}

/** The exporter of a native-mode instance. */
export function createExporter(state: InstanceState): TranscriptExporter {
  const waiting: Array<{ id: string; kind: TranscriptKind }> = [];
  let running: RunningExport | undefined;
  let scheduled = false;
  let stopped = false;

  const schedule = (): void => {
    if (scheduled || running !== undefined || stopped || waiting.length === 0) return;
    scheduled = true;
    // A microtask: the export starts right after the event being processed, never inside it.
    queueMicrotask(() => {
      scheduled = false;
      void runNext();
    });
  };

  const runNext = async (): Promise<void> => {
    if (stopped || running !== undefined) return;
    const item = waiting.shift();
    if (item === undefined) return;
    try {
      const target = resolveTarget(state, item.id, item.kind);
      if (target !== undefined && canExport(item.id)) {
        running = startExport(buildExportCommand(item.id, state.dir, process.env), EXPORT_TIMEOUT_MS);
        const bytes = await running.result;
        if (!stopped && bytes !== undefined && isExportOf(bytes, item.id)) writeTranscript(target, bytes);
      }
    } catch {
      // a failed export writes nothing
    }
    running = undefined;
    schedule();
  };

  const request = (id: string, kind: TranscriptKind): void => {
    if (stopped || waiting.some((item) => item.id === id)) return;
    waiting.push({ id, kind });
    schedule();
  };

  return {
    requestChild: (id) => request(id, "child"),
    requestRoot: (id) => request(id, "root"),
    stop(): void {
      stopped = true;
      waiting.length = 0;
      try {
        running?.kill();
      } catch {
        // cleanup never throws
      }
    },
  };
}

/** Request a refresh of the child's transcript; nothing in Runner mode. Never throws. */
export function requestChildTranscript(state: InstanceState, child: string): void {
  try {
    state.exporter?.requestChild(child);
  } catch {
    // a request never fails the caller
  }
}

/** Request a refresh of the root's orchestrator transcript; nothing in Runner mode. Never throws. */
export function requestRootTranscript(state: InstanceState, root: string): void {
  try {
    state.exporter?.requestRoot(root);
  } catch {
    // a request never fails the caller
  }
}
