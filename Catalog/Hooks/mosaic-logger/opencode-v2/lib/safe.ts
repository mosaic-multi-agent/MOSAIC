/**
 * safe.ts - defensive readers for host payloads.
 *
 * Every payload the host hands to the plugin is untrusted: it may be null,
 * carry fields of the wrong type, or have members that throw when read.
 * These helpers never throw and never mutate what they read.
 */

/** The value as a plain object, else undefined. */
export function rec(value: unknown): Record<string, unknown> | undefined {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

/** Read one member; undefined when the source is no object or the read throws. */
export function get(source: unknown, key: string): unknown {
  try {
    const obj = rec(source);
    return obj === undefined ? undefined : obj[key];
  } catch {
    return undefined;
  }
}

/** Walk a path of members; undefined as soon as a step is missing. */
export function path(source: unknown, ...keys: string[]): unknown {
  let current: unknown = source;
  for (const key of keys) {
    current = get(current, key);
    if (current === undefined) return undefined;
  }
  return current;
}

/** The value when it is a non-empty string, else undefined. */
export function str(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

/** A string member at the given path, else undefined. */
export function strAt(source: unknown, ...keys: string[]): string | undefined {
  return str(path(source, ...keys));
}
