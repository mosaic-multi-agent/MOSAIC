# Claude Code — Issue Index

> Last updated: 2026-10-25 (run 20)
> Latest platform version: v2.1.310 (released 2026-10-23)

## Active Issues (87)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| CC-001 | Background task notifications phantom-cancel pending calls with fake "user declined" sentinel | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-002 | Background/scheduled agents hang indefinitely on unanswered permission prompt, fail silently | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-003 | Subagent killed by usage limit falsely reported "completed" | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-006 | Compaction can preserve fabricated "system message" concealment instructions | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-007 | Mid-conversation behavioral rules dropped after compaction | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-008 | One malformed MCP tool schema drops ALL tools from that server | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-009 | Custom subagent `tools: Bash` frontmatter silently never grants Bash | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-010 | `PreToolUse` `defer` decision permanently strands subagent Bash call | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-011 | `disallowedTools` not enforced on default-`subagent_type` nested dispatch | Limitation | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-012 | Stale `ToolSearch` reference permanently bricks session (400 error) | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-013 | `permissions.allow`/`bypassPermissions` intermittently ignored for subagents | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-014 | `PreToolUse` `ask` decision silently not enforced | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-015 | Killed/crashed `PreToolUse` hook fails OPEN instead of closed | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| CC-016 | Background subagents denied Write/Bash regardless of permission mode | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-018 | Self-backgrounding Bash under `run_in_background` false-completes, orphans process | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| CC-019 | Background Bash task killed ~17-20s after arming as turn's last call | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-020 | Interactive-session LSP client never sends `didChange` after Edit-tool write | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| CC-022 | Mid-session `/model` switch doesn't apply to running session | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-023 | Subagent Bash `cwd` reset can land in sibling subagent's worktree | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-024 | `SessionStart` env file grows unboundedly, eventually wedges Bash tool | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-025 | Plugin-native `PreToolUse` hooks unenforced; plugin `deny` unenforced | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| CC-026 | Second `/compact` in one process corrupts transcript, breaks `/rewind` | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-027 | Leading whitespace before slash command sends it as plain text | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-028 | `--continue` is directory-scoped, can co-write into another live session | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-029 | Manual `/compact` can silently no-op on large conversations | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-030 | Skill/command positional arg substitution off-by-one; `$2`+ never works | Bug/Limitation | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-031 | Explicitly-named `subagent_type` dispatch can ignore subagent's own definition | Bug | Unverified | HIGH | No | No | Unevaluated |
| CC-032 | Subagent plan-mode approval dialog can show PARENT session's plan | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-033 | Subagents receive full auto-memory index and skill listing (no opt-out) | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CC-034 | `CLAUDE.md`/path-scoped rules in `--add-dir` dirs silently skipped | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CC-035 | Project skills beyond undocumented per-session cap become non-invocable | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-036 | `CLAUDE.md` above git repo root never loaded into repo's own worktrees | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| CC-037 | Hard-coded plan-mode notice text injected at prompt layer, invisible in transcript | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| CC-040 | Multi-iteration turns sum token usage across iterations, inflate 2x-6x | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-041 | `.claude/settings.json` in ancestor directory silently never loaded | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-042 | `/rewind` can fail to restore never-git-tracked newly-created file | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| CC-043 | statusLine refresh has no debouncing — slow command spawns unbounded copies | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-044 | Duplicated base64 image content + unclamped render cost hard-freezes at 100% CPU | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-047 | One malformed hook entry silently kills all hooks for that event type | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-048 | `WebFetch` `prompt` param ignored for pre-approved docs domains under ~100KB | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-050 | `SubagentStop` hook never fires when subagent killed via `TaskStop`/session exit | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CC-051 | Idle background shells reaped under macOS memory pressure, mislabeled "stopped by user" | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-052 | Background Bash task killed with no TaskStop, no pressure, no crash (Windows/Linux) | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-054 | `EnterWorktree` isolation state is a session-wide latch, not scoped per-agent | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-055 | `isolation:"worktree"` subagents: Edit writes into parent's tree, git-guard bypassable, auto-reap orphans cwd | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-056 | `Write`/`Edit` stay pinned to launch-time worktree after subagent's own `EnterWorktree` switch | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-057 | Session cwd resolution can become permanently unresolvable (EPERM) after worktree use | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-059 | Project-level `.claude/rules/` symlinks to outside-project targets: silent drop fixed in 2.1.284, but `paths:`-scoped symlinked rules still never inject | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CC-060 | Auto Mode's Bash-first steering defeats nested CLAUDE.md/rules loading, `/rewind`, hooks, and Edit/Write-scoped permissions | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-063 | `EnterWorktree`/project switches don't propagate to hook subprocess `CLAUDE_PROJECT_DIR` | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-067 | Nested/scoped skills discovered-but-not-invoked become permanently uninvocable after compaction | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-068 | Windows statusline `pwsh.exe` host processes never exit, leak hundreds of orphans/hour | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CC-070 | Plugin hooks' `commandWindows` field silently ignored on Windows, forces Git Bash | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CC-071 | Windows hook commands with unquoted backslash paths silently never execute | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-072 | `UserPromptSubmit` skips firing for mid-turn/interrupt messages (stale, unresolved) | Bug | Likely | MEDIUM | No | No | Unevaluated |
| CC-073 | Bash cwd resets outside project dir/added dirs; hooks then receive stale, not actual, cwd | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-074 | `/rewind` checkpoint race can drop restore option for tracked-file edits/deletions | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| CC-075 | `EnterWorktree` unconditionally refuses inside any pinned-cwd subagent, no fallback | Limitation | Unverified | HIGH | Yes | No | Unevaluated |
| CC-079 | `mcpServers:` does not bypass `tools:` allowlist — server connects (instructions delivered) but tools not granted unless `mcp__<server>__*` is in `tools:`. Reframed from "silently ignored" after Round 5 showed the server IS connected; the allowlist behavior is consistent/by-design. Original framing from v2.1.269; re-tested v2.1.270. Workaround: use `mcp__<server>__*` in `tools:` (the correct pattern). | Limitation | Confirmed | LOW | Yes | Yes | Awareness |
| CC-080 | MCP server instructions routed by session, not by agent tool scope — primary/`--agent` gets every connected server's block at start; subagents also get every server's block (late, see CC-081), even with zero MCP tools. Partial workaround: define subagent-only servers inline in `mcpServers:` so they never connect in the main session (#85307 open, 4 more stale-closed) | Bug | Confirmed | LOW | Partial | Yes | Awareness |
| CC-081 | Subagents receive MCP server instructions late — as a mid-conversation reminder after their first tool turn, not at start. MCP tools themselves work from turn one (foreground and background); an MCP call in the first turn runs without guidance, and the late block may be distrusted as injection. Re-tested on v2.1.270 — the earlier "no instructions, MCP unusable" finding was a probe artifact (#85307, #29655) | Bug | Confirmed | LOW | Yes | Yes | Awareness |
| CC-082 | Async hooks are killed when a `-p` process exits (async `Stop` cut off mid-execution, async `SessionEnd` writes nothing); sync hooks complete and delay exit. Transcript is still being written at `Stop`, final at `SessionEnd`; under `--no-session-persistence` the file never exists. | Limitation | Confirmed | HIGH | Yes | Yes | Mitigated |
| CC-083 | Changed MCP tool description/schema never reaches a session that already loaded the tool via ToolSearch (frozen `deferred_tools_record`; survives reconnect and `--resume`) | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-084 | MCP tool call with intended `""` string argument emitted as invalid JSON (`"key": ,`), rejected client-side; deterministic, causes retry loops | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CC-085 | Concurrent subagents with byte-identical inline `mcpServers:` configs share ONE MCP server process/session (stale-closed; needs re-verification on current version) | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-086 | Background subagents (default) silently lose built-ins outside a fixed allowlist even if listed in `tools:` (MCP resource tools, LSP, ReportFindings, AskUserQuestion) | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-087 | Subagent's own `run_in_background` Bash/`Monitor` completion never re-invokes idle (named in-process teammate) subagent; stalls until lead messages it or burns tokens polling | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-088 | `SubagentHandback` from a teammate-spawned subagent fails ("spawner no longer running") in Agent Teams + auto mode; report lost/misrouted to lead, enforce loop burns tokens | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-089 | `SubagentHandback` accepted (or forced by `[handback-send-enforce]`) while subagent's own background children still run; parent sees unfinished subagent as done, second hand-back refused | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-090 | Streamable-HTTP MCP `tools/call` hangs forever when server dies mid-call (client reconnects GET stream but never fails pending POST); bounded only by `CLAUDE_CODE_MCP_TOOL_IDLE_TIMEOUT` | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-093 | Windows/Git Bash: Bash tool command truncated at ~8,186 chars (silent partial run, exit 0, possible) and backslash runs halved | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-094 | `EnterWorktree(path=...)` to a non-nested worktree moves Bash cwd but Write/Edit guard keeps old binding, mislabels target "base repo checkout" | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| CC-095 | Bash `timeout` is a hard SIGTERM kill (exit 143, no partial output) for commands failing an undocumented auto-background check; `-p` mode reportedly never auto-backgrounds | Limitation | Likely | MEDIUM | Yes | No | Unevaluated |
| CC-096 | 32 MB request-body cap: token-based auto-compact never fires for byte-heavy history; recommended `/compact` re-sends the same request, session bricked | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-097 | Client-side unicode "repair" pass decodes literal `\uXXXX` in ALL tool inputs (Write/Edit/Bash/PowerShell/MCP) before dispatch, silently corrupting file content | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-098 | Write/Edit silently strip trailing whitespace from `new_string`/`content` in non-Markdown files (tokens glued; whitespace-only replacement becomes deletion) | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-099 | Auto mode fails closed on Edit/Write when server classifier returns "skipped" (no verdict), regression from 2.1.299 (`-p --permission-mode auto` repro) | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CC-100 | Windows: Bash tool does not kill descendant process tree on timeout/cancel/TaskStop/exit; orphaned find/grep/bash accumulate, can lock the machine and block hooks | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| CC-103 | Windows: session freezes after a hook has exited - hook verdict lost, tool never dispatched, no timeout (incl. background subagents) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| CC-104 | Idle-session sweep SIGTERMs all harness-tracked background Bash tasks (~30 min after last human message), reported as bare "was stopped" | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-105 | In `bypassPermissions`, `PreToolUse` hook `ask` is overridden when the same call writes into `.git/` or `.claude/` (`-p` repro) | Bug | Unverified | HIGH | Yes | No | Unevaluated |
| CC-106 | Under `bypassPermissions`, built-in "dangerous rm" prompts still block unattended background subagents (hours; also fires with no `rm`) | Limitation | Likely | HIGH | Yes | No | Unevaluated |
| CC-107 | Worktree-isolated subagents: Bash isolation guard refuses harmless worktree-local commands as "too complex to verify" | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| CC-108 | Subagent's own `run_in_background` Bash completion notification never wakes the idle subagent; agent and coordinator hang silently | Bug | Likely | HIGH | Yes | No | Unevaluated |
| CC-109 | Windows: `TaskStop` kills only the shell wrapper, not the process tree; children keep running while "Successfully stopped" is returned | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| CC-110 | `Monitor` tool: `persistent` removed in 2.1.268, every watch capped at 30 min (schema still allows 1h); re-arm loses events | Limitation | Confirmed | MEDIUM | Yes | No | Unevaluated |
| CC-111 | Hook payload identity gaps: compaction hooks lack `agent_id` for subagents; `SubagentStop` fires for internal summarizer calls with phantom transcript paths | Bug | Confirmed | MEDIUM | Yes | No | Unevaluated |

## Resolved / Retired Issues (6)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|--------------------|-----------------|
| CC-078 | `CLAUDE.md`/path-scoped rules never load when Claude creates a brand-new file matching the pattern (loading gated on a prior `Read`) | v2.1.288 (2026-10-02) | 2026-10-25 |
| CC-069 | `UserPromptSubmit` hook, correctly registered and verified working, never fires for interactively-typed prompts in the CLI | N/A (not relevant - probable user error) | 2026-10-25 |
| CC-005 | `PostToolUse` hook doesn't fire for `Agent` tool completions at the parent session level | N/A (not relevant — unsourceable) | 2026-09-10 |
| CC-046 | Mid-session-created `.claude/settings.json` hooks never arm until restart | Fixed by v2.1.301 (reporter-verified) | 2026-10-15 |
| CC-038 | Every Bash dispatch in large/resumed session can freeze process ~80-90s (glob Read rule + sandbox walk) | Fixed v2.1.290 (changelog; reporter-indicated) | 2026-10-25 |
| CC-039 | `.claude/settings.json` non-atomic write — concurrent sessions can tear/clobber (VS Code extension path) | Fixed v2.1.274 (maintainer) | 2026-10-25 |

---

## Quick Summary

*Run 20 (2026-10-25) re-verified all entries against v2.1.310 and added 25 new ones (CC-083..090, 093..100, 103..111). Upstream context: the stale bot auto-closes inactive issues as `not_planned` even when they carry the `reproduced` label (#87647), so a closed issue is not a fix. Entries closed that way stay active with a "Needs re-verification" note unless a fix is verified in a release. MCP-in-frontmatter test history is in `mcp-frontmatter-blocker.md`.*

**Background tasks, process lifecycle and completion signalling** — The largest and most operationally dangerous cluster for unattended fan-out. Background Bash tasks are killed or reaped without a clean reason: an idle-session sweep SIGTERMs all tracked background tasks ~30 min after the last human message (CC-104), other unexplained kills (CC-052, CC-019, CC-018), macOS memory-pressure reaping (CC-051, narrowed in 2.1.274), and a 30-minute cap on `Monitor` watches (CC-110). On Windows, process trees are not killed on timeout, cancel, `TaskStop` or exit (CC-100, CC-109), so orphans accumulate and `TaskStop` can report success while children keep writing. Completion signals fail the other way: a subagent's own background Bash/`Monitor` completion never wakes an idle subagent or teammate (CC-087, CC-108, probably the same defect), `SubagentHandback` can be accepted while children still run (CC-089) or fail inside Agent Teams (CC-088), `SubagentStop` does not fire on `TaskStop`/session exit (CC-050), and a usage-limit kill is reported as "completed" (CC-003). Background notifications can phantom-cancel pending calls (CC-001).

**Permission and hook enforcement gaps** — Configured enforcement is silently not honored in several places: `permissions.allow`/`bypassPermissions` for subagents (CC-013), `PreToolUse` `ask` (CC-014, and overridden for `.git/`/`.claude/` writes under bypass, CC-105), `defer` stranding calls (CC-010), killed hooks failing open (CC-015; v2.1.295 added opt-in `onFailure: "block"`), plugin hooks (CC-025), one malformed entry killing all hooks of an event type (CC-047), and Auto mode failing closed on Edit/Write (CC-099). Unattended runs hang on prompts: background/scheduled agents on an unanswered permission prompt (CC-002), the built-in "dangerous rm" confirmation under bypass (CC-106, Limitation), background subagents denied Write/Bash (CC-016). Windows adds hook-specific failures: frozen sessions after a hook has already exited (CC-103), unquoted backslash paths (CC-071), ignored `commandWindows` (CC-070), `UserPromptSubmit` skipped mid-turn (CC-072). Hook payloads lack identity: no `agent_id` on compaction hooks and phantom `SubagentStop` events from internal summarizer calls (CC-111), and async hooks are killed when a `-p` process exits (CC-082, MOSAIC-verified).

**Subagent dispatch, scoping and isolation** — `disallowedTools` does not cascade (CC-011, by design), a named `subagent_type` may ignore its own definition (CC-031), subagents get the full memory/skill listing (CC-033) and MCP instruction blocks late (CC-080, CC-081, MOSAIC-verified), a plan-mode dialog can show the parent's plan (CC-032), background subagents silently lose built-ins outside a fixed allowlist even when listed in `tools:` (CC-086, Limitation), and `mcpServers:` does not bypass the `tools:` allowlist (CC-079, Limitation; use `mcp__<server>__*` in `tools:`). Concurrent subagents with identical inline `mcpServers:` configs may share one server process (CC-085, needs re-verification).

**Worktree isolation** — Threatens MOSAIC's multi-worktree fan-out from several angles: the `EnterWorktree` state is a session-wide latch (CC-054, partly fixed in 2.1.290), `Write`/`Edit` stay pinned to launch-time worktrees (CC-055, CC-056, CC-094), sibling-worktree cwd aliasing that reaches the `Read` tool (CC-023), unresolvable cwd after worktree use (CC-057), worktree switches not reaching hook `CLAUDE_PROJECT_DIR` (CC-063) and stale hook cwd (CC-073), `EnterWorktree` refused inside pinned subagents (CC-075), and the isolation guard refusing harmless local commands (CC-107).

**Context compaction and conversation stability** — Compaction can preserve fabricated "system message" instructions (CC-006), drops mid-conversation rules (CC-007), and the second `/compact` replays prior history with quadratic growth (CC-026, now Confirmed; the `/rewind` chain break is fixed on the main thread in 2.1.271). Manual `/compact` can silently no-op (CC-029). A 32 MB request-body cap can brick a session because token-based auto-compact never fires for byte-heavy history (CC-096). Multi-iteration token inflation is only partly fixed (CC-040), `--continue` can attach to another live session in the terminal (CC-028, fixed in the VS Code extension only), and discovered-but-uninvoked nested skills become uninvocable after compaction (CC-067, CC-035).

**MCP and ToolSearch** — One malformed schema drops all of a server's tools (CC-008), a stale `ToolSearch` reference permanently bricks a session with a 400 (CC-012, including names over 64 chars), changed tool descriptions never reach a session that already loaded the tool (CC-083), an intended empty-string argument is emitted as invalid JSON (CC-084), and a Streamable-HTTP call hangs forever if the server dies mid-call (CC-090).

**Tool-input fidelity and Windows Bash** — Silent content corruption: a client-side pass decodes literal `\uXXXX` in all tool inputs (CC-097), `Write`/`Edit` strip trailing whitespace outside Markdown (CC-098), and Windows Git Bash truncates Bash commands at ~8,186 characters with possible silent partial runs (CC-093). Bash `timeout` is a hard SIGTERM for commands failing an undocumented auto-background check (CC-095), and `WebFetch` ignores `prompt` for small pre-approved docs (CC-048, by design). Duplicated base64 images can hard-freeze the CLI (CC-044), and a `SessionStart` env file can grow until Bash wedges (CC-024).

**Rules, settings and `/rewind` loading gaps** — Nested `CLAUDE.md`/rules loading is gated on a prior `Read`; fixed for Write/Edit (2.1.288, CC-078) and for plain single-file Bash reads (2.1.293), but pipes, `&&` chains and Bash writes still load nothing, which Auto Mode's Bash-first steering makes common (CC-060). Other gaps: `--add-dir` rules (CC-034), `CLAUDE.md` above the repo root (CC-036), ancestor settings (CC-041), `paths:`-scoped symlinked rules (CC-059), and `/rewind` that misses never-tracked files or Bash edits (CC-042, CC-074).

**By-design limitations to adapt to permanently** — CC-011, CC-048, CC-051, CC-079, CC-082, CC-086, CC-095, CC-106, CC-110, and CC-075 (protective design with no documented fallback).

## Recommended Mitigations

- Never trust a subagent's "completed" status alone: independently verify output, and for background work verify completion yourself, because kills and missed wake-ups are reported as bare "stopped" or as success (CC-003, CC-001, CC-087, CC-108, CC-104, CC-109).
- For long-running background commands, use a detached session (tmux/`setsid`) plus a status file and a same-turn blocking wait, and add an external per-dispatch watchdog; the harness may SIGTERM idle background tasks (CC-104, CC-052, CC-110).
- Pre-approve every tool a background/scheduled subagent needs and set `permissions.defaultMode` at **user-level** settings for scheduled launchers; a hook that returns **deny** returns immediately in an unattended run, while `ask` hangs (CC-002, CC-016, CC-106).
- Do not rely on hooks as a security boundary: use `onFailure: "block"` (v2.1.295+, untested) so a crashed hook fails closed, avoid mixing `ask` hooks with bypass mode, and do not rely on `disallowedTools` for nested dispatch (CC-015, CC-105, CC-014, CC-011, CC-031).
- On Windows: quote hook paths and do not use `commandWindows`; keep Bash command strings under ~8,000 characters and write long scripts with the Write tool; do not assume `TaskStop` or a Bash timeout killed child processes (CC-070, CC-071, CC-093, CC-100, CC-109).
- **MCP:** list MCP tools in `tools:` with server wildcards (`mcp__github__*`); `mcpServers:` connects a server but does not grant tools (CC-079). Inline `mcpServers:` isolation may share one process across concurrent subagents (CC-085). `/compact` or `/clear` refreshes a frozen ToolSearch tool description; resume does not (CC-083).
- Avoid mid-session model switches when deferred tools are loaded, and keep MCP tool names short (CC-012, CC-022).
- Put durable rules in `CLAUDE.md`, but remember nested rules load only after a plain `Read` of a file in scope; with Auto Mode, add one plain Read per folder from a root brief (CC-007, CC-060, CC-078).
- Treat `isolation:"worktree"` as leaky: have each subagent re-verify its worktree at the start of every Bash call, do not assume `Write`/`Edit` follow a later `EnterWorktree`, and do not nest a worktree switch inside a pinned subagent (CC-023, CC-054, CC-055, CC-056, CC-063, CC-073, CC-075).
- Avoid concurrent sessions with `--continue` in one directory, and be skeptical of context figures during multi-iteration turns; a 32 MB request body can brick a session (CC-028, CC-040, CC-096, CC-026).
- Prefer `Edit`/`Write` over Bash for file changes (needed for `/rewind` and Edit/Write-scoped permissions), and check written files for stripped trailing whitespace and decoded `\u` sequences where fidelity matters (CC-074, CC-097, CC-098).
- Keep statusline commands fast and non-PowerShell on Windows; escape literal `$<digit>` text in skill prose with a backslash (CC-043, CC-068, CC-030).

**Needs re-verification on the current version:** many entries were closed upstream by the stale bot or the author with no fix and no maintainer engagement (for example CC-010, 012, 014, 016, 019, 020, 030, 032, 034, 041, 043, 044, 047, 052, 067, 068, 071, 073), and CC-018, 025, 042, 050, 055, 056, 072, 085 have been quiet for 6+ weeks. CC-103, 108, 109 have evidence only from the VS Code extension/desktop app or older versions and need a Windows `-p` check against v2.1.310. A new report (#104745) of every Bash command failing with `unexpected EOF` on 2.1.310 may share CC-093's cause and is undiagnosed. For CC-109, the changelog for 2.1.222 to 2.1.295 was not checked for `TaskStop`/process-tree fixes.

**Leads for a future pass:** SubagentHandback and background-completion cluster (#102867, #96849, #87874, #85422, #103309, #89555); `SubagentStop` not firing for async Agent-tool subagents (#82249); MCP (#101280, #90362, #104714, #88882, #91855, #104877, #30280); auto-compact trigger cluster (#104304, #72857, #103708, #101430); permission leads (#104576, #95741, #101433, #101849); a claim that agents declaring `Glob, Grep` alongside `Bash` get neither (#79728, a possible MOSAIC-testable `tools:` quirk related to CC-009); #104309 (tool search defers core built-ins behind a custom `ANTHROPIC_BASE_URL` gateway; flagged for a stock-infrastructure re-check); `PreToolUse` `updatedInput` replacing the whole tool input (#90725); and the Auto Mode steering write-up (#90449).
