# Claude Code — Resolved Issues

> Last updated: 2026-10-25 (run 20)

---

### CC-005: `PostToolUse` hook doesn't fire for `Agent` tool completions at the parent session level

| Field | Value |
|-------|-------|
| **Source** | Unconfirmed — could not be relocated on `anthropics/claude-code` |
| **Fixed In / Status** | N/A (not relevant — unsourceable) |
| **Resolution Date** | 2026-09-10 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
Entry claimed the `PostToolUse` hook does not fire when an `Agent` tool call (subagent dispatch) completes, at the parent session level. The original capture never retained a source URL, and a prior backfill pass (2026-09-09) already searched extensively without relocating a matching issue.

**Resolution:**
A second independent search pass (2026-09-10, run 15 batch 1) repeated and extended the prior backfill attempt — combining PostToolUse/Agent/Task/parent-session/completion keyword queries and reviewing `area:hooks`-labeled issues — and still could not locate any GitHub issue matching this precise claim. Two adjacent candidates were re-examined and ruled out as the true source, same as the prior pass: #82249 ("`SubagentStop` still doesn't fire for subagents launched via the Agent tool (async/background)") describes a real, evidenced, still-open defect, but the specific hook is `SubagentStop` (not `PostToolUse`) and it fires/fails to fire in the subagent's own hook context, not at the parent session level; #90662 describes misattributed `agent_id` on hooks inside a running subagent, a different mechanism entirely. With no locatable source after two independent thorough searches, and no way to distinguish a real-but-mis-transcribed claim from a fabricated/conflated one, this entry fails the knowledge base's evidentiary bar and is retired as unsourceable rather than carried indefinitely as "Likely" confidence with unknown fields. Flagged as a suggested follow-up: #82249 itself is a real, well-evidenced, currently-untracked gap (hook lifecycle events not firing for Agent-tool async/background subagent completions) that a future discovery pass should consider adding as its own new KB entry on its own merits, rather than as a proxy for this retired claim.

---

### CC-046: `.claude/settings.json` created mid-session never gets its hooks armed until the session is restarted

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anthropics/claude-code/issues/77357 |
| **Fixed In / Status** | Fixed by v2.1.301 (reporter-verified; still broken on 2.1.233) - issue closed `completed` 2026-10-15 |
| **Resolution Date** | 2026-10-15 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
If a session started in a directory with no `.claude/` directory, a `.claude/settings.json` (with hooks) created later in the same session never had its hooks armed until restart, silently, because the settings watcher computed its watch targets once at startup. Maintainer reproduced on 2.1.233.

**Resolution:**
The original reporter re-tested on 2.1.301 on macOS and Linux and found a `PreToolUse` hook written mid-session into a new `.claude/settings.json` now fires about 5 seconds later in the same session, while the same check on 2.1.233 still shows restart-only arming. They closed the issue as completed on 2026-10-15. No fix PR or changelog line is explicitly linked; the exact fixing release lies between 2.1.233 and 2.1.301. Moved to resolved in run 20.

---

### CC-069: `UserPromptSubmit` hook, correctly registered and verified working, never fires for interactively-typed prompts in the CLI

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anthropics/claude-code/issues/92353 |
| **Fixed In / Status** | N/A (not relevant - probable user error) |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
Single reporter (Windows, Git Bash CLI, v2.1.261) claimed a registered `UserPromptSubmit` hook never fired for typed prompts because a secret-detecting prompt was not blocked. No maintainer response, no corroboration; issue still open with no activity since 2026-09-05.

**Resolution:**
Run 20 re-check (2026-10-25): the reporter's own repro hook reads `prompt_text` from the stdin JSON, but the documented `UserPromptSubmit` payload field is `prompt` (docs: code.claude.com/docs/en/hooks, UserPromptSubmit input). The hook very likely fired and simply found no secret in a nonexistent field, so the 'never invoked' claim (inferred only from absence of a block, no hook-side log/heartbeat) is probably a reporter script bug, not a harness defect. Confidence was already Unverified; no other reporter confirmed it. Retired as not relevant. Related still-tracked hook entries: CC-072 (mid-turn interrupt messages skip UserPromptSubmit), CC-047, CC-071. Prior-pass notes: VS Code extension / Cowork / desktop-app-specific non-firing reports #92652, #90784, #92074, #85669, #90176, #87657, #90296 were investigated and deliberately not added (surface-specific, not MOSAIC's CLI/Runner path); #87657 (intermittent hook-registry loss on desktop entrypoint) is the one worth revisiting if corroborated.

