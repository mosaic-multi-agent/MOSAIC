# LogAnalyzer Guide

How to read the views of `mosaic-log-analyzer` and how to configure its pricing and analysis settings.

This guide is for **users of the log analyzer**. The tool reads the logs MOSAIC's logging hook writes (`OrchestrationLogs/`) and reports token usage, estimated cost, context size, compactions and cache behaviour per run. Start it without arguments for the interactive TUI, or use the CLI subcommands (`report`, `total`, `export`, `ingest-otel`, `pricing path`, `settings path`).

---

## At a Glance

| Concept | Summary |
|---------|---------|
| **Two config levels** | Global (your user config folder, all projects) and project (`MosaicLogAnalyzer/config/` under the folder you start the tool from) |
| **Two config files per level** | `pricing.yaml` (model prices) and `analysis.yaml` (analysis settings) |
| **Where are they?** | `mosaic-log-analyzer pricing path` and `mosaic-log-analyzer settings path`; the TUI help overlay lists them too |
| **Missing VS Code usage** | `mosaic-log-analyzer ingest-otel <otel-file> --run <run-id>` (GHCP CLI usage comes from its transcript and needs no recovery) |
| **Transcript format changed** | A finding on a GHCP CLI run whose transcript has assistant turns but no usage: the run's usage and cost are incomplete |
| **Currency** | One `currency` key (`USD` or `EUR`) in the pricing file; no conversion |

---

## What the Views Show

The run screen has these views as tabs: Dispatches, Overview, By type, By status, Tools, Timeline and Context. Open the help overlay (`?`) in the TUI for the keys of the current screen.

### Tables and the column legend

Tables are aligned, stacked tables with the instance names always visible. Column titles wrap onto two lines or show in full instead of being cut. The Dispatches, Timeline and Context lists no longer have a `type` column (the instance name already carries the agent type), the Context view's call count is titled `API calls`, and shares show the percent next to the share bar.

The `?` overlay of every view with a table has a **Columns** group for that view, with one sentence per column. It replaces the notes that used to sit below the Context table. Views with several tables (the Tools view, the Session screen) have one group per table.

### Session screen

Press `enter` on a row of Dispatches, of a By type drill-down, or of Context to open the Session screen of that actor (the orchestrator or one dispatched instance). It replaces the old Context drill-down and shows:

- the **head**: who the actor is, its model, status and totals, and a **cost split** of what its calls cost,
- the **findings** about the actor, as measured facts,
- the **per-call table**: for each API call its time, model, kind (main or compaction), context, tokens, cost, the **requested tools**, and the **context growth** after the call, with notes for the first write, compactions and rewrites,
- the **actor's tools**: the tools it used with their figures and growth.

The requested tools show `not provided` when the log does not record them (see below).

### Context growth per tool

The Tools view, the Session screen and the JSON report (`report`, additive `context_growth` object per tool with `samples`, `average_tokens`, `min_tokens`, `max_tokens`, `negative`, `not_measurable`) show how much the context grew after a tool was called. What it means:

