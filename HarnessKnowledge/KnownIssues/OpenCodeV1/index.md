# OpenCode 1.x — Issue Index

> Last updated: 2026-10-25 (run 20)
> Latest platform version: v1.19.4 (released 2026-10-22)

> **Scope: OpenCode 1.x ONLY.** This folder (`OpenCodeV1/`) tracks the OpenCode **v1** line (`anomalyco/opencode`, 1.x releases), which is still being developed and released in parallel with 2.x. **OpenCode 2.0 is treated as a separate, new harness** (its own future folder and ID range) because it is too fresh and differs substantially (e.g. v1 plugins and server API are incompatible; MCP tools default to Code Mode; `permission.ask` replaced by `permission.evaluate`; `~/.config/opencode` loading changed). Do **not** assume any `OC-` entry applies to 2.x, and do not add 2.x findings here.
>
> **Upstream "closed" status is weak evidence for v1.** In October 2026 maintainers mass-closed v1 issues as `not_planned` ("OpenCode 2.0 is out, closing issues filed against OpenCode 1") and the stale bot closes the rest. A closed v1 issue is not a fix; keep the entry active (flagged "Needs re-verification") unless a fix on a 1.x release is verified.

## Active Issues (46)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| OC-001 | Nested (depth ≥2) subagent permission ask silently dropped — parent hangs forever, interrupt is a no-op | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-002 | Child-session events carry no parent lineage — external integrations can't distinguish running subagent from blocked root | Bug | Likely | MEDIUM | No | No | Unevaluated |
| OC-003 | Rejecting one of a subagent's multiple pending permission asks cancels the entire subagent | Bug | Unverified | HIGH | No | No | Unevaluated |
| OC-004 | Rejecting a permission ask during parallel subagents applies same rejection to all of them | Bug | Unverified | HIGH | No | No | Unevaluated |
| OC-005 | Foreground subagent failure/cancellation returns no task_id — parent can't resume, must re-dispatch from scratch | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-006 | Subagent hangs after bash tool call if it left a live detached descendant process | Bug | Likely | HIGH | Partial | No | Unevaluated |
| OC-007 | Task tool's subagent list ignores session-level permission overrides (description mismatch) | Bug | Likely | MEDIUM | No | No | Unevaluated |
| OC-008 | Per-subagent model configuration ignored — all subagents run on primary model | Bug | Likely | HIGH | Partial | No | Unevaluated |
| OC-021 | Compaction produces empty summary for reasoning models — history silently dropped | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-022 | `compaction.prune` never trims context in single-turn/headless sessions — compaction thrash every ~10 min | Limitation | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-023 | Auto-compaction not a barrier — session loop races ahead, swallows pending input, drops task goal | Bug | Confirmed | HIGH | No | No | Unevaluated |
| OC-024 | MCP tool manifest drops to 0 after compaction — server stays connected, only new session recovers | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| OC-025 | Manual `/compact` (and auto overflow guard) has no context-fit check vs. compaction model — hangs or silent no-fire | Bug | Likely | HIGH | Partial | No | Unevaluated |
| OC-026 | Native auto-compaction never fires on long sessions (`session_context_epoch` stuck at 0) — runaway token/credit burn | Bug | Unverified | HIGH | No | No | Unevaluated |
| OC-027 | Context-overflow errors misclassified as "Network connection lost" — no compaction, no retry, session dies | Bug | Confirmed | HIGH | No | No | Unevaluated |
| OC-028 | `compaction.reserved` silently ignored for models lacking `limit.input` — compaction fires too late, degrades quality/throughput | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-081 | Injected prompts without agent/model silently switch session's agent/model | Bug | Confirmed | HIGH | No | No | Unevaluated |
| OC-082 | Agent frontmatter `permission` merges additively instead of replacing defaults | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| OC-083 | `OPENCODE_CONFIG_DIR` overrides (not adds to) global AGENTS.md path | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-084 | `permission.ask` plugin hook never triggers (dead since ~v1.2.x) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-085 | Wildcard `"*": deny` intermittently overrides specific `allow` rules (long-running) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-088 | Built-in Skills hardcode `/tmp`, ignore AGENTS.md temp-dir instructions | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| OC-090 | Task subagents inherit stale parent denies; separately, subagent's own permission block can be bypassed based on caller identity | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-091 | `task` tool entirely absent from tool list under certain permission configs — total loss of delegation, not partial | Bug | Likely | HIGH | No | No | Unevaluated |
| OC-092 | Cancelling a subagent doesn't kill its spawned child processes — resource/safety risk | Bug | Unverified | HIGH | No | No | Unevaluated |
| OC-093 | OpenCode Go advertises 1M context for glm-5.2 but routes to 262K backends; overflow misclassified, session bricked | Bug | Unverified | HIGH | No | No | Unevaluated |
| OC-094 | Subagent (Task-tool) sessions never granted execution permission for MCP tools — model sees tools, every call denied | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| OC-095 | Parallel tool-call turn finalizer races a slow MCP tool — turn never completes, slow tool stuck at running forever | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| OC-097 | Plugin agent manifest `tools: {question: true}` doesn't override hardcoded default-deny — tool silently unavailable | Bug | Unverified | MEDIUM | Yes | No | Unevaluated |
| OC-098 | Truncation is undocumented/unconfigurable, gives misleading recovery instructions for disabled tools, inconsistent hook ordering | Bug | Likely | MEDIUM | Partial | No | Unevaluated |
| OC-099 | `session.idle` plugin handler not awaited on `opencode run` exit; work after its first `await` (incl. SDK calls) is lost | Limitation | Confirmed | HIGH | Yes | Yes | Mitigated |
| OC-041 | Write tool fails silently on large files (~1000+ lines) | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| OC-042 | Write tool has no guard against empty content, silently overwrites existing files with nothing | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-043 | Tool output truncation drops the most important content (background_output synthesis, read offset, grep head_limit) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| OC-044 | Bash tool hangs indefinitely on commands that spawn background/daemon child processes | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-045 | Tool-output encode failure in one concurrent tool call marks unrelated sibling tool calls as failed | Bug | Confirmed | HIGH | No | No | Unevaluated |
| OC-046 | glob (V2) drops the truncation flag — model can't tell results were limited | Bug | Confirmed | MEDIUM | Partial | No | Unevaluated |
| OC-047 | grep tool silently returns empty results when the search path does not exist | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-048 | Plan mode file-write restriction is prompt-level only — model can bypass it via bash | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| OC-061 | MCP tools connected but not exposed to agent (stale tool-set cache) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| OC-062 | MCP tool errors (`isError: true`) reach model as bare `Error executing tool <name>` | Bug | Likely | HIGH | No | No | Unevaluated |
| OC-063 | MCP tool schemas not sanitized for Anthropic (root anyOf/oneOf/allOf 400s); MCP tools skip `tool.definition` hook | Bug | Confirmed | HIGH | Yes | No | Unevaluated |
| OC-064 | MCP tool-list failures collapse to constant "Failed to get tools" string, underlying error discarded | Bug | Likely | HIGH | No | No | Unevaluated |
| OC-065 | Local MCP servers can silently fail/crash mid-session, no restart mechanism | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| OC-066 | Parallel MCP tool calls cause CPU saturation + event-loop blocking (no yieldNow) | Bug | Likely | MEDIUM | No | No | Unevaluated |
| OC-067 | MCP tool-call can hang session indefinitely, no timeout, `/interrupt` non-functional (deadlock) | Bug | Likely | HIGH | Partial | No | Unevaluated |

