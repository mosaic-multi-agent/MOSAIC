/**
 * The export command the plugin runs for one transcript: a fixed argument
 * vector, the instance directory as working directory, and a child
 * environment that cannot put the child into Runner mode.
 */

import { describe, expect, it } from "vitest";

import { TRANSCRIPT_NATIVE_FORMAT, TRANSCRIPT_SOURCE_MECHANISM, buildExportCommand } from "../lib/transcript";

const DIR = process.platform === "win32" ? "C:\\work\\proj" : "/work/proj";
const SESSION = "ses_e161e29afffe8w3w7eyl30w0bx";

describe("transcript constants", () => {
  it("name the V2-specific export format and the CLI mechanism", () => {
    expect(TRANSCRIPT_NATIVE_FORMAT).toBe("opencode-v2-session-export-json");
    expect(TRANSCRIPT_SOURCE_MECHANISM).toBe("cli_session_export");
  });
});

describe("buildExportCommand", () => {
  it("runs `opencode session export <id> --standalone` in the instance directory", () => {
    const cmd = buildExportCommand(SESSION, DIR, {});

    expect(cmd.command).toBe("opencode");
    expect(cmd.args).toEqual(["session", "export", SESSION, "--standalone"]);
    expect(cmd.cwd).toBe(DIR);
  });

  it("never asks for sanitized output", () => {
    const cmd = buildExportCommand(SESSION, DIR, {});

    expect(cmd.args.join(" ")).not.toContain("--sanitize");
  });

  it("keeps a session id with shell metacharacters as exactly one argument", () => {
    const hostile = "ses_x; rm -rf / && echo $(whoami) \"q\"";

    const cmd = buildExportCommand(hostile, DIR, {});

    expect(cmd.args).toHaveLength(4);
    expect(cmd.args[2]).toBe(hostile);
  });

  it("removes the Runner-mode variables and sets PWD to the directory", () => {
    const cmd = buildExportCommand(SESSION, DIR, {
      MOSAIC_ROLE: "subagent",
      MOSAIC_RUN_ID: "20261117T150000Z-abcd",
      MOSAIC_AGENT_INSTANCE_ID: "Worker#1",
      PWD: "/somewhere/else",
      PATH: "/usr/bin",
    });

    expect(cmd.env.MOSAIC_ROLE).toBeUndefined();
    expect(cmd.env.MOSAIC_RUN_ID).toBeUndefined();
    expect(cmd.env.MOSAIC_AGENT_INSTANCE_ID).toBeUndefined();
    expect(cmd.env.PWD).toBe(DIR);
    expect(cmd.env.PATH).toBe("/usr/bin");
  });

  it("keeps every other string entry and drops entries without a value", () => {
    const cmd = buildExportCommand(SESSION, DIR, { HOME: "/home/u", OPENCODE_X: "1", EMPTY: "", GONE: undefined });

    expect(cmd.env.HOME).toBe("/home/u");
    expect(cmd.env.OPENCODE_X).toBe("1");
    expect(cmd.env.EMPTY).toBe("");
    expect("GONE" in cmd.env).toBe(false);
  });

  it("matches the removed names case-insensitively on Windows and exactly elsewhere", () => {
    const cmd = buildExportCommand(SESSION, DIR, { Mosaic_Role: "orchestrator", Pwd: "/old" });

    if (process.platform === "win32") {
      expect(cmd.env.Mosaic_Role).toBeUndefined();
      expect(cmd.env.Pwd).toBeUndefined();
    } else {
      expect(cmd.env.Mosaic_Role).toBe("orchestrator");
      expect(cmd.env.Pwd).toBe("/old");
    }
    expect(cmd.env.PWD).toBe(DIR);
  });
});
