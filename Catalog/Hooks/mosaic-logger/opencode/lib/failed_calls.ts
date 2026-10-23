/**
 * failed_calls.ts - Shared mark for tool calls known to have failed.
 *
 * A failed subtask fires the after-hook (empty result) before OpenCode writes
 * its error part. The after-hook handler marks the call here and leaves it
 * open; whichever closing signal arrives next (error part or dispatch closure)
 * reads the mark. State is held per correlation store so the tool handlers and
 * the tool-part handlers, built over the same store, share it.
 */

const marks = new WeakMap<object, Set<string>>();

/** Record that the open call with this id has failed. */
export function markCallFailed(store: object, callId: string): void {
  let set = marks.get(store);
  if (!set) {
    set = new Set<string>();
    marks.set(store, set);
  }
  set.add(callId);
}

/** Whether the call was marked failed. */
export function isCallFailed(store: object, callId: string): boolean {
  return marks.get(store)?.has(callId) ?? false;
}
