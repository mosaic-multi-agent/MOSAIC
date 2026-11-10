---
run_id: "20260801T202027Z-ad3d"
created_by: "knowledge-base-generator#4"
last_updated: "2026-10-12"
---

# compat

> Responsibility: Gatekeeps which parsed workflow routing tables the runner is allowed to execute ("admission"), and — for admitted workflows — resolves each declared block into its contiguous row range, its groups (from the Phase column's group segment), its approach rows and its schedule-source classification, so downstream consumers read one admitted model per block.

## Overview

The runner does not support every shape a WorkflowFormat 2.16 file could theoretically express — only a specific subset that its state machine (`engine`) and its block-driven dispatch loop (`session`) know how to handle. `compat` is the single checkpoint between workflow parsing (`workflow`) and everything downstream: it either refuses a routing table outright with a precise, named reason, or it produces an `AdmittedWorkflow` that carries both the original table and the per-block structure the rest of the runner relies on.

The single entry point is `Admit(table, mode) (AdmittedWorkflow, error)`; the run's `ExecutionMode` selects the fork/join rule and the cycle check (see below). There is no partial admission — a table is either fully admitted or refused. 1.x notation never reaches `compat`: the strict `workflow` parser has already refused it.

## Components / Subdomains

| Component | Purpose |
|-----------|---------|
| **Admission checks** | Run in a fixed order (parsed-shape check, parallelism, nested blocks, block references, contiguous block ranges, blocks have rows, per-block groups, approach table, scopes, schedule sources, cycles), each refusing with its own reason and, where the format has one, a rule ID (A1-A7, A11, A12, A15, B1) so a KB consumer/debugger can tell exactly which unsupported shape was encountered. The fork/join rule and the cycle check depend on the execution mode. |
| **Block and group resolution** | Splits the rows referencing each declared block into its range and its groups by reading each row's `PhaseParsed.Blocks` and `PhaseParsed.Group` fields. Agent identifiers are never inspected. Cross-validates the declared group set against the workflow's approach table (A1, A3, A4, A5). |
| **Scope and schedule-source checks** | Checks that `On Success`, `On Findings` and `Waits For` targets stay in the row's own scope (A11, A12), classifies each block's schedule `Source` as seeded or produced and finds its producing and in-block writer rows (A7), and checks for routing cycles in the engine-routed modes (A15). |

## Key Flows

### Admission (`Admit`)

Given a `RoutingTable` and the run's `ExecutionMode`, checks run in this order:

1. **Parsed shape:** a row whose typed entries could not have come from the workflow parser (a zero entry kind, an unresolved kind, a row index out of range, `IsStaged` disagreeing with the block references, an unknown block in a block entry) is refused with no rule ID and a reason saying the row was not produced by the parser. Hand-built tables fail here.
2. **Parallelism (mode-dependent):** in `auto`, `auto-review` and the unset sentinel, a fork (comma list in `On Success`) or join (`Waits For` other than `-`) is refused; on a block row it carries rule ID B1, on a run-scope row no rule ID. In `orchestrated` mode the consultant dispatches one agent at a time, so a fork/join table is admitted, subject to the **fork-branch boundary check**: a branch (a row named in the multi-target `On Success` of an earlier row of the same phase) may not be the last row of its phase, which also covers the last row of a group, because the last-row helpers answer by table position and a branch has no join to end on.
3. **Nested blocks:** a row with more than one block reference, or a block whose declared `Source` fills in another block's ID, is refused by name (no rule ID).
4. **Block references and ranges:** every referenced block must be declared and every declared block must have rows (A2); the rows of one block must be contiguous (A6).
5. **Groups per block (A3, A1) and approach table (A4, A5):** see Block and Group Resolution below.
6. **Scopes (A11, A12):** `[Name]` entries must name a child block of the row's scope, and every target must name a row of the row's own scope.
7. **Schedule sources (A7):** a block's `Source` must be a seed (the first row begins the block or lists the `Source` as an Input) or be output by a row outside the block that runs before the block is entered; a block's classification (`SeedSeeded` or `SeedProduced`) and its `ProducingRows` / `WriterRows` are recorded.
8. **Cycles (A15, `auto`, `auto-review` and unset only):** the routes `On Success`, `NEXT` and block entry may not contain a cycle; `On Findings` routes do not count. `orchestrated` mode is not checked.

On success, `Admit` returns an `AdmittedWorkflow{Table, Blocks}`; each `AdmittedBlock` carries `Name`, `Source`, `Section`, `StartRow`/`EndRow` (zero-based, half-open ranges into `Table.Rows`), `Grouped`, `Groups`, that block's `Approaches`, `Seed`, `ProducingRows` and `WriterRows`. `BlockOf(rowIdx)` and `BlockByName` look a block up.

### Block and Group Resolution

Operates on the rows of one block at a time. Group membership is read exclusively from each row's `PhaseParsed.Group` field — the group segment parsed from the Phase column (e.g. `EXECUTION.[Stage].Test` yields block `Stage`, group `Test`). Agent identifiers are never inspected.

**Ungrouped block (no group segment on any of its rows):**
- All the block's rows form a single implicit group with an empty `Name`, spanning the block's row range.
- `Grouped = false`; the block schedule's `Approach` column is not read.
- An approach-table row keyed to an ungrouped block is refused (A4).

**Grouped block (at least one group segment declared):**
- Rows are partitioned by group token into contiguous half-open `[StartRow, EndRow)` ranges, ordered by first appearance. There is no upper bound on the number of groups.
- A3: if one row of the block is bare while another carries a group, admission is refused.
- A1: if a row re-opens a group that had already ended (non-contiguous), admission is refused.
- A4: every grouped block needs at least one approach-table row, and every approach-table row's `Block` must name a grouped block.
- A5: every group an approach row names must exist in that block, and every group in a block must be named by some approach row for it.

Group boundaries are expressed as zero-based, half-open `[StartRow, EndRow)` row-index ranges into the original `RoutingTable.Rows`. Two rows may name the same agent only through different mode qualifiers (`workflow` refuses identical `Subagent` cells, T8); grouping is Phase-column-based, not agent-identity-based.

## Relationships

| Talks To | For |
|----------|-----|
| **workflow** | Consumes the `RoutingTable` produced by parsing a workflow region — this is `Admit`'s only input. |
| **domain** | Uses `RoutingTable`, `RoutingRow`, `PhaseParsed`, `AdmittedWorkflow`, `AdmittedBlock`, `ExecutionGroup`, `GroupName`, `ApproachTable`, and `RefusalError` — compat has no types of its own. |
| **session** (run-start sequence) | `session` calls `Admit` (passing the run's execution mode) once per run, after parsing the workflow and before resolving agents/reading block schedules; the resulting `AdmittedWorkflow` is threaded into `engine.Next` on every subsequent tick. `session` reads each block's `Seed`, `Source`, `Section`, `Grouped`, `Approaches`, `ProducingRows` and `WriterRows` to read and re-read block schedules. |
| **engine** | Reads `AdmittedWorkflow.Blocks` (ranges, groups, approaches) to decide dispatch order — compat does not call engine, it only produces the data engine consumes. |

## Key Concepts

| Concept | Meaning |
|---------|---------|
| **Admission** | The gate that decides whether a routing table's *shape* is one the runner's state machine can drive — independent of whether individual agents/artifacts referenced in it exist. |
| **Block** | A set of contiguous rows that repeats once per iteration of its block schedule, declared in the `**Blocks:**` table and referenced as `[Name]` in the Phase column (`PhaseParsed.Blocks`). The Runner admits blocks on any phase, several sibling blocks, but not nested blocks. |
| **Group** | A contiguous sub-range of a grouped block's rows identified by a workflow-defined name taken from the Phase column's group segment. An ungrouped block resolves to one implicit group with an empty name. |
| **Seed classification** | `SeedSeeded` when the block begins at the first row or the first row lists its `Source` as an Input (the artifact is supplied when the run starts, so a defective schedule refuses the run), `SeedProduced` when a row outside the block outputs it before the block is entered. |
| **Contiguity requirement** | Both each block and each named group within it must be one unbroken run of rows — no interleaving of rows belonging to different blocks or groups is supported. |

## Boundaries

- **Owns:** Validating that a parsed routing table's *structural shape* (blocks, groups, dispatch fan-out, scopes, schedule sourcing) is inside the supported subset, and deriving per-block row ranges and group ranges from admitted rows.
- **Does Not Own:** Parsing workflow markdown into a `RoutingTable` (that's `workflow`); reading and validating a block schedule from its source artifact (that's `planstages`); resolving agent identifiers to agent definition files (that's `agentresolve`); deciding what to dispatch next at runtime (that's `engine`); anything about artifact I/O.

## Invariants & Conventions

- Every refusal returns `*domain.RefusalError{Component: "compat", ...}` — never a plain error, panic, or silent fallback (see project-level "Refusal over silent fallback" pattern). It carries `Rule`, the 1-based `Row` and `Token` where they apply.
- Each condition is checked independently and produces a distinct, human-readable reason string naming the offending row index and/or token — refusals are diagnosable without re-reading the routing table.
- `AdmittedBlock` ranges and `Groups` use zero-based, half-open `[start, end)` conventions consistently with `RoutingRow.Index`.
- A routing table with no blocks is not an error at the `Admit` level — it returns an `AdmittedWorkflow` with an empty `Blocks` slice and the run is a plain sequence of run-scope rows.

## Known Complexity

None identified beyond what's captured above — the admission checks and the group-resolution partition rules (including the A1-A5 cross-consistency checks) are fully described at this tier. No deeper-tier document is recommended for `compat`.
