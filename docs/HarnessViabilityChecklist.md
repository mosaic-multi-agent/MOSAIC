# Harness Viability Checklist

Pre-implementation gate for adding a new agentic harness to MOSAIC. **Complete this checklist before writing any deployment code.** Each capability must be verified with a hands-on spike against the target harness, not just documentation review -- docs can be aspirational, outdated, or describe features behind flags that don't work yet.

**Time budget:** 1-2 hours. If a capability can't be verified in that window, it's a red flag.

**Origin:** This checklist exists because the Codex CLI harness was fully implemented (3 days) before discovering that its subagent dispatch mechanism couldn't reference custom agents by name -- a blocker that a 30-minute spike would have surfaced. See `MOSAIC-CODEX-FEASIBILITY-20260921.md` for the full post-mortem.

---

## Tier 1: Orchestration Blockers

These three capabilities are required for MOSAIC's orchestration loop to function. If any is "No," the harness supports **standalone agent mode only** -- individual agents can run, but the orchestrator cannot coordinate workflows. Do not implement orchestrator deployment for this harness until all three are "Yes."

### 1.1 System Prompt Injection (Append, Not Replace)

**Question:** Can the orchestrator's full system prompt (~30K tokens) be loaded into the primary session *alongside* the harness's built-in instructions?

**Why it matters:** The orchestrator prompt assumes the harness provides tool mechanics, sandbox awareness, and environment conventions. Replacing the built-in prompt breaks tool usage. Appending preserves both.

