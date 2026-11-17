/**
 * artifacts.ts - the human-readable per-invocation markdown artifacts.
 *
 * renderInput and renderOutput are pure and use the OpenCode V1 layout with
 * the Harness row opencode-v2. Rows with an absent value are omitted.
 */

import { HARNESS } from "./core.js";

function row(label: string, value: string | undefined | null): string {
  if (value === undefined || value === null) return "";
  return `| ${label} | ${value} |\n`;
}

interface Identity {
  agentInstanceId: string;
  agentType?: string | null;
  sessionId?: string | null;
  runId?: string | null;
  timestamp: string;
}

function table(title: string, id: Identity, extraRows: string[]): string[] {
  return [
    `# ${title}: ${id.agentInstanceId}\n\n`,
    "| Field | Value |\n",
    "|---|---|\n",
    row("Agent Instance ID", id.agentInstanceId),
    row("Agent Type", id.agentType),
    row("Session ID", id.sessionId),
    row("Run ID", id.runId),
    row("Harness", HARNESS),
    row("Captured At", id.timestamp),
    ...extraRows,
  ];
}

/** The 01_input.md body: metadata table, then the raw prompt. */
export function renderInput(params: Identity & { prompt?: string | null }): string {
  const lines = table("Invocation Input", params, []);
  if (params.prompt !== undefined && params.prompt !== null) {
    lines.push("\n## Prompt\n\n", params.prompt, "\n");
  }
  return lines.join("");
}

/** The 02_output.md body: metadata table, then the raw response. */
export function renderOutput(
  params: Identity & { statusCode?: string | null; response?: string | null },
): string {
  const lines = table("Invocation Output", params, [row("Status Code", params.statusCode)]);
  if (params.response !== undefined && params.response !== null) {
    lines.push("\n## Response\n\n", params.response, "\n");
  }
  return lines.join("");
}
