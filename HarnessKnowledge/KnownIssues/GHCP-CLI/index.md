# GitHub Copilot CLI — Issue Index

> Last updated: 2026-10-25 (run 20)
> Latest platform version: v1.0.103 (released 2026-10-22)

## Active Issues (32)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| GC-001 | Memory-pressure watchdog force-compacts conversation at low context usage, can loop until OOM | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| GC-002 | Repeated compaction recursively re-summarizes prior summaries, degrading durable context | Limitation | Likely | MEDIUM | Yes | No | Unevaluated |
| GC-004 | Subagent launched with a since-deprecated/unsupported model crashes the entire primary session | Bug | Unverified | HIGH | No | No | Unevaluated |
| GC-005 | Sub-agents with full tool access silently return empty once MCP tool-schema volume crosses an undocumented budget | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| GC-007 | OOM crash on long `--resume` sessions; crash dumps written into user's cwd | Bug | Likely | HIGH | Partial | No | Unevaluated |
| GC-008 | Session hangs after work appears complete; Escape recovery enters permanent "Cancelling" | Bug | Likely | HIGH | Partial | No | Unevaluated |
| GC-009 | Non-interactive (`-p`) sessions: tool-call approval silently and permanently revoked mid-session | Bug | Unverified | HIGH | No | No | Unevaluated |
| GC-010 | No approval required for any tool execution inside Docker sandboxes, even with `/yolo off` | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| GC-012 | `/allow-all` slash command does not suppress bash/shell tool execution prompts | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| GC-013 | Agent Skill `allowed-tools` frontmatter ignored in non-interactive (`-p`) mode | Bug | Unverified | HIGH | No | No | Unevaluated |
| GC-014 | `allowed_directories` in permissions-config.json never loaded | Bug | Unverified | HIGH | No | No | Unevaluated |
| GC-015 | `commandIdentifiers` with spaces still require approval | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| GC-017 | `preToolUse` hook `ask` decision auto-approved by TUI since v1.0.53 | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| GC-018 | `--allow-tool='shell(docker ps)'` pattern matching fails for non-git commands | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| GC-019 | Non-interactive `--yolo` bypasses `disableBypassPermissionsMode` managed setting | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| GC-020 | Repo-level hooks (`.github/hooks/*.json`) silently not loaded in `-p` mode unless the working directory equals or is under a `trustedFolders` entry of `config.json` (exit 0, no warning) | Limitation | Confirmed | HIGH | Yes | Yes | Mitigated |
| GC-021 | No non-argv prompt input; `-p <text>` is mangled by cmd.exe through the npm `copilot.cmd` shim on Windows | Limitation | Confirmed | HIGH | Yes | Yes | Mitigated |
| GC-023 | Never-completing compaction OOMs the process and leaves the session permanently unresumable | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| GC-024 | Interactive sessions hit V8 heap OOM (~4.3 GB cap) without `--resume`; heap raise only delays; reports through 1.0.93 | Bug | Likely | HIGH | Partial | No | Unevaluated |
| GC-025 | Session host event-delivery ack timeouts (`permission.requested` 5 s / others 120 s) permanently break `session.send` until exit and `--resume` | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-026 | Failed compaction silently retried on nearly every turn, no backoff or visible error; context grows, billed retries | Bug | Likely | HIGH | Partial | No | Unevaluated |
| GC-027 | Tool output above ~20 KB replaced by a preview plus OS temp-file path; no stable artifact id or paging | Limitation | Likely | MEDIUM | Yes | No | Unevaluated |
| GC-028 | Large MCP fleets connect partially at startup with no retry; `tool_search_tool` returns clean "No tools found" while servers still registering | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-033 | Subagents started in the same turn as an approved `exit_plan_mode` get no built-in tools (regression after 1.0.95) | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-034 | Custom agent selection disables deferred MCP tool loading from 2nd model call; all MCP schemas flood context, invalidate prompt cache | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| GC-035 | Child custom agent's MCP tool discovery appears scoped to the parent agent's tool list | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-036 | Non-interactive MCP tool call hangs after progress notification until ~5 min idle timeout; no error reaches model | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-037 | `--agent <name>` silently dropped after startup (`subagent.deselected`); untrusted workspace falls back to default agent | Bug | Likely | HIGH | Yes | No | Unevaluated |
| GC-038 | Autopilot stops after fixed 600 s wait for background subagents still running; cap cannot be disabled (1.0.102 regression) | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| GC-039 | Background sub-agent model-stream failure exhausts 5 retries in ~5 s; parent sees ordinary completion/idle instead of failure | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-040 | Headless mode exits 0 mid-task when model response has `finish_reason: "tool_calls"` but no tool calls | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| GC-041 | `view` refuses files with full path of 257+ chars in non-interactive runs, misleading "Permission denied" error (Windows) | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |

## Resolved / Retired Issues (5)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|-------------------|-----------------|
| GC-022 | `--agent <id>` dropped at session start (`subagent.deselected`); the default agent runs instead (1.0.91 regression) | 1.0.92 | 2026-10-06 |
| GC-003 | Workspace `.mcp.json` detected by `mcp list`/`mcp get` but not wired into actual agent session | 1.0.85/1.0.89 (requires persisted workspace trust) | 2026-09-30 |
| GC-006 | Create-file tool returns invalid/null response and retries forever when content is too large | 1.0.85 (bot closure) | 2026-09-16 |
| GC-011 | Steering message in `preToolUse` "ask" denial prompt is silently dropped | 1.0.86 (bot closure) | 2026-09-19 |
| GC-016 | `preToolUse` hook `deny` decision does not block tool execution | 1.0.86 (bot closure) | 2026-09-20 |

---

## Quick Summary

*Run 20 (2026-10-25) re-verified all entries against v1.0.103 and added 15 new ones (GC-023..028, GC-033..041). Copilot CLI's bot closes many issues as "resolved in vX" without a linked PR or reporter confirmation; that is weak evidence. Four such closures were accepted as resolved (GC-003, GC-006, GC-011, GC-016) and are worth a spot check on the current release.*

**Memory, OOM and compaction** — The most serious cluster for long-running and headless sessions. A process-level memory watchdog can force-compact at low context usage and loop (GC-001; release 1.0.97 added an "emergency memory relief" path, so this is still deliberate). Interactive sessions hit a V8 heap OOM at ~4.3 GB even without `--resume` (GC-024, reports through 1.0.93), `--resume` sessions OOM and dump multi-GB core files into the cwd (GC-007; one commenter found headless ACP stayed flat across 1,668 turns, which is encouraging for the Runner but a single source), and a never-completing compaction can leave a session permanently unresumable (GC-023). Failed compactions are silently retried on nearly every turn with no backoff (GC-026), and repeated compaction re-summarizes prior summaries and loses the execution cursor (GC-002, Limitation). No OOM reports after 1.0.93 were found, so GC-001 and GC-007 need re-verification on 1.0.97+.

**Subagents, custom agents and MCP delegation** — Subagents can fail without a signal: a deprecated model crashes the primary session (GC-004), heavy MCP schema volume makes subagents return empty (GC-005), a background subagent's model-stream failure is reported as an ordinary completion (GC-039), and subagents started in the same turn as an approved `exit_plan_mode` get no built-in tools (GC-033, regression after 1.0.95). Custom-agent selection is fragile: `--agent` is silently dropped in an untrusted workspace (GC-037, same signature as the fixed GC-022), disables deferred MCP tool loading so all schemas flood the context (GC-034), and a child agent's MCP discovery appears scoped to the parent's tools (GC-035). Autopilot stops after a fixed 600 s wait for background subagents (GC-038). MCP startup and calls are unreliable: large fleets connect only partially with no retry (GC-028), and a non-interactive MCP call can hang until a ~5 min idle timeout with no error (GC-036).

**Non-interactive (`-p`) permission and trust behavior** — The most concentrated risk for the headless Runner. Approval can be silently revoked mid-session (GC-009), `/allow-all` does not suppress shell prompts and a managed-policy variant ignores allow-all entirely (GC-012), Skill `allowed-tools` is ignored in `-p` (GC-013), `--yolo` bypasses `disableBypassPermissionsMode` (GC-019), and repo hooks are not loaded unless the working directory is under a `trustedFolders` entry (GC-020, MOSAIC-verified; `--trust-folder` added in 1.0.100 is a possible new route, unprobed). Trust also drives which agent runs (GC-037). Permission config matching has its own failures: `allowed_directories` never loads (GC-014), `commandIdentifiers` containing a space are dropped (GC-015, independently reproduced on 1.0.98), and `--allow-tool='shell(...)'` matching fails for non-git commands (GC-018). `preToolUse` `ask` decisions are auto-approved by the TUI (GC-017; `deny` enforcement GC-016 was closed as fixed in 1.0.86 without a PR). The pattern: enforcement differs between interactive and non-interactive paths, sometimes making automation fail and sometimes making it more permissive than intended.

