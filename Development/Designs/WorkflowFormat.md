# Workflow Format

> **Status:** Final
> **Version:** 2.16
> **Created:** 2026-10-09
> **Last Updated:** 2026-10-10
> **Scope:** The schema of a workflow definition file — `Catalog/Workflows/{Category}/{id}.md`. Defines the frontmatter, the `<Workflow>` region, the routing table and the meaning of every cell notation, blocks (nested included), the parallel (fork / join) and group extensions, the contract with block schedules, the rules that make a workflow valid, and the free-form sections that follow the region. This is the authoring reference for anyone writing a workflow, with or without the workflow-creator agent.

---

## Contents

- [1. Purpose](#1-purpose) — what the format binds (tables, not executors) and what lives elsewhere
- [2. Design Principles](#2-design-principles)
- [3. File Model](#3-file-model) — where workflow files live; the three parts of a file
- [4. Frontmatter](#4-frontmatter)
- [5. The Workflow Region](#5-the-workflow-region) — the deployed part; tag, tables, notes, whitespace, ASCII
- [6. Routing Table](#6-routing-table)
  - [6.1 Columns](#61-columns)
  - [6.2 Phase](#62-phase) — phase names, block references and groups in the Phase cell
  - [6.3 Subagent](#63-subagent) — agent ids, mode qualifiers, what a target is
  - [6.4 HITL](#64-hitl)
  - [6.5 Scopes, On Success and On Findings](#65-scopes-on-success-and-on-findings) — scopes, `NEXT`/`COMPLETE`, block entry, exit and re-entry
  - [6.6 The Row column](#66-the-row-column)
  - [6.7 Input and Output](#67-input-and-output) — fill-ins, wildcards, iteration isolation, seeds, artifact flow
  - [6.8 Blocks](#68-blocks) — terms and rules for blocks, including nesting
- [7. Parallel Work](#7-parallel-work) — fixed (forks/joins) vs planned (block iterations)
  - [7.1 Fork](#71-fork)
  - [7.2 Join](#72-join)
  - [7.3 Every fork closes](#73-every-fork-closes)
- [8. Groups](#8-groups) — per-iteration row orderings selected by `Approach`
  - [8.1 Phase notation and activation](#81-phase-notation-and-activation)
  - [8.2 The approach table](#82-the-approach-table)
  - [8.3 Omission, ordering, repetition](#83-omission-ordering-repetition)
  - [8.4 Contiguity](#84-contiguity)
  - [8.5 How iterations appear in a run](#85-how-iterations-appear-in-a-run)
  - [8.6 Vocabulary is the workflow's](#86-vocabulary-is-the-workflows)
- [9. Block Schedules](#9-block-schedules) — where a block's iterations come from
  - [9.1 The blocks table](#91-the-blocks-table)
  - [9.2 The block schedule](#92-the-block-schedule)
  - [9.3 Producers and changes](#93-producers-and-changes) — who writes a block schedule and what may change mid-run
- [10. Validity Rules](#10-validity-rules) — all rules by ID — F file, T routing table, P notation, A cross-checks, B parallel, V block schedule, R routing
- [11. After the Region](#11-after-the-region) — maintainer sections and versioning
  - [11.1 Sections](#111-sections)
  - [11.2 Versioning](#112-versioning)
- [12. Complete Example](#12-complete-example) — an annotated workflow using a fork, a join and a grouped block
- [Changelog](#changelog)

---

## 1. Purpose

A workflow is a configuration table. The orchestrator carries no workflow-specific logic; it reads the routing table injected into its instructions and runs it as a state machine. Any executor — an LLM orchestrator interpreting the table, or a deterministic executor running the same table — reads the same file, so the format has to be precise enough for a parser and readable enough for an LLM or a human.

This document defines what a workflow file means. It does not define how a particular executor implements that meaning, which shapes it supports, or how it recovers when routing fails; those are executor concerns, documented with each executor.

**The rules bind tables, not executors.** Every rule here restricts what a workflow may declare and what its tables mean. None restricts what an executor may decide beyond the table: routing an executor chooses from its own policy — re-targeting findings upstream, calling back to a block schedule's producer, escalating to the user — is that executor's concern, and the scope, branch and target rules (§6.5, §7, §10) say nothing about it. Wherever this document says routing goes, stays or never leaves somewhere, it means the routes the table declares. **The table defines the happy path** — where the run goes on `SUCCESS` — and, in `On Findings`, a suggested fix target. Everything off the happy path, including where findings actually go and how the run continues after a fix, is the executor's decision. The most common case is routine, not exceptional: an agent inside a block reports that the plan itself is wrong, and the orchestrator sends the work to the planner outside the block — a route no table declares, and none needs to. An executor's obligations here are about the table: read it as defined, check it against §10, and refuse what it does not support.

What this document does **not** define:

| Concern | Where it lives |
|---|---|
| How a run records its position (`Phase`, `Iteration`, `Created In`) | `OrchestrationArtifactFormat.md` §4.1 |
| Artifact naming, and the `Plan` / `Progress` / `Review` keywords | `OrchestrationSemantics.md` §2, §4 |
| Agent naming patterns (creator/reviewer pairs, `-audit`, `-research`) | `OrchestrationSemantics.md` §3 |
| Status codes the routing columns react to | `CommunicationProtocol.md` |
| Region tag syntax shared by all catalog files | `Catalog/CatalogFilesFormat.md` |
| Reusable workflow shapes and why they work | `Catalog/Workflows/WorkflowPatterns.md` |
| What a specific executor supports, and how it routes and recovers | That executor's design document |

## 2. Design Principles

| Principle | Description |
|---|---|
| **The table is the workflow** | Everything an executor routes on is inside the `<Workflow>` region. Frontmatter is catalog metadata for tools; prose after the region is for maintainers. Neither reaches an orchestrator. |
| **Domain-free** | A workflow names agents and artifacts, never one project's conventions. Project specifics reach agents through injections, not rows (`AgentTemplateArchitecture.md` §6). |
| **Executor-neutral** | Every notation here has one meaning, whoever executes it. An executor may support a subset of the format; it must refuse a shape it does not support before the first dispatch, and never run part of it or reinterpret it. |
| **Declared, not inferred** | An executor never guesses from artifact names. Which iterations a block runs, which rows repeat and which rows an iteration selects are all declared in the region. |
| **Opaque vocabulary** | Phase names, agent ids, block names, group names and approach tokens are matched verbatim and case-sensitively. The format carries no fixed list of any of them; it reserves only `COMPLETE` and `NEXT` as `On Success` entries (§6.5), `INIT` and `COMPLETED` as phase names (§6.2), and forbids `HITL` and `Approach` as block names (§6.8). |
| **Invalid is invalid everywhere** | The validity rules (§10) belong to the format, not to an executor. A workflow that breaks one is malformed, even if some executor happens to tolerate it. |

## 3. File Model

```
Catalog/Workflows/
  Index.md                  ← registry (human lookup table)
  WorkflowPatterns.md       ← pattern reference
  {Category}/
    {id}.md                 ← one workflow per file
```

- **Location.** A workflow is a `.md` file directly inside a category folder. The category is the folder name, used verbatim. Files directly in `Workflows/`, files in deeper subfolders, and files whose name starts with `_` are not workflows.
- **Index.md is not authoritative.** Deployment loads every eligible file on disk; Index.md's content has no effect on what is loaded. It still must be kept current, because it is how humans and the workflow-creator find workflows.
- **Encoding, line endings, frontmatter fences** follow `Catalog/CatalogFilesFormat.md` (UTF-8, LF recommended, `---` fences on line 1).

A workflow file has three parts, in this order:

1. YAML frontmatter (§4)
2. The `<Workflow type="core">` region (§5) — the only part an executor sees
3. Free-form sections after the region (§11)

## 4. Frontmatter

| Field | Required | Read by | Rule |
|---|---|---|---|
| `id` | Yes | Deployment (workflow key, selection) | Kebab-case (§6.3). Equals the file's base name and the region's `name` attribute. Unique across the catalog. |
| `version` | Yes | Deployment | Quoted string, e.g. `"3.10"`. Equals the region's `version` attribute. Bumped on every region edit (§11.2). |
| `name` | Yes | Deployment (display) | Human title. |
| `description` | Yes | Deployment (display), Index.md | One sentence: what the workflow is for. Mirrors the **Use when:** line. |
| `hint` | Yes | Deployment (display), Index.md | Practical advice for choosing it — strengths, failure modes, when not to use it. |
| `author` | Yes | Index.md | `MOSAIC` for catalog workflows. |
| `referenced_agents` | Yes | Deployment (agent ordering in the deployed workspace) | Block list of every distinct agent id in the `Subagent` column, without `(mode)` qualifiers, in first-appearance order. |
| `artifacts` | Yes | Catalog tooling, maintainers | Block list of every artifact path named in the Input and Output columns, plus every `Source` in the blocks table, verbatim (fill-ins and wildcards included), in any order. A path listed in several places appears once. |

Unknown fields are ignored. The region carries no second version line (`> **Version:** …`); the region's `version` attribute is the only in-body version.

## 5. The Workflow Region

```markdown
<Workflow type="core" name="{id}" version="{version}">
## {Workflow Name}

**Use when:** {one or two sentences}

{routing table}                                   ← §6, the first table in the region

**Blocks:**                                       ← §9, workflows with blocks only
{blocks table}

**Groups:**                                       ← §8, workflows with grouped rows only
{approach table}

**Notes:**                                        ← optional, not recommended
- {caveats}
</Workflow>
```

- **Heading.** The `## {Workflow Name}` heading names the same workflow as the frontmatter `name`; the spelling may differ (`example-tdd` and `Example TDD Workflow` are not a discrepancy).

- **Tag.** Opening and closing tags on their own lines; attribute order `type`, `name`, `version` (`CatalogFilesFormat.md`, Compound names). Source files use `type="core"`; deployment rewrites it to `type="managed"` when it assembles the workflow into an orchestrator's `<AvailableWorkflows type="managed">` region.
- **Only the region is deployed.** Content between the tags is copied into every orchestrator that selects the workflow, unchanged except for two deployment edits: the `type` rewrite above and the `Row` column (§6.6). Frontmatter and everything after `</Workflow>` are not. Anything an executor needs must be inside.
- **The routing table is the first markdown table in the region.** No other table may precede it. The blocks table and the approach table each follow their reserved heading; the heading, not the position, identifies them. The region holds no other table — none in the notes either.
- **Notes are optional and not recommended.** The tables should be clear enough without them. Notes are guidance, not routing: a deterministic executor routes from the tables alone; an LLM executor also reads the notes. A rule every executor must obey has to be expressible in the tables.
- **Self-contained.** Notes do not point at text outside the region (an orchestrator cannot see it).
- **Whitespace is trimmed everywhere.** Every heading line, every table cell, and every comma-separated entry inside a cell is read with surrounding whitespace removed — in the routing, blocks and approach tables, and in block schedules (§9.2). Comparisons and character-set rules apply to the trimmed text.
- **ASCII only.** Wherever this document says letters or digits — in phase names, agent ids, block names, iteration IDs, group and approach tokens — it means ASCII letters `A`–`Z`, `a`–`z` and digits `0`–`9`. These strings reach run artifacts, which are ASCII-only (`CommunicationProtocol.md`), and file paths.
- **No extra columns.** The routing table, the blocks table and the approach table have exactly the columns this document defines (T11, P4, P6). Block schedules are the exception: they live in run artifacts and may carry columns of their own (§9.2).

## 6. Routing Table

### 6.1 Columns

Columns are identified by header name. The order below is the convention every catalog workflow follows. A routing table has no other columns (T11).

| Column | Required | Meaning |
|---|---|---|
| `Row` | **Never authored** | Added by deployment (§6.6) |
| `Phase` | Yes | Phase name, optionally with block references and a group (§6.2) |
| `Subagent` | Yes | Agent to invoke (§6.3) |
| `HITL` | Yes | `TRUE` or `FALSE` (§6.4) |
| `On Success` | Yes | Where the run goes on `SUCCESS` (§6.5) |
| `On Findings` | Yes | Fix target on `COMPLETED_NEEDS_ACTION` (§6.5) |
| `Waits For` | When, and only when, a row joins | Join barrier (§7.2) |
| `Input` | Yes | Artifacts the agent reads → `input_artifacts` (§6.7) |
| `Output` | Yes | Artifacts the agent writes → `output_artifacts` (§6.7) |

Two source table widths result (deployment adds the `Row` column, §6.6):

- 7 columns: `Phase | Subagent | HITL | On Success | On Findings | Input | Output`
- 8 columns, when some row joins (§7.2): `Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output`

A table without a join has no `Waits For` column (T13); a fork that closes at the end of the run needs no join and works in a 7-column table.

An empty `On Findings`, `Waits For`, `Input` or `Output` cell is written `-`. `On Success` is never empty.

**Rows, not agents, are the unit of execution.** The same agent may appear in several rows (a creator re-dispatched after review, a focused research pass); each row is distinct, and its `Subagent` cell is unique in the table (§6.3).

### 6.2 Phase

**Phase names are open.** A phase name is a label for the kind of work a row does; nothing routes on it. Rows run where `On Success` and blocks send them, and repetition comes from blocks, in any phase. A name uses only uppercase letters, digits and `_`, and starts with a letter (no `.`: it separates phase, block references and group). `INIT` and `COMPLETED` are reserved: `COMPLETED` marks a finished run in `current_state.phase`, and `INIT` marks user-supplied seeds in `Created In` (`OrchestrationArtifactFormat.md` §4.1, §6). Phase names appear verbatim in the run artifact (§4.1 there).

**Conventional names.** Prefer `RESEARCH`, `ARCHITECTURE`, `PLANNING`, `DESIGN`, `EXECUTION`, `REVIEW`, `COMPLETION` where one fits; they read consistently across workflows and run logs. Use only the phases the workflow needs; a phase may hold several rows. Introduce a new name only when none of these describes the work.

**Phase order is a convention, not a rule.** List the conventional phases in the order above where the work allows it. A row's phase labels the kind of work it does; it does not decide when the row runs. A workflow whose dependencies call for another order may use it — a focused research pass that only starts once planning has run (`brownfield-pr-audit`), or a planner that defines the research iterations (`hw-schema-kb-generation`).

**The Phase cell** is a phase name, optionally followed by one or more block references and then, optionally, a group, separated by `.` — in nesting order: outer block, inner block, group:

| Form | Example | Meaning |
|---|---|---|
| Plain | `PLANNING` | Runs once |
| In a block | `EXECUTION.[Stage]` | Runs once per iteration of block `Stage` (§6.8) |
| In a block, grouped | `EXECUTION.[Stage].Test` | Runs once per iteration of `Stage`, as part of group `Test` (§8) |
| Nested | `EXECUTION.[Domain].[Stage].Test` | Runs once per iteration of `Stage` within each iteration of `Domain` (§6.8) |

- A **block reference** `[Name]` is literal text, brackets included. Block references directly follow the phase name, outermost first. A block appears at most once per row.
- A **group** is the single segment after the last block reference; it belongs to that block. A group exists only inside a block: `EXECUTION.Test` is invalid.
- Iteration IDs are never written in the workflow; they come from the run (§9).

### 6.3 Subagent

The cell holds an agent id: the agent file's base name, the same id `referenced_agents` uses. Kebab-case, matched verbatim. **Kebab-case**, wherever this document requires it, means runs of lowercase letters and digits joined by single `-`, starting with a letter: `codebase-research`, `build-review`, `tdd2`. No leading, trailing or doubled `-`, so `-`, the "none" marker, is never kebab-case. `COMPLETE` and `NEXT` are reserved `On Success` entries (§6.5); being uppercase, they can never be agent ids.

| Notation | Example | Meaning |
|---|---|---|
| Mode qualifier | `codebase-research(architecture)` | The same agent in a distinct row with a focused task. The parenthetical follows the id directly, with no space, and is non-empty kebab-case like the id. It is part of the cell, so it tells the rows apart, and it is passed into the task description. `referenced_agents` lists the bare id. |

**Every `Subagent` cell is unique in the routing table** (T8). An agent that appears in one row needs no qualifier. An agent that appears in several rows is told apart by mode qualifiers — `build-review(tests)` and `build-review(impl)`; one of its rows may stay bare. The qualifier is the row's identity and its task focus at once, so it names what the row does differently.

**Targets name rows.** A target is an entry in `On Success`, `On Findings` or `Waits For` that names a row: a `Subagent` cell written exactly, meaning the one row holding that cell. `COMPLETE`, `NEXT` and block references are not targets. Cells are compared whole: `codebase-research` does not match `codebase-research(architecture)`. A target that matches no row is invalid (T9); there is no fallback. `On Success` may also name a block, `[Name]` (§6.5).

A Subagent cell never contains a fill-in (§6.8): the agent set must be known before the first dispatch, so every subagent can be validated and listed in `referenced_agents`.

### 6.4 HITL

The cell holds exactly `TRUE` or `FALSE`, uppercase. `TRUE` — the agent is dispatched with `human_in_the_loop: true` and obtains user review before returning (`CommunicationProtocol.md`). `FALSE` — autonomous. Anything else — `-`, empty, `True`, a typo — makes the workflow invalid (T7); it is never read as `FALSE`, because a silently disabled gate is exactly the failure a human gate exists to prevent.

For block rows, the block schedule may carry a per-iteration `HITL` (§9). An instance is gated when its row's cell or the `HITL` of any iteration it runs in is `TRUE`, which lets a plan switch human review on for individual iterations.

### 6.5 Scopes, On Success and On Findings

**Scopes.** Every row belongs to exactly one scope: a set of rows that routes like a table of its own, for one iteration at a time. No route the table declares leaves a scope: entering a child block hands the work to the block's scopes, and the block's exit returns it to the scope that entered it (Blocks, below).

| Scope | Its rows | Runs |
|---|---|---|
| The run | Rows outside every block | Once |
| An iteration of an ungrouped block | The block's own rows (§6.8) | Once per iteration |
| A group, for one iteration | The block's own rows in that group (§8) | Once per iteration whose `Approach` lists the group |

A scope's **child blocks** are the blocks directly inside it: for the run, every block that is not nested; for an iteration of an enclosing block, its inner blocks. A group has no child blocks, because only innermost blocks have groups (§8.1).

**On Success** — where routing goes after this row returns `SUCCESS`:

| Entry | Meaning |
|---|---|
| *target* | Dispatch the row this target names (§6.3) — a row of the current scope |
| `[Name]` | Enter the child block `Name` |
| *a*`, `*b* | Fork — dispatch every target at once. Distinct targets, rows of the run's scope only (§7) |
| `COMPLETE` | Ends the current scope — or, on a branch, only that branch (§7) |
| `NEXT` | The row below when it is in the current scope; entering the block it begins when it is the first row of a child block; otherwise `COMPLETE`. Every rule is checked against what `NEXT` resolves to, as if that were written in the cell. Never used on a branch (B6) |

**On Findings** — the suggested fix target after this row returns `COMPLETED_NEEDS_ACTION`: one target, or `-` for none. It names the row a reviewer's findings usually go back to — typically its creator. It is a default, not a route: where the findings actually go, and how the run continues once the fix has been made, is the executor's decision (§1). A review reporting failing tests may trace them to a wrong plan and go to the planner, whatever the cell says. Write exactly one target per `On Findings` cell — no prose, no alternatives.

**Every target column follows one rule: a target names a row of the current row's scope** (A12) — `On Success`, `On Findings`, and `Waits For` (§7.2). This limits what the table declares, not where an executor may route (§1). Direction does not matter — a target may lie above or below. No target reaches into a block from outside, out of a block from inside, or from one group into another: a block is entered at its first row — by `[Name]`, by `NEXT`, by a scope's start, or by a preceding block's exit — and left only by its exit.

The other four status codes (`CommunicationProtocol.md`) are the executor's to handle and never appear in the table.

**Start and end of a scope.**

- **Start.** A scope starts at its first row — the run at row 1, an iteration at its block's first row, a group at the first of its rows. When that row begins a child block, the scope starts by entering the block.
- **One row at a time.** A block's scopes never fork: each runs one row at a time for its iteration, and parallel work inside a block comes from its iterations (§7). The run's scope may fork; it ends when none of its branches is still running or waiting to run.
- **End.** Ending the run's scope ends the run. Ending a group moves the iteration on to its next group (§8); after its last group, the iteration has completed. Ending an iteration's scope completes that iteration.
- **Every row is reached, nothing loops** (A15). Every row is reached from the start of its scope by the routes the table declares — `On Success`, `NEXT`, block entry and exit — every block is entered from the scope that contains it, and those routes contain no cycle: no row leads back to itself. `On Findings` does not count, so a row reached only as a fix target is invalid. Repeated work comes from blocks and from the executor's routing (§1), never from a declared loop.

**Blocks.** Entering a block runs it over its block schedule (§9):

- **Within an iteration:** a grouped block runs the groups the iteration's `Approach` lists, in that order (§8), each as its own scope. An ungrouped block runs the iteration's scope.
- **Across iterations:** an iteration starts once every iteration in its `Depends On` (§9) has completed. Iterations are not sequential by nature: those whose dependencies are met may run concurrently, and the block schedule, not the routing table, decides which iterations run in parallel. Without `Depends On`, all iterations may run concurrently.
- **Exit:** once every iteration has completed, the scope that entered the block continues at the first row after the block when that row is in that scope, entering it when it begins another child block. Otherwise that scope ends — a block that ends the table ends the run.
- **Re-entry:** entering a block again — after it has exited — runs every iteration that has not completed; a completed iteration is not run again. A block with no such iteration exits at once. Further work on a completed iteration is a new iteration (§9.3).
- **Nested blocks:** an inner block runs inside one iteration of its enclosing block, over the inner block schedule for that iteration (§9.1). It is a child block of that iteration's scope: entered by `[Name]`, `NEXT` or the iteration's start, and exiting into the same iteration.

### 6.6 The Row column

Deployment adds a leading `Row` column numbered `1..N` to each routing table it assembles into an orchestrator, so the run artifact's `WorkflowRow` column can name the exact row that ran (`OrchestrationArtifactFormat.md` §5). **Authors never write it:** a source workflow has no `Row` column (T10). In a deployed table, every number equals the row's 1-based position (T4); a deployed table without one is numbered by position.

### 6.7 Input and Output

Comma-separated artifact paths relative to the run folder; `-` when there are none. The run-folder prefix (`Orchestration-{run_id}/`) is never written — executors add it.

| Notation | In Input | In Output |
|---|---|---|
| Literal (`Plan.md`) | Literal | Literal |
| Fill-in (`Stage-{Stage}/Plan.md`) | The current iteration's ID | The current iteration's ID |
| Wildcard (`Stage-*/Plan.md`, `*-Research.md`) | Every existing artifact that matches | **Literal** — the producing agent decides which artifacts to create |

**`*` stays within one path segment.** It stands for any text, possibly empty, that contains no `/` — the same meaning when an executor lists the files an Input matches and when the rules match Outputs to Inputs (A13): `*-Research.md` matches `Acme-Research.md`, not `Acme/X-Research.md`. A wildcard only lists files; it is never a barrier. Waiting is expressed by blocks (§6.5) and `Waits For` (§7.2), never by Input.

**Per-iteration artifacts.** Plan stages conventionally live under `Stage-{Stage}/`; other blocks choose their own naming (`{Product}-Research.md`). Inside a nested block, a per-iteration artifact (below) carries the ID of every enclosing iteration too, so iterations do not collide (A16): `{Domain}/Stage-{Stage}/Plan.md`, never `Stage-{Stage}/Plan.md` in a Stage block nested in Domain. Groups never appear in a path. Naming follows `OrchestrationSemantics.md` §4.1.

**Iterations are isolated.** An instance reads only the per-iteration artifacts of the iterations it runs in, never another iteration's. A **per-iteration artifact** of a block is an Output of a row carrying the block's reference that fills in the block's ID (`Stage-{Stage}/Notes.md`); in a nested block it also fills in every enclosing block (A16). An Input of a block row that matches a per-iteration artifact fills in each block the row belongs to wherever that artifact does (A14): `Stage-{Stage}/Notes.md` is valid in a Stage row, `Stage-*/Notes.md` and `*/Notes.md` are not; in a nested Stage row, `{Domain}/Stage-{Stage}/Notes.md` is valid and `*/Stage-{Stage}/Notes.md` is not. `Depends On` (§9.2) orders iterations; it never makes one iteration's work another's input. Rows outside the block — e.g. rows of the run's scope, rows of another block, and an enclosing iteration's own rows (`{Domain}/Stage-*/Notes.md` in a `REVIEW.[Domain]` row) — may read every iteration's artifacts by wildcard.

**Seeds.** The first row's Inputs are the run's **seeds** — the artifacts the user supplies. When the first row begins blocks, the schedule source of every block it begins is a seed too (`Plan.md` for `implementation-only`). A seed without a wildcard is required: it exists when the run starts, or, with a fill-in, for each iteration when that iteration starts (`Stage-{Stage}/Plan.md`). A seed with a wildcard (`*-Notes.md`) matches whatever the user has supplied by then, possibly nothing. Apart from wildcards there are no optional seeds; a user who wants to give the run more context does so by reference inside a seed.

**Artifact flow is complete** (A13). Every other Input — wildcards included — is a seed or is output by a row that runs before it. An Input that only a later row produces, or that nothing produces, makes the workflow invalid; an executor may cope with a missing file, but a workflow is not written to need that.

- **Outputs.** An Output entry produces an Input entry when some path matches both, with `*` standing for any text within one path segment and each `{Name}` for any ID — `Stage-*/Plan.md` produces `Stage-{Stage}/Plan.md`.
- **Runs before.** Row X runs before row Y when every way the table reaches Y passes through X first: from the start of the run, by `On Success`, `NEXT`, block entry and exit, and — inside a grouped block — the group order of every approach that runs Y's group. A join is reached only after every row it waits for, so those rows and the rows before them run before it. A row reached through a block's exit counts as coming after every row of that block, even though an approach may skip a group or the block schedule may be empty; its Inputs from the block match whatever those rows actually wrote, possibly nothing (`Stage-*/implementation-review.md` in a `REVIEW` row after the block). `On Findings` routes do not count: they re-run rows that have already run.

A row of one group therefore relies on another group's Output only when every approach that runs it runs that group first. With an `Implementation-Only` approach in the table, Implementation rows cannot rely on what the Test group writes.

### 6.8 Blocks

A **block** is a named, contiguous range of rows that runs one iteration per row of a block schedule: the workflow names the block, a run artifact lists its iterations. Every block is declared in the blocks table (§9). No block name is built in — `Stage` is the conventional name for plan stages, with no special status.

| Term | Meaning |
|---|---|
| **Block reference** | `[Name]`: the block named `Name`. As a segment of `Phase` it places the row in that block (§6.2); in `On Success` it enters the block (§6.5) |
| **Block row** | A row whose `Phase` carries a block reference. A row without one is outside every block and belongs to the run's scope (§6.5) |
| **Fill-in** | `{Name}` in the Input or Output cell of a block row, or in a blocks-table `Source` (§9.1): replaced by the ID of block `Name`'s current iteration |
| **Block schedule** | The table in a run artifact that lists a block's iterations — their IDs and attributes (§9) |
| **Iteration** | One pass of a block for one row of its block schedule. The block schedule lists the iterations; the block runs each one |
| **ID** | An iteration's identifier, the first cell of its block-schedule row: `1`, `2`, `Acme` |
| **Instance** | One execution of a row — one dispatch of its subagent. A block row runs in every iteration — a grouped row only in those that run its group; in a nested block, in every combination of iterations — and each dispatch, a re-run after a fix included, is an instance of its own: `test-writer-tdd` for `Stage` `2` of `Domain` `Billing`, logged as `test-writer-tdd#18`. The run names an instance by that `agent_instance_id` and records its iterations separately (`OrchestrationArtifactFormat.md` §4.1) |
| **Own rows** | A block's own rows are the rows whose last block reference is that block's. An inner block's rows are its own, not its enclosing block's |
| **Enclosing / inner block** | Block Y is nested in block X when every row carrying `[Y]` also carries `[X]` before it. X encloses Y; Y is X's inner block |

| Rule | |
|---|---|
| **Names** | Block names are letters only, matched verbatim and case-sensitively. `HITL` and `Approach` are not block names: they are schedule columns (§9.2) |
| **Declared** | Every block reference names a declared block. Every `{Name}` in a row names a block the row belongs to — `[Name]` is in that row's `Phase`: a row cannot fill in the ID of a block it does not iterate in |
| **Where** | In the routing table, `{Name}` may appear in Input and Output only (the blocks table's `Source` may also fill in enclosing blocks, §9.1). Never in `Subagent` (§6.3), and never in a target cell — `On Success`, `On Findings`, `Waits For`: a target names a row, and no row's `Subagent` can hold an ID (`reviewer({Product})` could only match a row hard-coding `reviewer(Acme)`) |
| **Contiguous** | All rows carrying a given block reference form one contiguous range. A workflow that needs two blocks over the same kind of iteration declares two blocks (they may share a source artifact, §9) |
| **Nesting** | Every row carrying `[Y]` carries the same block references before it, in the same order — Y's enclosing chain is fixed. Because blocks are contiguous, an inner block lies wholly inside its enclosing block. An enclosing block may hold rows before and after its inner block |
| **Run as a unit** | A block runs one iteration per row of its block schedule. Within an iteration, its groups run in the order the iteration's `Approach` lists, and rows route within their scope (§6.5, §8). Iterations run concurrently as far as `Depends On` allows. An inner block runs within each enclosing iteration, over that iteration's own schedule (§9.1) |
| **Exit** | Once every iteration has completed, the scope that entered the block continues at the first row after it, when that row is in that scope; otherwise that scope ends (§6.5). For an inner block, that scope is the enclosing iteration. IDs are not carried out of the block that introduced them |
| **Targets** | A target needs no fill-in to stay with its iteration: it names a row of the current row's scope, and a scope is one iteration — inside a nested block, within the current enclosing iterations (§6.5) |
| **Frontmatter** | `referenced_agents` lists the concrete agent ids; `artifacts` lists paths with fill-ins as written |

```
| PLANNING.[Domain]                         | planner-tdd-soft    | ...   ┐ Domain block
| EXECUTION.[Domain].[Stage].Test           | test-writer-tdd     | ...   │  ┐ Stage block, in every Domain iteration,
| EXECUTION.[Domain].[Stage].Implementation | implementation-tdd  | ...   │  ┘ iterations from {Domain}/Plan.md
| REVIEW.[Domain]                           | test-runner(domain) | ...   ┘ ← inner exit, same Domain iteration
| REVIEW                                    | test-runner         | ...     ← outer exit, after every Domain
```

## 7. Parallel Work

A workflow runs work in parallel in two ways, and they never overlap:

| | Fixed: forks and joins | Planned: block iterations |
|---|---|---|
| Declared in | The routing table | A block schedule the run produces (§9) |
| Unit | One row | One iteration — the rows its `Approach` selects |
| Dependencies | `Waits For`, between rows | `Depends On`, between iterations (§9.2) |
| Where | The run's scope, outside every block | Inside a block (§6.5) |

Planned parallelism needs nothing beyond blocks: a block's iterations run concurrently as far as `Depends On` allows (§6.5). This section defines fixed parallelism — work known when the workflow is written, such as research and audits that do not depend on each other. A workflow with a join carries the `Waits For` column (§6.1).

### 7.1 Fork

`On Success` lists several distinct targets, comma-separated. All are dispatched at once, and each starts a **branch**: the rows its target leads to by `On Success`, up to the fork's close (§7.3). A closing join is not on the branches it closes. Only rows of the run's scope fork, and a fork's entries are always targets, never block references (B1, B3).

```
| RESEARCH | codebase-research(architecture) | FALSE | codebase-research(contracts), architecture-audit | - | - | ...
```

### 7.2 Join

A **join** is a row whose `Waits For` is not `-`. It lists rows by their target (§6.3), each once, never a block. Every join is the closing join of exactly one fork (§7.3, B5); there is no join without a fork. `Waits For` gates the join when its fork's branches reach it by `On Success`: it is dispatched once per pass of the fork, when every row it lists has returned `SUCCESS` — however many branches name it. Any other dispatch of the row — as a fix target, or by the executor's own routing (§1) — is not gated by `Waits For`. `-` means no barrier. When a fork runs again, its closing join waits for this pass; which branch work is repeated is the executor's decision (§1).

### 7.3 Every fork closes

A fork closes in one of two ways (B2):

- **At a join.** Every branch ends by naming one row, the **closing join**, whose `Waits For` names the last row of every branch. There are no partial joins: a row never waits for some branches while others run on. A fork never lists its own closing join: every fork entry starts a branch of at least one row.
- **At the end of the run.** Every branch's last row says `COMPLETE`, and nothing follows the fork; the run ends when the last branch does (`brownfield-system-audit`).

A fork never mixes the two. `COMPLETE` appears on a branch only as its last step, and only in a fork that closes at the end of the run — a branch that ended early would leave its fork's join without a way forward.

A branch is self-contained: its rows are reached only from its fork and from its own rows, and their `On Success` leads only to its own rows and, at its end, its closing join or `COMPLETE`. A branch may fork again; that inner fork closes inside the branch: at its own join, before the branch ends, or — only when every fork around it also closes at the end of the run — with `COMPLETE`, which ends the branch too.

```
| RESEARCH   | codebase-research(architecture) | ... | codebase-research(contracts), architecture-audit | - | -          ← fork
| RESEARCH   | codebase-research(contracts)    | ... | contracts-audit | - | -                                            ← branch B
| EXECUTION  | architecture-audit              | ... | report          | - | -                                            ← branch A
| EXECUTION  | contracts-audit                 | ... | report          | - | -                                            ← branch B
| COMPLETION | report                          | ... | COMPLETE        | - | architecture-audit, contracts-audit          ← closing join
```

Between a fork and its close, on every branch:

- **No `NEXT`** (B6). A branch's rows are linked by named targets only, so every branch can be read from its cells alone, whatever order its rows are listed in.
- **No block is entered** (B3). A block is entered only when no fork is open, so fixed and planned parallelism never run at the same time. A workflow whose planner works alongside other branches joins them before entering the block.
- **`On Findings` never reaches into a branch** (B4). A fix loop never re-runs a row of a sibling branch, and a row outside a branch never sends findings into it — the same boundary blocks draw with scopes (§6.5). A branch row's fix target may lie outside its fork — typically the planner that prepared the work before it.

## 8. Groups

Groups let one block run its rows in different orders per iteration — or skip some — chosen by the block schedule, without editing the workflow. The approach orders the groups; within a group, rows route by `On Success` and `On Findings` like any scope (§6.5). The canonical use is TDD versus implementation-first ordering of a test group and an implementation group.

### 8.1 Phase notation and activation

A grouped row is `{PHASE}.[{Block}].{Group}`, or `{PHASE}.[{Outer}].[{Block}].{Group}` in a nested block (§6.2).

- `{Group}` is a workflow-defined token: letters, digits and `-` only, starting with a letter or digit — the same characters as an ID (§9.2), since both appear in the run's `Iteration` and a group also sits in a comma-separated `Groups` cell (§8.2). Compared verbatim and case-sensitively. CamelCase by convention.
- A group belongs to the block it follows — the row's **last** block reference. Two blocks may use the same group token; they are different groups.
- **Only innermost blocks have groups.** A block that encloses another block has no grouped own rows (A9): each of its iterations is one scope, with the inner block as a child block.
- **Groups are active for a block if and only if at least one of its own rows carries a group.** Activation is all-or-nothing per block: once one own row of a block is grouped, every own row of that block is, and the approach table must cover that block. A block with only ungrouped own rows has no groups, ignores the `Approach` column of its block schedule, and runs each iteration as one scope.

### 8.2 The approach table

A workflow with grouped rows carries, after the routing table and inside the region, the reserved heading followed by the approach table:

```markdown
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
```

- The heading is exactly `**Groups:**` on its own line. A blank line before the table is allowed.
- `Block` — the block the row applies to.
- `Approach` — an opaque token — letters, digits and `-`, starting with a letter or digit, like a group token — matched verbatim against the `Approach` column of that block's schedule. Unique per block: the same token may mean different things for different blocks.
- `Groups` — a comma-separated, ordered list of the block's groups to run for that approach. A token does not repeat within a cell.
- One approach table covers every grouped block in the workflow. It has no other columns (P6). Row order carries no meaning; lookup is by (`Block`, `Approach`).

### 8.3 Omission, ordering, repetition

- **Omission** — a group absent from a row is skipped for iterations using that approach.
- **Ordering** — groups run in the order listed. Different rows may order them differently.
- **Always-run groups** — there is no always-run keyword. A group that always runs is listed in every row for its block, at an explicit position.
- **Run once** — a group cannot run outside an iteration. To have a planner choose a set of rows to run once, give the block a schedule with a single row.

### 8.4 Contiguity

Each group is a contiguous range of rows within its block. A group does not reopen after another group has started.

```
EXECUTION.[Stage].Test            ← Test
EXECUTION.[Stage].Test
EXECUTION.[Stage].Implementation  ← Implementation
EXECUTION.[Stage].Implementation
EXECUTION.[Stage].Test            ← invalid (A1): Test already ended
```

### 8.5 How iterations appear in a run

Routing-table notation is never written into `Orchestration.md`. The run records `Phase` = the bare phase name and `Iteration` = `{ID}[.{ID}…][.{Group}]`, in the Phase cell's nesting order: IDs outermost first, then the group — `1.Test`, `1`, `Acme.Security`, `Acme`, `Billing.2.Test`. Block names are not written; the row (`WorkflowRow`) identifies them and how many IDs follow. Artifacts' `Created In` is `{Phase}.{Iteration}` (`EXECUTION.1.Test`). The format is defined in `OrchestrationArtifactFormat.md` §4.1. Group names and IDs are visible in every run artifact — choose them to read well there.

### 8.6 Vocabulary is the workflow's

Groups and approaches are defined entirely by the workflow. Whoever writes a block schedule must emit approach tokens that match the table exactly; when you introduce a new approach vocabulary, check that the producer your workflow routes to can produce it, or have it spelled out in that producer's injection.

## 9. Block Schedules

A workflow does not know its iterations; a run artifact does. The workflow declares where to find them.

### 9.1 The blocks table

A workflow with blocks carries, after the routing table and inside the region, the reserved heading followed by the blocks table:

```markdown
**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
```

- The heading is exactly `**Blocks:**` on its own line. A blank line before the table is allowed.
- `Block` — the block's name, used in block references and fill-ins. Declared once.
- `Source` — the artifact that holds the block schedule, relative to the run folder, no wildcard. For a block that is not nested, a literal path (no fill-in). For an inner block, the path may fill in its enclosing blocks (`{Domain}/Plan.md`): each enclosing iteration has its own block schedule.
- `Section` — the heading text of the section in `Source` that holds the block schedule. `Source` has exactly one `## {Section}` heading, and the block schedule is the first table after it, before the next level-1 or level-2 heading.
- The blocks table has no other columns (P4).
- Several blocks may share one source artifact, each in its own section (`KBProgress.md` with a `Generation` and a `Corrections` section).

```markdown
| Block  | Source           | Section |
|--------|------------------|---------|
| Domain | SystemDesign.md  | Domains |
| Stage  | {Domain}/Plan.md | Stages  |
```

Nothing is inferred from artifact names. A run-root artifact named `*Plan.md` is a human convention (`OrchestrationSemantics.md` R1); executors read only the declared source.

### 9.2 The block schedule

| Column | Required | Rule |
|---|---|---|
| `{Block}` | Yes | The first column, headed by the block's name. One iteration per row, named by its ID: letters, digits and `-` only, starting with a letter or digit — so an ID is never `-`, which means "none" — and unique ignoring case (IDs appear in paths, which some file systems compare without case, and in the run's `Iteration`) |
| `HITL` | No | Exactly `TRUE` or `FALSE` in every row; anything else is invalid (V5). No column means `FALSE` for every iteration (§6.4) |
| `Depends On` | No | Comma-separated IDs from earlier rows of the same table, each once, or `-` for none; never empty (V4). An iteration starts only once those iterations have completed; it never reads their artifacts (§6.7). No column means no dependencies. An inner block schedule's dependencies stay within its enclosing iteration |
| `Approach` | When the block has groups | A token from the approach table for this block (§8.2) |

**IDs are compared ignoring case** wherever one is matched against another — a `Depends On` entry against the IDs above it, uniqueness (V2), and a changed block schedule against the iterations that have started or completed (§9.3, R2). An ID is still written verbatim, as its row spells it, into fill-ins and the run's `Iteration`.

Extra columns are ignored — a block schedule lives in a run artifact, and its producer may add columns of its own (a planner's `Name` or `Goal`). Cells are trimmed (§5). Row order is the iteration order: iterations are listed in it, and `Depends On` names only rows above.

**An empty block schedule is valid.** A table with a header and no rows means the block has no work this run: no iteration runs and the block exits at once (§6.5). A missing `## {Section}` heading or table is still invalid (V1).

```markdown
## Stages                                    ## Products

| Stage | HITL  | Depends On | Approach |    | Product | Approach      |
|-------|-------|------------|----------|    |---------|---------------|
| 1     | FALSE | -          | TDD      |    | Acme    | Full          |
| 2     | TRUE  | 1          | TDD      |    | Beta    | Security-Only |
```

### 9.3 Producers and changes

- **Exists before the block.** A block schedule's source is a seed (§6.7) — `Plan.md` for `implementation-only`, products in a user-written `Requirements.md` — or is output by a row outside the block that runs before it is entered (A7) — e.g. a row outside every block, or for an inner block a row of the enclosing block (`PLANNING.[Domain]` producing `{Domain}/Plan.md`). Rows of the block may write it too, but never alone. A block never runs before its block schedule exists; a missing one is invalid when the block is entered (V1). When one agent both creates the source and works its iterations, it appears in two rows — a row outside the block that outputs the source, then the block row that reads it — distinguished by a mode qualifier (`knowledge-base-generator(bootstrap)`, then `knowledge-base-generator(generate)`).
- **Changes after the block has started.** A block schedule may change once its block has started — while it runs, or after it has exited and before it is entered again (§6.5). Its producer may revise it when the executor sends work back to it (§1), and rows of the block may write it themselves, adding iterations as they discover work (the knowledge-base family). What a change may do depends on each iteration's state:

  | Iteration | A change may |
  |---|---|
  | Not started | Add, remove or rework it |
  | Started, not completed | Rework it — its `Approach`, `HITL`, `Depends On` and per-iteration artifacts, including work it has already done — but not remove or rename it, not even by a change of case: the run's log and its per-iteration artifacts are keyed on the ID as spelled |
  | Completed | Nothing in the columns this format reads — its ID, `HITL`, `Depends On`, `Approach` — or in its per-iteration artifacts. The producer's own columns (a planner's `Goal`) may still change. Further work on a completed iteration is a new iteration (`5`, `2-fix`) |

  A change may reorder rows, as long as every `Depends On` still names only rows above it (V4). R2 checks what the block schedule shows; keeping a completed iteration's per-iteration artifacts unchanged is a duty of whoever writes them, which no rule can check from the schedule.

  Where a reworked iteration resumes — which of its rows run again — is not defined by the format; the executor decides. The block schedule is what its source holds when it is read, so a change takes effect once the source has been written.

## 10. Validity Rules

A workflow or block schedule that breaks a rule below is malformed. An executor refuses to run what is malformed, and nothing falls back silently. The groups say where the information first exists: in the file around the region (F), in the region (T, P), across the whole table (A, B), in a block schedule (V), or once iterations are routed (R). When and how an executor checks, and how it reports, is that executor's design. The IDs are stable references for messages, tests and reviews.

**File and frontmatter**

| ID | Rule |
|---|---|
| F1 | The frontmatter has every required field (§4), none empty |
| F2 | `id` is kebab-case, equals the file's base name and the region's `name` attribute, and is unique across the catalog |
| F3 | `version` is a quoted string and equals the region's `version` attribute |
| F4 | `referenced_agents` lists every distinct agent id in the `Subagent` column, without mode qualifiers, in first-appearance order, and nothing else |
| F5 | `artifacts` lists every path in the Input and Output columns and every `Source` in the blocks table, verbatim, each once, in any order, and nothing else |
| F6 | The file has exactly one `<Workflow>` region; its opening and closing tags are on their own lines, and a source file's opening tag is `<Workflow type="core" name="{id}" version="{version}">`, attributes in that order (§5) |

**Routing table**

| ID | Rule |
|---|---|
| T1 | The region contains a routing table, and no other table precedes it; besides the routing table, the region holds only the blocks table and the approach table (§5) |
| T2 | The routing table has every required column (§6.1) |
| T3 | The routing table has at least one data row |
| T4 | In a deployed table with a `Row` column, each number is the row's 1-based position |
| T5 | Every `On Success` cell holds an entry from §6.5, or a fork's entries; it is never `-` or empty |
| T6 | Every `Subagent` cell and every target in `On Success`, `On Findings` and `Waits For` is a kebab-case (§6.3) agent id, optionally followed directly by a non-empty kebab-case mode qualifier in parentheses — no suffix, no prose; `On Findings` holds at most one |
| T7 | Every `HITL` cell is exactly `TRUE` or `FALSE` (§6.4) |
| T8 | No two rows have the same `Subagent` cell (§6.3) |
| T9 | Every target matches a row's `Subagent` cell exactly (§6.3) |
| T10 | A source workflow has no `Row` column (§6.6) |
| T11 | The routing table has no column other than those in §6.1 |
| T12 | No routing-table cell is empty; `On Findings`, `Waits For`, `Input` and `Output` write `-` for none (§6.1) |
| T13 | The routing table has a `Waits For` column only when some row's `Waits For` is not `-` (§6.1) |
| T14 | No target appears twice in one `On Success` or `Waits For` cell (§7.1, §7.2) |

**Phase notation, blocks table, approach table**

| ID | Rule |
|---|---|
| P1 | A group follows a block reference, and at most one segment follows the last block reference (`EXECUTION.Test` and `EXECUTION.[Stage].A.B` are invalid) |
| P2 | A group token contains only letters, digits and `-`, and starts with a letter or digit |
| P3 | Each reserved heading (`**Blocks:**`, `**Groups:**`) appears at most once and is followed by a table |
| P4 | The blocks table has exactly the `Block`, `Source` and `Section` columns and at least one data row; no cell is empty |
| P5 | No block is declared twice; names are letters only and not `HITL` or `Approach`; `Source` contains no wildcard, and fills in only blocks that enclose the declared one (none for a block that is not nested) |
| P6 | The approach table has exactly the `Block`, `Approach` and `Groups` columns and at least one data row |
| P7 | No `Block`, `Approach` or `Groups` cell, or token inside `Groups`, is empty; an `Approach` token contains only letters, digits and `-`, and starts with a letter or digit |
| P8 | No (`Block`, `Approach`) pair repeats, and no group repeats within one `Groups` cell |
| P9 | Every `{Name}` in a row has a matching block reference `[Name]` in that row's `Phase` (§6.8) |
| P10 | Block references directly follow the phase name — nothing between them, and nothing after them but the group — and no block appears twice in one row (§6.2) |
| P11 | No `Subagent`, `On Success`, `On Findings` or `Waits For` cell contains a fill-in (§6.3, §6.8) |
| P12 | A phase name uses only uppercase letters, digits and `_`, starts with a letter, and is not `INIT` or `COMPLETED` (§6.2) |

**Cross-checks**

| ID | Rule |
|---|---|
| A1 | Each group's rows are contiguous within its block |
| A2 | Every block reference names a block declared in the blocks table, and every declared block has at least one row |
| A3 | Within one block, either every own row is grouped or none is (§8.1) |
| A4 | Every grouped block has at least one approach-table row; every approach-table `Block` names a grouped block |
| A5 | Every group an approach-table row names exists in that block, and every group in a block is named by some approach-table row for that block |
| A6 | All rows carrying the same block reference are contiguous: each block is one range of rows (§6.8) |
| A7 | Each block schedule's source is a seed or is output by a row outside its block that runs before the block is entered (§6.7, §9.3) |
| A8 | Every row carrying a given block reference carries the same block references before it, in the same order (§6.8) |
| A9 | A block that encloses another block has no grouped own rows (§8.1) |
| A10 | *Retired in 2.7* — the ID is not reused |
| A11 | Every block reference `[Name]` in `On Success` names a declared block that is a child block of the current row's scope (§6.5) |
| A12 | Every target in `On Success`, `On Findings` and `Waits For` names a row of the current row's scope (§6.5) |
| A13 | Every Input of a row other than the first — wildcards included — is a seed or is output by a row that runs before it (§6.7) |
| A14 | In a block row, every Input that matches a per-iteration artifact fills in each block the row belongs to wherever that artifact does — no instance reads another iteration's per-iteration artifacts (§6.7) |
| A15 | Every row is reached from the start of its scope by `On Success`, `NEXT`, block entry and exit; every block is entered from the scope that contains it; and those routes contain no cycle (§6.5) |
| A16 | An Output that fills in a block's ID also fills in the ID of every block enclosing that block (§6.7) |

**Parallel work**

| ID | Rule |
|---|---|
| B1 | Only rows of the run's scope fork or join: a block row's `On Success` holds one entry, and its `Waits For` is `-`. A `Waits For` entry is a target, never a block reference (§7) |
| B2 | Every fork closes, never mixing the two ways: either every branch ends at one closing join whose `Waits For` names exactly the last row of each branch — the join is never itself a fork entry, so every branch has at least one row — or every branch's last row is `COMPLETE`. `COMPLETE` appears on a branch only as its last row, and only when the fork and every fork around it close at the end of the run. A branch's rows are reached only from its fork and its own rows, and their `On Success` leads only to its own rows and, at its end, its closing join or `COMPLETE`; a fork inside a branch closes inside it (§7.3) |
| B3 | Every fork entry is a target — never a block reference, `COMPLETE` or `NEXT` — and no row on a branch enters a block (§7.1, §7.3) |
| B4 | An `On Findings` target lies on a branch the current row is on, or on no branch — never on a branch the current row is not on (§7.3). |
| B5 | Every row whose `Waits For` is not `-` is the closing join of exactly one fork (§7.2) |
| B6 | No row on a branch has `NEXT` in `On Success` (§7.3) |

**Block schedule**

| ID | Rule |
|---|---|
| V1 | The source artifact has exactly one `## {Section}` heading, and a table follows it before the next level-1 or level-2 heading; the table's first column is headed by the block's name |
| V2 | IDs contain only letters, digits and `-`, start with a letter or digit, and are unique ignoring case |
| V3 | When the block has groups, the block schedule has an `Approach` column and no iteration's `Approach` is empty or `-` |
| V4 | Every `Depends On` cell is `-` or names only IDs in earlier rows of the same table, each at most once, compared ignoring case; it is never empty (§9.2) |
| V5 | When the block schedule has a `HITL` column, every cell in it is exactly `TRUE` or `FALSE` |
| V6 | In a block with groups, every iteration's `Approach` has a row in the approach table for its block. There is no default approach. A block without groups ignores `Approach` (§8.1) |

**Routing**

| ID | Rule |
|---|---|
| R1 | *Moved to V6 in 2.16* — the ID is not reused |
| R2 | After the block schedule changes, every iteration that had started still exists, its ID spelled as it was, and every iteration that had completed keeps its `HITL`, `Depends On` and `Approach`; the producer's own columns may change (§9.3) |

## 11. After the Region

Everything after `</Workflow>` is for maintainers and is never deployed. Separate sections with `---`.

### 11.1 Sections

```markdown
---

## Design Rationale

Why the workflow is shaped this way: the trade-offs, why the phases are ordered as they are,
which alternatives were rejected. This is the only record of the reasoning — write it.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | YYYY-MM-DD | MOSAIC | Initial version |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- (none yet)

**Dead ends (tried and rejected):**
- (none yet)
```

Design Rationale and Changelog are expected in every catalog workflow. Open Ideas / Dead Ends is optional but recommended — it keeps rejected ideas from being retried unknowingly.

### 11.2 Versioning

A run records the workflow id and version it runs (`OrchestrationArtifactFormat.md` §4), and the version string is the only thing tying the run to a table. **Any edit to the region — a row, a cell, the blocks or approach table, a note — bumps the version** in the frontmatter, the region attribute and Index.md, with a new Changelog row. Edits after the region do not. A version names exactly one table: a table whose source region differs is a different version (deployment's own edits, §5, do not count), and a run recorded against one version is a run of that table.

## 12. Complete Example

A workflow with a research fork closed by a join, and one grouped block:

```markdown
---
version: "1.0"
name: "Example TDD Workflow"
description: "Test-first changes to an existing codebase with parallel research, planning and per-stage review."
hint: "Illustrative only."
author: MOSAIC
id: example-tdd
referenced_agents:
  - codebase-research
  - library-research
  - planner-tdd-soft
  - plan-review
  - test-writer-tdd
  - tests-review-tdd
  - implementation-tdd
  - implementation-review
  - test-runner
artifacts:
  - Requirements.md
  - Research.md
  - ResearchContracts.md
  - LibraryResearch.md
  - Plan.md
  - Stage-*/Plan.md
  - Stage-*/PlanProgress.md
  - plan-review.md
  - Stage-{Stage}/Plan.md
  - Stage-{Stage}/PlanProgress.md
  - Stage-{Stage}/tests-review-tdd.md
  - Stage-{Stage}/implementation-review.md
  - TestResults.md
---

<Workflow type="core" name="example-tdd" version="1.0">
## Example TDD Workflow

**Use when:** Test-first changes to an existing codebase with parallel research, planning and per-stage review.

| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |
|-------|----------|:----:|------------|-------------|-----------|-------|--------|
| RESEARCH | codebase-research | FALSE | codebase-research(contracts), library-research | - | - | Requirements.md | Research.md |
| RESEARCH | codebase-research(contracts) | FALSE | planner-tdd-soft | - | - | Requirements.md, Research.md | ResearchContracts.md |
| RESEARCH | library-research | FALSE | planner-tdd-soft | - | - | Requirements.md, Research.md | LibraryResearch.md |
| PLANNING | planner-tdd-soft | TRUE | NEXT | - | codebase-research(contracts), library-research | Requirements.md, Research.md, ResearchContracts.md, LibraryResearch.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | [Stage] | planner-tdd-soft | - | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | NEXT | - | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | NEXT | - | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-Only | Implementation |
</Workflow>

---

## Design Rationale
...
```

**The fork uses named targets.** `codebase-research` forks into two branches, `codebase-research(contracts)` and `library-research`, each one row long. Both name `planner-tdd-soft`, the closing join, whose `Waits For` lists the last row of each branch; it runs once, after both have returned `SUCCESS`. Branch rows never use `NEXT` (B6) — the targets say which branch a row belongs to, whatever order the rows are listed in.

**Everything after the join is sequential, so it uses `NEXT`.** The join itself is on no branch: `planner-tdd-soft` continues with `NEXT` to `plan-review`. `plan-review` enters the Stage block with `[Stage]` (`NEXT` would do the same, since the row below begins the block; the reference makes the entry visible). Inside the block, each creator continues with `NEXT` to its reviewer, and each reviewer's `COMPLETE` ends its group.

**Groups and iterations.** Each group is a scope of its own: the Test group ends at `tests-review-tdd`, and the iteration's `Approach` decides what follows — for a `TDD` stage, the Implementation group. An `Implementation-Only` stage runs only the Implementation group; that is why no Implementation row reads an artifact only the Test group writes (A13) — `Stage-{Stage}/PlanProgress.md` is written by the Test group too, but the planner produces it first. Each reviewer's suggested fix target is the creator in its own group, whose `On Success` leads back to the reviewer. Once every stage has completed, the block exits to `test-runner`, the first row after it.

**After the block.** `test-runner` names `planner-tdd-soft` as its fix target, rather than a block row: a block row is in another scope (A12), and once the block has exited there is no current `Stage` iteration to return to. Completed stages are frozen, so the planner adds a stage for the fix (§9.3). When the executor continues along the planner's `On Success`, `plan-review` enters `[Stage]` again, and only the new stage runs (§6.5). The planner is a join, but a fix dispatch is not gated by its `Waits For` (§7.2).

A workflow without a join drops the `Waits For` column (§6.1, T13). A workflow without groups drops the `**Groups:**` table and leaves its block rows ungrouped (`EXECUTION.[Stage]`).

---

## Changelog

| Version | Date | Summary |
|---------|------|---------|
| 2.16 | 2026-10-10 | **"Value table" renamed "block schedule"**: the run-artifact table listing a block's values, their order (`Depends On`), `Approach` and `HITL`. "Value-table source" is now "schedule source". §9 is now *Block Schedules* and §9.2 *The block schedule*; the V rules keep their letter. **Kebab-case defined** (§6.3): runs of lowercase letters and digits joined by single `-`, starting with a letter, so `-` is never an agent id (T6, §4 `id`). **Nested isolation is a rule** (§6.7): a per-value artifact of an inner block fills in every enclosing block too (A16 added); A14 checks every block the row belongs to. **`*` stays within one path segment** (§6.7), for executor file listing and A13 matching alike. **Values compared ignoring case** (§9.2): in `Depends On` (V4), uniqueness and schedule changes; a started value cannot be renamed, not even by case (§9.3, R2). **Completed values freeze only the format's columns** (§9.3, R2): a producer may still edit its own columns. **"Value" retired as a term:** a block runs one **iteration** per block-schedule row, and each iteration has an **ID** (`2`, `Acme`) — fill-ins, paths and `Instance` carry IDs; "per-value artifact" is now "per-iteration artifact". **Instance** means one execution of a row — one subagent dispatch, a re-run after a fix included (§6.8). Generic uses of "value" (cells, `On Success` entries, `Row` numbers) reworded. §8.5 is now *How iterations appear in a run*: the run records them in the `Iteration` field (replacing `Stage` in `OrchestrationArtifactFormat.md` 3.0), and "instance" names one execution, the protocol's `agent_instance_id`. Existing rule IDs keep their meaning. **R1 moved to V6** (checkable when the schedule is read); R1 retired. §4 `referenced_agents` reads from the `Subagent` column, matching F4. §11.2: deployment edits do not make a different version. §5 cites `CommunicationProtocol.md` for ASCII. **Every block is entered** (§6.5, A15): a block never entered from the scope containing it is invalid, even though its rows are reachable within their own scope. **An iteration starts at its block's first row** (§6.5), so an enclosing block whose first rows belong to an inner block starts by entering it. **Empty `Depends On` is invalid** (§9.2, V4): write `-` for none. T6 no longer mentions block references, which are not targets. §6.8: an instance runs only in iterations that run its group; fill-ins also appear in a blocks-table `Source`. Wording: a seed *without a wildcard* is required (§6.7); a fork inside a branch that closes with `COMPLETE` ends the branch too (§7.3); a block-schedule change may reorder rows within V4, and the freeze of a completed iteration's per-iteration artifacts is a writer's duty no rule checks (§9.3); T5 covers a fork's entries. Status set to Final. |
| 2.15 | 2026-10-10 | **Tokens tightened:** values, group tokens and approach tokens start with a letter or digit, so none can be `-`, the "none" marker (§8.1, §8.2, §9.2; P2, P7, V2); phase names start with a letter (§6.2, P12); every "letters and digits" is ASCII (§5). **Values are unique ignoring case** (§9.2, V2): they appear in paths on case-insensitive file systems. **No duplicate entries:** a fork's targets are distinct (§6.5, §7.1), a join lists each row once (§7.2, T14 added), `Depends On` names each value once (§9.2, V4). **Wildcard seeds** (§6.7): match whatever the user has supplied, possibly nothing; literal seeds stay required. **Value-table lookup** (§9.1, V1): exactly one `## {Section}` heading; the table must come before the next level-1 or level-2 heading. **Region tables** (§5, T1): only the routing, blocks and approach tables. **Branches** (§7.1): a closing join is not on the branches it closes. **Scopes** (§6.5): entering a child block and returning from its exit is how work crosses scopes. `artifacts` is in any order (§4, F5). §6.4 no longer repeats HITL placement guidance from `WorkflowPatterns.md`. Table of contents added. |
| 2.14 | 2026-10-10 | **Targets defined** (§6.3): a target is an entry that names a row; `COMPLETE`, `NEXT` and block references are not targets. **Fill-in placement** (§6.8): the Input/Output-only rule applies to the routing table; the blocks table's `Source` may fill in enclosing blocks. §6.5 `On Success` stand-ins written in italics, not braces, so they do not read as fill-ins. **Whitespace and columns unified** (§5): every heading, cell and comma-separated entry is trimmed; the routing, blocks and approach tables have no extra columns (P4, P6); value tables still ignore extra columns. §6.7: rows of another block listed among rows that may read a block's artifacts by wildcard. §9.1: a non-nested `Source` is "a literal path (no fill-in)". **Rows after a block** (§6.7, A13): a row reached through a block's exit counts as coming after every row of the block; its Inputs from the block may match nothing. **A fork never lists its own closing join** (§7.3, B2): every branch has at least one row. **`artifacts` includes block sources** (§4, F5): every `Source` in the blocks table is listed, so a seed that no row reads (`Plan.md` in `implementation-only`) still appears. |
| 2.13 | 2026-10-10 | **"Line of work" removed:** `COMPLETE` ends the current scope, or on a branch only that branch (§6.5); B3's explanation says a block is entered only when no fork is open. **Block is the one named concept.** "Placeholder" and "marker" are retired: a **block** is a named, contiguous range of rows that runs once per value; `[Name]` is a **block reference** wherever it appears (in `Phase` it places the row in the block, in `On Success` it enters it); `{Name}` is a fill-in, the block's current value; a **block row** is a row in a block. Syntax follows: the `**Placeholders:**` heading is now `**Blocks:**` with columns `Block`, `Source`, `Section`; the approach table's `Placeholder` column is `Block`; a value table's first column is headed by the block's name. Rule IDs unchanged; wording of P1, P4–P10, A2, A4–A6, A8, A11, V1 and R1 updated. **Every row is reached, nothing loops** (A15, §6.5): every row is reached from its scope's start by declared routes, and those routes have no cycle; a row reached only as a fix target is invalid. **Block entry stated whole** (§6.5): a block is entered at its first row by `[Name]`, `NEXT`, a scope's start or a preceding block's exit. **Seeds** include the value-table source of every block the first row begins (§6.7). **Region:** the heading names the same workflow as frontmatter `name` (spelling may differ); `**Notes:**` is optional and not recommended; T1 also requires the routing table to be the region's first table. §11.2 no longer states an executor duty. §2 wording: `HITL` and `Approach` are forbidden as block names. **B3** states that every fork entry is a target, ruling out `COMPLETE` and `NEXT` as well as block references. Example (§12): `**Notes:**` dropped; the group narrative no longer claims Implementation rows read nothing the Test group writes. |
| 2.12 | 2026-10-10 | **The table defines the happy path** (§1): `On Findings` is a suggested fix target, not a route — where findings go and how the run continues after a fix are the executor's decision; removed "routing continues from the target's own `On Success`" (§6.5). **Wording:** `NEXT` definition split into its three cases (§6.5); the group in a Phase cell is optional (§6.2); the target-scope rule names all three target columns (§6.5); §6.1 widths are source widths; §9.3 producer list given as examples of A7. **No empty cells** (T12). **`Waits For` only when a row joins** (T13): a column of `-` only is invalid. Example note no longer reads as a seed declaration. **Values are isolated** (§6.7, A14): an instance never reads another value's per-value artifacts; `Depends On` orders values and carries no data. **§7.4 Route back removed:** a branch row's fix target may lie outside its fork (B4), and what happens to running branches is the executor's decision — the format says nothing about it; the term "route back" is gone. **Joins gate only their fork's branches** (§7.2): dispatched once per pass of the fork; a fix dispatch or executor-chosen dispatch is not gated by `Waits For`. Example (§12) narrative follows. **R1 scoped to blocks with groups**, matching §8.1. **File and frontmatter rules F1–F6** added to §10. §1 and §9.3 no longer use "route back" for executor routing. |
| 2.11 | 2026-10-10 | **The rules bind tables, not executors** (§1): scope, branch and target rules restrict declared routes only; an executor's own policy (re-targeting, producer callbacks, escalation) is outside the format. **`next` renamed `NEXT`**, so it can never collide with a kebab-case agent id. **Every join closes a fork** (B5). **`NEXT` is checked as what it resolves to** (§6.5): a target, block entry or `COMPLETE`. **No `NEXT` on a branch** (B6): branch rows link by named targets only. **Example (§12) extended:** a research fork closed by a join (named targets, `Waits For`), then `NEXT` through the sequential rows. **Inputs come from earlier rows** (A13, A7): every Input other than a seed, wildcards included, is output by a row that runs before it — defined by the table's routes, not by row position; `On Findings` re-runs do not count. **Executor specifics trimmed:** references to one orchestrator's Routing Policy replaced by §1; §10 no longer prescribes when rules are checked or how they are reported, and R1 drops its checkpoint list; §9.3 states when a value-table change takes effect instead of an executor re-read duty. Removed the §6.7 sentence about passing Inputs that do not exist yet (a creator reading its review on rework is not a table concern). **Route back** (§7.4): a branch row's `On Findings` may name a row outside its branch; the branches it leaves stop first, letting running rows finish. How work continues afterwards is the executor's decision. **Repeated fork:** the closing join still waits for the new pass, but which branch work is repeated is no longer fixed by the format. B4 now forbids only targets on a branch the row is not on; B2's self-containment covers `On Success`. **No extra routing-table columns** (T11). Removed the leftover clause about a join waiting on an empty block's rows (gone since 2.7). `HITL` and `Approach` listed as reserved placeholder names in §2. A10 marked retired. Cross-references for A3 and the innermost-groups rule corrected. |
| 2.10 | 2026-10-10 | **Character sets:** agent ids and mode qualifiers are kebab-case, the qualifier directly after the id (T6); approach tokens are letters, digits and `-` (P7). **`HITL` and `Approach` are not placeholder names** (P5). **Deployment edits stated:** region content is copied unchanged except the `type` rewrite and the `Row` column; a source workflow has no `Row` column (T10), and T4 applies to deployed tables. The *Creator-Gated HITL* anti-pattern is cited by name, not as "A2". |
| 2.9 | 2026-10-10 | **`Waits For` is optional:** needed only when a row joins; a fork that closes at the end of the run works in a 7-column table. **Repeated fork:** its branches start afresh and its closing join waits for the new pass. **No mixed closes (B2):** a fork closes at one join or with every branch ending the run; `COMPLETE` sits only on a branch's last row, and only when every enclosing fork closes at the end of the run. |
| 2.8 | 2026-10-10 | **Seeds are the first row's Inputs**, plus the value-table source of a block the first row begins; all are required, none optional. Every other Input is a seed, a wildcard or output by some row (A13), with a matching rule for wildcards and fill-ins; "earlier row" is gone. A7 drops the ordering: a source is a seed or output by a row outside its block. Inputs produced but not yet written are passed once they exist. Notes no longer declare seeds. |
| 2.7 | 2026-10-10 | **Two kinds of parallel work, never overlapping.** Fixed parallelism — forks and joins — exists only in the run's scope, outside every block; planned parallelism is a block's values. Every fork closes, at one full join on the last row of every branch or with every branch ending in `COMPLETE`; no partial joins (B2). No block is entered between a fork and its close (B3); `On Findings` stays on its branch (B4); block scopes run one row at a time (B1). Removed: block references in `Waits For`, forks inside blocks and with block entries, A10, "a row that will never run counts as satisfied". §7 rewritten. |
| 2.6 | 2026-10-10 | **Re-entry:** entering a block again runs only the values that have not completed. **Value-table changes by value state:** unstarted values may be added, removed or reworked; a started value may be reworked — `Approach` included, and work it has already done — but not removed or renamed; a completed value is frozen, and further work on it is a new value. Where a reworked value resumes is the executor's decision. R2 reworded; example updated. |
| 2.5 | 2026-10-10 | **Phase cell in nesting order:** `PHASE.[Outer].[Inner].Group` (was `PHASE.Group.[Outer].[Inner]`) — `EXECUTION.[Stage].Test`. The run's `Instance` follows: `{Value}[.{Value}…][.{Group}]` — `1.Test`, `Billing.2.Test`; `Created In` `EXECUTION.1.Test`. P1 (one segment at most after the last marker) and P10 reworded. |
| 2.4 | 2026-10-10 | **Routing scopes.** Every row belongs to one scope — the run, one value of an ungrouped block, or one group for one value — and `On Success`, `On Findings` and `Waits For` name rows of that scope only (A12), in either direction. Block rows route by `On Success` again: inside a value, forks, joins and fix loops work as at the top level, and after a fix the creator's `On Success` continues. Groups order the groups, not the rows. A block is entered only by its reference `[Name]` in `On Success` (or by `next`, or a scope's start) at its first row, and left only by its exit; `COMPLETE` and `next` end the current branch, and a scope ends when no branch remains. `On Findings` loses the "above" restriction. **Own rows** defined (a block's rows whose last marker is that block's); A3, A9 and group activation count own rows. Removed: block entry by naming any block row, and block-row `On Success` as "default ordering only". A11 now covers `On Success`; A10 applies when the row after a block is in the block's scope. Example updated. |
| 2.3 | 2026-10-10 | **The format states no consumer status.** `next` is an ordinary `On Success` value; the "not yet usable" caveat, the `artifacts` "None yet" reader and §13 Open Items (which tracked catalog, orchestrator and Runner drift) are removed. Drift is tracked in the workspace backlog, not here. **Targets name rows:** every `Subagent` cell is unique (T8) — a repeated agent is told apart by mode qualifiers — and a target matches one cell exactly; the nearest-row resolution for `On Success` and `On Findings` and the "no match counts as no target" fallback are removed; a target that matches no row is invalid (T9). **Joins wait for blocks by name:** `Waits For` takes a block reference `[Name]`, satisfied when the block has exited, and never names a block row (A11); a join runs once however many branches reach it. **A10 reworded:** in a parallel table the row after a block is a join on that block, which makes the positional exit visible and rules out a double dispatch. |
| 2.2 | 2026-10-09 | **HITL is strict:** exactly `TRUE` or `FALSE` in the routing table (T7) and in a value table's `HITL` column (V5); other values are invalid instead of reading as `FALSE`. **Block at the end of the table** exits to `COMPLETE` (ends the branch after a fork). Example: `test-runner` sends findings to `planner-tdd-soft`, not into the exited block. **No fill-ins in target cells** (`On Success`, `On Findings`, `Waits For`; P11 widened): a target cannot name a row that would have to hard-code a value, and targets already stay within the current value. **Parallel block exit** (§7.3, A10): the row after a block must be on the block's own branch. **Group tokens** use the value character set — letters, digits, `-` (P2 tightened). **Empty value table** is valid: the block exits at once. **Phase names are open** (§6.2, P12): any uppercase name, `INIT` and `COMPLETED` reserved; the seven familiar phases become recommended conventions. Nothing in the format routes on a phase name. |
| 2.1 | 2026-10-09 | **Nested blocks.** A row may carry several markers, outermost first (`EXECUTION.Test.[Domain].[Stage]`); an inner block runs once per enclosing value, over a per-value value table (`Source` may fill in enclosing placeholders). Inner exit stays within the enclosing value; targets and `Waits For` resolve within the current enclosing values; only innermost blocks have groups; `Instance` carries every value, outermost first. P5, P10 relaxed; A8, A9 added. Additive: every 2.0 workflow stays valid. Specified ahead of implementation — motivating case in `Brainstorm-GreenfieldDomains.md`. |
| 2.0 | 2026-10-09 | **Placeholders unified; backward compatibility dropped.** Stages are no longer special: `[Stage]` is an ordinary declared placeholder (renamed from `[StageNumber]` to match the planner's `Stage` column). New `**Placeholders:**` table declares each placeholder's source artifact and section; master-Plan and tracker inference removed. `HITL`, `Depends On` and `Approach` are generic value attributes; any marker forms a block with groups, value-table ordering and block exit. Approach table renamed `**Groups:**` and gains a `Placeholder` column. `*` barrier in Input removed; carried values removed (values do not leave their block). One marker per row. Legacy `*` target suffix and prose in `On Findings` are invalid (T6). Run records `Instance` = `[{Group}.]{Value}`. Validity rules reworked (P3–P11, A2–A7, V1–V4). |
