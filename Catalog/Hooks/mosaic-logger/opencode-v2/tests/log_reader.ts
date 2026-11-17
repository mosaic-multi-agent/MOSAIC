/**
 * log_reader.ts - read back what the plugin wrote under OrchestrationLogs.
 */

import * as nodeFs from "node:fs";
import * as nodePath from "node:path";

export type LogRecord = Record<string, any>;

export const LOGS = "OrchestrationLogs";
export const UNKNOWN_RUN = "unknown-run";
export const ORCHESTRATOR_FILE = "00_orchestrator_events.jsonl";

const TIMESTAMP_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

/** Parse a JSONL file; a missing file is an empty list. */
export function readJsonl(file: string): LogRecord[] {
  if (!nodeFs.existsSync(file)) return [];
  return nodeFs
    .readFileSync(file, "utf-8")
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as LogRecord);
}

export function readText(file: string): string | undefined {
  return nodeFs.existsSync(file) ? nodeFs.readFileSync(file, "utf-8") : undefined;
}

export function orchestratorStream(dir: string, run: string): LogRecord[] {
  return readJsonl(nodePath.join(dir, LOGS, run, ORCHESTRATOR_FILE));
}

/** Names of the invocation folders directly under a run folder. */
export function invocationFolders(dir: string, run: string): string[] {
  const runDir = nodePath.join(dir, LOGS, run);
  if (!nodeFs.existsSync(runDir)) return [];
  return nodeFs
    .readdirSync(runDir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

export function invocationStream(dir: string, run: string, folder: string): LogRecord[] {
  return readJsonl(nodePath.join(dir, LOGS, run, folder, "03_events.jsonl"));
}

export function runFolders(dir: string): string[] {
  const root = nodePath.join(dir, LOGS);
  if (!nodeFs.existsSync(root)) return [];
  return nodeFs
    .readdirSync(root, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

const LEFTOVER_RE = /(\.lock|\.tmp|\.temp|\.part|~)($|\.)|\.(lock|tmp)\b/;

/** Relative paths of lock sidecars and temp files left under OrchestrationLogs. */
export function leftoverFiles(dir: string): string[] {
  const root = nodePath.join(dir, LOGS);
  const out: string[] = [];
  const walk = (current: string): void => {
    if (!nodeFs.existsSync(current)) return;
    for (const entry of nodeFs.readdirSync(current, { withFileTypes: true })) {
      const full = nodePath.join(current, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (LEFTOVER_RE.test(entry.name)) out.push(nodePath.relative(root, full));
    }
  };
  walk(root);
  return out;
}

/** Every file path (relative to dir, forward slashes) created anywhere in dir outside .mosaic. */
export function createdFiles(dir: string): string[] {
  const out: string[] = [];
  const walk = (current: string): void => {
    for (const entry of nodeFs.readdirSync(current, { withFileTypes: true })) {
      const full = nodePath.join(current, entry.name);
      if (entry.isDirectory()) walk(full);
      else out.push(nodePath.relative(dir, full).split(nodePath.sep).join("/"));
    }
  };
  walk(dir);
  return out.filter((p) => !p.startsWith(".mosaic/"));
}

export interface Located extends LogRecord {
  _run: string;
  _stream: string;
}

/** Every record of every JSONL stream, tagged with its run folder and stream path. */
export function allRecords(dir: string): Located[] {
  const out: Located[] = [];
  for (const run of runFolders(dir)) {
    for (const rec of orchestratorStream(dir, run)) {
      out.push({ ...rec, _run: run, _stream: ORCHESTRATOR_FILE });
    }
    for (const folder of invocationFolders(dir, run)) {
      for (const rec of invocationStream(dir, run, folder)) {
        out.push({ ...rec, _run: run, _stream: `${folder}/03_events.jsonl` });
      }
    }
  }
  return out;
}

export function byEvent(records: LogRecord[], event: string): LogRecord[] {
  return records.filter((rec) => rec.event === event);
}

export interface InvocationView {
  run: string;
  folder: string;
  records: LogRecord[];
  start?: LogRecord;
  end?: LogRecord;
}

/** Every invocation folder in every run, with its start and end records. */
export function invocations(dir: string): InvocationView[] {
  const out: InvocationView[] = [];
  for (const run of runFolders(dir)) {
    for (const folder of invocationFolders(dir, run)) {
      const records = invocationStream(dir, run, folder);
      out.push({
        run,
        folder,
        records,
        start: byEvent(records, "invocation_start")[0],
        end: byEvent(records, "invocation_end")[0],
      });
    }
  }
  return out;
}

/** The invocation whose invocation_start belongs to the given child session. */
export function invocationOf(dir: string, childSession: string): InvocationView | undefined {
  return invocations(dir).find((inv) => inv.start?.session_id === childSession);
}

function hasEmptyValue(value: unknown): boolean {
  if (value === null || value === "") return true;
  if (Array.isArray(value)) return false;
  if (value && typeof value === "object") {
    return Object.values(value).some(hasEmptyValue);
  }
  return false;
}

const EXPECTED_VERSION: Record<string, string> = {
  invocation_start: "1.4.0",
  compaction: "1.5.0",
  tool_call_end: "1.6.0",
  usage_record: "1.7.0",
};

/** Envelope and pruning rules every record must satisfy. Returns the problems found. */
export function envelopeProblems(rec: LogRecord): string[] {
  const problems: string[] = [];
  if (rec.harness !== "opencode-v2") problems.push(`harness ${String(rec.harness)}`);
  if (typeof rec.event !== "string" || rec.event === "") problems.push("event missing");
  const wanted = EXPECTED_VERSION[rec.event] ?? "1.3.0";
  if (rec.schema_version !== wanted) problems.push(`schema_version ${String(rec.schema_version)} != ${wanted}`);
  if (typeof rec.timestamp !== "string" || !TIMESTAMP_RE.test(rec.timestamp)) {
    problems.push(`timestamp ${String(rec.timestamp)}`);
  }
  const payload = { ...rec };
  delete payload._run;
  delete payload._stream;
  if (hasEmptyValue(payload)) problems.push("null or empty value present");
  return problems;
}
