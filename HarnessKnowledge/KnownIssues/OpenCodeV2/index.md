# OpenCode 2.x — Issue Index

> Last updated: 2026-11-18 (run 1)
> Latest platform version: v2.3.2 (released 2026-11-18)

> **Scope: OpenCode 2.x ONLY.** This folder (`OpenCodeV2/`) tracks the OpenCode **v2** line (`anomalyco/opencode`, 2.x releases; ID prefix `O2-`). OpenCode 1.x is a separate harness tracked in `../OpenCodeV1/` (`OC-` IDs). Both lines live in the same GitHub repository and the same issue tracker, so every issue must be checked for the version it was reported against. Do **not** add 1.x-only findings here, and do not assume an `OC-` entry applies to 2.x (or the reverse). If a behavior is confirmed on both lines, keep one entry per harness and cross-reference the other ID in Notes.
>
> **Why V2 differs:** rewrite with async `subagent` tool (replaces `task`), `shell` instead of `bash`, `patch` for GPT models, Code Mode MCP tools, `permission.evaluate` replacing `permission.ask`, new plugin and server APIs. See `SystemPromptCapture/OpenCodeV2/v2.3.2/AnalysisReport.md`.

## Active Issues (46)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| O2-001 | `subagent` sessionID resume fails (model emits bare "ses") | Bug | Likely | HIGH | Yes | No | Unevaluated |
| O2-002 | 2.3.0 always-background `subagent`; no blocking option | Limitation | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-003 | Declined/rejected permission recorded as shutdown; background child job stuck, parent never notified | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-004 | `subagent` without/with mistyped `sessionID` silently starts a new child | Quirk | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-005 | No default step/time/cost bound; `doom_loop` unenforced; unbounded subagent fan-out | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-006 | No agent tool to list/inspect/cancel child sessions | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-007 | Client replaces shared service after 3 x 2 s health-probe timeouts, aborting all in-flight tools/sessions/subagents | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-008 | V1-to-V2 session import is one-time: later V1 sessions/messages never appear in V2 (404) | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-009 | Starting `serve --service` sweeps sessions another live server (incl. `run --standalone`) runs; two servers drive one session | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| O2-010 | Service resumes killed `run --standalone` session without its permissions/step limit | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-011 | Permission rules match by literal spelling: in-project path rules and shell deny rules silently inert | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-012 | Subagent `ask` permission unanswerable (hidden/404 in child session); call hangs up to 3600 s | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| O2-013 | Global `permissions` appended after built-in/plugin agent rules and override them (Plan guard lost) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-014 | Tool calls leaked as DSML/XML text (DeepSeek family): nothing runs, turn ends silently | Quirk | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-015 | Corrupted/empty tool arguments at very long context; unknown properties dropped and remainder executes | Quirk | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-020 | Compaction fails "summary did not match required template" on GPT-6.2 (Responses provider) | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| O2-021 | Run/turn blocks silently on start snapshot of huge git work tree | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| O2-022 | `opencode run` never exits with busy child session or non-waking inbox row | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-023 | Auto-compaction loops every step when provider reports input tokens above context limit | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| O2-024 | `opencode run --format json` emits only root-session events (no subagent activity) | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-025 | Model step stalls forever on keepalive-only stream; no inactivity timeout, parent waits | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| O2-026 | Unreachable `agents.compaction.model` fails every compaction (fallback only when unresolvable) | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-027 | V1 plugin hook names accepted by V2 `ctx.session.hook()` but never fire (no warning) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-028 | Service start auto-resumes all suspended sessions/jobs (no age limit, consent or off switch) | Limitation | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-029 | Compaction fails terminally on output token limit / no summary (reasoning models, tight windows, growing summaries) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-040 | `shell` fails to spawn Store PowerShell 7 on Windows (0xC0070005, pty pipe mode) | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| O2-041 | Denying `execute` exposes every MCP tool directly (prompt size explosion) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-042 | Code Mode `execute`/MCP calls have no effective timeout; lost MCP reply hangs subagent for hours | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-043 | Code Mode `execute` silently returns `null` for uncalled function program | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-044 | `edit`/`write`/`patch` silently overwrite concurrent change made during permission wait | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-045 | `glob`/`grep` include never match absolute patterns ("No files found") | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-046 | Tool-output truncation keeps head only; tail of MCP/skill/log output lost | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-047 | `edit` fails `path: Missing key` when model sends `filePath`; error hides dropped key | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-048 | PowerShell scanner refuses commands with en/em dash, curly quotes or syntax errors (`invalid-structure`) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-049 | MCP servers (local and remote) that fail once are never retried; only `/mcps` reconnect, config re-add or restart recovers | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-050 | Code Mode lists inline MCP tools by first 120 chars of first description line; later rules never read | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| O2-051 | After compaction, kept tool-call inputs are not truncated (large write/patch inputs resent every turn) | Bug | Confirmed | MEDIUM | No | No | Unevaluated |
| O2-052 | Summary compaction on Responses models may save commentary-phase text into the summary | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-053 | Native compaction (OpenAI/Azure) rejects retained `developer` messages: `Invalid compaction response` | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-054 | `edit` tolerates only quote/dash/space variants and trailing whitespace; indentation/tab mismatch hard-fails, error and hint contradict | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-055 | Every child session reports to its parent on each idle and wakes it; `notify` option removed, no way to silence | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-056 | Rate/usage-limited subagent silently waits and retries; parent cannot reroute | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| O2-057 | Every `shell` call stalls 5-20 s when `opencode-pty` daemon unreachable (2.3.0 regression, no latch) | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-058 | `opencode_worktree_remove` gated only by `edit`; `opencode_worktree_*` deny does not apply | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| O2-059 | `edit`/`patch`/`write` silently corrupt non-UTF-8 files (U+FFFD) and report success | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| O2-060 | `patch` rewrites CRLF files into mixed line endings (V2) | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |

