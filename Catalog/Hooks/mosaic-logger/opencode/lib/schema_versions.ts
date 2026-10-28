/**
 * schema_versions.ts — Per-event log schema version stamps.
 *
 * Every event carries the default version unless the event name has an entry
 * in EVENT_SCHEMA_VERSIONS.
 */

export const DEFAULT_SCHEMA_VERSION: string = "1.3.0";

/** Events whose shape follows a newer log format version than the default. */
export const EVENT_SCHEMA_VERSIONS: Readonly<Record<string, string>> = {
  compaction: "1.5.0",
};

/** Schema version stamped on the named event. */
export function schemaVersionFor(event: string): string {
  return Object.prototype.hasOwnProperty.call(EVENT_SCHEMA_VERSIONS, event)
    ? EVENT_SCHEMA_VERSIONS[event]
    : DEFAULT_SCHEMA_VERSION;
}
