# LogAnalyzer Guide

How to start `mosaic-log-analyzer`, read its views, share runs and configure its pricing and analysis settings.

This guide is for **users of the log analyzer**. The tool reads the logs MOSAIC's logging hook writes (`OrchestrationLogs/`) and reports token usage, estimated cost, context size, compactions and cache behaviour per run.

---

## At a Glance

| Concept | Summary |
|---------|---------|
| **Start it** | Run `mosaic-log-analyzer` in your project folder for the interactive TUI |
| **Share a run without its content** | `mosaic-log-analyzer export --out run.digest.json`, or `e` in the TUI; see [Sharing Runs](#sharing-runs-without-content) |
| **Two config levels** | Global (your user config folder, all projects) and project (`MosaicLogAnalyzer/config/` under the folder you start the tool from) |
| **Two config files per level** | `pricing.yaml` (model prices) and `analysis.yaml` (analysis settings) |
| **Where are they?** | `mosaic-log-analyzer pricing path` and `mosaic-log-analyzer settings path`; the TUI help overlay lists them too |
| **Missing usage on VS Code GHCP Local** | `mosaic-log-analyzer ingest-otel <otel-file> --run <run-id>`. The other harnesses need no recovery |
| **Transcript format changed** | A finding on a GHCP CLI run whose transcript has assistant turns but no usage: the run's usage and cost are incomplete |
| **Currency** | One `currency` key (`USD` or `EUR`) in the pricing file; no conversion |

---

## Starting the Tool

Without a subcommand the TUI opens the `OrchestrationLogs/` folder under the current directory, and asks for a source when there is none. To open something else, use `--path` or press `s` on the runs list. A source can be:

- an `OrchestrationLogs/` folder,
- a single run folder,
- a `.digest.json` file, or a folder of digests.

Output sent to a pipe or file skips the TUI; `--tui` forces it.

### CLI commands

| Command | What it does |
|---------|--------------|
| `report` | Full report of the source, as text or with `--format json` |
| `total --run <run-id>` | The total of one run as JSON. Reads logs, not digests |
| `export --out <file>.digest.json` | Writes a [digest](#sharing-runs-without-content); `--run <run-id>` for one run, `--force` to replace a file |
| `ingest-otel` | [Recovers usage on VS Code GHCP Local](#token-usage-on-vs-code-ghcp-local) |
| `pricing path`, `settings path` | Prints the [configuration files](#where-the-files-live) |

Every command takes `--path`.

### The runs list

The first screen lists the runs of the source with their cost, invocations, duration and findings. `enter` opens a run. Its other keys:

| Key | Action |
|-----|--------|
| `o` | Cycle the sort order |
| `m`, then `c` | Mark two runs (A, then B) and compare them by agent type: invocations, tokens, cost and peak context, with the change from A to B |
| `a` | Across-runs view: every run's cost, and each agent type summed over all runs |
| `e` / `E` | Export the selected run / all runs as a digest |
| `f` | Open the [findings overlay](#findings-overlay-and-export) |
| `p` | Enter model prices |
| `s` | Choose another log source |

#### The head of the runs list

Above the table, three lines sum up the runs:

```
All runs  12 runs (2 in progress) · Jan 3 – Jan 9 · total $41.20 · 5.2M tokens
Per run   cost  avg $3.43 · median $2.90 · p90 $7.10   (over 9 of 12 runs)
          time  avg 14m 20s · median 11m 05s   (over 10 of 12 runs)
```

- **All runs** shows the number of runs (and how many are still in progress), the period they cover, the total cost and the total tokens. On a narrow terminal the period is dropped first.
- **Per run** shows what one run typically costs and how long it lasts. The **average** is the sum divided by the number of runs counted. The **median** is the middle value of the runs sorted by size; with an even number of runs it is the mean of the two middle values. The **p90** is the value that 90 % of the runs stay at or below (nearest rank: the run at position 0.9 times the count, rounded up). It is shown only with at least 10 priced runs.

Which runs count:

- A run still in progress counts in the run total and the period, but not in the cost and time statistics.
- A run with a model that has no price is left out of the cost statistics. A run without a timeline is left out of the time statistics.
- Usage that cannot be attributed to a run never counts in the statistics.
- When a statistic covers fewer runs than the list has, the note `(over X of Y runs)` says how many it covers. With no run to count, the line reads `no priced runs` or `no timed runs`.

#### Findings overlay and export

Press `f` on the runs list, or inside a run, to see what the log analysis found. The overlay shows one row per kind of finding: its severity (`warning` or `info`), how many findings of that kind exist, the kind, and an explanation of what it means and what it does to the figures. On the runs list the rows also say `in N runs`, the number of runs the kind occurs in.

- **Scope:** on the runs list the overlay covers every finding of the source. Inside a run it covers only that run's findings.
- **Order:** warnings before infos, then the larger count first, then the kind name.
- **Keys:** `up`/`down` scroll, `e` exports, `f` or `esc` closes. While the overlay is open it takes all keys.

`e` asks for a file name and writes the findings the overlay shows, so the file holds exactly the counted findings. The file name is pre-filled with `findings.json` on the runs list and `findings-<run>.json` inside a run. It must end in `.json` but not in `.digest.json`. A file that already exists is replaced only after you confirm.

The file has a `counts` object (findings per kind) and a `findings` list. Every finding has `kind`, `severity`, `path`, `line`, `run_id`, `actor` and `detail`; none is left out when empty. Findings that come from a digest have an empty `detail`, because a digest does not keep it.

##### Finding kinds

| Kind | What it means |
|------|---------------|
| `malformed_line` | a log line is not valid JSON; it is skipped, so its event is missing from the figures |
| `truncated_line` | the last line of a log file was cut off mid-write; it is skipped, so its event is missing from the figures |
| `unreadable_file` | a log file could not be read; none of its events are in the figures |
| `unknown_schema_version` | a log line uses a schema version this tool does not know; it is skipped, so its event is missing from the figures |
| `unrecognised_harness` | an event comes from a harness this tool does not recognise; it is skipped, so its event is missing from the figures |
| `missing_token_usage` | a model call has no token usage; it counts as zero tokens and its cost is incomplete |
| `missing_model` | a model call names no model; its cost cannot be priced and is left out of the totals |
| `unidentified_event` | an event could not be tied to an agent or dispatch; it is counted in the totals but not attributed |
| `unrecognised_folder` | a folder in the log directory is not a run or actor folder; it is ignored |
| `missing_event_file` | a run or actor folder has no event file; nothing is read from it, so its figures are missing |
| `invalid_pricing_entry` | a pricing entry is invalid; it is ignored, so the models it covers may be unpriced |
| `invalid_settings_entry` | a settings entry is invalid; it is ignored and the default applies |
| `cross_stream_record` | the same usage record appeared in two actors' logs; counted once, for the first actor read |
| `invalid_cache_tier_split` | cache-write tokens split across tiers do not add up; the figures use the assumed tier for them |
| `compaction_mismatch` | a context compaction does not match the surrounding token counts; the context growth figures may be off |
| `invalid_digest` | a digest file is not valid; it is ignored, so its runs are missing from the report |
| `unsupported_digest_version` | a digest file has a version this tool does not support; it is ignored, so its runs are missing |
| `duplicate_run` | the same run was found more than once (logs and digest); it is counted once |
| `transcript_format_changed` | the transcript format differs from what this tool expects; transcript-based figures may be incomplete |
| `pricing_currency_mismatch` | a pricing entry uses a different currency than the rest; its amounts are not converted, so totals may mix currencies |
| `instance_type_mismatch` | an agent instance reports different agent types in different places; the figures use the first one seen |
| `unresolved_invocation` | an invocation could not be tied to a dispatch; it is counted but not attributed to one |
| `overlapping_invocation_start` | an invocation started while another of the same agent was still open; the overlap may blur per-invocation figures |
| `dispatch_without_invocation` | a dispatch has no matching invocation; it counts as a dispatch with no recorded work |
| `transcript_capture_gap` | part of an agent's transcript was not captured; its usage and cost may be incomplete |
| `instance_prompt_mismatch` | an agent instance received a prompt that differs from its dispatch; the figures are kept but may be mis-attributed |

The `report` command lists the same findings; new kinds appear there as additional entries.

---

## What the Views Show

A run has seven views, on keys `1` to `7`. Press `?` on any screen for its keys and a **Columns** group with one sentence per column; views with several tables have one group per table. `o` sorts a table and `esc` goes back.

| Key | View | Shows |
|-----|------|-------|
| `1` | **Dispatches** | Every dispatched instance with its status, cost, tokens, duration, peak context, cache figures and flags |
| `2` | **Overview** | Cost split, token categories, models, outcomes, wall time, waits and the cache block. `v` switches between the visual and the figures-only version |
| `3` | **By type** | Cost and context per agent type, including repeated dispatches and likely rework (a heuristic). `enter` lists a type's dispatches |
| `4` | **By status** | Dispatches and cost per returned status code |
| `5` | **Tools** | Tool use and [context growth](#context-growth-per-tool). `u` marks a tool as user-interaction tool |
| `6` | **Timeline** | [Lanes per actor](#timeline). `v` switches between Gantt and list, `g` groups the lanes by agent type |
| `7` | **Context** | [Context size per agent](#context) and compactions |

### Session screen

Press `enter` on a row of Dispatches, of a By type drill-down, or of Context to open the Session screen of that actor (the orchestrator or one dispatched instance). It shows:

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
- The tools each call requested are logged on all four harnesses. **VS Code GHCP Local needs OpenTelemetry content capture** for it; without it the requested tools are `not provided` and no growth is measured. See [Token Usage on VS Code GHCP Local](#token-usage-on-vs-code-ghcp-local).

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
| **harness (OTel)** | No hook event, but the context drops and OTel compaction calls (API requests that summarise the context) sit around the drop. This is how compactions on VS Code GHCP Local are recognised |
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

## Sharing Runs Without Content

A **digest** is a statistics-only copy of runs, so you can share or analyse them elsewhere without their prompts, responses, tool input or tool output.

```sh
mosaic-log-analyzer export --out run.digest.json --run <run-id>
```

Leave out `--run` to export every run; in the TUI, press `e` or `E`. The file name must end in `.digest.json`. To read a digest, open it as a [source](#starting-the-tool): all views work on it.

- **What stays in a digest:** token counts, model and provider names, timestamps, agent types, status codes, tool names (including MCP server names), and run, session and call ids. Check these are fine to share.
- **Costs use the reader's prices.** A digest holds tokens, not money, so the same run can show a different cost with a different `pricing.yaml`.
- **Findings** keep their kind but lose their detail text.
- **Digests beside logs are merged.** A digest in a logs folder adds its runs to the list. If a run exists twice, the logs win, then the most recently written digest; a finding reports the discarded copy.

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
- **Settings:** the built-in defaults come first, the global file overrides them, the project file overrides the global file. This applies per setting and, for tool lists, per harness. A harness list you write **replaces** that harness's default list, and an empty list means no tools.

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

Requests are priced by provider and model. Each model entry has five rates per million tokens, written as quoted strings (`input`, `cache_read`, `cache_write_5m`, `cache_write_1h`, `output`), and optionally one `long_context` tier with a `threshold`. The template at the head of the file shows complete examples. Lookup is strict: a request with a provider is priced only from that provider's section, a request without a provider only from the top-level `models`. A request that matches nothing is unpriced; the TUI then asks for the price. Copilot usage is priced from `github-copilot` entries you maintain yourself: the shipped file has none, so without them Copilot costs stay unpriced.

### The `currency` key

An optional top-level key states the currency of every rate in the file:

```yaml
currency: EUR
```

- Allowed values: `USD` and `EUR`. There is **no conversion**: the rates are shown and summed as stated.
- The **effective currency** of a session is the project file's currency, else the global file's, else `USD`. A file without the key is read in the effective currency.
- **Mismatch rule:** if both files state **different** currencies, the global entries are ignored (except models the project file also prices) and a warning names the ignored models. Fix it by aligning the two `currency` keys.

---

## Token Usage on VS Code GHCP Local

Only the **VS Code GHCP Local** target needs this. Every other target reads usage from the harness's own session data, with no setup and no recovery. If a GHCP CLI transcript's format changes so that no usage can be read, the run shows a finding instead of a silent zero.

VS Code GHCP Local reports usage through Copilot's OpenTelemetry file export, which the logging hook reads. To set it up:

1. Set the environment variable `COPILOT_OTEL_FILE_EXPORTER_PATH` to the path of a `.jsonl` file. This one variable enables both Copilot's export and MOSAIC's ingest. Do not use the VS Code user setting for the export instead: the hooks cannot read it.
2. Set it at user level (Windows user environment variables; Linux login environment), not in a single shell, and restart VS Code afterwards. VS Code started from the desktop sees only user-level variables.
3. Content capture is optional for usage and only makes the file larger, but it is needed for the tools each call requested, and so for context growth per tool. MOSAIC never copies content to the logs.

The same export also lets the hook:

- record **compaction calls** and other helper calls with their kind (this is how compactions become visible on this target), and
- complete a **failed tool call** with its end event when the harness fires no post-tool hook for it.

### Recovering missing usage

When usage is missing from a run (variable unset, file missing, session crashed, hooks did not see the variable), recover it:

```sh
mosaic-log-analyzer ingest-otel <otel-file> --run <run-id>
```

- `--run` is required; add `--path` when the logs are not in the default `OrchestrationLogs/`.
- Without the file argument, the command uses `COPILOT_OTEL_FILE_EXPORTER_PATH`.
- It ingests the whole file as a final ingest and prints `run <id>: N records written, N finalised as unattributed, N pending`.
- "Usage not fully ingested" in the views means usage of the run is still pending; the recovery ingest resolves it.
- **Unattributed** usage is usage tied to the run but not to an agent.

The OTel file is never modified, and no prompt or response content is copied to the logs.
