# OpenCode V2 Harness Analysis Report

| Field | Value |
|-------|-------|
| **Harness** | OpenCode V2 |
| **Harness version** | v2.3.2 |
| **Model observed** | openai/gpt-6-sol (both tiers) |
| **Captured** | 2026-11-18 |
| **Analyzed** | 2026-11-18 |
| **Source files** | `PrimaryAgent/` and `Subagent/` in this folder; V1 comparison against `OpenCode/` (captured 2026-09-06, gpt-5.6-sol) |

**Read this first: the tool set is shaped by our deploy.** Both captures ran as the deployed `system-prompt-capturer` (`.opencode/agents/system-prompt-capturer.md`, `mode: all`). Its V2 `permissions` start with `"*": deny` and allow only read, edit, glob, grep, shell, question and subagent. Tools the capturer denies (`webfetch`, `websearch`, `execute`, `experimental_image_generate`, `todowrite`) are missing from the prompt because of that config, not because V2 lacks them. The OpenCode V2 spike found `execute`, `websearch` and `experimental_image_generate` on for every agent unless denied.

---

## 1. Primary Agent vs Subagent

The two tiers are **practically identical**. The prompt structure, the tool set (10 tools), the API header (oververbosity 1, Juice 10), the env block and the injection point all match.

| # | Difference | Primary | Subagent | Assessment |
|---|-----------|---------|----------|------------|
| D1 | `subagent` agent list | `explore`, `general` | `explore`, `general`, **`system-prompt-capturer`** | Real. The capturer (`mode: all`) is not offered to itself as primary, but a child sees it and could spawn another capturer. Nothing stops an agent from spawning its own type at the subagent tier. |
| D2 | `shell.background` description | includes "You do not need to use '&' at the end of the command when using this parameter." | sentence absent | Probably transcription noise; unconfirmed. |
| D3 | `parallel` parameter description | "Ensure that the parameters provided to each tool are valid…" | "Ensure these are valid…" | Transcription noise (V1 had the same kind of single-word drift). |
| D4 | `Array<` vs `Array[` | angle brackets kept | converted | Bracket-notation inconsistency in the captures, not a harness difference. |

**Same in both tiers:** `subagent` (nested spawning), `question` (asking the user works from a subagent: the subagent capture has an answered test question), `patch`, `shell`, `skill`, `opencode_worktree_remove`.

### Tool output differences between tiers

Output formats match where both tiers exercised the same tool. Two notes:

- `shell` nonzero exit: the subagent ran plain `exit 7` and got `(no output)Exit code: 7`. The primary ran `Write-Error …; exit 7` and got the Windows PowerShell error record followed by `Exit code: 7`. Different commands, same format.
- `subagent` output was captured at the primary tier only. The subagent tier did not run a test spawn.

### What this means for MOSAIC

As in V1, V2 subagents are full peers of the primary. A MOSAIC subagent on V2 can ask the user directly and can spawn children. The deploy's permissions are the only thing that limits them. The no-child-agents rule therefore has to come from permissions (`subagent: deny`) or agent text, not from the harness.

---

## 2. V2 vs V1 OpenCode

V2 is a rewrite, and the prompt shows it. The clean prompt is half the size of V1's (≈12.7k vs ≈25k chars), almost entirely because the tool descriptions lost their behavioral guidance.

Model differences confound part of the comparison: V1 ran gpt-5.6-sol, V2 ran gpt-6-sol. The API header (oververbosity, Juice, channels) is added on OpenAI's side, so its changes are not attributed to the harness.

### 2.1 Prompt structure

