# Cross-Harness Known Issues Comparison

| Field | Value |
|-------|-------|
| **Harnesses Compared** | Claude Code, GHCP-CLI, OpenCode 1.x, VS Code GHCP |
| **Created** | 2026-09-10 |
| **Last updated** | 2026-10-25 (run 20) |
| **Source** | Individual `index.md` and `active-issues.md` files in each harness folder (OpenCode is the `OpenCodeV1/` folder) |
| **Purpose** | Side-by-side comparison of known issue landscapes to inform MOSAIC's harness-specific mitigation strategy, identify cross-cutting risks, and prioritize which harness behaviors require permanent adaptation vs. waiting for upstream fixes |

**Scope note:** OpenCode here means the **1.x line** only. OpenCode 2.0 is treated as a separate, new harness and is not covered by this comparison yet.

**Reading note:** An upstream issue that is closed is not necessarily fixed. Maintainers mass-closed OpenCode 1.x issues as `not_planned` in October 2026, the Claude Code stale bot auto-closes inactive issues even when they carry the `reproduced` label, and the Copilot CLI bot often closes issues as "resolved in vX" with no linked PR. Entries in that state stay active here, flagged "Needs re-verification", unless a fix is verified in a release.

---

## 1. Issue Landscape Overview

| Dimension | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-----------|:-----------:|:--------:|:------------:|:------------:|
| **Active Issues** | 87 | 32 | 46 | 22 |
| **Resolved Issues** | 6 | 5 | 4 | 1 |
| **Latest Version** | v2.1.310 | v1.0.103 | v1.19.4 | 1.143.1 |
| **HIGH Impact Count** | 59 | 16 | 38 | 13 |
| **MEDIUM Impact Count** | 25 | 16 | 8 | 8 |
| **LOW Impact Count** | 3 | 0 | 0 | 1 |
| **Confirmed** | 39 | 2 | 22 | 7 |
| **Likely** | 26 | 11 | 13 | 5 |
| **Unverified** | 22 | 19 | 11 | 10 |
| **Any Workaround** | 85 (Yes/Partial) | 26 | 30 | 17 |
| **No Workaround** | 2 | 6 | 16 | 5 |
| **Reproduced at MOSAIC** | 4 | 2 | 1 | 0 |
| **Maintainer Engagement** | Moderate (some confirmed, some engaged); heavy reliance on stale-bot closure | Low — bot closures ("resolved in vX") but almost no maintainer comments | Low for 1.x — v1 is in maintenance and was mass-closed `not_planned` after 2.0 | Some — maintainers merged the VC-001 fix, engaged on VC-008, VC-016 and VC-020; most of the delegation cluster remains one contributor's work |
| **Scan Maturity** | Run 20 (mature) | Run 20 (all entries re-verified in this pass) | Run 20 (all entries re-verified in this pass) | Run 20 (all entries re-verified in this pass) |

### Key Takeaways

- **Claude Code has the most issues tracked and the most mature scan.** Its large count reflects thoroughness of capture and the size of the surface (hooks, background tasks, worktrees, Windows), not only worse reliability. The newest additions (run 20) are concentrated in background-task lifecycle, completion signalling, Windows process handling and tool-input fidelity.
- **GHCP-CLI and VS Code GHCP rest mostly on reporter evidence.** Few classifications have maintainer confirmation, so their lists include more Unverified entries (GHCP-CLI 19 of 32).
- **OpenCode 1.x has a high proportion of Confirmed issues (22 of 46), but almost none are expected to be fixed.** Maintainers moved to 2.0, closed the v1 tracker as `not_planned`, and said v1 compaction code gets only critical fixes (OC-022 is now a Limitation). For MOSAIC on 1.x, treat these as permanent behaviors to adapt to.
- **Seven entries have been reproduced at MOSAIC** (CC-079, CC-080, CC-081, CC-082, GC-020, GC-021, OC-099). The rest of the knowledge base is upstream-sourced intelligence, not hands-on validation, and all other entries carry `Unevaluated` MOSAIC Response.

---

## 2. Cross-Cutting Risk Themes

The most important finding: **every harness shares the same core risk themes**, though the specific failure modes differ. These are not harness-specific problems — they're systemic to the agentic coding tool category.

### 2.1 Subagent Dispatch & Delegation Reliability