## Resolved / Retired Issues (0)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|-------------------|-----------------|

---

## Quick Summary

*Run 1 (2026-11-18) was the first capture for OpenCode 2.x. Evidence mostly comes from 2.2.x to 2.3.x reports; none of the entries has been re-tested on v2.3.2, and the 2.3.1/2.3.2 release notes show none of the tracked fix PRs shipping. IDs are sparse (O2-001..015, 020..029, 040..060) because discovery passes had separate ranges.*

**Async subagent model and parent wake-ups.** Since 2.3.0 `subagent` is always background with no blocking mode (O2-002), and every child reports to and wakes its parent on each idle with no way to silence it (O2-055). Resume by `sessionID` fails when the model emits a bare `ses` (O2-001), and a missing or mistyped `sessionID` silently spawns a fresh context-free child (O2-004). A rejected permission can leave a background child stuck with the parent never told (O2-003). There is no step, time or cost bound by default (O2-005), no tool to list or cancel children (O2-006), and a rate-limited child waits instead of failing back to the orchestrator (O2-056).

**Service lifecycle and restart-resume (V2-specific, HIGH).** The client replaces the shared service after about 6 s of busy health-probe timeouts, aborting every in-flight tool, session and background subagent (O2-007). Starting a service sweeps and resumes sessions that another live server (including `run --standalone` workers) is running (O2-009), resumes killed runs without their permissions or step limit (O2-010), and has no age limit or off switch for auto-resume (O2-028). V1-to-V2 session import is a one-time latch (O2-008).

**Headless `opencode run` hangs.** A run never exits with a busy child or a non-waking inbox row (O2-022), a keepalive-only stream stalls a step forever (O2-025), start snapshots of huge work trees block silently (O2-021), and Code Mode or MCP calls have no effective timeout (O2-042). `--format json` emits only root-session events (O2-024), so a runner parsing output cannot see subagent activity.