## Resolved / Retired Issues (4)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|-------------------|-----------------|
| OC-089 | V2 `subagent` tool doesn't enforce per-target permissions (2.0-only issue; fixed in 2.x) | N/A (not relevant: 2.x-only) | 2026-10-25 |
| OC-087 | `--auto` auto-approval doesn't cascade to subagents - hangs (reused-`task_id` edge case reported unfixed) | v1.18.20 (PR #43675) | 2026-10-25 |
| OC-086 | Plugin-registered agent permissions can't override root default deny (2.0-beta-only report) | N/A (not relevant: 2.x-only) | 2026-10-25 |
| OC-096 | No timeout/idle-watchdog on LLM stream — stalled response hangs session (fix adds only 5-min per-chunk/header timeout, no subagent wall-clock timeout) | v1.18.27 (PR #46890) | 2026-10-25 |

---

## Quick Summary

*Run 20 (2026-10-25) re-verified every entry against v1.19.4 and applied the v1-only scope: findings that exist only in 2.x are not tracked here. OpenCode 2.0 has been out since October and maintainers mass-closed v1 issues as `not_planned` on 2026-10-11 and 2026-10-20, and the stale bot closed others earlier. None of those closures is a fix, and the v1.19.x releases are maintenance only (update channel, Bun, session sharing removed). Almost every entry therefore stays active; "Needs re-verification" now mostly means "closed upstream with no 1.x fix, not retested on 1.19.x". Many fix PRs were closed unmerged.*

**Subagent permissions, delegation and lifecycle — the most dangerous cluster for hub-and-spoke dispatch.** Permission-ask requests are dropped at depth 2 and above (OC-001) and rejecting one ask cancels the whole subagent or all parallel subagents (OC-003, OC-004). Beyond delivery, enforcement itself fails: subagents inherit stale parent denies and, worse, a stale catch-all deny can leave a child with no tools at all (OC-090), subagent sessions are never granted execution permission for MCP tools (OC-094, five failed fix PRs; the 2.x-style config workaround is not verified on 1.x, the one reported 1.x workaround is wrapping the MCP server as a native plugin tool), the `task` tool can be absent entirely under some permission configs (OC-091), per-subagent model configuration is ignored (OC-008), and the Task tool's subagent list ignores session-level overrides (OC-007). A failed foreground subagent returns no `task_id` (OC-005), cancelling a subagent does not kill its child processes (OC-092), a subagent hangs after a bash call that left a detached descendant (OC-006), and child-session events carry no parent lineage (OC-002). Stream stalls now have a 5-minute per-chunk timeout (OC-096, fixed in 1.18.27; no total or subagent-level timeout, keep an external watchdog). `opencode run --auto` now answers subagent permission requests (OC-087, fixed in 1.18.20; a reused-`task_id` edge case may still hang).

**Context compaction is unreliable across several failure modes.** Summaries come back empty on reasoning models (OC-021), the overflow guard misfires or never fires depending on model metadata (OC-025, OC-026, OC-028), headless single-turn sessions thrash on it (OC-022, now a Limitation: a maintainer said v1 compaction gets only critical fixes), compaction is not a barrier and the post-compaction auto-continue has been taken as approval to act on held work (OC-023), and overflow can be misreported as a network error that kills the session (OC-027, OC-093). The MCP tool manifest can drop to zero after compaction (OC-024).

**MCP integration has several independent silent-failure points.** Tools connect but are never exposed (OC-061), real error text is lost (OC-062, OC-064), schemas are not sanitized for Anthropic (OC-063), local servers die without restart (OC-065), parallel calls saturate the event loop (OC-066), a call can hang the session with no timeout (OC-067), and a parallel-call race can drop a slow tool's result (OC-095). Confidence on OC-062 and OC-064 dropped to Likely because their fix PRs were closed unmerged.

**Permission and config evaluator bugs.** Wildcard `"*": deny` intermittently overrides specific allows (OC-085), agent frontmatter `permission` merges additively instead of replacing (OC-082), plugin agents cannot override hardcoded default denies (OC-097), the `permission.ask` hook never fires on 1.x (OC-084; the `permission.asked` event is the only 1.x route), injected prompts switch the session's agent or model (OC-081), `OPENCODE_CONFIG_DIR` overrides rather than extends the global AGENTS.md path (OC-083), and built-in Skills hardcode `/tmp` (OC-088).

**Tool execution.** Writes silently fail on large files or overwrite with empty content (OC-041, OC-042), truncation drops the important content or is undocumented (OC-043, OC-098), bash hangs on commands that spawn daemons (OC-044; six fix PRs all closed unmerged), a failed concurrent tool call can fail its siblings (OC-045), glob and grep give misleading empty or truncated results (OC-046, OC-047), plan mode's write restriction is prompt-level only (OC-048), and `session.idle` plugin handlers are not awaited when `opencode run` exits (OC-099, MOSAIC-verified).

## Recommended Mitigations

- **Treat subagent permission enforcement as unreliable.** Verify with a canary call for sensitive tasks, wrap subagent dispatch in an external watchdog (the stream timeout does not bound total runtime), and expect that a stale deny in a parent's history can strip a child's tools (OC-001, OC-003, OC-004, OC-006, OC-067, OC-090, OC-091, OC-092, OC-096).
- **For headless fan-out, use 1.18.20 or later and still guard `opencode run --auto`.** The main subagent-permission case is fixed, but a reused-`task_id` edge case can hang (OC-087).
- **Start every permission allowlist with a catch-all deny, avoid broad wildcards, and do not depend on `permission.ask` plugin interception** (OC-082, OC-085, OC-097, OC-084). Explicitly set `permission.external_directory` and every permission key, and tell models not to pass `workdir` to bash to avoid the depth-2 ask drop (OC-001).
- **Pass explicit `agent` and `model` when injecting prompts**, and do not rely on per-subagent model configuration being honored (OC-081, OC-008).
- **Do not rely on automatic compaction for long headless sessions.** Checkpoint state externally and cap session length below the honored context limit, not the advertised one (OC-021, OC-022, OC-023, OC-025, OC-026, OC-027, OC-028, OC-093).
- **Verify writes after the fact** with a read-back check, and use chunked write plus edit for large files (OC-041, OC-042).
- **Assume MCP availability and MCP permissions can degrade silently.** If an MCP-dependent subagent produces few or no tool calls or every call fails, check for OC-061, 062, 064, 065, 094 and 095 symptoms; consider wrapping the MCP server as a native plugin tool for subagents (OC-094, OC-061).
- **Avoid bash commands that spawn daemons or background processes with open stdio inside subagents**: redirect output to a file and detach explicitly (OC-044, OC-092).
- **Do not read tracker silence or an upstream "closed" as resolution.** For OpenCode 1.x, a `not_planned` closure means maintainers will not look at it, not that it is fixed; the same applies to the stale-bot closures.

**Needs re-verification on 1.19.x:** nearly every entry; the highest-priority ones are the HIGH-impact entries whose evidence is oldest or whose fix PRs were closed unmerged (OC-001, 003, 004, 006, 021, 026, 044, 048, 091, 092, 093, 094, 097). OC-088 is idle since 2026-09-02 and likely to be stale-closed around 2026-11-01. OC-045 and OC-046 may be 2.x-only defects (their fixes live in V2 code) and are candidates to retire if they cannot be reproduced on 1.x.

**Leads for a v1 discovery pass:** #18108 (a truncated tool call, `finishReason: length`, is treated as normal completion, and a subagent can return an empty "successful" result when reasoning-channel truncation exhausts the output budget; untracked, mass-closed `not_planned`, probably a HIGH entry), #17471 (auto-continue on `length`), #29363 (honour configured `limit.output`), subagent permission siblings (#33334, #33276, #36868, #13715, #33223), the hang cluster (#31235, #13841, #11865, #33028, #38266, #17167), parent-lineage events (#45549, #30043), `agent.compaction.variant` ignored (#41578), bounded timeout for stuck tasks (#46461), the subagent bash hang cluster (#33028, #40468, #38564), and #32656 (the `reserved` 20K cap, possibly fixed between 1.18.21 and 1.18.31).