The single most critical theme for MOSAIC's hub-and-spoke model. Every harness has issues where a subagent dispatch either silently fails, hangs, loses its tools, or returns misleading results.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Subagent silently returns empty/no response** | CC-003 (killed by limit, reported "completed") | GC-005 (MCP schema budget, zero error signal), GC-039 (stream failure reported as ordinary completion) | OC-090 (stale deny leaves child with no tools, text-only answer) | VC-003 (tool call hangs forever, no timeout), VC-020 (subagent stops instead of using a tool) |
| **Subagent completion signal lost/dropped** | CC-001 (phantom-cancel with fake "user declined"), CC-087/CC-108 (background completion never wakes idle subagent), CC-089 (handback accepted while children run), CC-088 (handback fails in Agent Teams) | GC-038 (autopilot stops after fixed 600 s wait) | OC-005 (no task_id on failure, can't resume) | VC-011 (completion signals dropped after parent turn ends) |
| **Subagent hangs indefinitely** | CC-002 (permission prompt, fails silently), CC-106 (bypass-mode "dangerous rm" prompt) | GC-008 (session hangs, permanent "Cancelling"), GC-036 (MCP call hangs until idle timeout) | OC-001 (nested permission ask dropped, parent hangs), OC-006 (detached process), OC-067 (MCP call hangs) | VC-003 (tool call hangs forever) |
| **Subagent model/config crash** | — | GC-004 (deprecated model crashes entire session) | OC-008 (per-subagent model config ignored) | VC-023 (frontmatter model fails to resolve) |
| **Subagent loses or mis-scopes tools** | CC-086 (background subagents lose built-ins outside a fixed allowlist), CC-009 | GC-033 (no built-ins after plan approval), GC-035 (MCP discovery scoped to parent) | OC-091 (`task` tool absent), OC-094 (MCP calls all denied) | VC-019, VC-021, VC-022, VC-024 (tool inventory does not match the definition) |
| **Nested dispatch broken or unenforced** | CC-011 (disallowedTools not cascading, by design), CC-031 | GC-037 (`--agent` dropped in untrusted folder) | OC-007, OC-090 | VC-010 (allow-list skipped when agentName omitted; clones the caller since 1.140.0) |
| **No internal total timeout/watchdog** | CC-104 (harness SIGTERMs idle background tasks), CC-090 (HTTP MCP call hangs) | — | OC-096 (only a 5-minute per-chunk timeout since 1.18.27; no total or subagent timeout) | VC-003 (no timeout) |

**Cross-harness verdict:** No harness provides reliable, guaranteed subagent dispatch with deterministic success-or-failure signaling. MOSAIC **must** implement external timeout/watchdog logic around every subagent dispatch on every harness — this cannot be delegated to the harness.

**Harness ranking (subagent reliability):**
1. **Claude Code** — Core dispatch generally works, but run 20 found a larger cluster of background-completion and handback edge cases (CC-087 to CC-089, CC-108) that matter for unattended fan-out
2. **OpenCode 1.x** — Core dispatch works but permission scoping is unreliable and will not be fixed on 1.x
3. **GHCP-CLI** — Custom-agent and subagent paths have several silent failures (empty returns, dropped `--agent`, tool-less subagents after plan approval)
4. **VS Code GHCP** — Systematic instability in Agent Host subagent plumbing and tool inventory; most fix PRs were closed unmerged

### 2.2 Permission & Hook Enforcement Gaps

Every harness has issues where configured permissions, hooks, or approval mechanisms are silently not enforced.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Hook deny/ask not enforced** | CC-014 (ask not enforced), CC-015 (killed hook fails open; opt-in `onFailure: "block"` since 2.1.295), CC-010 (defer strands call), CC-105 (ask overridden for `.git/`/`.claude/` writes under bypass) | GC-017 (ask auto-approved by TUI); GC-016 (deny ignored) was closed as fixed in 1.0.86 with no PR | OC-084 (`permission.ask` hook never fires on 1.x), OC-085 (wildcard deny clobbers allow) | — |
| **Permission bypass / prompts in non-interactive/headless mode** | CC-016 (background subagents denied Write/Bash), CC-106 (bypass-mode prompts still block), CC-099 (Auto mode fails closed on Edit/Write) | GC-009 (approval revoked mid-session), GC-012 (allow-all ignored, incl. managed policy), GC-013 (Skill allowed-tools ignored in -p), GC-019 (managed setting bypassed) | OC-001 (nested ask dropped); `--auto` now cascades to subagents since 1.18.20 (OC-087, edge case may hang) | VC-008, VC-018 (terminal auto-approve fails on `--` and `./path`) |
| **Tool scoping not enforced on subagents** | CC-011 (by design), CC-031 (named dispatch ignores definition) | GC-035 | OC-090 (stale parent denies and caller-identity bypass) | VC-005, VC-010, VC-019 to VC-022 |
| **Config/allowlist not loaded** | CC-041 (ancestor settings), CC-034 (--add-dir rules), CC-036 (CLAUDE.md above repo root), CC-059 | GC-014 (allowed_directories), GC-015 (commandIdentifiers with spaces) | OC-082 (permission merges instead of replaces), OC-083 (config dir overrides path) | VC-025 (frontmatter keys dropped, no AGENTS.md) |
| **Hook/plugin never runs or is cut off in headless exit paths** (MOSAIC-reproduced) | CC-082 (async `Stop`/`SessionEnd` hooks killed on `-p` exit; use synchronous registration), CC-103 (Windows freeze after hook exit) | GC-020 (repo hooks not loaded unless the folder is trusted) | OC-099 (`session.idle` handler not awaited on `opencode run` exit) | — |

**Cross-harness verdict:** Permission enforcement is unreliable across **all** harnesses, especially in non-interactive/headless mode — exactly where MOSAIC's Runner pipeline operates. MOSAIC should treat harness-level permission/hook enforcement as a convenience layer, not a security boundary. Critical guardrails must be enforced externally (OS/sandbox level, or by MOSAIC's own protocol-level validation).

**Worst harness for headless permission reliability:** **GHCP-CLI** — five separate issues (GC-009, GC-012, GC-013, GC-019, GC-037) show the non-interactive path's permission and trust handling diverging from interactive, in both directions (unexpectedly restrictive and unexpectedly permissive).

### 2.3 Context Compaction & Session Stability