| Aspect | V1 (OpenCode) | V2 (OpenCode V2 v2.3.2) | Notes |
|--------|---------------|-------------------------|-------|
| Preamble | "You are an AI assistant accessed via an API." | Same | API-side |
| Oververbosity | 3 | **1** | API-side or model-side; not harness |
| Juice | 16.855 (primary) / 16 (subagent) | **10** (both) | API-side |
| Channels | analysis, commentary, final, summary | Same | API-side |
| `# Instructions` | Empty | Empty | Same in both |
| Agent body position | After the tool namespaces | After the tool namespaces | Same in both. The agent `.md` body is a plain block after `multi_tool_use`, before the model line. (The V1 report's claim that it fills `# Instructions` is one of its known errors.) |
| Model line | "You are powered by the model named …" | `# Your Model` block (name, provider ID, model ID) + date | Reformatted |
| `[env]` | cwd, workspace root, git repo, platform, date | cwd, workspace root, git repo, platform, **preferred temp dir** (`…\Temp\opencode`, pre-approved for external access) | Date moved out of `[env]` |
| Trailer | `[available_skills]` list | **No skills list.** One line about worktrees and `execute` → `tools.opencode.session_move` | See 2.3 |

### 2.2 Tools

| V1 | V2 | Change |
|----|----|--------|
| `bash` (PowerShell 5.1 under a POSIX name) | **`shell`** | Honest name. Description: "Commands run on Windows using powershell." |
| `task` (`subagent_type`, `task_id`) | **`subagent`** (`agent`, `sessionID`, `model`) | Renamed and **asynchronous**, see 2.4 |
| `apply_patch` | **`patch`** | Same patch grammar; new rule "Do NOT call this tool in parallel with other tools." The spike found `patch` is the file tool only for GPT models; V2 also has `edit`/`write` |
| `read` | `read` | `filePath` → `path` (V1 demanded absolute paths; V2 accepts relative); still reads directories; capped at 2000 lines; V1's usage list (parallel reads, avoid tiny slices) cut to two sentences |
| `glob` | `glob` | New `hidden` and `limit` parameters |
| `grep` | `grep` | New `literal`, `caseSensitive`, `limit` parameters |
| `question` | `question` | Same schema; V1's `custom` toggle and "answers are returned as arrays of labels" note dropped |
| `skill` (`name`) | `skill` (`id`) | Parameter renamed |
| `todowrite` | not offered | In V2 it needs a flag (V2 research F3) |
| `webfetch`, `websearch` | not offered | Denied by the capturer's permissions; exist in V2 |
| — | **`opencode_worktree_remove`** | New; offered although the capturer's `"*": deny` lists no worktree permission |
| — | `execute` | Exists, denied by the capturer; still named in the env trailer |
| `multi_tool_use.parallel` | Same | Unchanged |

### 2.3 Behavioral content removed

V1 carried a lot of harness guidance inside tool descriptions. V2 drops almost all of it:

- **Shell guidance gone:** V1's `bash` had ~60 lines: `Test-Path` checks before creating files, quoting examples, "avoid `&&` in PowerShell 5.1, use `cmd1; if ($?) { cmd2 }`", prefer dedicated tools over cmdlets, use `workdir` instead of `cd`. V2 keeps only "quote paths", "prefer dedicated tools", "use `workdir`". **Nothing warns the model that `&&` fails.** The nonzero-exit output (`CategoryInfo` / `FullyQualifiedErrorId`) is Windows PowerShell's format, so the shell is most likely still PS 5.1.
- **Git/GitHub rules gone:** V1's "only commit when asked, never force-push, use `gh`" section no longer exists.
- **Dispatch guidance gone:** V1's "when NOT to use Task", "launch agents concurrently", "agent outputs should generally be trusted" are replaced by a short description of async behavior.
- **todowrite "use proactively / when in doubt, use it"** is no longer in context (the tool is not offered).
- **Skills list gone:** the `skill` tool is offered, but there is no `[available_skills]` block, and the tool only says the ID "must match an available skill". Most likely the capturer's `"*": deny` (no `skill` allow) filters the skill catalog while leaving the tool. Inference, not tested.

### 2.4 Dispatch is now asynchronous

V1 `task` blocked and returned `[task id=… state="completed"][task_result]…`. V2 `subagent`:

- **Always runs in the background.** The call returns at once with a `sessionID` and an instruction: "DO NOT sleep, poll, proactively check on its progress, or duplicate this subagent's work. Work on non-overlapping tasks, or briefly tell the user what you launched and end your response."
- **The result arrives later as a message:** `[subagent sessionID="…" state="completed" description="…"] … [/subagent]`, and again after every later round of work by that child.
- **Follow-ups:** calling `subagent` with a running child's `sessionID` joins its current run.
- **Per-dispatch `model`** (`providerID/modelID#variant`) is new, guarded by "NEVER set this unless the user explicitly asks". The description refers to a "models tool" to look IDs up; it was not offered here.
- Fresh context per child unless a `sessionID` is passed (same as V1).

### 2.5 Tool output formats

| Tool | V1 | V2 |
|------|----|----|
| `read` | `[path]…[/path][type]file[/type][content]…(End of file - total N lines)[/content]` | `Read file X, lines 1-20` header, `N: content` lines; `[Output truncated. Continue reading with offset: N]` when cut. Default read of the 3000-line file stopped at line 1422 (byte limit, not the 2000-line cap) |
| Errors | Plain text | JSON envelope: `{"error":{"type":"tool.execution","message":"…"},"content":[]}` |
| `shell` nonzero exit | `(no output)` (exit code not shown) | `Exit code: 7` appended |
| `shell` long output | Truncated, saved to file | Shows the **last** 2000 lines with `[showing lines 3001-5000 of 5000; full output saved to …]` |
| `grep` no match | `No files found` | `No matches found` |
| `glob` / `grep` paths | Absolute | Absolute |
| `patch` success | `Success. Updated the following files:` + `A`/`M` lines | Same |

---

## 3. MOSAIC Implications

| # | Implication | Where it lands |
|---|-------------|----------------|
| I1 | **The orchestrator must wait for `[subagent]` messages, not tool results.** A dispatch returns immediately. Hub-and-spoke still works, and parallel dispatch comes naturally, but the orchestrator has to end its turn (or work on something else) and route when the completion message arrives. The existing watch item in `HarnessInjectionsOrchestrator.md` (re-calling `subagent` with a running child's `sessionID`) belongs to the same behavior. | Orchestrator injection, Runner OpenCode V2 adapter |
| I2 | **No shell guardrails.** V1's `&&` warning was harness text; V2 has none. The V2 injections are empty by design ("add only once observed in use"), so the first POSIX-ism in a generic template will fail with no hint. | Watch in use; candidate shared injection |
| I3 | **Permissions are the only scope control.** Subagents can spawn children (including their own type) and ask the user. Least privilege, including `subagent: deny` for workers, has to come from the deploy. | `mosaic-deploy` V2 rules (already emits deny-by-default) |
| I4 | **Skills may be invisible under `"*": deny`.** If the skill catalog is filtered when `skill` is not allowed, agents with `required_skills` must get an explicit `skill` allow, or be told skill paths directly. | Deploy rules; verify with an agent that allows `skill` |
| I5 | **Wildcard deny does not remove every tool.** `skill` and `opencode_worktree_remove` were offered despite `"*": deny`. Worktree removal with `force` is destructive; its description says to set `force` only after user confirmation. | Deploy rules: deny `opencode_worktree_remove` explicitly, if the permission action can be named |
| I6 | **Much less harness text to fight.** No git policy, no todowrite push, no dispatch etiquette. MOSAIC agent text meets almost no competing instructions. | Positive; keeps V2's cleanliness ranking at the top |
| I7 | **Per-dispatch model selection exists** but the harness tells the model not to use it unless the user asks. | Possible future use for role-based model routing |

## 4. Open Questions

| # | Question |
|---|----------|
| Q1 | Does the `[available_skills]` block come back when the agent's permissions allow `skill`? |
| Q2 | Is the shell Windows PowerShell 5.1 or PowerShell 7 (`&&` works in 7)? The error format points to 5.1. |
| Q3 | Which permission action names `opencode_worktree_remove`, and does denying it remove the tool? |
| Q4 | Do `edit`/`write` replace `patch` for non-GPT models (spike finding), and what do their descriptions say? |
