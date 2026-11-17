/**
 * identity.ts - identity extraction and fallback id generators.
 *
 * run_id / agent_instance_id / status_code extraction from dispatch and
 * response text, plus fallback ids for when extraction fails. Same rules as
 * the OpenCode V1 variant: structured key first, then the earliest bare match.
 * All functions are pure and never throw.
 */

// Structured match: run_id key (optional quotes, colon or equals) then a run id.
const RUN_ID_STRUCTURED_RE =
  /(?<![A-Za-z0-9_])run_id\s*['"]?\s*[:=]\s*['"]?\s*([0-9]{8}T[0-9]{6}Z-[0-9a-f]{4})/;

// Bare match: a run id not directly preceded by an alphanumeric or ._- character.
const RUN_ID_BARE_RE = /(?<![A-Za-z0-9_.-])([0-9]{8}T[0-9]{6}Z-[0-9a-f]{4})/;

/** The MOSAIC run id in text (structured key first, then earliest bare match), else undefined. */
export function extractRunId(text: string | undefined | null): string | undefined {
  try {
    if (!text) return undefined;
    const structured = RUN_ID_STRUCTURED_RE.exec(text);
    if (structured) return structured[1];
    const bare = RUN_ID_BARE_RE.exec(text);
    return bare ? bare[1] : undefined;
  } catch {
    return undefined;
  }
}

const INSTANCE_STRUCTURED_RE =
  /(?<![A-Za-z0-9_])agent_instance_id\s*['"]?\s*[:=]\s*['"]?\s*([A-Za-z][A-Za-z0-9_.-]*#[0-9]+)/;

const INSTANCE_BARE_RE = /(?<![A-Za-z0-9_.-])([A-Za-z][A-Za-z0-9_.-]*#[0-9]+)/;

/** The MOSAIC {Name}#{Number} instance id in text, else undefined. */
export function extractInstanceId(text: string | undefined | null): string | undefined {
  try {
    if (!text) return undefined;
    const structured = INSTANCE_STRUCTURED_RE.exec(text);
    if (structured) return structured[1];
    const bare = INSTANCE_BARE_RE.exec(text);
    return bare ? bare[1] : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Run id of a MOSAIC dispatch prompt: a dispatch carries an agent instance id
 * as well as a run id. Only such a prompt binds a session to a run.
 */
export function dispatchRunId(prompt: unknown): string | undefined {
  if (typeof prompt !== "string" || prompt === "") return undefined;
  if (extractInstanceId(prompt) === undefined) return undefined;
  return extractRunId(prompt);
}

/** The closed set of valid MOSAIC communication protocol status codes. */
export const VALID_STATUS_CODES: ReadonlySet<string> = new Set([
  "SUCCESS",
  "COMPLETED_NEEDS_ACTION",
  "PARTIALLY_DONE",
  "NEEDS_CLARIFICATION",
  "CAPABILITY_EXCEEDED",
  "BLOCKED",
]);

const STATUS_CODE_RE = /(?<![A-Za-z0-9_])status_code\s*['"]?\s*[:=]\s*['"]([A-Z_]+)['"]/g;

/**
 * The status code of a subagent's final response: structured key with a quoted
 * value from the closed set, last match wins, else undefined.
 */
export function extractStatusCode(text: string | undefined | null): string | undefined {
  try {
    if (!text) return undefined;
    const re = new RegExp(STATUS_CODE_RE.source, "g");
    let last: string | undefined;
    let match: RegExpExecArray | null;
    while ((match = re.exec(text)) !== null) last = match[1];
    return last !== undefined && VALID_STATUS_CODES.has(last) ? last : undefined;
  } catch {
    return undefined;
  }
}

function compactTimestamp(): string {
  const now = new Date();
  const pad = (n: number): string => String(n).padStart(2, "0");
  return (
    `${now.getUTCFullYear()}${pad(now.getUTCMonth() + 1)}${pad(now.getUTCDate())}` +
    `T${pad(now.getUTCHours())}${pad(now.getUTCMinutes())}${pad(now.getUTCSeconds())}Z`
  );
}

function randomHex4(): string {
  return Math.floor(Math.random() * 0x10000)
    .toString(16)
    .padStart(4, "0");
}

/** {prefix}_{YYYYMMDD}T{HHMMSS}Z-{4hex}; "agent" when the prefix is empty. */
export function fallbackInstanceId(prefix?: string): string {
  return `${prefix || "agent"}_${compactTimestamp()}-${randomHex4()}`;
}
