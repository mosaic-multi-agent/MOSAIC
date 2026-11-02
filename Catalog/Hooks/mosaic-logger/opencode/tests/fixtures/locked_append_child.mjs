// Child writer for the locked-append tests: appends long lines through the module under test.
// Usage: node locked_append_child.mjs <sync|async> <target> <tag> <count> <pad>
import { lockedAppend, lockedAppendSync } from "../../lib/locked_append.ts";

const [mode, target, tag, countText, padText] = process.argv.slice(2);
const count = Number(countText);
const pad = "p".repeat(Number(padText));

let failed = 0;
for (let seq = 0; seq < count; seq++) {
  const line = JSON.stringify({ event: "probe", writer: tag, seq, pad }) + "\n";
  const written = mode === "sync"
    ? lockedAppendSync(target, line)
    : await lockedAppend(target, line);
  if (!written) failed++;
}
process.exit(failed === 0 ? 0 : 3);