- It is **measured on single-tool calls only**: an API call that requested exactly one tool, followed by the actor's next main call with no compaction and no user turn in between. A call that requested several tools cannot say which tool added what, so it is not a sample.
- It is **the tokens the tool's result adds to the context**, read as the difference between the two calls' contexts. It is **not a cost split of the run**: output of the model and everything else that changes the context between the two calls is part of the difference, and the sum over tools does not equal the run's cost or context.
- **Samples** are the calls that were measured. **Negative growth** counts samples where the context shrank (for example after a compaction) and **not measurable** counts calls whose growth could not be determined; both are left out of the average, minimum and maximum, and shown as counts so you can judge how many calls the figures rest on. With no sample the figures are empty.
- The tools each call requested are logged on all four harnesses. **VS Code needs OpenTelemetry content capture** for it; without it the requested tools are `not provided` and no growth is measured. See [Getting Started](GettingStarted.md#token-usage-on-copilot-harnesses).

### Timeline

The Timeline draws one lane per actor. Two kinds of time are shaded, each with its own shade and a legend under the chart that names them:

| Shade | Meaning |
|-------|---------|
| `waiting on the user` | An actor waited for the user (for example for an answer to a question) |
| `between sessions` | Time between two sessions of the run in which nobody worked |

Compaction markers (logged and detected) are in the legend as well. Idle stretches **longer than 10 minutes** with nobody working (a wait on the user, or time between sessions) are collapsed: the chart does not give them their full width but marks them with their length, so a long break does not squeeze the working time into a sliver.

### Context

The Context view shows how large each agent's context was over the run, with a compaction column group (logged, harness, detected only, unapplied, cost). Figures:

- **Peak context**, **Final context** and **First write context** (marked `*`, a heuristic) per actor.
- Only **main calls** (the agent's own conversation steps) are used for context figures. Compaction calls and harness helper calls still count for usage and cost, but not for context size.
- Every context size falls into a band, always named as text next to the figure:

| Band label | Range |
|------------|-------|
| `optimal` | below 250,000 tokens |
| `above optimal` | 250,000 to below 350,000 |
| `degraded` | 350,000 and above |

The context curve on the Session screen draws one guide per band.

### Compactions

A compaction replaces a long context with a shorter one. The Context view and the Session screen count three kinds per actor:

| Kind | Meaning |
|------|---------|
| **logged** | The harness hook wrote a `compaction` event to the MOSAIC log |
| **harness (OTel)** | No hook event, but the context drops and OTel compaction calls (API requests that summarise the context) sit around the drop. This is how VS Code compactions are recognised |
| **detected only** | A context drop with no logged compaction and no compaction calls |

A drop counts as detected when the context falls by at least `detected_compaction.drop_percent` (default 30) between two consecutive calls; see [Settings](#analysis-settings-analysisyaml).

Each harness compaction line shows the context before and after, the number of compaction calls it took and their cost. Two more lines can follow:

- **Unapplied compaction calls**: count and cost of compaction calls whose result no context drop shows (the harness requested a summary but did not apply it).
- **Compaction calls in total**: count and cost of all compaction calls. This cost is already part of the actor's spend.

Where a logged compaction and a context drop disagree (no drop near a logged compaction, or no logged compaction near a drop), the run shows a data-quality finding.

### Cold rewrites

The Overview "Cache" block shows `Context rewrites` for the run and `Orchestrator rewrites` for the orchestrator alone, for example `3 (model changed 1, cold 1, partial 1)`, with the tokens rewritten, the cost and the extra cost. A call is a rewrite when it writes at least half of the previous call's context to the cache and reads less than half of it from the cache. The drill-down labels each rewrite with its cause:

| Label | Cause |
|-------|-------|
| `model changed` | The model differs from the previous call's model, so the cache did not carry over |
| `cold after N min idle` (or `cold after <1 min idle`) | Nothing was read from the cache. N is the **pause**: the minutes between this call's start and the previous call's start |
| `partial rewrite` | Part of the context was read, the rest written again |

**Extra cost** is what the cache writes cost over reading the same tokens at the cache-read rate. It is unpriced when the model has no price.

### Hidden columns

When a table does not fit the terminal width, it drops its lowest-priority columns and marks the header with `+N >` (`+N` and an arrow; the arrow is `>` in ASCII mode), where N is the number of hidden columns. Widen the terminal to see more. The Session screen shows the fields of an actor in full, so nothing is lost to the width.

---

## Configuration Levels

Two files exist at two levels:

| Level | Applies to | Pricing file | Settings file |
|-------|-----------|--------------|---------------|
| **Global** | all projects of your user | `pricing.yaml` | `analysis.yaml` |
| **Project** | the folder you start LogAnalyzer from | `MosaicLogAnalyzer/config/pricing.yaml` | `MosaicLogAnalyzer/config/analysis.yaml` |

### Where the files live

The global folder is `MOSAIC/LogAnalyzer` inside your operating system's user config folder:

| OS | Global folder |
|----|---------------|
| Windows | `%AppData%\MOSAIC\LogAnalyzer\` (for example `C:\Users\you\AppData\Roaming\MOSAIC\LogAnalyzer\`) |
| Linux | `$XDG_CONFIG_HOME/MOSAIC/LogAnalyzer/`, or `~/.config/MOSAIC/LogAnalyzer/` when `XDG_CONFIG_HOME` is not set |

If the operating system has no user config folder, the global level is unavailable and only the project level is used.

The project files are relative to the **working directory** LogAnalyzer is started from, for example `<your project>/MosaicLogAnalyzer/config/pricing.yaml`.

Print the exact paths and whether each file exists:

```sh
mosaic-log-analyzer pricing path
mosaic-log-analyzer settings path
```

Output is one `global:` line and one `project:` line, each marked `(exists)` or `(absent)`. The TUI help overlay shows the same four files under "Configuration files".

### How the levels merge

- **Pricing:** entries of both files are combined. When both files price the same provider and model, the **project entry wins**.
- **Settings:** the built-in defaults come first, the global file overrides them, the project file overrides the global file. This applies per setting and, for tool lists, per harness. A harness list you write **replaces** that harness's default list.

### When files are created

- The **global** files are created with a commented template when LogAnalyzer starts and they are absent. An existing file is never overwritten. If creation fails, a warning is printed and the program continues.
- The **project** files are not created at start. They are created, with the same template ahead of the entry, the first time you save something at project level.
- Reading never creates a file.

### Saving asks for the level

Nothing in the CLI writes configuration. In the TUI, when you enter a missing price or mark a tool as user-interaction tool, LogAnalyzer asks "Save for all projects or for this project only?" (`All projects` = global file, `This project` = project file) and shows both file paths. When the global level is unavailable its option is disabled. Skipping the question saves nothing. The confirmation names the file written.

### Analysis settings (`analysis.yaml`)

All keys are optional; a missing key keeps the default. The template in the file lists every default tool name.

| Key | Meaning |
|-----|---------|
| `write_tools` | Tool names that write file content, per harness |
| `user_interaction_tools` | Tool names that ask the user a question, per harness |
| `detected_compaction.drop_percent` | Context fall (1 to 99 percent) between two calls that counts as a detected compaction. Default 30 |
| `default_cache_write_tier` | Per harness, the cache-write tier that prices cache writes without a tier split. Default `5m` |

Harness keys are the names as logged: `claude-code`, `ghcp-cli`, `opencode`, `vscode-ghcp`. Tool names are exact or patterns where `*` matches any run of characters; quote patterns.

```yaml
user_interaction_tools:
  claude-code: [AskUserQuestion, "mcp__contact-user__*"]
default_cache_write_tier:
  claude-code: 1h
detected_compaction:
  drop_percent: 30
```

#### Default cache-write tier

Some harnesses report cache writes without saying whether they used the 5-minute or the 1-hour tier. Those writes are priced with the harness's `default_cache_write_tier` (`5m` or `1h`, default `5m`). Records that carry the split are always priced as reported.

---

## Pricing (`pricing.yaml`)

Requests are priced by provider and model. Each model entry has five rates per million tokens, written as quoted strings (`input`, `cache_read`, `cache_write_5m`, `cache_write_1h`, `output`), and optionally one `long_context` tier with a `threshold`. The template at the head of the file shows complete examples. Lookup is strict: a request with a provider is priced only from that provider's section, a request without a provider only from the top-level `models`. A request that matches nothing is unpriced; the TUI then asks for the price.

### The `currency` key

An optional top-level key states the currency of every rate in the file:

```yaml
currency: EUR
```

- Allowed values: `USD` and `EUR`. There is **no conversion**: the rates are shown and summed as stated.
- The **effective currency** of a session is the project file's currency, else the global file's, else `USD`. A file without the key is read in the effective currency.
- **Mismatch rule:** if both files state **different** currencies, the global entries are ignored (except models the project file also prices) and a warning names the ignored models. Fix it by aligning the two `currency` keys.

---

## Token Usage Recovery (`ingest-otel`)

**GHCP CLI** usage and compactions come from the CLI's session transcript and need no setup, so `ingest-otel` does not apply to it. **VS Code Copilot Chat** reports usage through Copilot's OpenTelemetry file export, which the logging hook reads. Setup (VS Code only) is described in [Getting Started](GettingStarted.md#token-usage-on-copilot-harnesses). The same export also lets the hook:

- record **compaction calls** and other helper calls with their kind (this is how VS Code compactions become visible), and
- complete a **failed tool call** with its end event when the harness fires no post-tool hook for it.

When VS Code usage is missing from a run (variable unset, file missing, session crashed, hooks did not see the variable), recover it:

```sh
mosaic-log-analyzer ingest-otel <otel-file> --run <run-id>
```

- `--run` is required; add `--path` when the logs are not in the default `OrchestrationLogs/`.
- Without the file argument, the command uses `COPILOT_OTEL_FILE_EXPORTER_PATH`.
- It ingests the whole file as a final ingest and prints `run <id>: N records written, N finalised as unattributed, N pending`.
- "Usage not fully ingested" in the views means usage of the run is still pending; the recovery ingest resolves it.
- **Unattributed** usage is usage tied to the run but not to an agent.

The OTel file is never modified, and no prompt or response content is copied to the logs.
