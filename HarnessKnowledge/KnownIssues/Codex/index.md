# Codex CLI — Issue Index

> Last updated: 2026-11-21 (run 2)
> Latest platform version: 0.170.0 (released 2026-11-18); 0.171.0 is referenced in issue comments, not yet reviewed

> **Scope: Codex CLI (`openai/codex`, Rust releases tagged `rust-vX.Y.Z`; ID prefix `CX-`).** MOSAIC runs Codex **Runner-only**: each agent step is a headless `codex exec` invocation (JSONL event stream via `--json`, `-o/--output-last-message`, `--output-schema`, resume, per-call sandbox, MCP servers such as `contact-user`). Native in-Codex orchestration is not supported, so multi-agent (`spawn_agent`/`wait_agent`) issues are tracked only when they can affect a `codex exec` run. Agent files are TOML; skills resolve from a shared `.agents/skills` root. See `SystemPromptCapture/Codex/v0.170.0/`.

> **Coverage status: run 2 closed the run-1 gaps** (rate-limit rescan, Windows backend default, `apply_patch` encoding, network/hooks, MCP startup/timeouts, all run-1 leads). Remaining, lower-priority leads for a run 3:
> - 0.171.0 changelog and regressions (not reviewed).
> - Windows sandbox backends: #62720 (error 448 elevated vs MXC), #61311 (TEMP write ACE error 5), #61268 (elevated helper exits 1, related to CX-160).
> - #61651 (`codex exec` behind an authenticated HTTP proxy), #62513 (signed git commits from sandbox), #62291 (DNS failure / blocked escalation).
> - `apply_patch`: #14675, #13574, #12350 (UNC/mapped paths), #13918; compaction accounting root cause #49026, #39767; #62681 (MCP tool-list change notifications over Streamable HTTP).
> - Windows default sandbox backend for `codex exec` is `unelevated` (source-verified at rust-v0.170.0, not runtime-tested): CX-029 is the weight-bearing sandbox entry; CX-021 (elevated) and CX-025 (mxc) apply only if the CODEX_HOME is configured that way (e.g. by the Desktop app).
> - Most entries are not re-verified on 0.170.0 (evidence mostly 0.144-0.169); see per-entry Version(s) Affected.
> - Skipped as non-material (watch only if MOSAIC starts using them): goals (#60308), `exec review` (#38545, #41984), `codex queue` (#41060), PreToolUse metadata (#44836), Desktop-only sandbox issues (#62185 and related).

## Active Issues (48)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| CX-001 | `codex exec` refuses/cancels MCP tool calls needing approval before dispatch | Limitation | Likely | HIGH | Yes | No | Unevaluated |
| CX-002 | `codex exec` silently declines MCP elicitations: exit 0, no `--json` event | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-003 | `codex exec` panics (exit 101) on EAGAIN when pipe reader is slow (Linux) | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-004 | `codex exec --json` omits some tool items (unmapped variants, MCP nested in code-mode exec) | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-005 | `--output-schema` applies to all assistant messages, not just the final one | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| CX-006 | `--output-schema` rejects UTF-8 BOM schema files (Windows) | Bug | Likely | LOW | Yes | No | Unevaluated |
| CX-007 | `-o` binds an empty file before the run; failed runs leave 0-byte file (0.168.0+) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CX-008 | `-o` write failure (missing parent dir) only logged; exit 0, final message lost | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-020 | Windows sandbox: `codex exec` shell commands fail (CreateProcessAsUserW 5 / runner pipe-in timeout) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-021 | Windows elevated sandbox: CreateProcessWithLogonW 1326/1909 lockout, worsened by parallel `codex exec` | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CX-022 | `codex exec` aborts at startup when `installation_id` in CODEX_HOME is not writable | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-024 | Windows `apply_patch` fails "path contains a reparse point" for regular files on non-C: drives | Bug | Likely | MEDIUM | No | No | Unevaluated |
| CX-025 | Windows MXC sandbox: rename/delete denied inside writable workspace | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-026 | `notify` hook silently stops once cumulative payload exceeds argv limit; Desktop re-adds notify hook | Bug | Confirmed | LOW | Yes | No | Unevaluated |
| CX-027 | `workspace-write` sandbox keeps `.git`/worktree gitdir read-only: git add/commit/fetch fail (by design) | Limitation | Confirmed | HIGH | Partial | No | Unevaluated |
| CX-028 | Windows `exec_command` rejected before launch: "CreateProcess Rejected ... blocked by policy" (also under full access) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-029 | Windows `unelevated` sandbox refuses split writable roots: exec_command / apply_patch update+delete fail | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-040 | Full ChatGPT-apps tool catalog sent every request; fresh session exceeds window, auto-compaction loops | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CX-041 | Remote compaction v2 loops: next request larger than pre-compaction, no no-progress guard | Bug | Likely | HIGH | No | No | Unevaluated |
| CX-042 | `codex exec` reports untrusted/modified hooks as bare "Failed"; symlink silently drops plugin hooks | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CX-043 | config.toml default model/reasoning effort ignored at startup unless `-m` passed (ChatGPT workspace auth) | Bug | Likely | LOW | Yes | No | Unevaluated |
| CX-060 | Concurrent first-run `codex exec` on fresh CODEX_HOME loses `state_5.threads` rows | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CX-061 | Completed subagents stay resident, block `spawn_agent` ("agent thread limit reached") | Bug | Likely | LOW | Partial | No | Unevaluated |
| CX-080 | `codex exec` stalls 5-17 min on silent response stream; 300 s idle timeout not configurable (built-in provider) | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-081 | Session start fails with `state_5.sqlite` "database is locked" when many processes share one CODEX_HOME | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-082 | `codex exec resume` silently starts a new thread / wrong session when name or `--last` does not match | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-100 | `apply_patch` silently applies a hunk whose final blank context line is not in the file | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| CX-102 | Windows hooks report bare "Failed": `bash` resolves to WSL shim, CRLF / unknown fields in hook stdout rejected | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-103 | `apply_patch` deletes file when `Move to` resolves to the source (incl. case-only rename) | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-104 | Short shell command returns empty output with exit 0 when bytes arrive after 50 ms post-exit cap (loaded host) | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| CX-105 | Large tool/command output middle-truncated by design; errors in the middle lost, no spill-to-file | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| CX-106 | `apply_patch` Delete File fails where Add/Update succeed (Linux; Windows MXC) | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-120 | `codex exec` MCP tools unreachable when deferred behind `tool_search` (>100 tools / `unsupported call: tool_searchtool_search_tool`) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-121 | Stdio MCP replies over 16 MiB silently discarded; call hangs until tool timeout (0.167.0+) | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-122 | `--output-schema` fails model-dependently: whitespace stalls or instant `max_output_tokens` with zero usage | Quirk | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-123 | `exec --json` error events message-only: retry notices indistinguishable from fatal errors, no typed auth/quota info | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-124 | `codex exec resume` rejects `-s` and does not inherit sandbox/network policy; resumed turns can run broader | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CX-125 | Explicit `$skill` of skills with `allow_implicit_invocation: false` reported "not available in this session" | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CX-126 | AGENTS.md chain silently truncated at `project_doc_max_bytes` (32 KiB), deepest file first, no visible warning | Limitation | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-140 | Windows `apply_patch` writes LF into CRLF files (mixed EOLs), drops trailing newline / BOM | Limitation | Confirmed | MEDIUM | Partial | No | Unevaluated |
| CX-141 | `apply_patch` fails "Failed to find expected lines" on first line of UTF-8-BOM files; retry loops | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-142 | Windows `apply_patch` fails for large patches (os error 206): patch body passed via argv | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CX-160 | Windows elevated sandbox: exec cannot run shell commands after Desktop provisions sandbox (`proxy_ports` marker mismatch); exit 0 | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-161 | Turn ends (`task_complete`) after a progress message promising a next action; no tool call follows | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CX-162 | `workspace-write`/`read-only` sandbox has network off by default (dead proxy `127.0.0.1:9`, Windows loopback 10013) | Limitation | Likely | MEDIUM | Yes | No | Unevaluated |
| CX-180 | Windows stdio MCP servers fail to initialize since 0.168.0-alpha.7 (IOCP pipe error 536) | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| CX-181 | Optional stdio MCP server ready before turn 1 still missing from `tool_search` on first `exec` turn | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CX-182 | Premature auto-compaction: client adds reasoning-token estimate on top of server usage (50-300k over) | Bug | Likely | MEDIUM | Partial | No | Unevaluated |

## Resolved / Retired Issues (0)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|-------------------|-----------------|

---

## Quick Summary

*Snapshot after run 2 (2026-11-21). Most evidence comes from 0.144-0.169 reports; few entries were re-tested on 0.170.0, and several (CX-004, CX-008, CX-080, CX-102, CX-124, CX-140..142, CX-161) need re-verification. IDs are sparse because discovery ran in themed slots. Scope is the Runner path (`codex exec`); native multi-agent items are watch-only.*

**MCP availability and human-in-the-loop in `exec`.** MCP calls needing approval are refused before dispatch (CX-001) and elicitations are silently declined with exit 0 (CX-002), which directly threatens the `contact-user` channel. On Windows, stdio MCP servers may fail to initialize since 0.168.0-alpha.7 (CX-180, single reporter: smoke-test `contact-user` on 0.170.0). MCP tools can also be unreachable behind `tool_search` (CX-120, CX-181), and oversized stdio replies are dropped (CX-121).

**Windows sandbox and file tools (HIGH).** The default `exec` backend on Windows is `unelevated`, so CX-029 (split writable roots refused) is the weight-bearing entry; CX-021, CX-025 and CX-160 apply only if the CODEX_HOME was set to `elevated`/`mxc` (e.g. by the Desktop app). Shell commands can fail outright (CX-020, CX-028, CX-160). `.git` is read-only in `workspace-write` by design (CX-027) and network is off by default (CX-162). `apply_patch` has many edge failures: reparse-point errors (CX-024), CRLF/BOM handling (CX-140, CX-141), large patches over argv limits (CX-142), and silent or destructive hunks (CX-100, CX-103, CX-106). Sandbox rename/delete denial breaks atomic writes (CX-025).

**Context and compaction.** The full apps tool catalog overflows a fresh session and compaction loops (CX-040, CX-041), and the client over-counts tokens so compaction fires early (CX-182). AGENTS.md is silently truncated at 32 KiB (CX-126), large tool output is middle-truncated (CX-105), and config defaults can be ignored (CX-043).

**Premature turn end.** A turn can complete right after a progress message that promises a next action, so a headless step ends without doing the work (CX-161).

**Result capture and event stream.** `--output-schema` applies to every assistant message (CX-005) and fails model-dependently (CX-122). `-o` leaves 0-byte files on failure (CX-007) or loses the message if the write fails (CX-008). `--json` omits some items (CX-004) and its error events are untyped (CX-123).

**Resume, stalls and concurrency.** `exec resume` silently starts a new thread (CX-082) and does not inherit the sandbox policy (CX-124). Silent response streams stall for minutes (CX-080), and many processes sharing one `CODEX_HOME` hit SQLite locks or lost thread rows (CX-081, CX-060, CX-022).

**Hooks and skills.** Hooks fail with a bare "Failed" (CX-042, CX-102) and explicit `$skill` calls can be rejected (CX-125).

## Recommended Mitigations

- Pre-approve `contact-user` and other required MCP tools in Runner config (`approval_mode="approve"` per tool or `default_tools_approval_mode`); do not rely on approval prompts or elicitation in `exec` (CX-001, CX-002).
- Re-pass `-c sandbox_mode=...` on every `exec resume`, and resume by explicit thread UUID, verifying `thread.started` (CX-124, CX-082).
- Treat exit code plus the JSONL stream as the success signal, never the `-o` file or its existence (CX-007, CX-008, CX-123); write schema files without a BOM (CX-006).
- Add a Runner-side no-event timeout for stalled streams (CX-080); disable unused apps with `features.apps=false` (CX-040); keep the MCP tool count under 100 (CX-120).
- Stagger or limit parallel `codex exec` launches and warm up `CODEX_HOME` first (CX-021, CX-060, CX-081).
- Keep the AGENTS.md chain small or raise `project_doc_max_bytes` (CX-126); do git operations outside the sandbox (CX-027).
- Windows: record which `[windows] sandbox` value the CODEX_HOME uses (default `unelevated`); keep Desktop-app provisioning out of the Runner's CODEX_HOME (CX-160, CX-021); install PowerShell 7 via MSI, not the Store package (CX-020).
- Smoke-test a `contact-user` call on Windows with the deployed Codex version (CX-180); set `required = true` on critical MCP servers (CX-181).
- Write files with LF-only or verify line endings after `apply_patch`; keep patches small (CX-140, CX-142). Verify the step actually did work, not just that it completed (CX-161).
- Raise `model_auto_compact_token_limit` and keep steps short to avoid early compaction (CX-182).
