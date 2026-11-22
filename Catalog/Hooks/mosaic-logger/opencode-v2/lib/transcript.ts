/**
 * transcript.ts - the command of one native-mode transcript export.
 *
 * A transcript is the output of `opencode session export <id> --standalone`, run
 * as a child process. --standalone makes the export use its own private server, so
 * it never starts or depends on the shared service.
 */

export const TRANSCRIPT_NATIVE_FORMAT = "opencode-v2-session-export-json";
export const TRANSCRIPT_SOURCE_MECHANISM = "cli_session_export";

/** Variables that would put the export's own private server into Runner mode (plus PWD, set below). */
const REMOVED_VARIABLES: ReadonlySet<string> = new Set([
  "MOSAIC_ROLE",
  "MOSAIC_RUN_ID",
  "MOSAIC_AGENT_INSTANCE_ID",
  "PWD",
]);

/** The child process one transcript export runs. */
export interface ExportCommand {
  command: string;
  args: string[];
  cwd: string;
  env: Record<string, string>;
}

/**
 * Builds the export command for sessionID. env is the host environment: every
 * string entry is kept except the Runner-mode variables and PWD, and PWD is set to
 * the instance directory. Names match case-insensitively on Windows only.
 */
export function buildExportCommand(
  sessionID: string,
  directory: string,
  env: Record<string, string | undefined>,
): ExportCommand {
  const insensitive = process.platform === "win32";
  const childEnv: Record<string, string> = {};
  for (const [name, value] of Object.entries(env ?? {})) {
    if (typeof value !== "string") continue;
    if (REMOVED_VARIABLES.has(insensitive ? name.toUpperCase() : name)) continue;
    childEnv[name] = value;
  }
  childEnv["PWD"] = directory;
  return {
    command: "opencode",
    args: ["session", "export", sessionID, "--standalone"],
    cwd: directory,
    env: childEnv,
  };
}