---

### CC-078: `CLAUDE.md`/path-scoped rules never load when Claude creates a brand-new file matching the pattern (loading gated on a prior `Read`)

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anthropics/claude-code/issues/63142 (closed `not_planned` by stale-bot 2026-09; fix shipped via changelog) |
| **Fixed In / Status** | v2.1.288 (2026-10-02) |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | MEDIUM |

**Summary:**
Path-scoped `.claude/rules/*.md` (`paths:` frontmatter) and nested `CLAUDE.md` files were only injected into context when a matching file was read via the `Read` tool, so a subagent creating a brand-new file in a governed path never received the rules. Single reporter, stock Anthropic infra, no maintainer comment; issue itself was stale-bot closed.

**Resolution:**
Claude Code changelog 2.1.288 (released 2026-10-02): 'Fixed path-scoped `.claude/rules` and nested CLAUDE.md files not loading when Write or Edit creates or changes a file in their scope (previously only Read loaded them)'; same release fixed the InstructionsLoaded hook omitting agent_id/agent_type for subagent-triggered loads. Related 2.1.293 fix: rules/nested CLAUDE.md now also load when Claude views a file via single-file cat/head/tail/sed -n/grep in Bash (relevant to CC-060). Fix not directly retested at MOSAIC; sibling entries CC-034, CC-036, CC-059, CC-060 remain tracked separately (other trigger paths). Workaround of hoisting rules into root CLAUDE.md is no longer needed on >= 2.1.288 for the new-file case.

---

### CC-038: Every Bash dispatch in a large or `--resume`d session can freeze the ENTIRE process for ~80-90 seconds

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anthropics/claude-code/issues/89772 |
| **Fixed In / Status** | v2.1.290 (changelog: "Fixed a freeze before the first request and in the /sandbox Config tab on Linux and WSL when a sandbox read rule such as ~/**/.env covers a large folder"); further improved in 2.1.300. Reporter noted a deliberate fix in 2.1.290 but had not formally confirmed; issue still open |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
On Linux, every `Bash` dispatch in a large/`--resume`d session blocked the whole process for ~80-90s at 100% CPU before the shell spawned. Thread narrowed the cause to a glob-based Read permission rule (e.g. `Read(~/**/.ssh)`) combined with bubblewrap sandboxing, which triggered a recursive home-directory walk on every dispatch. A regression was reported in 2.1.284 (glob rules forwarded to the sandbox expander again).

**Resolution:**
Run 20 (2026-10-25): changelog 2.1.290 fixes the freeze for sandbox read rules covering a large folder on Linux/WSL, and 2.1.300 makes Esc/Ctrl-C work while wildcard read rules are expanded. The original reporter (2026-10-13) stated there is a deliberate fix in v2.1.290 and would resolve the issue once confirmed. Retired as fixed; if stalls recur on >= 2.1.290 with broad glob Read rules plus sandboxing, re-track. Workaround on older versions: avoid broad `~/**/...` Read rules, or stay below the affected versions (2.1.283 per reporter).

---

### CC-039: `.claude/settings.json` is written via a non-atomic, unlocked read-modify-write — concurrent sessions can tear or clobber it

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anthropics/claude-code/issues/91520 |
| **Fixed In / Status** | v2.1.274 (Anthropic maintainer comment 2026-10-23: "Fixed in Claude Code 2.1.274"; issue closed completed. Changelog 2.1.274: "[VSCode] Fixed overlapping settings writes from the extension leaving ~/.claude/settings.json unparseable or dropping a setting") |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
The VS Code extension's `writeUserSettingsAndPush` persisted `~/.claude/settings.json` via a truncating `writeFile` with no lock or tmp+rename, so concurrent sessions could leave a torn/invalid file (session falls back to defaults, dropping permissions/hooks) or lose each other's keys.

**Resolution:**
Run 20 (2026-10-25): maintainer confirmed fixed in 2.1.274; the changelog line scopes the fix to the VS Code extension write path. The CLI's own settings write path is not explicitly mentioned and was not independently verified; related reports (#79403, #82167, #76749, #2810) not re-checked. If torn settings.json is observed in CLI-only concurrent sessions on >= 2.1.274, re-track.