**Permissions.** Rules match by literal spelling, so in-project path rules and shell deny rules can be inert (O2-011). A subagent's `ask` is unanswerable and hangs up to an hour (O2-012). Global `permissions` override built-in agent guards (O2-013), `opencode_worktree_remove` is gated only by `edit` (O2-058), and V1 plugin hook names are accepted but never fire (O2-027).

**Compaction.** Terminal failures on output limit or missing summary (O2-029), template mismatches (O2-020), loops when the provider reports tokens above the limit (O2-023), an unreachable compaction model failing every compaction (O2-026), and bugs in what survives compaction (O2-051, O2-052, O2-053).

**File tools and shell on Windows.** Non-UTF-8 files are silently corrupted by `edit`/`patch`/`write` (O2-059), `patch` mixes line endings (O2-060), concurrent changes are overwritten during permission waits (O2-044), `edit` matching is narrow and its hints contradict (O2-054), and `edit` rejects the V1 `filePath` key with a misleading error (O2-047). `glob`/`grep` miss absolute patterns (O2-045), output truncation keeps only the head (O2-046), Store PowerShell 7 fails to spawn (O2-040), the PTY daemon can stall every `shell` call (O2-057), and the command scanner refuses dashes and curly quotes (O2-048).

**MCP and Code Mode.** Failed servers are never retried (O2-049), inline tool lists show only 120 characters of description (O2-050), `execute` returns `null` for uncalled functions (O2-043), and denying `execute` explodes the prompt (O2-041).

**Model-level quirks.** Tool calls leaked as XML/DSML text (O2-014) and corrupted arguments at very long context (O2-015); compaction is the usual remedy.

## Recommended Mitigations

- **Set `steps` on every agent and put an external watchdog around `opencode run`.** Nothing bounds runtime, stalls or fan-out by default (O2-005, O2-022, O2-025, O2-042).
- **Run one service per data directory, or disable it for headless runs** (`~/.config/opencode/service.json` `{"disabled":true}`, or a separate `XDG_DATA_HOME` per server) so other servers do not sweep or resume your sessions (O2-009, O2-010, O2-028, O2-007).
- **Do not rely on the orchestrator getting a blocking result.** Treat the `[subagent]` message as the signal, prefer a result file as the completion check, always pass the full `sessionID` (`ses_...`) on follow-ups, and pin to 2.2.7 only if blocking is essential (O2-001, O2-002, O2-004, O2-055).
- **Deny `subagent` and unneeded tools for worker agents, and keep guard rules under `agents.<id>.permissions`.** Verify path and shell deny rules with canary calls (O2-011, O2-012, O2-013, O2-058).
- **Use Windows PowerShell 5.1 and avoid non-ASCII punctuation in shell commands** (O2-040, O2-048).
- **Verify file writes by read-back and avoid non-UTF-8 or CRLF files for edit/patch** (O2-059, O2-060, O2-044).
- **Configure compaction explicitly** (reachable `agents.compaction.model`, output headroom) and checkpoint state externally for long sessions (O2-023, O2-026, O2-029).
- **Check the logger hook names** in `Catalog/Hooks/mosaic-logger`: V1 hook names fail silently on V2 (O2-027).

**Needs re-verification on v2.3.2:** nearly every entry; first priority O2-007, 009, 010, 012, 040, 044, 046, 047, 051, 057, 059, 060 (their fix PRs were open at capture time).

**Leads not yet investigated:** #53146 (`session_message.seq` UNIQUE collisions after respawns), #51784 (restart recovery loses client environment), #64627 (`sudo *` always-allow grant), #56247 (permission evaluations not logged), #58681/#51071 (parent-chosen subagent `model` overrides), #62973 and #53226 (failed MCP servers look unconfigured, no restart after transport error), #39565/#62977 (partial shell output lost on interrupt), #64352 (model switched by subagent-completion prompt, Desktop app), #61284 (subagent write denied under `opencode run`), #62792 (restart recovery drops child completion notice).
