# VS Code GitHub Copilot — Issue Index

> Last updated: 2026-10-25 (run 20)
> Latest platform version: 1.143.1 (released 2026-10-22)

## Active Issues (22)

| ID | Title | Type | Confidence | Impact | Workaround | Reproduced | Response |
|----|-------|------|------------|--------|------------|------------|----------|
| VC-002 | Subagent client-tool call can be mis-routed to parent chat after restart | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| VC-003 | Subagent tool call can hang forever, leaving its turn (and parent's) open indefinitely | Bug | Likely | HIGH | Partial | No | Unevaluated |
| VC-004 | Forked Skill subagents have no correlation handle — invisible to Agent Host UI | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| VC-005 | "Agents" window subagent spawn: main chat ignores custom agent's tool restrictions | Bug | Unverified | MEDIUM | No | No | Unevaluated |
| VC-008 | Terminal auto-approval fails for commands containing a bare `--` token (PowerShell) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| VC-009 | Agent mode hallucinates completed task and derails at very high token counts | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| VC-010 | `runSubagent` skips allow-list/invocation-gating validation when agentName is omitted | Bug | Unverified | HIGH | No | No | Unevaluated |
| VC-011 | Background subagent output/completion signals silently dropped once parent's turn ends | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| VC-012 | Client-tool execution races cause double invocation, empty args, or dropped results (hang) | Bug | Likely | HIGH | Partial | No | Unevaluated |
| VC-015 | Copilot edit tools report "Updated" success even when the underlying file save fails | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| VC-016 | MCP tools silently rejected as "disabled by the user" due to a discovery-timing race (ask mode; maintainer declined fix) | Bug | Confirmed | LOW | Partial | No | Unevaluated |
| VC-017 | MCP client discards structured result payload when a long-running "task" fails | Bug | Unverified | HIGH | Partial | No | Unevaluated |
| VC-018 | tree-sitter PowerShell parser fails on `./path` commands (not covered by `--` fix) so terminal auto-approve never applies | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| VC-019 | Nested subagent inherits root agent's tools, not spawner's or its declared tools (needs re-verification) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| VC-020 | Subagents told to call tool_search first though request omits it; subagent can stop instead of using available tool (fix PR #340701 open) | Bug | Confirmed | HIGH | Partial | No | Unevaluated |
| VC-021 | Custom agent run as subagent lacks search/web tool groups declared in frontmatter | Bug | Unverified | HIGH | No | No | Unevaluated |
| VC-022 | todo/askQuestions tools force-disabled for subagents despite frontmatter opt-in | Limitation | Likely | MEDIUM | Partial | No | Unevaluated |
| VC-023 | Custom agent frontmatter model fails to resolve as subagent (model unavailable / invalid_model); explicit override works | Bug | Likely | MEDIUM | Yes | No | Unevaluated |
| VC-024 | Agents window: subagents started in the plan-approval turn get none of the built-in tools (1.143.0 regression) | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| VC-025 | Agent Host drops custom-agent frontmatter keys (e.g. include-custom-instructions); custom subagents never get AGENTS.md | Bug | Unverified | MEDIUM | Partial | No | Unevaluated |
| VC-026 | Agent terminal tool hangs forever after command ends when user's prompt hooks (starship etc.) defeat shell-integration detection | Quirk | Likely | MEDIUM | Yes | No | Unevaluated |
| VC-027 | Agent Host loses BYOK context limits (0/0): context bar hidden, silent auto-compaction at ~100K regardless of configured window | Bug | Confirmed | MEDIUM | No | No | Unevaluated |

## Resolved / Retired Issues (1)

> Resolved entries older than 3 months are automatically removed.

| ID | Title | Fixed In / Status | Resolution Date |
|----|-------|-------------------|-----------------|
| VC-001 | Nested runSubagent calls rejected 3 levels deep even when allow-listed | 1.140.0 (PR #337611) | 2026-09-24 |

---

## Quick Summary

*Run 20 (2026-10-25) re-verified all 13 earlier entries against VS Code 1.143.1 and added 10 new ones (VC-018..027). VC-001 (nested `runSubagent` rejected 3 levels deep) is fixed in 1.140.0. Most of the delegation cluster was filed by one contributor whose fix PRs were closed unmerged on 2026-10-03, so "fix PR open/closed" tells little about whether a fix is coming.*

**Subagent tool inventory and scoping — the dominant theme.** What a subagent is offered does not match what its definition declares or what its spawner has. A nested subagent inherits the root agent's tools, not the spawner's or its own (VC-019, needs re-verification); subagents are told to call `tool_search` first although their request omits it, so they can stop instead of using an available tool (VC-020, fix PR #340701 open); a custom agent run as a subagent lacks the `search` and `web` groups declared in its frontmatter (VC-021); `todo` and `askQuestions` are force-disabled for subagents despite an opt-in (VC-022, Limitation); a frontmatter `model` fails to resolve for BYOK/custom-provider models while an explicit override works (VC-023); the Agent Host drops frontmatter keys such as `include-custom-instructions`, so custom subagents never get AGENTS.md (VC-025); and in the Agents window, subagents started in the plan-approval turn get none of the built-in tools (VC-024, 1.143.0 regression, same defect as Copilot CLI GC-033). `runSubagent` skips allow-list and invocation-gating checks when `agentName` is omitted (VC-010; since the 1.140.0 fix a nested subagent that omits `agentName` clones the calling subagent), and the main chat ignores a custom agent's tool restrictions in the Agents window (VC-005). The `ToInvestigate.md` item TI-001 (subagents offered the parent's tool descriptions without its permissions) belongs to this family.

**Delegation lifecycle and completion signalling.** A subagent's tool call can hang forever with no timeout (VC-003, Likely; its only evidence is the contributor's own scripted repro and the fix PR was closed unmerged). Background/parallel subagent output and completion signals are silently dropped once the parent's turn ends (VC-011, Confirmed, now reproduced by two independent third parties; Reload Window does not recover the session, a full quit does, and the on-disk transcript is complete; a new fix PR #336753 is open). Client-tool execution races cause double invocation, empty arguments or dropped results (VC-012, Likely). Subagent client-tool calls can be mis-routed to the parent chat after a restart (VC-002), and forked Skill subagents have no correlation handle and are invisible in the UI (VC-004).

**Terminal and tool execution.** Terminal auto-approve fails for PowerShell commands containing a bare `--` (VC-008; PR #335465 is milestoned 1.144.0 and a maintainer wants it in next week's release) and, separately, for any `./path` command because the tree-sitter parser fails (VC-018, about 8% of realistic agent commands; not covered by the `--` fix). The agent terminal tool can hang forever when a user's prompt hooks (starship and similar) defeat shell-integration detection (VC-026; a shell-config workaround is confirmed). Edit tools can report "Updated" before the save resolves (VC-015). MCP: tools can be rejected as "disabled by the user" by a discovery race in ask mode (VC-016, maintainer declined the fix because the code path is going away, candidate to retire) and a failed long-running MCP task's structured payload is discarded (VC-017).

**Context and stability.** In very long Agent-mode sessions the model can hallucinate a completed task and act on it (VC-009, single reporter, GPT-5.6 only). The Agent Host loses BYOK custom-endpoint context limits (0/0), hiding the context bar and silently auto-compacting at about 100K tokens (VC-027; PR #336798 merged in 1.139.0 but a 2026-10-16 comment says limits still arrive as 0/0).

## Recommended Mitigations

- **Do not rely on a subagent's tool list or `agents:` allow-list being enforced as declared.** Subagents may be offered tools they cannot use (TI-001), miss tools their frontmatter lists, or inherit the root agent's tools (VC-019, VC-020, VC-021, VC-022). Keep delegation to one hop from the primary where possible (a nested clone is created when `agentName` is omitted, VC-010), and treat a subagent that stops without using an available tool as a possible `tool_search` prompt mismatch (VC-020).
- **Do not pick a subagent model through frontmatter alone for BYOK or custom-provider models**; pass an explicit override and verify it took effect (VC-023). Do not rely on AGENTS.md reaching a custom subagent; state needed rules in the delegation prompt (VC-025).
- **Wrap every subagent dispatch in an external wall-clock timeout and do not equate "no completion signal" with failure.** VC-003 is a true hang; VC-011's work is usually recoverable from the on-disk transcript, and only a full VS Code quit (not Reload Window) restores the session. Cross-check before re-dispatching.
- **Treat client-tool calls as potentially non-idempotent under contention** (VC-012) and do not trust the Agent Host UI or transcript as the audit trail for forked Skill subagents; have subagents write findings to a file (VC-004).
- **Design terminal auto-approve rules defensively.** Prefer broad command-prefix rules over ones expecting `--` to match (VC-008) and expect `./path` commands never to be auto-approved on PowerShell (VC-018); keep prompt hooks such as starship out of the agent terminal's shell config (VC-026).
- **Verify file edits landed**, especially on remote or network-mounted workspaces (VC-015), and retry an MCP call that fails with "disabled by the user" on a fresh request (VC-016).
- **Cap session context growth.** Prefer periodic resets or splitting long orchestration across fresh sessions (VC-009), and do not trust the context bar with BYOK endpoints (VC-027).
- **Re-verify before planning around any linked fix PR**; most of this cluster's PRs were closed unmerged and maintainer engagement is mostly limited to VC-008, VC-016 and VC-020.

**Needs re-verification on the current version:** VC-002, 004, 005, 009, 010, 015, 017, 019, 022 (idle 6+ weeks, no maintainer response), VC-003 and VC-012 (evidence rests on closed PRs), VC-023, 024 and 025 (re-check after 1.144.0), VC-027 (fix merged but still reported), and VC-005 and VC-016 as retirement candidates next pass.

**Leads for a future pass:** #334999 (a large `PreToolUse` hook output races with large tool output spilled to `content.txt`), #342794 (Agent Host OOM on about 10 MiB of command output), #332145 (`run_in_terminal` embeds image paths from stdout, renderer OOM), #343184 and #314918 (approval-prompt truncation), and the 1.144.0 release (VC-008 fix, #341705 custom agents ignored in a WSL remote).
