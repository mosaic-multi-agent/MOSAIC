# Cross-Harness Comparison — DRAFT

> **⚠ DRAFT — Contains known factual errors. Needs review against individual harness reports before treating as reliable.**

| Field | Value |
|-------|-------|
| **Harnesses Compared** | Claude Code, GHCP-CLI, OpenCode, OpenCode V2, Codex (subagent tier only) |
| **Created** | 2026-09-07 |
| **Updated** | 2026-11-18 — Codex v0.170.0 subagent column added; OpenCode V2 v2.3.2 column added |
| **Source** | Individual `AnalysisReport.md` files in each harness folder (OpenCode V2: `OpenCodeV2/v2.3.2/`); Codex from its `v0.170.0/Subagent/` capture directly ("—" in a Codex cell = not captured) |
| **Purpose** | Side-by-side comparison to inform MOSAIC transformation strategy and identify harness-agnostic vs harness-specific design decisions |

---

## 1. Harness Identity Summary

| Dimension | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|-----------|-------------|----------|----------|-------------|------------------|
| **Default Model** | claude-opus-4-6 | claude-sonnet-5 | openai/gpt-5.6-sol | Per agent file (`openai/gpt-6-sol` for the capturer) | Role-pinned per agent (`gpt-6-sol` for the capturer) |
| **Shell** | Git Bash (POSIX sh) | PowerShell 5.x only | PowerShell 5.1 (named `bash`) | PowerShell (tool named `shell`; error format points to 5.1) | User's default shell (PowerShell here); `shell` parameter can override |
| **Primary Identity** | "You are Claude Code, Anthropic's official CLI" | "You are the GitHub Copilot CLI" | "You are an AI assistant accessed via an API" | Same as V1 (API header) | — |
| **Subagent Identity** | "You are a Claude agent" (generic) | No identity statement | Same as primary (generic) | Same as primary (generic) | API header "You are an AI assistant accessed via an API", then "You are Codex, an agent based on GPT-6" |
| **Agent Dispatch Tool** | `Agent` | `task` (custom agent types) | `task` (typed `subagent_type`) | `subagent` (`agent`; **asynchronous**, result arrives later as a `[subagent]` message) | `spawn_agent` (`agent_type` = role from `.codex/agents/*.toml`) |
| **File Editing** | `Edit` (string replace) + `Write` | `edit` (string replace) + `create` | `apply_patch` only (patch format) | `patch` for GPT models (same grammar as V1, never in parallel); `edit`/`write` for others (spike, not captured) | `apply_patch` only (freeform patch) |
| **User Interaction** | `AskUserQuestion` (primary only) | `ask_user` (primary only) | `question` (both tiers) | `question` (both tiers) | `request_user_input`: root thread only (handler rejects subagents), Plan mode by default |

---

## 2. Subagent Capability Parity

This is the single most consequential architectural difference between harnesses. It determines how much MOSAIC subagents can accomplish autonomously.

Codex cells describe the subagent tier only.

| Tool / Capability | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|-------------------|:-----------:|:--------:|:--------:|:-----------:|:----------------:|
| File read | Both | Both | Both | Both | Shell only (no read tool) |
| File write/edit | Both | Both | Both | Both | Yes (`apply_patch`) |
| Shell execution | Both | Both | Both | Both | Yes (`exec_command`) |
| Content search (grep) | Both | **Primary only** | Both | Both | Shell only (`rg` preferred) |
| File search (glob) | Both | **Primary only** | Both | Both | Shell only (`rg --files`) |
| Nested agent spawning | Both (depth-capped) | **Primary only** | Both | Both (a child can spawn its own agent type) | Yes (discouraged unless asked) |
| Web fetch | Configurable | Both | Both | Configurable (denied in capture) | Configurable (`web__run`) |
| Web search | Configurable | **Primary only** | Both | Configurable (on unless denied) | Configurable (`web__run`) |
| User interaction | **Primary only** | **Primary only** | Both | Both (answered from a subagent in capture) | **Primary only** (tool listed, but rejected for subagents) |
| Task/todo tracking | None built-in | **Primary only** | Both | Behind a flag (not captured) | None (goal tools only) |
| Skills | Both | Both | Both | Tool in both; skills list not shown under a wildcard deny | Yes (auto-discovered from `.agents/skills`) |
| Memory/persistence | Both | Both | N/A | Not observed | Not observed |
| Sibling communication | No | **Subagent only** | No | No | Yes (`send_message`, `followup_task`) |