Every harness has issues where long sessions degrade, compact incorrectly, or crash.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Compaction loses/corrupts state** | CC-006 (preserves fabricated instructions), CC-007 (behavioral rules dropped), CC-026 (second /compact replays history, quadratic growth) | GC-002 (recursive re-summarization degrades facts) | OC-021 (empty summary on reasoning models), OC-023 (races ahead, drops task goal) | — |
| **Compaction doesn't fire / fires wrong** | CC-029 (manual compact silently no-ops), CC-096 (32 MB request cap bricks session) | GC-001 (force-compacts at low usage, loops), GC-026 (failed compaction retried every turn) | OC-022 (never trims in single-turn; now a Limitation), OC-025, OC-026, OC-028 | VC-027 (BYOK limits 0/0, silent compaction at ~100K) |
| **OOM / session crash** | CC-044 (100% CPU on base64), CC-040 (token inflation) | GC-007 (OOM on --resume), GC-023/GC-024 (compaction OOM, interactive OOM at ~4.3 GB) | OC-027 (overflow misclassified as network error), OC-093 (advertised vs actual context) | VC-009 (hallucinates at ~340k+ tokens) |
| **Post-compaction tool/state loss** | CC-067 (skills uninvocable after compaction), CC-035 | — | OC-024 (MCP tools drop to 0 after compaction) | — |

**Cross-harness verdict:** No harness provides reliable automatic compaction for long-running sessions. MOSAIC's blackboard pattern (`Orchestration.md`) and the principle of externalizing durable state to on-disk artifacts are validated as essential — not optional — by every harness's compaction failures. The specific recommendation: **cap session lifetime and checkpoint aggressively** rather than trusting any harness's compaction to preserve orchestration state.

**Worst harness for session stability:** **GHCP-CLI** for long-lived or resumed sessions (four memory/compaction entries rated HIGH, with no OOM fix confirmed). **OpenCode 1.x** has the widest compaction failure spread (eight entries covering detection, execution and post-compaction state), now with no fix expected.

### 2.4 Tool Execution Reliability

Issues where a tool call succeeds (no error) but the result is wrong, incomplete, or never happened.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Write/edit silently fails or alters content** | CC-097 (literal `\uXXXX` decoded), CC-098 (trailing whitespace stripped), CC-093 (Windows Bash command truncated at ~8,186 chars) | GC-041 (long-path `view` refused), GC-006 (create-file retry loop, closed as fixed in 1.0.85) | OC-041 (write fails silently on large files) | VC-015 (edit reports success before save resolves) |
| **Write/edit wrong target or no-op** | CC-055, CC-056, CC-094 (Edit/Write pinned to wrong worktree) | — | OC-042 (empty content silently overwrites) | — |
| **Tool output truncated/lost** | CC-044 | GC-027 (>20 KB output replaced by temp-file path, no paging) | OC-043, OC-046, OC-047, OC-098 | — |
| **Tool call races / double execution** | — | — | OC-045 (encode failure fails sibling calls), OC-095 (parallel race, turn never completes) | VC-012 (double invocation, empty args, dropped results) |
| **Bash/shell hangs or orphans** | CC-038 (freeze, fixed in 2.1.290), CC-100/CC-109 (Windows process trees not killed), CC-095 (hard SIGTERM on timeout) | GC-008 | OC-044 (hangs on daemons — six failed fix attempts) | VC-026 (terminal hangs when prompt hooks defeat shell integration) |

**Cross-harness verdict:** File writing reliability is a universal concern, and run 20 found a new kind: the harness silently *changes* content on the way to disk (CC-097, CC-098). MOSAIC should **verify writes after the fact** for any safety-critical artifact, rather than trusting a tool's reported success. The pattern of silent write failures directly threatens MOSAIC's artifact-based workflow where subagents write `Research.md`, `Plan.md`, etc.

### 2.5 MCP Tool Integration

Issues specific to MCP (Model Context Protocol) tool connectivity and execution.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **MCP tools not wired/connected** | CC-008 (one malformed schema drops all tools), CC-012 (stale ToolSearch reference bricks session), CC-083 (changed description never reaches session) | GC-028 (large fleets connect partially), GC-003 (workspace .mcp.json; fixed 1.0.85/1.0.89, needs persisted trust) | OC-061 (stale tool-set cache), OC-024 (tools drop to 0 after compaction) | VC-016 (discovery-timing race, tools rejected as "disabled"; maintainer declined fix) |
| **MCP subagent gap** | CC-086 (MCP resource tools unavailable to background subagents), CC-085 (inline servers shared across concurrent subagents). Native subagents otherwise get MCP tools from turn one; server guidance arrives late (CC-080/CC-081, LOW) | GC-005 (schema budget), GC-034 (custom agent disables deferred MCP loading), GC-035 | OC-094 (subagent MCP calls all denied) | — |
| **MCP tool errors swallowed/unhelpful** | CC-084 (empty-string argument emitted as invalid JSON) | — | OC-062, OC-064 | VC-017 (failed task payload discarded) |
| **MCP server crashes / hangs** | CC-090 (HTTP call hangs forever if server dies) | GC-036 (call hangs until idle timeout) | OC-065, OC-067 | — |
| **MCP concurrency issues** | — | — | OC-066 (CPU saturation), OC-095 | — |

**Cross-harness verdict:** MCP integration is the least mature subsystem across all harnesses. If MOSAIC subagents depend on MCP tools, they face silent tool-loss, permission gaps, and unhelpful error reporting on every harness. Recommendation: treat MCP tool availability as **best-effort**, not guaranteed, and have agents fall back to native tools (grep, glob, bash) when possible.