**Verification spike:**
1. Create a file with 500+ words of custom instructions
2. Launch the harness with those instructions added to the session
3. Confirm: the session follows both the custom instructions AND still uses its native tools correctly (file read/write, shell, etc.)
4. Confirm: the instructions survive a long conversation (aren't diluted or dropped)

**Mechanisms to test:**
- CLI flag (e.g., `--append-system-prompt`)
- Config file field that *appends* (not replaces)
- Project docs file (e.g., CLAUDE.md, AGENTS.md) -- note: these are typically lower-priority than system prompt and may be ignored under pressure

| Harness | Result | Mechanism | Date Verified | Notes |
|---------|--------|-----------|---------------|-------|
| Claude Code | Yes | `--append-system-prompt` | 2026-07 | Foundation harness |
| Codex CLI | **No** | `model_instructions_file` replaces, not appends | 2026-09 | `AGENTS.md` is additive but low-priority. `--append-system-prompt` requested in GH #11117, closed without implementation |

---

### 1.2 Programmatic Subagent Dispatch by Name

**Question:** Can the primary session programmatically invoke a named subagent (defined in a config file) and receive its structured response?

**Why it matters:** The orchestrator dispatches 10-20+ different subagents per workflow run. Each must receive its own system prompt (from its agent definition file) and return a JSON response the orchestrator can parse. If dispatch is manual or unnamed, the orchestration loop cannot function.

**Verification spike:**
1. Define two custom agents with distinct instructions and names
2. From the primary session, programmatically spawn agent A by name
3. Confirm: agent A receives its own instructions (not the parent's)
4. Confirm: agent A's response is returned to the primary session in a parseable format
5. Repeat for agent B to confirm name-based selection works
6. Confirm: agents inherit their declared model and permission settings

**What to look for:**
- A "spawn agent" or "dispatch task" tool available to the primary session
- A parameter that accepts an agent name/key matching a config file
- Response delivery back to the calling session (not just fire-and-forget)

| Harness | Result | Mechanism | Date Verified | Notes |
|---------|--------|-----------|---------------|-------|
| Claude Code | Yes | `Agent` tool with `subagent_type` parameter | 2026-07 | Agent key matches `.claude/agents/*.md` filename |
| Codex CLI | **No** | `spawn_agent` exists but accepts only `agent_type` + overrides, no name reference | 2026-09 | GH #15250, #26363, #26868. Custom TOML files parse but runtime ignores them on spawn |

---

### 1.3 Subagent Configuration Inheritance

**Question:** Does a spawned subagent actually use the model, permissions, and instructions from its definition file?

**Why it matters:** MOSAIC assigns different models to different agents (e.g., high-reasoning for orchestrator, efficient for research). If the subagent ignores its config and inherits the parent's, agents run on wrong models with wrong permissions, producing unpredictable results.

**Verification spike:**
1. Define a custom agent with a specific model (different from default) and restricted permissions
2. Spawn it and have it report: what model it's running, what permissions it has
3. Confirm these match the definition file, not the parent session

| Harness | Result | Mechanism | Date Verified | Notes |
|---------|--------|-----------|---------------|-------|
| Claude Code | Yes | Agent definitions in `.claude/agents/*.md` with `model` in frontmatter | 2026-07 | Model specified per-agent |
| Codex CLI | **No** | TOML files define model/sandbox_mode but spawned agents inherit parent values | 2026-09 | GH #26868 confirms systemic bug |

---

## Tier 2: Quality of Life

These affect how well MOSAIC runs but don't break the core loop. A "No" means degraded functionality, not a blocker.

### 2.1 Structured Response from Subagents

**Question:** Does the subagent's response arrive as parseable text (not just a summary or status indicator)?

MOSAIC subagents return JSON with status codes. The orchestrator must parse `status_code`, `status_message`, and optionally `error_code` from the response.

| Harness | Result | Notes |
|---------|--------|-------|
| Claude Code | Yes | Full agent output returned in tool result |
| Codex CLI | Unreliable | Handoff failures in recent versions (v0.118.0+); recommended to stay on v0.117.0 |

### 2.2 User Interaction from Subagents (HITL)

**Question:** Can a spawned subagent contact the user for approval/feedback during execution?

MOSAIC's HITL gates require subagents to present output and get user sign-off before returning.

| Harness | Result | Notes |
|---------|--------|-------|
| Claude Code | Yes | MCP tools (`mcp__contact-user__*`) available to subagents |
| Codex CLI | Unknown | No documented mechanism for subagent-to-user interaction |

### 2.3 Concurrent Subagent Dispatch

**Question:** Can multiple subagents run in parallel?

Some MOSAIC workflows (kb-generation) dispatch parallel stages.

| Harness | Result | Notes |
|---------|--------|-------|
| Claude Code | Yes | Harness-managed, practical limit ~5-8 |
| Codex CLI | Limited | Ceiling of 6 threads (`max_concurrent_threads_per_session`) |

### 2.4 Skills Loading

**Question:** Are skill files automatically loaded, or must the agent read them manually?

| Harness | Result | Notes |
|---------|--------|-------|
| Claude Code | Auto-loaded | `.claude/skills/` contents injected into context |
| Codex CLI | Manual read | Agent must explicitly read from `.agents/skills/` |

---

## Tier 3: Deployment Mechanics

These determine what the deploy tool needs to produce. They don't affect feasibility -- they affect implementation.

### 3.1 Agent File Format

What format does the harness expect for agent definitions?

| Harness | Format | Location | Notes |
|---------|--------|----------|-------|
| Claude Code | Markdown with YAML frontmatter | `.claude/agents/*.md` | |
| Codex CLI | TOML | `.codex/agents/*.toml` | |

### 3.2 Project Instructions File

What is the harness's equivalent of CLAUDE.md / AGENTS.md?

| Harness | File | Notes |
|---------|------|-------|
| Claude Code | `CLAUDE.md` | Loaded automatically, additive |
| Codex CLI | `AGENTS.md` / `AGENTS.override.md` | Chain from global to cwd, closer files win |

### 3.3 Permissions Model

How does the harness express tool/file access?

| Harness | Model | Notes |
|---------|-------|-------|
| Claude Code | Per-agent tool allowlist | Tools listed in agent definition |
| Codex CLI | `sandbox_mode` field | `read-only`, `workspace-write`, `danger-full-access` |

---

## Decision Matrix

After completing the checklist:

| Tier 1 Results | Decision |
|----------------|----------|
| All three "Yes" | Proceed with full harness implementation (orchestrator + subagents + harness injections) |
| Any "No" | Implement **standalone agent mode only**: deploy individual agent files, skills, and project instructions. Do not implement orchestrator deployment. Document the blockers and which upstream issues to watch |
| All three "No" | Do not implement. The harness is not ready for MOSAIC in any mode if agents can't even load custom instructions |

---

## Adding a New Harness

When evaluating a new harness:

1. **Spike first** (1-2 hours): Run through Tier 1 checks with hands-on tests
2. **Document results**: Add a row to each table above
3. **Decide scope**: Use the decision matrix
4. **Then implement**: Only after you know what's viable

The deploy tool supports standalone-only harnesses -- it can produce agent files and skills without an orchestrator. This is a valid deployment target for harnesses that fail Tier 1.