### Parity Rating

| Harness | Subagent Parity | Summary |
|---------|:-----------:|---------|
| **OpenCode** | **Highest** | Near-identical tool sets. Subagents are full peers. |
| **OpenCode V2** | **Highest** | Identical tool sets and descriptions at both tiers; only the dispatch list differs. Deploy permissions are the only limit. |
| **Claude Code** | High | Only `AskUserQuestion` missing from subagents. All core tools present. |
| **GHCP-CLI** | **Lowest** | Subagents lose 7 tools including `grep`, `glob`, `task`. Severely handicapped for autonomous work. |
| **Codex** | Not rated | `spawn_agent` states children get the same tools as the parent, but the primary tier is not captured to confirm. |

**MOSAIC Impact:** GHCP-CLI subagents require explicit PowerShell fallback instructions for code search and file discovery. OpenCode, OpenCode V2 and Claude Code subagents work out of the box. Codex subagents do all reading and searching through the shell. On Windows PowerShell 5, `Get-Content` without `-Encoding UTF8` garbles UTF-8 files (observed in the tool output capture).

---

## 3. File Writing Restrictions

Most harnesses have some form of anti-file-writing guidance — a common friction point for MOSAIC's artifact-based Blackboard pattern. OpenCode (V1 and V2) is the exception.

| Harness | Restriction | Scope | Severity | Escape Clause |
|---------|------------|-------|----------|---------------|
| **Claude Code** | "Do NOT Write report/summary/findings/analysis .md files" | Subagent only | HIGH | None explicit — must override via agent instructions |
| **Claude Code** | "NEVER create documentation files (*.md) or README files" | Both tiers (Write tool) | MEDIUM | "unless explicitly requested by the User" |
| **GHCP-CLI** | "Do NOT write output to files" + no temp files, no redirection | Subagent only | **CRITICAL** | None — hard prohibition in system prompt |
| **OpenCode** | None observed | — | None | N/A |
| **OpenCode V2** | None observed | — | None | N/A |
| **Codex** | No prompt prohibition observed; sandbox (`workspace-write`) allows writes only in cwd and writable roots | Subagent (primary not captured) | LOW | Escalation via `sandbox_permissions: "require_escalated"` |

### Recommended Override Strategy (per harness)

| Harness | Strategy |
|---------|----------|
| **Claude Code** | Add explicit counter-directive in agent instructions: "Writing artifact files IS your task. The general prohibition does not apply." |
| **GHCP-CLI** | Needs empirical testing — if prompt-level only, same override works. If runtime-enforced, must relay artifacts through orchestrator response text. |
| **OpenCode** | No override needed. |
| **OpenCode V2** | No override needed. |
| **Codex** | No override needed; keep run folders inside the writable root. |

---

## 4. System Prompt Architecture

How each harness structures its system prompt, and where MOSAIC agent instructions get injected.

