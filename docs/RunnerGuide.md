# Runner Guide

How to use `mosaic-run` to execute MOSAIC orchestration workflows automatically.

This guide is for **project authors** who want to run multi-agent workflows without manually driving the orchestrator. The Runner reads a workflow definition, dispatches subagents through your AI coding tool, records results, and handles deviations — replacing the orchestrator's mechanical work while preserving its intelligent routing when needed.

---

## At a Glance

| Concept | Summary |
|---------|---------|
| **What it does** | Executes a MOSAIC workflow end-to-end: reads the workflow table, dispatches subagents, writes `Orchestration.md`, handles deviations |
| **Two ways to run** | Interactive TUI (no subcommand) or scriptable CLI (`mosaic-run run`) |
| **Three execution modes** | **Orchestrated** (orchestrator decides everything), **Auto** (engine routes happy path), **Auto-review** (engine also routes review loops) |
| **Supported harnesses** | `claude-code`, `opencode`, `ghcp-cli`, `codex` (experimental, see [Codex](#codex)) and `opencode-v2` (experimental, see [OpenCode V2](#opencode-v2)) |
| **State file** | `Orchestration-{run_id}/Orchestration.md` — atomic writes, crash-safe, resumable |

> **Known issue — GHCP CLI:** Runner runs on `ghcp-cli` are currently unreliable. With recent GHCP CLI versions (observed on 1.0.91), GHCP CLI sometimes ignores the requested `--agent` and runs its default agent instead, which shows up as protocol or parsing errors in the run. The failures are intermittent and the cause is not yet confirmed. In limited testing GHCP CLI 1.0.87 did not show the problem, so pinning it may help, but this is not verified. For reliable runs, use `claude-code` or `opencode`.

**Why not just use the orchestrator agent?** A persistent orchestrator session accumulates context with every dispatch — every tool call, artifact edit, and subagent response stays in the context window. Cost grows roughly quadratically with run length. The Runner offloads all mechanical work (artifact writes, sequence tracking, harness invocations) and keeps orchestrator calls bounded: each starts a fresh session reading only the compact `Orchestration.md`.

---

## Running a Workflow

### Interactive (TUI)

```sh
./mosaic-run
```

The TUI walks you through: harness selection (which auto-discovers the orchestrator script), workflow selection, mode configuration, and then shows live progress as the run executes.

Progress-screen keys: `s` graceful stop (asks for confirmation), `m` manual dispatch at the next routing decision, `h` HITL for the next agent, `g` switch the loop guard off or on for this session (see [Resuming a Run](#resuming-a-run)), `e` view and edit the standing guidance (auto modes with pre-consultation only; see [Pre-Consultation](#pre-consultation)), `a` artifact view, `l` hide or show the live activity panel, `x` force-stop the running invocation (asks for confirmation; see [Live activity and force stop](#live-activity-and-force-stop)), `ctrl+c` force quit. When the run has more steps than fit, `↑`/`↓` and `PgUp`/`PgDn` scroll the step list, `Home` jumps to the oldest step and `End` to the newest; the list follows new steps until you scroll up, and follows them again once you press `End` or scroll back to the bottom. A failed infrastructure step (checkpoint, commit, review agent) is marked on the progress screen, and the summary at the end of the run, in the TUI and the CLI, lists every infrastructure failure of the run.

### CLI

```sh
./mosaic-run run \
  --workflow quick-fix \
  --task "Fix the login timeout bug reported in issue #42" \
  --harness claude-code \
  --mode auto \
  --timeout 30m
```

The orchestrator script is discovered automatically from the harness-convention agents directory (e.g., `.claude/agents/orchestrator-script.md` for `claude-code`). The file may also be named `orchestrator-script.agent.md`; a folder holding both names is refused. If the workspace has not been deployed for the selected harness, the Runner exits immediately with a clear error.

---

## Execution Modes

The mode controls how much routing the Runner handles versus what it delegates to a script-mode orchestrator agent.

| Mode | Happy path routing | Deviation routing | Task description quality | Cost |
|------|-------------------|-------------------|-------------------------|------|
| **orchestrated** | Orchestrator decides every step | Orchestrator decides, including after `PARTIALLY_DONE`, `BLOCKED` with `E501` and a harness error | High — orchestrator crafts each one | Lowest of all approaches (bounded context), but most orchestrator calls |
| **auto** | Engine follows workflow table | Orchestrator consulted, except for the automatic retries below | Generic on happy path, crafted on deviation | Fewer orchestrator calls |
| **auto-review** | Engine follows table + review loops | Orchestrator only for unresolvable deviations, with the same automatic retries | Generic everywhere except deviations | Fewest orchestrator calls |

**Automatic retries (auto and auto-review).** Three outcomes are re-dispatched by the Runner itself, without consulting the orchestrator:

- `PARTIALLY_DONE`: the same agent runs again on the same row, with its previous output added to its inputs and its previous status message added to its task. This happens up to 3 times in a row; after that the orchestrator is consulted.
- `BLOCKED` with error code `E501` (the agent itself reports that a tool is unavailable): the same agent runs again, up to 3 attempts per row and iteration since the last `SUCCESS` there; once the budget is spent the orchestrator is consulted.
- `HARNESS_ERROR`: an invocation that ended without a usable response from the agent (timeout, process failure, unparseable or empty response, GHCP agent deselected or session error). The Runner records it as `HARNESS_ERROR`, not as a `BLOCKED` of the agent; the row's `Summary` names the kind of failure, for example `timeout: ...`. A response that arrived but could not be parsed first gets one direct re-run (a failed one is counted). Then the same agent runs again, up to 3 failed attempts per row and iteration since the last `SUCCESS` there, counted separately from `E501`; once the budget is spent the orchestrator is consulted. In orchestrated mode, and when no budget is left, the orchestrator is consulted at once. A `HARNESS_ERROR` attempt is never accepted: the run does not move on, and a resume runs the same assignment again. Stopping an invocation yourself with `x` is not a harness error.

Every other non-`SUCCESS` status still goes to the orchestrator, including `COMPLETED_NEEDS_ACTION` in auto mode. In auto-review mode a reviewer's `COMPLETED_NEEDS_ACTION` is routed back to its creator automatically until the review loop limit is reached, counted per reviewer, phase and stage since that reviewer's last `SUCCESS`. In orchestrated mode the orchestrator is consulted after every step and none of these shortcuts apply.

### Which mode to choose?

- **orchestrated** (recommended default) — Best quality. The orchestrator crafts targeted task descriptions for every dispatch. Each invocation is cheap (fresh session, bounded context).
- **auto** — Good when happy-path subagents perform well from their own instructions and input artifacts alone. Orchestrator still handles deviations.
- **auto-review** — Maximum automation. Use when review-loop routing is fully captured by the workflow table's On Findings column and subagents reliably self-direct.

All three modes are dramatically cheaper than running the orchestrator agent manually, because the Runner eliminates context window growth.

---

## Key CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--workflow` | New run | — | Workflow identifier (must exist in the orchestrator file); not needed with `--run` |
| `--task` | New run | — | Task description for the run; not needed with `--run` |
| `--harness` | Yes | `fake` | Harness adapter (`claude-code`, `opencode`, `ghcp-cli`, `codex`, `opencode-v2`); Codex and OpenCode V2 are experimental, see [Codex](#codex) and [OpenCode V2](#opencode-v2) |
| `--mode` | New run | — | Execution mode (`orchestrated`, `auto`, `auto-review`); on a resume only for a run that records no Runner settings |
| `--timeout` | No | `30m` | Per-invocation timeout for dispatches without human-in-the-loop |
| `--hitl-timeout` | No | `4h` | Per-invocation timeout for dispatches sent with `human_in_the_loop: true`, where a person may take long to answer |
| `--no-output-timeout` | No | `10m` | No-output watchdog for `opencode-v2` invocations without human-in-the-loop; `0` disables it. See [OpenCode V2](#opencode-v2) |
| `--pre-consult` | No | `true` | One-shot environment consultation at run start (auto/auto-review only); use `--pre-consult=false` to disable |
| `--manual-resolution` | No | `false` | When a consultation fails, let the user choose the next dispatch in an interactive dialogue; needs the TUI (see [Manual routing](#manual-routing)) |
| `--review-loop-limit` | New run | — | Maximum review loop iterations: a positive integer, or `none` for no limit. Required for a new run and fixed once the run exists |
| `--checkpoints` | No | `disabled` | Checkpoint support (`disabled`, `enabled`) |
| `--commits` | No | `disabled` | Commit-class infrastructure dispatch (`disabled`, `enabled`) |
| `--commit-branch` | No | `mosaic-owned` | Commit branch variant (`mosaic-owned`, `user-own`) |
| `--run` | No | — | Resume a specific run by run_id |
| `--new-run` | No | `false` | Force creation of a new run |
| `--executable-path` | No | _(per-harness default)_ | Executable path override for the harness selected by --harness; when absent, each harness uses its own default |
| `--infra-class` | No | — | Non-interactive agent-per-class mappings (e.g. `checkpoint=checkpoint-manager-git,commit=commit-manager-git`) |

### Orchestrator Auto-Discovery

There is no `--orchestrator-file` flag. The Runner derives the orchestrator path from the `--harness` value using the harness-convention agents directory:

| Harness | Expected orchestrator path |
|---------|---------------------------|
| `claude-code` | `.claude/agents/orchestrator-script.md` |
| `opencode` | `.opencode/agents/orchestrator-script.md` |
| `ghcp-cli` | `.github/agents/orchestrator-script.md` |
| `codex` | `.codex/agents/orchestrator-script.toml` |
| `opencode-v2` | `.opencode/agents/orchestrator-script.md` |

For the Markdown harnesses, `orchestrator-script.agent.md` is accepted in place of `orchestrator-script.md`, and a folder holding both is refused. If the file does not exist, the Runner exits immediately with a message indicating the workspace is not properly deployed for the selected harness. Run the Deploy tool to populate the agents directory before using the Runner.

### Agent Snapshot Directories

Every harness loads agents by name from its deployed agents directory. Some harnesses need harness-specific transforms (e.g., `mode: primary` for OpenCode); for those, the Runner backs up the agent files at run start so it can transform them without permanently altering the deployed agents.

**Claude Code -- no snapshot:** The Runner runs the deployed agents in `.claude/agents/` directly, by name, and nothing is copied. An `agents-runner-{run_id}/` directory left by an older Runner version is safe to delete.

**Codex -- no snapshot:** The Runner reads the deployed `.codex/agents/*.toml` files itself and passes each agent's settings to Codex on every invocation. Nothing is copied or transformed.

**OpenCode V2 -- no snapshot, no backup-and-transform:** The Runner runs the deployed agents in `.opencode/agents/` in place, by id, with the agent file used as it is. No agent file is copied, rewritten or restored, and no `RUNNER-RECOVERY.txt` marker is involved. OpenCode V2 can run `mode: subagent` agents, including hidden ones, directly by id. Leaving the files untouched also avoids rewriting agent files while a shared OpenCode service may be watching them.

**Backup-and-transform (OpenCode, GHCP CLI):** These harnesses always resolve agents by name from their canonical directory and cannot be redirected to a copy. Instead, the Runner copies originals to a shared backup directory (e.g., `.opencode/.agents-backup/`), transforms the originals in-place, and restores from backup on completion. Multiple concurrent runs share the same backup directory and coordinate via per-run lock files inside it.

**Automatic crash recovery:** If the Runner crashes mid-run while using backup-and-transform, it leaves a human-readable `RUNNER-RECOVERY.txt` marker inside the agents directory explaining what happened and providing step-by-step manual restore instructions. On the next run start, the Runner automatically detects and recovers any orphaned backup state. The `.txt` extension is used for the marker so that no harness indexes it as an agent definition.

---

## Pre-Consultation

A mechanism for **auto** and **auto-review** modes, on by default. At run start, the Runner invokes the orchestrator once to collect environment-level guidance (tool paths, project conventions, harness quirks) that gets appended to the dispatches the Runner routes itself (not to dispatches the orchestrator routes, nor to commit setup or infrastructure dispatches). If this call fails, the run stops before any workflow agent runs and the run folder is kept; resuming retries it.

The guidance is saved as plain text in `Orchestration-{run_id}/RunnerGuidance.txt`, with a `[task_description]` and a `[constraints]` section. The Runner shows it for review once the first pre-consultation has finished: choose **Accept** or **Edit** (multi-line fields; Enter adds a line break, ctrl+s saves, Esc cancels). When you resume a run, the saved guidance is reused instead of asking the orchestrator again, and the review offers **Accept**, **Edit** or **Regenerate** (a fresh pre-consultation replaces the file). During a run, press `e` on the progress screen to view and change the guidance; the change applies to every dispatch that starts afterwards. You can also edit the file by hand while the run is stopped; the Runner reads it when the next session starts. Every change is logged. The guidance belongs to one run and is not shared with other runs.

```sh
# Disable it
./mosaic-run run   --mode auto   --pre-consult=false   ...
```

**What it fixes:** Subagents not knowing project-specific conventions (e.g., "use `py` not `python`", "skills are at `.claude/skills/`"). It does NOT fix the per-dispatch intelligence gap — that's the fundamental trade-off of auto modes.

**Not needed in orchestrated mode** — the orchestrator already includes environment context in every crafted dispatch.

---

## Manual routing

Normally the orchestrator decides where a run goes when the workflow table does not. With `--manual-resolution` (or the matching TUI setting), a consultation that fails (for example an unusable orchestrator reply) hands that one decision to you instead of ending the run. From the TUI stop screen, **Manual dispatch** does the same once on demand.

The TUI asks you step by step: the **row** to run (or stop the run), the **iteration** for a row in a block, the **task** description, the **input** and **output** artifacts (the row's defaults are pre-checked, and you can add paths), optional **constraints**, and a **HITL** choice (default, on or off). The first question also offers **Ask the orchestrator to decide** when you asked for the dialogue yourself (not when it opened because a consultation failed), which hands that decision to the orchestrator. When you choose it, a last, optional question asks for a **message** to the orchestrator (several lines are allowed; Esc goes back to the first question). The message is sent verbatim with that one decision only: it is not stored and not sent again, and leaving it empty sends none. While the orchestrator decides, the consultation row on the progress screen shows `user message`. Esc steps back one question. A dispatch that does not fit the workflow (for example an iteration that is not in the block's schedule) is explained and you are taken back to correct it.

The dialogue ends without dispatching anything, and the run stops with its state saved and resumable, when you press Esc at the first question, when the interface cannot ask questions, or when the dialogue gives up (after 3 rejected dispatches, or after 64 questions in one attempt). The non-interactive CLI cannot answer the dialogue at all: with `--manual-resolution` there, a failed consultation ends the run as a resumable stop (exit code 6) rather than dispatching or waiting for input.

---

## Resuming a Run

Every run creates an `Orchestration-{run_id}/` folder in the working directory. If a run stops (graceful stop, deviation, crash), resume it:

```sh
# Resume a specific run
./mosaic-run run --run 20260815T143000Z-a3f9 ...

# The TUI shows resumable runs at startup
./mosaic-run
```

A resume needs no settings flags: the workflow, task, mode, checkpoints, commits, review loop limit and infrastructure selections are read from the run. The TUI asks only what belongs to this session (harness, the timeout and the human-in-the-loop timeout, and, when they apply, version drift and the commit branch of a pending commit setup); the CLI needs `--run` and `--harness`. A run created by native orchestration records no Runner settings, so its first resume still needs `--mode` (the TUI asks). Before the run continues, the TUI shows and the CLI prints a summary of the recorded settings. A resume that cannot work is refused early, before you are asked anything: the run records no workflow, its workflow is no longer in the orchestrator file, or its recorded infrastructure no longer matches what is deployed.

The Runner reads the existing `Orchestration.md`, determines where the run left off, and continues from there. If the last attempt was not accepted (it was interrupted, or force-stopped), the resume runs that assignment again instead of routing on its recorded status. It finds the row and iteration of the last step from the Execution Log, the same way it does while the run is live, so a resumed run continues where the live run would have. The counters behind the automatic retries (`PARTIALLY_DONE`, `E501`, `HARNESS_ERROR`) and the review loop limit are read from the log too, so they carry over; a log from an earlier Runner that recorded harness failures as `BLOCKED` with `E501` still resumes, and those rows count against the `E501` budget; only the guard against repeated identical orchestrator dispatches starts again from zero.

In the TUI you do not have to leave the program to continue. After a graceful stop, **Continue** on the done screen resumes the run in place; after an orchestrator stop, **Retry** and **Manual dispatch** on the stop screen do the same. All of them resume the existing run (a run whose `Orchestration.md` exists is never restarted as new) and the progress screen shows the history of what already ran, rebuilt from the Execution Log, rather than starting empty.

If the orchestrator keeps requesting a dispatch that the Runner's loop guard blocks (the same agent dispatched four times in a row on one row), the Runner asks the orchestrator again twice and then stops the run with its state saved, so you can continue it after looking at what happened. In the TUI the ending screen then offers `g  continue with loop guard off`, which resumes with the guard off for that session only. You can also switch the guard off while a run is in progress by pressing `g` on the progress screen (a red line, `LOOP GUARD OFF for this session`, stays visible until you press `g` again). It lasts for the session only and has no CLI flag. While it is off, same-agent dispatches are not limited, so watch for runaway loops; the other limits (automatic retries, review loop limit) still apply.

### Live activity and force stop

The progress screen shows what the running invocation is doing (tool calls and messages). Its header line shows the agent with harness and model, the elapsed time, `ctx <size>` (the context size of the latest model call), and either `running <tool> <duration>` while a tool call is under way or `quiet <duration>` since the last event; the indicator is highlighted once it passes 5 minutes, so a stalled invocation stands out. When the invocation ends the header shows the elapsed time, `ended`, the last context size and the cost in USD (Claude Code and OpenCode; GHCP, Codex and `opencode-v2` show no cost: for `opencode-v2` the cost of a run comes from LogAnalyzer, see the [LogAnalyzer Guide](LogAnalyzerGuide.md); Codex shows no model or context size either). `opencode-v2` shows the context size. Token totals are not shown. Press `l` to hide or show this panel. It reads `no live output available` until the invocation reports its first event. Claude Code runs in stream-json mode so that it reports activity too.

Press `x` to force-stop the running invocation. A separate confirmation (`y` or `n`) comes first. Only the running invocation is killed, with everything it started; the Runner itself keeps running. The attempt is logged as not accepted (`Force-stopped by the user; attempt not accepted`), and you choose whether to stop the run (resumable) or to open the manual routing dialogue. The same whole-tree kill is used when an invocation times out.

The step list on the progress screen scrolls: Up/Down move a row, PgUp/PgDn a page, Home/End jump to the oldest or newest row. The list follows new rows at the bottom; after you scroll up it stays where you put it until you press End. Indicators show how many older and newer rows are hidden, and the activity panel, banners, pending requests and help bar always stay visible. The help bar lists the scroll keys.

The Runner log (`RunnerLogs/<run_id>/<run_id>.log`) shows subagent output live: each stdout line is written as it arrives, attributed to its invocation. When an invocation is killed (timeout, no-output timeout, force stop or run cancellation), the log keeps its stdout, stderr and the reason it ended.

When you press stop in the TUI while the orchestrator is deciding, a decision that finishes after the stop request is discarded rather than dispatched. The Runner tells you so, and the decision is made again on resume.

### Workflow table, rows and resume

- **Row column.** mosaic-deploy adds a first-column `Row` (1..N) to each workflow routing table it rebuilds into an orchestrator; authors never write it. The Runner accepts tables with or without the column and refuses a table whose `Row` numbers do not match row positions, for example after a hand edit.
- **WorkflowRow in the Execution Log.** Each workflow step records the table row it ran in a `WorkflowRow` column directly after `Iteration` (`Seq | Agent | Phase | Iteration | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint`). The `Iteration` column holds the iteration ID of a row in a block (`{ID}`, or `{ID}.{Group}` for a grouped row, for example `1.Test`) and `-` for a row outside every block. Infrastructure, out-of-band and ad-hoc steps record `-`. Live routing and resume identify the last row from this value for every row, in a block or not, and stop and report when the agent or group at that row no longer matches. A block row whose log entry has no iteration is refused, with a message naming the row. A run written by an earlier Runner (an Execution Log with a `Stage` column) cannot be resumed.
- **Error marker.** A `BLOCKED` row's `Summary` ends with the error code as a marker such as `[error:E501]`. The Runner reads the marker back to count `E501` attempts, so leave it in place if you edit the log by hand. `HARNESS_ERROR` rows have no error code and no marker; the Runner counts them by their status.
- **On Findings.** The target is the row the `On Findings` cell names, matched to a `Subagent` cell exactly (including a mode qualifier). A cell that matches no row is refused when the workflow is read.
- **Version drift.** A run is pinned to its workflow only by the `workflow_version` string, and the table is re-read on every start, including resume. The recorded-row check catches only edits that change the agent or group at a recorded row. Workflow authors must bump the workflow version whenever they edit a routing table; the Runner then refuses to resume older runs against the changed table unless version drift is explicitly allowed (`--allow-version-drift`).

---

## Checkpoints and Commits

### Checkpoints

When enabled, the Runner dispatches a checkpoint infrastructure agent at configured intervals to snapshot the working tree state.

```sh
--checkpoints enabled --infra-class "checkpoint=checkpoint-manager-git"
```

### Commits

When enabled, the Runner dispatches a commit infrastructure agent to commit completed stage work to the branch.

```sh
--commits enabled --infra-class "commit=commit-manager-git" --commit-branch mosaic-owned
```

| Branch variant | Behavior |
|---------------|----------|
| `mosaic-owned` | Runner manages a dedicated branch |
| `user-own` | Commits go to the user's current branch |

---

## Codex

Codex (`--harness codex`) is an [experimental harness](#experimental-harnesses). It works only through `mosaic-run`: native orchestration inside Codex is not supported. The Runner loads each deployed agent from `.codex/agents/<id>.toml` itself on every invocation and starts one `codex exec` call per dispatch with the agent's model, sandbox, settings and instructions. It never modifies your agent files or your Codex configuration. Deploy the workspace for the `codex` harness first (see the [Deployment Guide](DeploymentGuide.md#codex-experimental)).

### One-time setup

- **Trust the workspace in Codex.** Codex reads the project's `.codex` configuration (agents and MCP servers) only for trusted projects. Mark the workspace as trusted in Codex (`trust_level = "trusted"` for the workspace path, or one of its parent folders, under `[projects]` in the Codex `config.toml`).
- **Configure the `contact-user` MCP server.** Agents reach you through an MCP server named `contact-user`. Configure it as `[mcp_servers.contact-user]` in your Codex home `config.toml` (`$CODEX_HOME/config.toml` when `CODEX_HOME` is set, otherwise `~/.codex/config.toml`), or in the project's `.codex/config.toml` (the project file counts only for a trusted workspace). A server with `enabled = false` does not count. Every invocation pre-approves its `ask_user_questions` and `get_answered_questions` tools, so no approval prompt interrupts a run.

### Platform prerequisites

- **Codex CLI 0.169 or newer.** Older versions are not supported.
- **Windows.** Codex runs its commands in a sandbox, and on Windows that sandbox needs a one-time setup that Codex performs itself; see the Codex documentation on the Windows sandbox. If the sandbox cannot be set up, the invocation fails and the Runner records it as a `HARNESS_ERROR` like any other harness failure.
- Codex does not need to run inside a Git repository; the Runner starts it with the repository check turned off, in the workspace folder.

### Start-of-run checks

At every start and resume with `--harness codex`, the Runner reads your Codex configuration (read-only) and warns when the workspace is not trusted, when no enabled `contact-user` server is configured, or when either check cannot be decided (for example an unreadable `config.toml`). In the CLI the warnings are printed and the run continues. In the TUI a definite problem (not trusted, or no `contact-user`) opens one question, "Proceed despite the Codex setup problems?"; answering no, or cancelling, ends the run before anything starts. A check that could not be decided only warns.

### Network access of deployed agents

Deploy turns an agent's generic tools into Codex settings, and the Runner applies them as written:

| Agent's tools | Result in Codex |
|---------------|-----------------|
| `web_search` | web search `live`; without `web_search` it is `disabled` |
| `web_fetch`, agent can write files | network access inside the workspace-write sandbox |
| `web_fetch`, read-only agent | no network access; Deploy reports the withheld `web_fetch` in its plan |
| `terminal` alone | no network access |

### What a Codex run does not show

- **No MOSAIC log invocation events.** A Codex run writes only the run start and run end events to the MOSAIC log, so per-invocation analysis in LogAnalyzer is not available for Codex runs.
- **Activity panel.** Tool calls and messages appear as for other harnesses, but the header shows no model, no context size and no cost.

---

## OpenCode V2

OpenCode V2 (`--harness opencode-v2`) is an [experimental harness](#experimental-harnesses): everything is supported, but it is not yet tested in real use. Deploy the workspace for the `opencode-v2` harness first (see the [Deployment Guide](DeploymentGuide.md#opencode-v1-vs-v2-opencode-v2-is-experimental)), and read the [OpenCode V2 warnings](GettingStarted.md#opencode-v2-warnings) before installing it.

- **Agent loading.** Every dispatch is one standalone `run` of the agent by id, in the workspace root, with the deployed file in `.opencode/agents/` used in place. There is no backup-and-transform (see [Agent Snapshot Directories](#agent-snapshot-directories)). The Runner passes no model: the agent file's own `model:` applies. Timeout and force stop end the whole process tree, which ends the session with it.
- **Orchestrator.** Auto-discovery finds `.opencode/agents/orchestrator-script.md`.
- **No-output watchdog.** V2 runs have been seen to hang silently. An invocation that writes nothing to its output stream for the watchdog time is treated as hung and ended like a timeout; the failure is recorded as a `no-output-timeout` harness error with the silent time. The default is 10 minutes, well above V2's legitimate silences (a long shell command, or the wait for background subagents). Configure it with `--no-output-timeout <duration>` (for example `20m`; `0` disables it); in the TUI the run configuration asks for it when `opencode-v2` is selected. It never applies to dispatches with human-in-the-loop, because an ask-the-user call is silent, and the normal `--timeout` and `--hitl-timeout` still apply.
- **Live activity.** The progress screen shows tool calls, messages and the context size. It shows no money cost for `opencode-v2`; the cost of a run comes from LogAnalyzer.
- **MOSAIC log.** The Runner writes the run start and run end events, and an invocation end for an invocation the logger plugin left without one. The mosaic-logger `opencode-v2` plugin writes the rest; see its [README](../Catalog/Hooks/mosaic-logger/README.md) and [`LIMITATIONS.md`](../Catalog/Hooks/mosaic-logger/opencode-v2/LIMITATIONS.md).
- **Human-in-the-loop in headless runs.** V2's `question` tool is auto-cancelled in headless runs ("make reasonable assumptions"), so it never reaches you. Headless human-in-the-loop on `opencode-v2` needs an ask-the-user MCP server that you supply and configure in OpenCode. Deploy denies every tool an agent does not declare, so you must also add a custom tool mapping in the Deploy configuration, so that agents declare the ask tool and it is allowed; without it the tool is denied. The steps are in the [Deployment Guide](DeploymentGuide.md#opencode-v1-vs-v2-opencode-v2-is-experimental).

---

## Experimental Harnesses

A harness is experimental when its support is not yet proven in real runs. The Runner keeps its own marking, separate from Deploy's: a harness can be experimental in one tool and not in the other. An experimental harness stays fully selectable. The TUI harness pickers show it with the word "Experimental" and the reason, and the CLI prints one warning with the harness name and reason at every start and resume. The warning never stops a run.

Experimental harnesses today: **Codex** and **OpenCode V2**. For OpenCode V2 the reason shown is "Everything is supported, but the harness is not yet tested."

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Run completed successfully |
| 1 | Unexpected infrastructure error |
| 2 | Stopped (graceful stop, or a stop by the Runner itself; state saved, resumable) |
| 3 | Invalid arguments |
| 4 | Workflow or artifact refused before any invocation |
| 5 | Deviation unresolved (state saved, resumable) |
| 6 | Stopped by orchestrator consultant, a failed consultation, or an ended manual routing dialogue (resumable) |
| 7 | Commit setup or pre-consultation failed after the run folder was created (kept; resuming retries it) |

---

## Quick Reference

| I want to... | Do this |
|--------------|---------|
| Run a workflow interactively | `mosaic-run` (no subcommand) |
| Run a workflow from CLI | `mosaic-run run --workflow <id> --task "<desc>" --harness claude-code --mode orchestrated` |
| Use the cheapest mode | `--mode auto-review` |
| Get the best task descriptions | `--mode orchestrated` |
| Skip the environment guidance in auto modes | `--pre-consult=false` |
| Resume a stopped run | `--run <run_id>` |
| Preview without a real harness | `--harness fake` |
| Enable checkpoints | `--checkpoints enabled --infra-class "checkpoint=checkpoint-manager-git"` |
| Enable stage commits | `--commits enabled --infra-class "commit=commit-manager-git"` |
| Set a longer timeout per invocation | `--timeout 1h` |
| Allow people more time to answer | `--hitl-timeout 8h` |
| Stop the running invocation at once | `x` on the progress screen, then `y` |
| Choose the next dispatch yourself when a consultation fails | `--manual-resolution` (needs the TUI) |
