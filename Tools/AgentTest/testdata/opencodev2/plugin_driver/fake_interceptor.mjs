// Fake interceptor executable. argv[2] is the scenario directory. It logs its
// arguments and stdin to <dir>/log.jsonl, prints <dir>/reply-<phase> when that
// file exists and exits with the code in <dir>/exit-<phase> (default 0).
import { readFileSync, appendFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const args = process.argv.slice(2);
const dir = args[0];
const phase = args[args.indexOf("--phase") + 1];

let stdin = "";
for await (const chunk of process.stdin) stdin += chunk;
appendFileSync(join(dir, "log.jsonl"), JSON.stringify({ args, phase, stdin }) + "\n");

const replyFile = join(dir, "reply-" + phase);
if (existsSync(replyFile)) process.stdout.write(readFileSync(replyFile, "utf8"));
const exitFile = join(dir, "exit-" + phase);
process.exitCode = existsSync(exitFile) ? Number(readFileSync(exitFile, "utf8")) : 0;