**Claude Code native mode specifically:** MCP tools work for subagents dispatched via the Agent tool, in both foreground and background. The only requirement is `mcp__<server>__*` in the agent's `tools:` field — no `mcpServers:` needed (servers defined in `.mcp.json` are already connected at session level). Tool names and input schemas are present from the first turn. The `mcpServers:` frontmatter field connects a server and delivers its instructions, but does NOT add tools past the `tools:` allowlist (CC-079, Limitation). The remaining low-impact gap is *instruction routing*: the server's usage guidance arrives after the subagent's first tool turn, covering every connected server (CC-080/CC-081). Caveats added in run 20: inline `mcpServers:` isolation may share one server process across concurrent subagents (CC-085), and background subagents lose some built-in tools (CC-086). See `ClaudeCode/mcp-frontmatter-blocker.md`.

### 2.6 Worktree & Isolation Issues

Specific to multi-worktree or isolation patterns.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Worktree isolation leaks** | CC-054/055/056/057/094 (session-wide latch, Edit writes wrong tree, cwd unresolvable) | — | — | — |
| **Subagent cwd confusion** | CC-023 (lands in sibling's worktree, now reaching `Read`), CC-073 (cwd resets outside project), CC-063 (hook CLAUDE_PROJECT_DIR stale) | — | — | VC-002 (tool call mis-routed to parent after restart) |
| **Nested worktree blocked / guard over-refuses** | CC-075 (EnterWorktree refused in pinned-cwd subagent), CC-107 (isolation guard refuses harmless commands) | — | — | — |

**Cross-harness verdict:** Worktree isolation issues are almost exclusively a Claude Code problem (about ten entries), reflecting Claude Code's unique `EnterWorktree` / `isolation:"worktree"` mechanism. MOSAIC should treat worktree isolation as **leaky, not airtight** on Claude Code, and design subagents to explicitly verify their working directory at the start of every Bash call.

### 2.7 Background Tasks & Process Lifecycle

New in run 20. Long-running commands and background work are where unattended fan-out meets the harness's own process management.

| Failure Mode | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|-------------|:-----------:|:--------:|:------------:|:------------:|
| **Background work killed or reaped unexpectedly** | CC-104 (idle sweep SIGTERMs tracked tasks ~30 min after last human message), CC-052, CC-019, CC-018, CC-051 (macOS pressure reaping), CC-110 (Monitor capped at 30 min) | GC-038 (autopilot gives up after 600 s) | — | — |
| **Child processes survive cancel/timeout** | CC-100, CC-109 (Windows process trees; `TaskStop` reports success) | — | OC-092 (cancelling a subagent does not kill its children), OC-044 | — |
| **Completion not delivered to waiting agent** | CC-087, CC-108, CC-089 | GC-039 | OC-005 | VC-011 |

**Cross-harness verdict:** Treat "stopped" and "completed" notifications as ambiguous on every harness. For long commands, use a detached session plus a status file, and verify completion and process state independently.

---

## 3. Mode-Specific Risk Assessment

MOSAIC operates in two fundamentally different execution modes, and a harness's risk profile can be radically different between them. A harness that is dangerous for Runner/headless execution may work perfectly well in harness-native interactive mode, and vice versa.

### 3.1 Runner / Headless Mode

MOSAIC's Runner pipeline drives harnesses as non-interactive subprocesses (`-p` mode, `--prompt`, or equivalent). No human is available to answer permission prompts, click approval dialogs, or manually recover from hangs.

| Risk Category | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|--------------|:-----------:|:--------:|:------------:|:------------:|
| **Permission enforcement** | MEDIUM — bypass-mode and background-subagent prompts can still block (CC-106, CC-016), Auto mode can fail closed (CC-099); `permissions.allow` generally works | **CRITICAL** — five breakdowns: approval revoked mid-session (GC-009), /allow-all ineffective (GC-012), Skill allowed-tools ignored (GC-013), managed setting bypassed (GC-019), `--agent` dropped in untrusted folder (GC-037) | MEDIUM-HIGH — `--auto` cascades to subagents since 1.18.20 (OC-087) but a reused-`task_id` edge case may hang; nested ask dropped (OC-001) | HIGH — auto-approve parser bugs on `--` and `./path` (VC-008, VC-018) block common commands |
| **Session stability** | HIGH — compaction issues (CC-026, CC-096), Windows Bash truncation (CC-093) | **CRITICAL** — OOM and compaction loops (GC-007, GC-023, GC-024, GC-026, GC-001) | HIGH — compaction never fires or fires wrong (OC-022/026), overflow kills session (OC-027); no fix expected on 1.x | HIGH — hallucination at high token counts (VC-009), silent compaction with BYOK (VC-027) |
| **Unrecoverable hangs** | HIGH — background tasks killed or never wake the agent (CC-104, CC-087/108), Windows freeze after hook exit (CC-103) | HIGH — permanent "Cancelling" state (GC-008), MCP hang (GC-036), no internal escape | HIGH — only a per-chunk stream timeout (OC-096), MCP and bash hangs (OC-067, OC-044) | HIGH — tool call hangs forever (VC-003), no timeout |
| **Silent tool-execution failure** | MEDIUM — silent content alteration (CC-097, CC-098), Bash truncation on Windows (CC-093); most other failures visible | HIGH — empty subagent (GC-005, GC-039), headless exit 0 mid-task (GC-040) | HIGH — silent write failure (OC-041), MCP denial (OC-094) | HIGH — edit reports success before save (VC-015) |

**Runner mode ranking:**
1. **Claude Code** — Still the lowest overall risk, but no longer "low": run 20 surfaced real unattended-run hazards (background-task kills, Windows process trees, content-fidelity bugs). Its workarounds are the most complete.
2. **OpenCode 1.x** — Good tool surface and the `--auto` cascade is fixed, but no total timeout, compaction is unreliable, and 1.x will not receive further fixes.
3. **VS Code GHCP** — Agent Host plumbing instability causes unrecoverable hangs; terminal auto-approve gaps block common commands.
4. **GHCP-CLI** — **Most dangerous for headless execution.** Five permission/trust breakdowns in the non-interactive path, OOM and compaction failures on long sessions, and an unrecoverable session-hang state. Every headless invocation is at risk.

### 3.2 Harness-Native / Interactive Mode

The primary agent runs interactively in the harness's own UI/CLI, with a human available to answer prompts, approve tool calls, and intervene on hangs. Subagents are dispatched through the harness's native agent/task tool.

| Risk Category | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|--------------|:-----------:|:--------:|:------------:|:------------:|
| **Subagent dispatch reliability** | LOW-MEDIUM — core dispatch works; edge cases around permission prompts (CC-002), usage-limit misreporting (CC-003), background completion (CC-087/108) | MEDIUM — silent empty returns (GC-005), tool-less subagents after plan approval (GC-033), but session hang (GC-008) can be recovered by kill+resume | HIGH — nested permission ask dropped (OC-001), stale denies strip tools (OC-090) | **HIGHEST** — systematic plumbing instability (VC-003/011/012) and tool-inventory mismatches (VC-019 to VC-025) |
| **Permission/hook enforcement** | MEDIUM — hook deny/ask gaps (CC-010/014/015), but human can intervene | MEDIUM — ask auto-approved (GC-017), but human can catch and retry | HIGH — `permission.ask` hook dead on 1.x (OC-084), wildcard deny clobbers allow (OC-085) | HIGH — allow-list bypasses (VC-010), tool restrictions ignored (VC-005), but human can observe |
| **Context/session stability** | MEDIUM — compaction loses behavioral rules (CC-007), skills lost post-compaction (CC-067) | MEDIUM-HIGH — recursive summarization (GC-002), interactive OOM at ~4.3 GB (GC-024) | HIGH — eight compaction issues, but human can checkpoint manually | LOW-MEDIUM — hallucination at ~340k+ tokens (VC-009), BYOK silent compaction (VC-027) |
| **Tool execution** | LOW-MEDIUM — worktree issues (CC-055/056/094) and content fidelity (CC-097/098) | LOW — tool-output spill (GC-027) is the main concern | MEDIUM — silent write failure on large files (OC-041), bash hangs on daemons (OC-044) | MEDIUM — edit success before save (VC-015), client-tool races (VC-012), terminal hang with prompt hooks (VC-026) |
| **HITL effectiveness** | **BEST** — human can approve, retry, and steer via AskUserQuestion (primary-only) | Good — human can approve via TUI, kill+resume on hangs | Good — human can approve, question tool works both tiers | Weakest — AskUserQuestion silently auto-denied when queue empty (VC-011), subagent UI visibility gaps (VC-004) |

**Harness-native mode ranking:**
1. **Claude Code** — Most issues are edge cases in interactive mode. Core dispatch, tools, and HITL all work. Worktree and background-completion clusters are the main concerns.
2. **GHCP-CLI** — Better than its Runner ranking. The `-p` permission cluster doesn't apply, and session hangs are recoverable via kill+resume. Memory growth in long interactive sessions is the main risk.
3. **OpenCode 1.x** — Permission evaluator bugs and compaction failures persist in interactive mode, with no fixes expected.
4. **VS Code GHCP** — **Most problematic for interactive use.** Agent Host plumbing instability and subagent tool-inventory mismatches occur regardless of whether a human is present; external kill is often the only recovery.

### 3.3 Mode Ranking Comparison

| Harness | Runner Rank | Native Rank | Delta | Takeaway |
|---------|:----------:|:----------:|:-----:|----------|
| **Claude Code** | 1 (best) | 1 (best) | — | Consistently lowest risk across both modes, with more unattended-run hazards than previously known |
| **GHCP-CLI** | 4 (worst) | 2 | **+2** | Its defining weakness (non-interactive permission and trust breakdown) vanishes in interactive mode |
| **OpenCode 1.x** | 2 | 3 | **-1** | Permission and compaction issues affect both modes roughly equally; no internal total timeout is universal |
| **VS Code GHCP** | 3 | 4 (worst) | **-1** | Agent Host plumbing instability is primarily a harness-native problem; headless/CLI-driven invocations may bypass some of this machinery |

---

## 4. Issue Classification Comparison

| Metric | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|--------|:-----------:|:--------:|:------------:|:------------:|
| **Bugs** | 76 | 28 | 44 | 20 |
| **Limitations** | 10 (CC-011, 048, 051, 075, 079, 082, 086, 095, 106, 110) | 4 (GC-002, GC-020, GC-021, GC-027) | 2 (OC-022, OC-099) | 1 (VC-022) |
| **Quirks** | 0 | 0 | 0 | 1 (VC-026) |
| **Bug/Limitation** | 1 (CC-030) | 0 | 0 | 0 |

**Observation:** Most entries across all harnesses are classified as Bug. Limitations are growing as scans mature and MOSAIC's own experiments pin down by-design behavior (CC-079, CC-082, GC-020, GC-021, OC-099). OpenCode 1.x entries are drifting toward permanent-limitation status because v1 is in maintenance (OC-022 was reclassified in run 20).

---

## 5. Workaround Coverage

| Workaround Status | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|------------------|:-----------:|:--------:|:------------:|:------------:|
| **Yes (confirmed workaround)** | 55 (63%) | 8 (25%) | 5 (11%) | 2 (9%) |
| **Partial (incomplete/trade-off)** | 30 (34%) | 18 (56%) | 25 (54%) | 15 (68%) |
| **No workaround** | 2 (2%) | 6 (19%) | 16 (35%) | 5 (23%) |

**Cross-harness verdict:** Claude Code has the best workaround coverage — almost every issue has at least a partial workaround. OpenCode 1.x has the worst: over a third of its issues have no known workaround, and no fixes are expected. This correlates with maintainer engagement — workarounds often come from maintainer suggestions in issue comments.

---

## 6. Thematic Clusters — Unique to Each Harness

Issues that are distinctive to a single harness and have no clear parallel elsewhere.

### Claude Code Only
| Theme | Key Issues | MOSAIC Impact |
|-------|-----------|---------------|
| **Worktree isolation failures** | CC-023, CC-054–057, CC-063, CC-073, CC-075, CC-094, CC-107 | Directly threatens multi-worktree fan-out patterns |
| **Auto Mode defeats safety mechanisms** | CC-060, CC-099 | Bash-first steering defeats nested rules loading, /rewind, hooks, and Edit/Write permissions; Auto mode can fail closed |
| **Background-task lifecycle and handback** | CC-087–089, CC-104, CC-108–110 | Unattended fan-out can stall, lose completions, or leave orphaned processes |
| **Windows process and hook handling** | CC-068–071, CC-093, CC-100, CC-103, CC-109 | Windows-specific: truncation, hooks never fire or freeze, process trees survive |
| **Tool-input fidelity** | CC-097, CC-098 | Harness silently alters written content |
| **Request-size cap** | CC-096 | 32 MB cap can brick a session because token-based auto-compact never fires |
| **Skills become uninvocable after compaction** | CC-067 | Skill-dependent subagents can silently lose their skill mid-session |
| **Token usage inflation** | CC-040 | Multi-iteration turns report inflated usage; only partly fixed |

### GHCP-CLI Only
| Theme | Key Issues | MOSAIC Impact |
|-------|-----------|---------------|
| **Non-interactive permission and trust breakdown cluster** | GC-009, GC-012, GC-013, GC-019, GC-037 | Five distinct ways the non-interactive path's permission and trust handling fails — the harness's defining weakness |
| **Memory growth, OOM and compaction loops** | GC-001, GC-007, GC-023, GC-024, GC-026 | Long-lived sessions crash or become unresumable; no confirmed fix |
| **Custom-agent and subagent tool loading** | GC-033, GC-034, GC-035 | `--agent` runs flood context with MCP schemas; subagents can start with no tools |
| **Cmd.exe prompt mangling** | GC-021 | `-p <text>` is mangled through the npm shim on Windows; pass the prompt via stdin |

### OpenCode 1.x Only
| Theme | Key Issues | MOSAIC Impact |
|-------|-----------|---------------|
| **Permission evaluator ordering defects** | OC-082, OC-085, OC-097 | Wildcard deny clobbers specific allow; plugin permissions can't override root deny |
| **Stale denies strip subagent tools** | OC-090, OC-094 | Subagents can end up with no tools or every MCP call denied |
| **Bash tool hangs on daemons (six failed fixes)** | OC-044 | Treat as durable — not getting fixed on 1.x |
| **Concurrent tool-call races** | OC-045, OC-095 | One failed tool can falsely fail siblings; slow MCP tool stuck forever |
| **v1 maintenance mode** | OC-022 and the mass-closed entries | No fixes expected on 1.x; adapt permanently |

### VS Code GHCP Only
| Theme | Key Issues | MOSAIC Impact |
|-------|-----------|---------------|
| **Agent Host subagent plumbing** | VC-002–004, VC-010–012 | Systematic architectural instability; most fix PRs were closed unmerged |
| **Subagent tool inventory mismatch** | VC-019–025 (and TI-001 in `ToInvestigate.md`) | Subagents are offered tools they cannot use, or miss tools their definition declares |
| **Terminal auto-approve parser gaps** | VC-008, VC-018 | `--` fix expected in 1.144.0; `./path` commands never auto-approve on PowerShell |
| **High-token hallucination** | VC-009 | Agent invents and acts on fictional tasks at ~340k+ tokens |

---

## 7. MOSAIC Mitigation Priority Matrix

Recommended mitigations ordered by cross-cutting impact. The **Mode** column indicates which execution mode each mitigation applies to — mitigations affecting only one mode can be skipped when planning for the other.

| Priority | Mitigation | Mode | Harnesses | Issue IDs |
|:--------:|-----------|:----:|-----------|-----------|
| **1** | **External wall-clock timeout around every subagent dispatch.** No harness provides reliable internal total timeout or guaranteed completion signaling. | Both | All four | CC-001/002/003/087/104/108, GC-005/008/039, OC-001/005/006/096, VC-003/011 |
| **2** | **Verify file writes after the fact** for safety-critical artifacts, including content fidelity (trailing whitespace, `\u` sequences). Don't trust a tool's reported success. | Both | All four | CC-097/098, OC-041/042, VC-015 |
| **3** | **Externalize durable state to on-disk artifacts early and often.** Don't rely on conversational context surviving compaction. Validates MOSAIC's Orchestration.md blackboard pattern as essential. | Both | All four | CC-006/007/026/067, GC-001/002/026, OC-021–028, VC-009/027 |
| **4** | **Cap session lifetime and restart proactively** rather than running one session indefinitely. | Both | All four | CC-040/096, GC-001/007/023/024, OC-022/026/027/093, VC-009 |
| **5** | **Treat harness permission/hook enforcement as convenience, not security.** Enforce critical guardrails externally. | Both | All four | CC-010/014/015/025/105, GC-017, OC-084/085/090, VC-005/010 |
| **6** | **Treat MCP tool availability as best-effort.** Have agents fall back to native tools when MCP tools are unavailable or silently failing. **Claude Code native mode specifically:** MCP works for native subagents in both foreground and background; list `mcp__<server>__*` in `tools:`. | Both | All four | CC-008/079/083/085/086, GC-005/028/034, OC-024/061/062/064/065/094, VC-016/017 |
| **7** | **For long-running background commands, use a detached session plus a status file, and verify completion and process state yourself.** | Both | CC, GC | CC-052/087/100/104/108/109/110, GC-038 |
| **8** | **Instruct subagents to split large artifact writes** into multiple smaller files. | Both | CC, OC | CC-093, OC-041 |
| **9** | **For GHCP-CLI headless: pre-trust the working directory or pass `--additional-mcp-config @.mcp.json`, pass prompts via stdin on Windows, and verify non-interactive permission behavior directly before relying on it.** | Runner | GC only | GC-003/009/012/013/019/020/021/037 |
| **10** | **For Claude Code: treat worktree isolation as leaky.** Have subagents explicitly verify their cwd at the start of every Bash call. | Both | CC only | CC-023/054–057/063/073/075/094 |
| **11** | **For VS Code GHCP: keep delegation to a single hop** from the primary; don't trust allow-lists or declared tool lists as hard enforcement. | Native | VC only | VC-003/010/011/012/019–022 |
| **12** | **For GHCP-CLI native: use kill+resume as the standard recovery** for session hangs. Don't wait indefinitely. | Native | GC only | GC-008 |
| **13** | **For OpenCode 1.x headless: use 1.18.20 or later and still guard `opencode run --auto` fan-out with a watchdog.** | Runner | OC only | OC-087, OC-096 |

---

## 8. MOSAIC Compatibility Scorecard — Known Issues Perspective

Rating each harness on how its known issue landscape affects MOSAIC's core patterns. **Separated by execution mode** because a harness's risk profile can be radically different between Runner and harness-native (see Section 3.3 for the ranking delta).

### 8.1 Runner / Headless Mode Scorecard

| MOSAIC Pattern | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|---------------|:-----------:|:--------:|:------------:|:------------:|
| **Headless permission enforcement** | Low-moderate risk | **Highest risk** (5 independent breakdowns) | Moderate risk (`--auto` fixed in 1.18.20; edge case) | High risk (auto-approve parser bugs) |
| **Artifact file writing** | Moderate risk (content-fidelity bugs, Windows truncation; workarounds exist) | Moderate risk (large-file retry loop closed as fixed; long-path `view` failure) | Moderate risk (silent fail on large files) | Moderate risk (success before save) |
| **Long-running session stability** | Moderate risk (compaction, 32 MB cap) | **Highest risk** (OOM, compaction loops) | High risk (8 compaction issues, no fix expected) | Moderate risk (hallucination at high tokens, BYOK compaction) |
| **Hang recovery (no human available)** | Moderate risk (background kills, missed wake-ups) | High risk (permanent "Cancelling" state, MCP hang) | **Highest risk** (only a per-chunk stream timeout) | High risk (tool call hangs forever) |
| **MCP tool reliance** | Moderate risk (Runner uses `--agent`; instructions arrive late; frozen descriptions) | High risk (partial startup connect, schema flooding) | **Highest risk** (7+ MCP issues, no fix expected) | Moderate risk (discovery race, task payload loss) |
| **Silent dispatch failure** | Moderate risk | High risk (empty subagent, stream failure as completion) | High risk (silent write, MCP denial) | High risk (edit success before save) |

**Runner mode ranking:**

| Rank | Harness | Verdict |
|:----:|---------|---------|
| 1 | **Claude Code** | **Lowest Runner risk.** Permission enforcement generally works in non-interactive contexts and workaround coverage is the best. Background-task kills, Windows process handling and silent content alteration are the main concerns. |
| 2 | **OpenCode 1.x** | **Moderate risk.** `--auto` cascade is fixed. Only a per-chunk stream timeout exists, compaction is unreliable, and no further 1.x fixes are expected. |
| 3 | **VS Code GHCP** | **High risk.** Agent Host plumbing instability causes unrecoverable hangs with no timeout. |
| 4 | **GHCP-CLI** | **Highest Runner risk.** Non-interactive permission and trust enforcement is broken across five mechanisms, OOM on long sessions, and most entries are Unverified. |

### 8.2 Harness-Native / Interactive Mode Scorecard

| MOSAIC Pattern | Claude Code | GHCP-CLI | OpenCode 1.x | VS Code GHCP |
|---------------|:-----------:|:--------:|:------------:|:------------:|
| **Hub-and-spoke dispatch** | Low-moderate risk (edge cases, background completion) | Moderate risk (silent empty returns, but human can detect) | High risk (permission bypass, no total timeout) | **Highest risk** (plumbing instability, tool-inventory mismatch, signal drops) |
| **HITL effectiveness** | **Best** (human can approve, retry, steer) | Good (TUI approval, kill+resume) | Good (question tool both tiers) | Weakest (AskUserQuestion silently auto-denied, subagent UI gaps) |
| **Permission/hook enforcement** | Moderate risk (hook gaps, but human intervenes) | Moderate risk (ask auto-approved, but human catches) | High risk (evaluator ordering bugs affect both modes) | High risk (allow-list bypasses, tool restrictions ignored) |
| **Long-running session stability** | Moderate risk (compaction, behavioral rule loss) | Moderate-high risk (interactive OOM, recursive summarization) | High risk (8 compaction issues) | Low-moderate risk (hallucination at extreme token counts) |
| **Artifact file writing** | Moderate risk (content fidelity) | Low risk | Moderate risk (silent fail on large files) | Moderate risk (success before save) |
| **MCP tool reliance** | Low-moderate risk (native subagents get MCP tools; guidance is late) | Moderate risk (schema flooding with `--agent`, partial startup connect) | **Highest risk** (7+ MCP issues) | Moderate risk (discovery race, task payload loss) |
| **Skills injection** | Moderate risk (CC-067 compaction loss, CC-035 cap) | Unknown (not enough data) | Moderate risk (OC-088 hardcoded /tmp) | High risk (VC-004 forked skills invisible) |

**Harness-native mode ranking:**

| Rank | Harness | Verdict |
|:----:|---------|---------|
| 1 | **Claude Code** | **Lowest interactive risk overall.** Best HITL surface. Worktree and background-completion clusters are the main concerns. |
| 2 | **GHCP-CLI** | **Much better than its Runner ranking.** The `-p` permission cluster doesn't apply interactively; session hangs recoverable via kill+resume. Long-session memory growth is the main risk. |
| 3 | **OpenCode 1.x** | **Permission and compaction issues persist in both modes** and are not expected to be fixed on 1.x. |
| 4 | **VS Code GHCP** | **Most problematic for interactive use.** Agent Host plumbing and tool-inventory mismatches have no in-harness recovery regardless of human presence. |

---

## 9. Open Questions

| # | Question | Why It Matters |
|---|----------|---------------|
| 1 | Has MOSAIC reproduced more than the 7 entries marked reproduced (CC-079/080/081/082, GC-020/021, OC-099)? | The rest of the knowledge base (about 180 active entries) is upstream-sourced. Until MOSAIC reproduces the highest-impact ones on its own infrastructure, their real-world relevance is theoretical. |
| 2 | Which compaction failures actually manifest at MOSAIC's typical session lengths? | CC/GC/OC all have compaction issues, but MOSAIC sessions may not run long enough to trigger them. Empirical session-length data would let us de-prioritize inapplicable entries. |
| 3 | Is GHCP-CLI's `-p` mode permission breakdown version-gated? | Several entries were reported against versions well behind v1.0.103, and some were closed by the bot as fixed. A single reproduction attempt on the current release could resolve or confirm a large fraction of the GHCP-CLI risk. |
| 4 | Does VS Code GHCP's Agent Host subagent instability affect Copilot Chat's built-in agent modes, or only the external Claude-provider path? | The original investigation is on a dev-container Claude provider setup. If MOSAIC uses Copilot's native models via the built-in provider, the blast radius may be narrower. |
| 5 | What is the practical threshold for OpenCode OC-041's silent large-file write failure? | OC-041 says "~1000+ lines" and the maintainer attributes it to the tool call being cut off at the model's output token limit. Knowing the exact threshold lets MOSAIC subagent templates proactively split artifacts. |
| 6 | Do the Windows Claude Code hazards (CC-093 command truncation, CC-100/CC-109 process trees, CC-103 hook freeze) reproduce under headless `-p` on v2.1.310? | Much of their evidence comes from older versions or the VS Code/desktop apps, while Windows is a primary MOSAIC environment. |
| 7 | When will OpenCode 2.x be tracked as its own harness? | 2.0 changes plugins, permissions, config loading and MCP defaults; several 1.x entries are fixed or irrelevant there, and 2.x has its own new issues. |

---

## 10. Model Precedence: Launch-Time Argument vs Agent Definition

When the launch-time model argument and the agent definition's `model` field disagree, **the launch-time argument wins in both harnesses**. The explicit launch argument is therefore the authoritative setting for the subject; AgentTest passes it on every launch and additionally checks the deployed definitions' model fields before the harness starts.

| Harness | Version | Evidence | Order (first wins) |
|---------|---------|----------|--------------------|
| OpenCode | v1.18.18 | Source: `packages/opencode/src/session/prompt.ts` (`input.model ?? agent.model ?? currentModel(session)`); `--model` reaches `input.model` via `packages/opencode/src/cli/cmd/run.ts` | `--model provider/model`, then the agent's `model`, then the session's current model |
| Claude Code | 2.1.310 | Manual probe 2026-10-24: agent `model: haiku` launched with `claude --agent <name> -p --output-format json --model sonnet` ran sonnet, in the user config home and in AgentTest's relocated credentials-only home | `--model`, then the agent's `model:`, then settings `model`, then the account default |

Claude Code subagents spawned inside a session (AgentTest stubs) do not follow `--model`; they follow the subagent order: per-invocation parameter, the definition's `model`, `CLAUDE_CODE_SUBAGENT_MODEL`, then the main model. That is why every stub definition's model field is checked as well. The transcript `message.model` remains the reliable signal of the model that actually ran (CC-022).

---

*Generated from individual harness issue knowledge bases in this folder. See each harness's `index.md` for per-harness summaries and `active-issues.md` for full issue details.*
