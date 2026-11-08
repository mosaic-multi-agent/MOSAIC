# MOSAIC on Codex CLI -- Feasibility Analysis

**Date:** 2026-09-21
**Author:** Orchestration Architect (mosaic-architect)
**Status:** Investigation complete -- blockers identified

---

## Executive Summary

Running MOSAIC's orchestrated multi-agent workflows on Codex CLI is **not currently feasible** due to three structural blockers in how Codex handles subagent dispatch. The deployment tool successfully produces `.codex/agents/*.toml` files, but the Codex runtime cannot consume them the way MOSAIC requires. Individual MOSAIC agents *can* run standalone on Codex, and there are degraded-mode workarounds worth considering depending on the use case.

---

## Blocker 1: No Programmatic Subagent Dispatch by Name

**Severity:** Critical -- breaks the core orchestration loop

MOSAIC's orchestrator dispatches subagents by name through the harness's native agent tool. On Claude Code, this is the `Agent` tool with `subagent_type` matching the agent's key. On Codex, the equivalent is `spawn_agent`, but:

- `spawn_agent` accepts only `agent_type` plus explicit `prompt`/`model` overrides
- There is **no parameter** to reference a custom agent defined in `.codex/agents/*.toml` by name
- This is a known, documented limitation: GitHub issues [#15250](https://github.com/openai/codex/issues/15250), [#14579](https://github.com/openai/codex/issues/14579), and [#26363](https://github.com/openai/codex/issues/26363) all track this gap
- Issue #26363 (June 2026) confirms custom agents under `.codex/agents/` are "no longer selectable" from the `spawn_agent` tool surface in recent CLI versions

**Impact:** The orchestrator cannot dispatch `codebase-research`, `planner-tdd-soft`, `implementation-tdd`, or any other MOSAIC subagent. The entire workflow loop is broken at the dispatch step.

**Workaround (poor):** The orchestrator could read each `.toml` file, extract `developer_instructions`, and inject it as a prompt override when spawning a generic worker. This is:
- Extremely token-expensive (each subagent's instructions are 5-15K tokens, injected into every spawn call)
- Fragile (model/sandbox_mode/reasoning_effort from the TOML are not honored)
- Untested at MOSAIC's instruction size (subagent prompts are 3,000-8,000 words)

### Evidence

From [Codex Subagent Gotchas](https://codex.danielvaughan.com/2026/03/29/subagent-gotchas-known-issues/):
> "The `spawn_agent` tool accepts: `agent_type` (required), `prompt` (optional override), `model` (optional override). Notable gaps: No file reference parameter, no named agent lookup capability."

From [GitHub #15250](https://github.com/openai/codex/issues/15250):
> "The exposed `spawn_agent` interface only accepts a generic `agent_type` plus explicit prompt/model overrides. There is no parameter to say 'spawn custom agent `review-bugs`'"

---

## Blocker 2: No `--append-system-prompt` Equivalent

**Severity:** Critical -- prevents loading the orchestrator as the primary session

MOSAIC's orchestrator needs to run *as* the primary Codex session with its full system prompt loaded. Claude Code supports `--append-system-prompt` which injects the orchestrator's instructions into the session. Codex has no equivalent:

- GitHub issue [#11117](https://github.com/openai/codex/issues/11117) requested this feature -- **closed without implementation**
- The `-c instructions="..."` flag exists but is limited to short overrides, not 30K+ token orchestrator prompts
- `AGENTS.md` / `AGENTS.override.md` are *additive* project docs, not system prompt replacements

**Partial alternative -- `model_instructions_file`:** This config.toml key **replaces** (not appends) Codex's built-in system prompt with a custom file. This means:

- You lose all of Codex's built-in tool usage instructions, sandbox awareness, and safety alignment
- The orchestrator prompt was never designed to be a *complete* system prompt -- it assumes the harness provides tool mechanics
- The `spawn_agent`, file I/O, and shell tools may behave unpredictably without their built-in instructions

**Partial alternative -- Profiles:** `codex --profile orchestrator` can overlay model/instructions settings. But the same `model_instructions_file` limitation applies -- it's a replacement, not an injection.

---

## Blocker 3: Subagent Identity and Configuration Not Honored

**Severity:** Critical -- even if dispatch worked, agents wouldn't behave correctly

Even when custom agents are defined in `.codex/agents/*.toml`, spawned subagents do not reliably inherit the TOML configuration:

- Model, reasoning effort, and sandbox_mode from the TOML file are **not applied** -- the parent session's settings are inherited instead (GitHub [#26868](https://github.com/openai/codex/issues/26868))
- Agent identity (nickname, agent_path) fields are null in spawned instances
- This is marked as a **systemic bug**, closed as duplicate of #26363

**Impact:** Even with the prompt-injection workaround for Blocker 1, subagents would run on the wrong model (e.g., `gpt-5.6-sol` instead of `gpt-5.6-terra`) and with wrong sandbox permissions.

---

## Additional Limitations (Non-Blocking but Significant)

### 4. Concurrency Ceiling

Codex limits concurrent subagent threads to 6 (configurable via `max_concurrent_threads_per_session`). MOSAIC's kb-generation workflow dispatches parallel stages that could exceed this. The brownfield-tdd workflow is sequential and would not hit this limit.

### 5. Result Return Reliability

[Codex Subagent Gotchas](https://codex.danielvaughan.com/2026/03/29/subagent-gotchas-known-issues/) documents:
- Subagents getting stuck in "awaiting instruction" state (resource leaks)
- v0.118.0+ alpha versions having unresolved handoff failures where subagents complete but fail to return result payloads
- Recommendation to "stick to stable v0.117.0 for production agentic workflows"

MOSAIC's protocol requires reliable JSON responses with status codes. Silent handoff failures would cause the orchestrator to hang or misroute.

### 6. No User Interaction Tools for Subagents

MOSAIC's HITL (Human-in-the-Loop) gates require subagents to present output to the user via interaction tools. Codex subagents run in isolated threads and have no documented mechanism to contact the user mid-execution. The `mcp__contact-user__ask_user_questions` pattern used in Claude Code has no Codex equivalent for spawned subagents.

### 7. Skills Path Difference

Skills deployed to `.agents/skills/` (Codex convention) vs `.claude/skills/` (Claude Code convention). The deploy tool handles this correctly, but the orchestrator's Claude Code harness injection tells it to point subagents at `.claude/skills/`. The Codex harness injection correctly says `.agents/skills/`, but the orchestrator's skill-path guidance in dispatch messages needs to match.

---

## What DOES Work

### Individual Agent Execution

Any single MOSAIC agent can run as the primary Codex session via `AGENTS.override.md` or `model_instructions_file`. This means:

- **`codex` with `AGENTS.override.md`** containing a single agent's instructions: the agent runs as the primary session with full tool access. Useful for standalone research, implementation, or review tasks outside the orchestration loop.
- **`codex --profile <agent-name>`** with per-agent profile configs: switch between agent personas per session.

This is single-agent mode only -- no workflow coordination, no status code routing, no Orchestration.md management.

### Deployment Artifacts

The deploy tool correctly produces `.codex/agents/*.toml` files with:
- Proper TOML structure (name, description, model, sandbox_mode, developer_instructions)
- Harness-specific injections (sandbox_mode constraints, skills path, single-file deployment)
- Version tracking via comments
- Skills deployed to `.agents/skills/`

These files are *ready* for when Codex fixes the subagent dispatch bugs.

### Script-Mode Orchestrator (mosaic-run)

The `orchestrator-script` agent (designed for the `mosaic-run` Runner) is a better architectural fit for Codex than the conversational orchestrator, because:
- Each routing decision is a fresh invocation (no context window degradation)
- The Runner handles mechanical dispatch, not the LLM
- But: the Runner itself (`mosaic-run`) would need to use Codex's `spawn_agent` tool, hitting Blocker 1 again

---

## Runner Mode Analysis (mosaic-run)

**Key insight: Runner mode bypasses all three interactive-session blockers.**

The `mosaic-run` Runner dispatches each subagent as a **separate CLI subprocess**, not as a spawned subagent within a parent session. Each agent IS the primary session for its `codex exec` invocation. This means:

- **Blocker 1 (no named dispatch) -- bypassed.** The Runner doesn't use `spawn_agent` at all. It invokes `codex exec` directly, one subprocess per agent.
- **Blocker 3 (config not inherited) -- bypassed.** Each agent runs as the primary session and loads its own configuration directly. No parent-to-child inheritance needed.
- **Blocker 2 (no append-system-prompt) -- partially remains** but has workable solutions (see below).

### How Existing Harness Adapters Work

The Runner already has harness adapters for Claude Code, OpenCode, and GitHub Copilot CLI. Each adapter maps the same abstract operation (invoke agent X with message Y) onto harness-specific CLI mechanics:

| Harness | Subagent Dispatch | Orchestrator Dispatch |
|---------|------------------|----------------------|
| Claude Code | `claude -p --append-system-prompt-file <agent.md> --output-format json` | `claude --agent <id> -p --output-format json` |
| OpenCode | `opencode run --agent <id> --format json "<prompt>"` | Same as subagent |
| GitHub Copilot CLI | `ghcp-cli run --agent <id> --format json "<prompt>"` | Same as subagent |

The prompt (task invocation JSON) goes to stdin (Claude Code) or as a positional arg (OpenCode). The response comes back as JSON on stdout.

### What a Codex Adapter Would Need

A Codex harness adapter would follow the same pattern. The key question is how to inject the agent's instructions. Three approaches were evaluated:

#### Option 1: `model_instructions_file` per invocation (Rejected)

```
codex exec -c model_instructions_file=".codex/agent-instructions/codebase-research.md" \
           -c model="gpt-5.6-terra" \
           --output-format json \
           "<task JSON>"
```

The `-c` flag overrides any config.toml key for one run. `model_instructions_file` replaces Codex's built-in system prompt with the agent's MOSAIC instructions.

**Rejected -- effectively a blocker.** Replaces Codex's built-in prompt entirely -- the agent loses Codex's native tool usage instructions (how to call shell, how file I/O works, sandbox rules). MOSAIC agent prompts were never designed to be a complete system prompt; they assume the harness provides tool mechanics. Without built-in instructions, tool usage will likely break or degrade unpredictably.

#### Option 2: System prompt prepended to message (Recommended -- OpenCode pattern)

```
codex exec -c model="gpt-5.6-terra" \
           --output-format json \
           "<agent system prompt>\n<task JSON>"
```

This is how the OpenCode adapter already works -- it prepends the agent's system prompt to the positional message argument. No config override needed beyond model selection.

**Pros:**
- **Preserves Codex's built-in instructions** (system prompt stays intact, tool usage works normally)
- Agent instructions ride alongside the task in the user message
- Simplest implementation -- matches an existing, proven adapter pattern
- Model can still be overridden per agent via `-c model="..."`

**Dilution concern is a non-issue in Runner mode.** The GitHub #11117 feature request for `--append-system-prompt` was motivated by instructions getting diluted over long multi-turn conversations. But in Runner mode, each `codex exec` invocation is a **single-turn, fire-and-forget subprocess** -- the agent receives one message, does the work, and returns. There is no multi-turn conversation where instructions drift out of the attention window. The exact problem that motivated the feature request does not apply here.

**Cons (minor):**
- Agent instructions are in the user message, not the system prompt -- theoretically lower priority, but irrelevant for single-turn invocations
- Combined message is large (5-15K agent prompt + task JSON), but this is normal for single-turn agent invocations

#### Option 3: AGENTS.override.md swap per invocation

Before each `codex exec`:
1. Write the agent's instructions to `AGENTS.override.md`
2. Run `codex exec --output-format json "<task JSON>"`
3. Restore/remove `AGENTS.override.md`

**Pros:**
- Preserves Codex's built-in instructions AND adds MOSAIC instructions
- Instructions are in project-docs priority (higher than user message)

**Cons:**
- File I/O per invocation -- fragile, crash-unsafe
- Breaks concurrent invocations (shared file)
- Messy

### Runner Mode Feasibility Assessment

| Aspect | Assessment | Notes |
|--------|-----------|-------|
| Architecture fit | **Strong** | Runner already separates mechanical dispatch from intelligent routing -- adding a harness adapter is the designed extension point |
| Orchestrator-script dispatch | **Works** | The script-mode orchestrator runs as primary session via `codex exec`, reading Orchestration.md and returning JSON routing instructions |
| Subagent dispatch | **Likely works** | Each subagent runs as primary session via `codex exec` with injected instructions. Option 1 or 2 need testing |
| Response parsing | **Unknown risk** | Codex's `--output-format json` envelope format needs investigation -- it may differ from Claude Code's envelope, requiring a new parser |
| Tool access | **Works** | `sandbox_mode = "workspace-write"` in the per-invocation config gives file read/write and shell access |
| HITL gates | **Works (via MCP)** | Same pattern as Claude Code Runner mode: subagents reach the user through an MCP server (e.g., `contact-user`), not through the CLI's native interactivity. Codex supports `mcp_servers` in config, injectable per invocation via `-c` |
| Infrastructure agents | **Works** | Checkpoint and commit agents run the same way -- each is a `codex exec` subprocess |
| Skills loading | **Works** | Skills at `.agents/skills/` are read by agents manually (already handled by Codex harness injection) |

### What Building a Codex Runner Adapter Requires

1. **Investigate `codex exec --output-format json` envelope format** -- what does the raw stdout look like? Is it an array of events, a single object, or something else? Build an envelope parser.

2. **Spike Option 1 vs Option 2** -- test whether `model_instructions_file` replacement breaks tool usage, and whether message-prepended instructions are reliable for MOSAIC-sized prompts.

3. **Implement `BuildCodexArgs` and `NewCodex` spawner** in `mosaic-common/harness/` -- following the pattern of `spawn_opencode.go`. Estimated ~200-400 lines of Go.

4. **Implement `CodexAdapter`** in `mosaic-run/internal/harness/` -- following the pattern of `opencode.go`. Estimated ~100-200 lines of Go.

5. **Deployment format decision** -- the Runner needs agent instructions as readable files. Options:
   - Extract `developer_instructions` from `.codex/agents/*.toml` at runtime (Runner reads TOML, extracts field)
   - Have the deploy tool also emit `.md` instruction files alongside the TOMLs
   - Use the TOML files directly with a TOML-aware instruction extractor

6. **Wire MCP server for HITL** -- same solution as Claude Code Runner mode. The `contact-user` MCP server provides an out-of-band channel for subagent-to-user interaction regardless of whether the CLI session is interactive. Pass the MCP server config to each `codex exec` invocation via `-c mcp_servers.contact-user="..."`. Codex supports per-agent `mcp_servers` in TOML config, so this should work with `-c` overrides too.

**Estimated effort:** 2-3 days for the adapter + parser + spike. Significantly less than the 3 days already spent on the deploy tool, and this time it would actually produce a working orchestration path.

### Runner Mode vs Interactive Orchestration

| Capability | Interactive (Blocked) | Runner Mode (Viable) |
|-----------|----------------------|---------------------|
| Full workflow execution | No | Yes (all 3 modes) |
| Orchestrator routing | No | Yes (script-mode orchestrator via `codex exec`) |
| Subagent dispatch | No (`spawn_agent` broken) | Yes (each agent = separate `codex exec`) |
| Agent config inheritance | No (bug) | N/A (each agent is primary session) |
| HITL gates | No | Yes (via MCP server, same as Claude Code Runner) |
| Infrastructure agents | No | Yes |
| Cost model | N/A | Best possible (bounded context per invocation, like all Runner modes) |

---

## Possible Paths Forward

### Path A: Build Codex Runner Adapter (Recommended)

Add a Codex harness adapter to `mosaic-run`. This bypasses all three interactive-session blockers because the Runner dispatches each agent as its own `codex exec` subprocess. The architectural pattern already exists for three other harnesses.

**Estimated work:** 2-3 days for spike + adapter + parser.
**Prerequisite:** `mosaic-run` must be buildable (it currently compiles and runs tests; needs the adapter added).
**Outcome:** Full MOSAIC orchestration on Codex in all three Runner modes.

### Path B: Wait for Codex to Fix Subagent Dispatch

The core blockers (#15250, #26363, #26868) are all acknowledged bugs. When Codex ships named agent dispatch with proper TOML config inheritance:
- The deployed `.codex/agents/*.toml` files will work as-is
- The orchestrator needs a Codex-specific harness injection telling it to use `spawn_agent` with agent names instead of Claude Code's `Agent` tool
- The `HarnessInjectionsOrchestrator.md` for Codex needs content about dispatch mechanics (currently empty)

**Estimated work when Codex is ready:** ~2-4 hours to write the orchestrator harness injection content and test.
**Risk:** Unknown timeline. The issues have been open for months.

### Path C: Degraded Single-Agent Mode

Use individual MOSAIC agents as standalone tools on Codex (via profiles or AGENTS.override.md), without orchestration. The user manually sequences their own workflow:

1. `codex --profile codebase-research` for research
2. `codex --profile planner` for planning  
3. `codex --profile test-writer` for test writing
4. etc.

**Pros:** Works today. Each agent brings its full MOSAIC-quality instructions.
**Cons:** No workflow coordination. No status code routing. No Orchestration.md state management. No infrastructure agents (checkpoints, commits). Essentially just "good prompts on Codex."

---

## Comparison: Claude Code vs Codex CLI for MOSAIC

| Capability | Claude Code | Codex CLI | Gap |
|-----------|------------|-----------|-----|
| Load orchestrator as primary session | `--append-system-prompt` | None (replace only via `model_instructions_file`) | **Blocker** |
| Dispatch subagent by name | `Agent` tool with `subagent_type` | `spawn_agent` -- no name reference | **Blocker** |
| Subagent inherits TOML config | N/A (uses .md files) | Bug -- inherits parent config | **Blocker** |
| Subagent returns structured response | Yes (Agent tool result) | Unreliable in recent versions | Significant |
| HITL via user interaction tools | `mcp__contact-user__*` | No documented equivalent for subagents | Significant |
| Concurrent subagents | Unlimited (harness-managed) | Ceiling of 6 | Minor |
| Skills loading | `.claude/skills/` auto-loaded | `.agents/skills/` must be read manually | Minor (handled) |
| File read/write | Full tool suite | Governed by `sandbox_mode` | Compatible |
| Git operations | Bash tool | Shell tool (if enabled) | Compatible |

---

## Recommendations

1. **Do not attempt orchestrated MOSAIC runs on Codex today.** The blockers are in the Codex runtime, not in MOSAIC's design or deployment.

2. **Keep the Codex harness and deployed TOML files.** They're correctly structured and will work when Codex ships the fixes. The deployment investment is not wasted.

3. **Consider Path D (single-agent mode) for immediate value.** Individual MOSAIC agents are high-quality standalone tools. Running `codex` with an agent's instructions in AGENTS.override.md gives you MOSAIC-grade research, planning, or implementation without orchestration.

4. **Watch GitHub issues [#26363](https://github.com/openai/codex/issues/26363) and [#15250](https://github.com/openai/codex/issues/15250)** for named subagent dispatch fixes. When these ship, the orchestrator harness injection needs ~2-4 hours of work to enable full MOSAIC.

5. **Invest in `mosaic-run` (Path B) if Codex is a priority.** The external Runner design already exists for `orchestrator-script`. Building it would bypass all Codex subagent bugs by treating each agent invocation as a separate `codex exec` subprocess.

---

## Sources

- [Codex Subagents Documentation](https://developers.openai.com/codex/subagents)
- [Codex Configuration Reference](https://developers.openai.com/codex/config-reference)
- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference)
- [Codex Advanced Configuration](https://developers.openai.com/codex/config-advanced)
- [GitHub #15250 -- Custom subagents not accessible from tool-backed sessions](https://github.com/openai/codex/issues/15250)
- [GitHub #26363 -- Custom agents no longer selectable in CLI v0.137.0](https://github.com/openai/codex/issues/26363)
- [GitHub #26868 -- Custom subagents TOML not applied on spawn](https://github.com/openai/codex/issues/26868)
- [GitHub #11117 -- Request for --append-system-prompt](https://github.com/openai/codex/issues/11117)
- [Codex Subagent Gotchas and Workarounds](https://codex.danielvaughan.com/2026/03/29/subagent-gotchas-known-issues/)
- [Custom Agent Definitions with TOML](https://codex.danielvaughan.com/2026/04/27/codex-cli-custom-agent-definitions-toml-specialised-subagents/)
- [Simon Willison -- Use subagents and custom agents in Codex](https://simonwillison.net/2026/Mar/16/codex-subagents/)
