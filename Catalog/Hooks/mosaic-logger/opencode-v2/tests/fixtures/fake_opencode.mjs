// Stand-in for the `opencode` command in transcript tests.
//
// Called as: opencode session export <id> --standalone
// Environment (set by the test):
//   FAKE_OPENCODE_DIR    folder with <id>.json (served verbatim) and optional <id>.exit (exit code)
//   FAKE_OPENCODE_LOG    JSON-lines file; one "start" and one "end" record per call
//   FAKE_OPENCODE_DELAY  milliseconds to wait before answering (default 0)

import * as fs from "node:fs";
import * as path from "node:path";

const args = process.argv.slice(2);
const dir = process.env.FAKE_OPENCODE_DIR ?? "";
const log = process.env.FAKE_OPENCODE_LOG ?? "";
const delay = Number(process.env.FAKE_OPENCODE_DELAY ?? "0");
const id = args[2] ?? "";

function record(phase) {
  if (!log) return;
  const entry = {
    phase,
    at: Date.now(),
    args,
    cwd: fs.realpathSync(process.cwd()),
    pwd: process.env.PWD ?? null,
    mosaicRole: process.env.MOSAIC_ROLE ?? null,
    mosaicRunId: process.env.MOSAIC_RUN_ID ?? null,
    mosaicInstance: process.env.MOSAIC_AGENT_INSTANCE_ID ?? null,
    keepMarker: process.env.FAKE_OPENCODE_KEEP ?? null,
    stdinIsTTY: Boolean(process.stdin.isTTY),
  };
  fs.appendFileSync(log, JSON.stringify(entry) + "\n");
}

record("start");
// The served document is read when the command starts, so a document changed
// while this call is running shows up only in a later call.
const body = path.join(dir, `${id}.json`);
const exitFile = path.join(dir, `${id}.exit`);
const present = fs.existsSync(body);
const bytes = present ? fs.readFileSync(body) : Buffer.alloc(0);
const code = present && fs.existsSync(exitFile) ? Number(fs.readFileSync(exitFile, "utf-8").trim()) : 0;
if (delay > 0) await new Promise((resolve) => setTimeout(resolve, delay));

if (!present) {
  record("end");
  process.exitCode = 1;
} else {
  process.stdout.write(bytes, () => {
    record("end");
    process.exitCode = code;
  });
}