**Headless reliability and tool output** — Sessions can hang after work appears complete and Escape makes it worse (GC-008), headless runs can exit 0 mid-task on a `tool_calls` finish reason with no tool calls (GC-040), the session host's event-ack timeouts permanently break `session.send` (GC-025), `view` refuses 257+ character paths with a misleading "Permission denied" on Windows (GC-041), tool output above ~20 KB is replaced by a preview and a temp-file path with no paging (GC-027, Limitation), and `-p <text>` is mangled by cmd.exe through the npm shim on Windows (GC-021, MOSAIC-verified; pass the prompt via stdin).

**Sandbox permission bypass (GC-010):** narrow (opt-in Windows Docker Sandboxes), but HIGH impact if MOSAIC adopts container-based subagent isolation.

## Recommended Mitigations

- **Do not treat `preToolUse` hooks as a hard security boundary.** `ask` verdicts are auto-approved by the TUI (GC-017) and `deny` was only recently fixed by a bot-closed report with no PR (GC-016). For a hard block, enforce outside the CLI.
- **Test `-p` permission behavior in the Runner's real environment.** Skill `allowed-tools`, `/allow-all`, `--yolo`, managed settings and `commandIdentifiers` diverge from documentation in headless mode (GC-009, GC-012, GC-013, GC-015, GC-019). A `permissionRequest` hook returning `{"behavior":"allow"}` for an allow-list is a community workaround for the matching failures (GC-015).
- **Pre-trust the working directory or keep passing `--additional-mcp-config @.mcp.json`.** Workspace `.mcp.json` now loads (fixed 1.0.85/1.0.89) only when the workspace has persisted trust; repo hooks need a `trustedFolders` entry (GC-003, GC-020).
- **Bound session lifetimes and add an external watchdog.** Restart long or resumed sessions periodically, set `ulimit -c 0` to avoid multi-GB core files, and wrap `-p` invocations in an external wall-clock timeout with kill/retry (GC-001, GC-007, GC-023, GC-024, GC-008, GC-009).
- **Externalize durable facts to on-disk artifacts** (the blackboard pattern) rather than relying on context surviving compaction (GC-002, GC-026).
- **Treat "no subagent response" as ambiguous**, not as success or a clean no-op: check each subagent's output, since failures can look like normal completion (GC-005, GC-039).
- **Pass long prompts via stdin on Windows** instead of `-p <text>` through the npm shim, and have subagents split very large file writes (GC-021, GC-006).
- **Prefer whole-command permission identifiers** over narrow subcommand patterns for pre-approved tools (GC-014, GC-015, GC-018).
- **When using `--agent`, run in a trusted folder and verify the agent actually stayed selected**; avoid starting subagents in the same turn as an approved plan exit (GC-037, GC-033).

**Needs re-verification on the current version:** GC-004, 005, 008, 010 (10–12 weeks idle), GC-013, 014, 018, 019 (idle with no maintainer response), GC-001 and GC-007 (against the 1.0.97 memory relief), GC-039 (reported on 1.0.86-2), and GC-036 (~5 weeks idle).

**Leads for a future pass:** a fresh `-p` test of two bot-closed reports (#3335, a subagent system-prompt line "Do NOT write output to files", and #3013, hooks not firing for background agents, fixed per a maintainer for preToolUse/postToolUse but not sessionStart); the orphan `tool_use`/`tool_result` session-corruption family (#2543, #2323, #3366, #3183, #762, all bot-closed as fixed in 1.0.86+); #5143 (follow-up to an idle background agent delayed by minutes or hours); #4829 (long subagent tool sequences break prompt caching); #4714 (very slow `--resume` on big sessions); and `--remote`/`--no-remote` retirement in 1.0.103 (scripts passing those flags may break).