| Aspect | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|--------|-------------|----------|----------|-------------|------------------|
| **Injection point** | Agent `.md` file becomes subagent system prompt via Agent tool | Agent body injected as system prompt section | Plain block after the tool namespaces (`# Instructions` itself stays empty) | Same as V1 | `developer_instructions` from the role `.toml` arrive as a separate developer message, directly after the harness's own developer message (same role, no precedence) |
| **Harness behavioral rules** | Embedded in tool descriptions + system-reminders | Extensive `[code_change_instructions]`, `[tips_and_tricks]`, etc. | Minimal — oververbosity setting + channel declarations | Even less than V1: tool descriptions lost the shell (`&&`, quoting), git/GitHub and dispatch-etiquette guidance | Very extensive and explicit: personality, writing style, autonomy ("bias towards action"), when to ask permission, answer formatting, visualizations, mandatory browsing, citation and word limits, testing policy |
| **Bloat in subagent prompt** | ~80 lines git instructions in Bash tool (both tiers) | Phantom tool references (grep/glob/task guidance for tools subagent doesn't have) | Clean — no phantom references | Clean, half V1's size; minor phantoms when tools are denied (`execute` in the env trailer, a "models tool" in `subagent`) | ~20k tokens of tool text (web with finance/sports/weather, image gen, computer use, goals, plugin install), consumer plugin skills (pets, sites, spreadsheets), recommended plugins; references nonexistent tools (`send_user_message_async`, `request_user_input_async`) |
| **Dynamic context blocks** | MEMORY.md, env, git status, model ID, token budget | Environment, version, model info | `[env]` block, `[available_skills]` | `# Your Model` block, `[env]` (adds a pre-approved temp dir); no `[available_skills]` seen under a wildcard deny | `[environment_context]` user message (cwd, shell, date), skills with roots, permissions/sandbox, collaboration mode, deferred tool namespaces, recommended plugins |
| **Context propagation to subagents** | Full — MEMORY.md + all system-reminders | Partial — some sections stripped | Full — identical prompt structure | Full — identical prompt structure; children start with fresh context unless a `sessionID` is passed | `spawn_agent` passes the **full parent history by default** (`fork_turns: "all"`); `"none"` gives a clean start |

The Codex API header (oververbosity, channels, "Juice") is identical in form to OpenCode's (V1 and V2), which also run OpenAI models. It is added on OpenAI's side, not by the harness. V2's values (oververbosity 1, Juice 10) differ from V1's (3, 16.855) but came with a different model, so they say nothing about the harness.

### Prompt Cleanliness Ranking

1. **OpenCode V2** — Least harness text of any harness; tool descriptions are one paragraph each
2. **OpenCode** — Minimal harness instructions, clean injection point, no phantom references
3. **Claude Code** — Some bloat (git instructions) but no contradictions in tool references
4. **GHCP-CLI** — Phantom tool references in subagent prompt create model confusion

Codex not ranked (subagent tier only). On the subagent evidence it would rank last: the most behavioral rules of any harness, delivered in the same role as MOSAIC agent text, plus phantom tool references.

---

## 5. Shell & Platform Considerations

| Aspect | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|--------|-------------|----------|----------|-------------|------------------|
| **Actual shell** | Git Bash (POSIX) | PowerShell 5.x | PowerShell 5.1 | PowerShell (5.1 likely, from the error format) | User's default (PowerShell here) |
| **Tool name** | `Bash` | `powershell` | `bash` (misleading) | `shell` | `exec_command`, called from JavaScript inside `exec` |
| **`&&` chaining** | Works | Forbidden | Doesn't work | Not tested; **no warning in the prompt** (V1 had one) | Not tested; harness discourages `echo` separators |
| **Path separators** | Forward slash | Backslash | Backslash | Backslash (relative paths accepted by tools) | Backslash |
| **Python command** | `py` (per workspace) | `py` (per workspace) | `py` (per workspace) | `py` (per workspace) | `py` (per workspace) |
| **CWD persistence** | Resets between calls (subagent) | Persists | Resets between calls (implied) | Per call via `workdir` | Per call via `workdir`, defaults to turn cwd |
| **POSIX utilities** | Available (grep, find, etc.) | Not available | Not available | Not tested | `rg` expected; others not tested |
| **Long-running commands** | `run_in_background` | Not compared | Timeout only | Move to background after timeout; `background` flag; completion is notified | Not compared |

**Transformation Note:** Generic MOSAIC templates must not include shell-specific syntax. Harness transformation must convert:
- POSIX `&&` → PowerShell `; if ($?) { }` (GHCP-CLI, OpenCode, OpenCode V2)
- `grep`/`find` CLI usage → `Select-String`/`Get-ChildItem` (GHCP-CLI, OpenCode, OpenCode V2)
- Relative paths → Absolute paths (Claude Code subagents, OpenCode; not needed for OpenCode V2)

---

## 6. Multi-Model & Dispatch Capabilities

| Aspect | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|--------|-------------|----------|----------|-------------|------------------|
| **Dispatch mode** | Not compared | Not compared | Blocking; `task` returns the result | **Always background**: returns a `sessionID` at once; result arrives as a `[subagent]` message each time the child goes idle; "do not sleep or poll" | Not compared |
| **Model selection** | Fixed (primary model) | 18 models available per-dispatch | Configurable per agent type | Per agent file, or per dispatch (`model` = `provider/model#variant`), the latter only "if the user explicitly asks" | Per role (`model` in `.toml`, fixed for that role) or per dispatch (`model`), the latter only with `fork_turns: "none"` |
| **Reasoning effort tiers** | Not exposed | low/medium/high/xhigh/max | Not observed | Only as a model `#variant` | low/medium/high/xhigh/max/ultra |
| **Context tier override** | Not exposed | `long_context` available | Not observed | Not observed | Not observed |
| **Agent type registry** | `.claude/agents/*.md` files | Custom agent types (27 observed) | `subagent_type` param from agent directory | `agent` param; built-in `explore`/`general` plus `.opencode/agents` files with `mode` subagent or all | `.codex/agents/*.toml` roles, listed in `spawn_agent` `agent_type` |
| **Session continuity** | Not exposed | Not observed | `task_id` for resuming sessions | `sessionID`; a follow-up to a running child joins its current run | Agents stay alive; `followup_task` / `send_message` by task name |
| **Concurrent subagent cap** | 20 (configurable) | Not documented | Not documented | Not documented | 4 threads including the root (multi-agent v2 default; configurable globally) |
| **Nesting depth cap** | 3 (configurable to 5) | 1 (no `task` for subagents) | No cap observed | No cap observed | None found for multi-agent v2 (`agents.max_depth` applies to v1 only) |
| **Session spawn cap** | 200 | Not documented | Not documented | Not documented | Not documented |

**MOSAIC Impact:** GHCP-CLI's multi-model support is the richest — the orchestrator could route different subagents to different models based on task complexity. Claude Code and OpenCode tie agents to the session's model. OpenCode V2 can pick a model per dispatch, but its tool text forbids that unless the user asks. V2's asynchronous dispatch changes the orchestrator's loop: a dispatch returns at once, and the orchestrator routes when the `[subagent]` completion message arrives. Codex pins models per role, and full-history forks ignore model and effort overrides. A Codex orchestrator must dispatch with `fork_turns: "none"` to keep subagent context isolated.

---

## 7. Unique Harness Features

Features that exist in only one harness and could benefit (or complicate) MOSAIC.

### Claude Code Only
| Feature | MOSAIC Relevance |
|---------|-----------------|
| **Configurable nesting depth/concurrency** | Safety net for hub-and-spoke — prevents runaway spawning |
| **Token budget visibility** | Orchestrator can make context-aware decisions about when to wrap up |
| **User memory propagation (MEMORY.md)** | Free persistence layer — project notes auto-propagate to all tiers |
| **Scratchpad directory** | Session-specific temp space for intermediate files |

### GHCP-CLI Only
| Feature | MOSAIC Relevance |
|---------|-----------------|
| **Sibling communication** | Subagents can message each other — partial workaround for hub-and-spoke, but conflicts with MOSAIC's routing model |
| **SQL session database** | Built-in SQLite with todos/deps tables — alternative state tracking mechanism |
| **Per-dispatch model selection** | Route complex tasks to stronger models, simple tasks to faster/cheaper ones |
| **Persistent memory (store/vote)** | Cross-session memory with voting — could supplement artifact persistence |

### OpenCode Only
| Feature | MOSAIC Relevance |
|---------|-----------------|
| **Channel system** | Output routing (analysis/commentary/final/summary) — could map to MOSAIC's structured responses |
| **Todowrite** | Native task tracking UI — complementary to Orchestration.md or potential alternative |
| **Session continuity (task_id)** | Multi-turn subagent interactions without re-establishing context |
| **Oververbosity control** | Default conciseness (3/10) aligns with MOSAIC's structured output preference |
| **Dynamic skill loading** | Skills injected mid-conversation on demand |

### OpenCode V2 Only
| Feature | MOSAIC Relevance |
|---------|-----------------|
| **Asynchronous dispatch with completion messages** | Every `subagent` call runs in the background; the parent gets a `[subagent]` message each time the child goes idle. Parallel dispatch is natural, but the orchestrator must route on the message, not the tool result |
| **Follow-up joins a running child** | Calling `subagent` with a running child's `sessionID` adds to its current run; watch item in the V2 orchestrator injection |
| **Worktree tools (`opencode_worktree_remove`, `execute` → `session_move`)** | Not used by MOSAIC; `opencode_worktree_remove` survived a wildcard deny and can force-delete files |
| **Pre-approved temp directory in `[env]`** | Scratch space for intermediate files, like Claude Code's scratchpad |

### Codex Only (subagent tier)
| Feature | MOSAIC Relevance |
|---------|-----------------|
| **Code mode (`exec`)** | All tools are called from JavaScript; independent calls can be batched with `Promise.allSettled`; `store`/`load` keep values between calls. Collaboration tools are outside `exec` |
| **History inheritance (`fork_turns`)** | Defaults to full parent history — breaks context isolation unless every dispatch sets `"none"` |
| **Collaboration modes (Default / Plan)** | Asking the user is only possible in Plan mode; Default mode tells the model to assume instead of asking |
| **Goals (`create_goal` / `update_goal`)** | Harness-level objective tracking with token budgets; overlaps with Orchestration.md and should stay unused |
| **Sandbox escalation (`prefix_rule`)** | Per-command approval with persistable prefixes; relevant to unattended Runner sessions |

---

## 8. MOSAIC Compatibility Scorecard

Rating each harness on key MOSAIC architectural requirements. Codex ratings are provisional (subagent tier only).

| Requirement | Claude Code | GHCP-CLI | OpenCode | OpenCode V2 | Codex (subagent) |
|-------------|:-----------:|:--------:|:--------:|:-----------:|:----------------:|
| **Hub-and-spoke dispatch** | Good | Good | Good | Good (asynchronous: route on `[subagent]` messages) | Good, only with `fork_turns: "none"` |
| **Subagent autonomy** | Strong | Weak | **Strongest** | **Strongest** | Strong |
| **Artifact file writing** | Friction (override needed) | **Blocked** (needs testing) | Clean | Clean | Clean |
| **Code search from subagents** | Strong | **Weak** (PS fallback) | Strong | Strong | Good (`rg` via shell) |
| **HITL at orchestrator** | Strong (AskUserQuestion) | Strong (ask_user) | Strong (question) | Strong (question) | — |
| **HITL at subagent** | Weak (relay needed) | Weak (relay needed) | **Strong** (question available) | **Strong** (question available) | Weak (native tool root-only; relay or MCP tool needed) |
| **Skills injection** | Strong | Strong | Strong | Unverified (no skills list under a wildcard deny) | Strong |
| **Behavioral override surface** | High | Moderate | **Highest** | **Highest** | **Low** (dense explicit rules in the same role as agent text) |
| **Prompt cleanliness** | Moderate | Low | **High** | **Highest** | Low |
| **Shell compatibility** | **Best** (POSIX) | Weak (PS-only) | Weak (PS masquerading as bash) | Weak (PowerShell, honestly named, but no `&&` warning) | Weak (PowerShell, PS5 encoding pitfalls) |

### Overall MOSAIC Fit

| Rank | Harness | Verdict |
|:----:|---------|---------|
| 1 | **OpenCode V2** | Same structural fit as V1 with even less harness text. Main friction: asynchronous dispatch (the orchestrator routes on completion messages), PowerShell without a `&&` warning, and skills visibility under deny-by-default permissions. |
| 2 | **OpenCode** | Best structural fit among the older harnesses. Full subagent parity, no file-write restrictions, clean injection point, minimal harness interference. Main friction: patch-only editing and PowerShell shell. |
| 3 | **Claude Code** | Strong fit with known workarounds. Rich tool surface at both tiers, good safety nets. Main friction: artifact writing prohibition requires explicit override. |
| 4 | **GHCP-CLI** | Viable with significant workarounds. Rich model selection, but subagent tool poverty and file-write ban are serious obstacles. Best for orchestration patterns where subagents return results via text rather than files. |
| — | **Codex** | Not ranked until the primary tier is captured. Subagent evidence: capable tools and no file-write friction, but the heaviest behavioral layer of any harness, which works against narrow protocol-bound agents. |

---

## 9. Transformation Strategy Implications

### What Must Be Harness-Specific

These cannot live in generic templates — they must be handled in QuickReference transformation rules or harness injection points:

| Concern | Generic Template | Transformation Target |
|---------|-----------------|----------------------|
| Artifact writing override | Not needed | Claude Code: add counter-directive; GHCP-CLI: add counter-directive + test; OpenCode, OpenCode V2, Codex: none |
| Shell syntax in examples | Use pseudocode or POSIX | GHCP-CLI/OpenCode/OpenCode V2/Codex: convert to PowerShell (V2's prompt no longer warns about `&&`) |
| Code search fallback | Reference grep/glob generically | GHCP-CLI: add PowerShell Select-String/Get-ChildItem instructions; Codex: `rg` through the shell |
| HITL relay pattern | Standard HITL checkpoint | Claude Code/GHCP-CLI: relay through orchestrator; OpenCode, OpenCode V2: direct; Codex: relay or MCP tool, against the harness's "don't ask" defaults |
| Agent tool prohibition | "Do not spawn child agents" | All: include this; Claude Code has harness safety net; GHCP-CLI: moot (no tool); OpenCode, Codex: instruction-only; OpenCode V2: `subagent: deny` in the deploy's permissions |
| Path format | Use forward slashes | GHCP-CLI/OpenCode/OpenCode V2/Codex: note backslash convention |
| Python command | `python` | All (this workspace): `py` |
| Dispatch context isolation | Not applicable | Codex: orchestrator always sets `fork_turns: "none"` and names the role |
| Asynchronous dispatch | Not applicable | OpenCode V2: the orchestrator ends its turn after dispatching and routes when the `[subagent]` message arrives; it must not re-dispatch a running child's `sessionID` |
| Tool scoping | Not applicable | OpenCode V2: tools are on unless denied, and a wildcard deny does not remove `skill` or `opencode_worktree_remove`; agents with skills need an explicit `skill` allow (to verify) |
| Harness behavioral defaults | Not applicable | Codex: name and override each conflicting rule (asking the user, answer formatting vs protocol JSON, testing policy, mandatory browsing, "persist until done"); prefer turning sections off in config over overriding them in text. Most switches are global or profile only; a role file can only set `personality = "none"`, a few `features.*` = false (shell_tool, apps, plugins, memories, request_permissions_tool) and skills restrictions |

### What Can Stay Generic

These work identically across all harnesses compared:

- Communication protocol (status codes, JSON format) — Codex unverified: its answer-formatting rules may pull against JSON output
- Agent identity and role assignment
- Capability and constraint definitions
- Error handling patterns
- Skills references (all harnesses support skills)
- Artifact naming conventions
- Workflow phase structure

---

## 10. Open Questions

| # | Question | Why It Matters |
|---|----------|---------------|
| 1 | Is GHCP-CLI's file-write ban runtime-enforced or prompt-only? | Determines if MOSAIC can work on GHCP-CLI at all with artifact-based patterns |
| 2 | Does OpenCode's `apply_patch` reliably handle large new file creation? | MOSAIC subagents create multi-hundred-line artifacts; patch format errors could corrupt them |
| 3 | Can GHCP-CLI's sibling communication be leveraged without breaking hub-and-spoke? | Could enable faster coordination for specific workflow patterns |
| 4 | How does OpenCode's channel system interact with MOSAIC JSON protocol output? | Need to confirm JSON responses route correctly through channels |
| 5 | What is the practical token cost of GHCP-CLI's phantom tool references? | Subagents waste tokens processing guidance for tools they don't have |
| 6 | What does the Codex primary tier look like (dispatch, HITL, propagation)? | Every Codex primary cell above is empty; no ranking is possible without it |
| 7 | Which Codex config keys remove prompt sections? **Partly answered** (library research 2026-11-18; source reading, not runtime-tested). Global/profile keys: `web_search`, `features.image_generation`, `features.computer_use`, `features.goals`, `features.code_mode.*`, `features.multi_agent*`, `features.apps`, `features.plugins`, `skills.bundled.enabled`, `include_apps_instructions`, `include_permissions_instructions`, `include_collaboration_mode_instructions`, `include_environment_context`, `model_instructions_file`. Role files apply only an allow-list: `developer_instructions`, `model`, `model_reasoning_effort`, `model_reasoning_summary`, `model_verbosity`, `personality`, `service_tier`, a few `features.*` = false, and skills restrictions. Still open: whether each feature switch also removes its prompt text | Removing sections beats overriding them in text, but global switches change the user's whole Codex install |
| 8 | Does a Codex subagent return protocol JSON reliably despite the harness's markdown answer rules? | The communication protocol must survive the harness's formatting layer |
| 9 | Is a second Codex base prompt ("based on GPT-5") also in context, beside the captured GPT-6 one? Research: `base_instructions` is the API `instructions` field, chosen as config override, then stored history (resume/fork), then the model template; the GPT-5 metadata is probably stale or inherited (inference). Not confirmed from context | Decides whether the capture is missing an ~18k block |
| 10 | Do `sandbox_mode` and `web_search` in our Codex role files do anything? Research (inferred from source): no. Roles inherit sandbox, approval policy, provider and MCP servers from the parent session | `mosaic-deploy` may be writing keys Codex ignores |
| 11 | Does OpenCode V2 show `[available_skills]` when the agent's permissions allow `skill`? | Agents with `required_skills` may otherwise see no skills at all |
| 12 | Is OpenCode V2's shell Windows PowerShell 5.1 or PowerShell 7? | Decides whether `&&` works; the prompt no longer says |
| 13 | Can `opencode_worktree_remove` be denied by name in V2 permissions? | It survived `"*": deny`, and its `force` flag deletes files |