---
version: "1.1"
name: "MosaicTest Staged Multi-Group Workflow"
description: "Runner mode fixture — multi-group staged execution with TDD approach. Test group dispatches first per stage, then Implementation group. Proves group ordering, block-reference phase notation, and {Stage} fill-in survive a real harness round trip."
hint: "Harness test — Test and Implementation group ordering inside the Stage block across two stages"
author: MOSAIC
id: staged-multigroup
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/multigroup-test.md
  - MosaicTestScript/multigroup-impl.md
  - Stage-{Stage}/MosaicTestStageTest.md
  - Stage-{Stage}/MosaicTestStageImpl.md
  - Plan.md
modes:
  - auto
---

<Workflow type="core" name="staged-multigroup" version="1.1">
## MosaicTest Staged Multi-Group Workflow

**Use when:** Verifying that the runner dispatches groups in the correct order (Test before Implementation) across the iterations of a block, and that the `EXECUTION.[Stage].{Group}` phase-column notation parses correctly through a real harness CLI.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage].Test | mosaictest-scripted(test) | FALSE | COMPLETE | - | MosaicTestScript/multigroup-test.md | Stage-{Stage}/MosaicTestStageTest.md |
| EXECUTION.[Stage].Implementation | mosaictest-scripted(impl) | FALSE | COMPLETE | - | MosaicTestScript/multigroup-impl.md, Stage-{Stage}/MosaicTestStageTest.md | Stage-{Stage}/MosaicTestStageImpl.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |

**Notes:**
- **`Plan.md` is a pre-placed fixture, not produced by the run.** It and the scripts must be seeded into the run folder at run creation. Seed `Fixtures/staged-multigroup` — the whole directory, not anything inside it — as the single seed path. See `Fixtures/README.md`.
- **No row produces `Plan.md`.** The `Stage` block's schedule source is therefore a seed, which is deliberate: it is what lets the plan be pre-placed.
- The Implementation row takes the Test row's output (`Stage-{Stage}/MosaicTestStageTest.md`) as an input. This chains the two groups within an iteration and proves `{Stage}` filled in the same value for both rows.
- Each group holds one row, so `COMPLETE` ends the group's scope; the `TDD` approach in the `**Groups:**` table orders the groups.
- The two rows run the same agent, so their `Subagent` cells are told apart by the mode qualifiers `test` and `impl`. Dispatch, instance ids and logs use the bare id `mosaictest-scripted`; the qualifier is added to each row's task description, which the scripts echo.

</Workflow>

---

## Design Rationale

### Why two groups, not one

`staged-preplaced-plan` proves iteration progression and `{Stage}` fill-in with rows that have no groups. This workflow adds groups: the engine must parse `EXECUTION.[Stage].Test` and `EXECUTION.[Stage].Implementation`, order the groups by the iteration's approach, and dispatch Test before Implementation within each iteration. A block without groups exercises none of that.

### Why two stages

One stage would prove group ordering within a stage but not that the ordering resets across stages. Two stages prove that the engine runs Test-then-Implementation for stage 1 AND then Test-then-Implementation for stage 2, rather than running all Test rows across all stages followed by all Implementation rows.

### Why TDD approach

The TDD approach orders Test before Implementation. This is the most interesting ordering because it is non-alphabetical — it would not be correct by accident. If the engine dispatched groups alphabetically, Implementation would run before Test, and the run log would show the wrong order.

### Why the Implementation row chains from Test

The Implementation row takes the Test row's output as its input. This guarantees that if the engine dispatched Implementation before Test within a stage, the run would fail visibly — the input artifact would not exist yet. The chaining turns a group-ordering bug into a loud failure rather than a subtle log-order discrepancy.

### Historical context

A past engine crash occurred at glob/wildcard dispatch after the design phase finished (§7.4 in TestCatalogDesign.md). Multi-group staged workflows are the most complex routing path the engine handles, and proving they work end-to-end through every harness is essential for a confident compatibility declaration.

---

## Expected Run: Auto Mode

Five dispatches total: one pre-consultation, then four workflow steps (2 stages × 2 groups). The pre-consultation allocates no `Seq` and leaves no `Orchestration.md` row — it surfaces only in the dispatch log.

| Log `Seq` | `Agent` | `Phase` | `Iteration` | `Status` | `Summary` shows |
|:---:|---|---|---|---|---|
| 1 | `mosaictest-scripted#1` | EXECUTION | 1.Test | SUCCESS | Test group / stage 1 / wrote test artifact |
| 2 | `mosaictest-scripted#2` | EXECUTION | 1.Implementation | SUCCESS | Implementation group / stage 1 / wrote impl artifact |
| 3 | `mosaictest-scripted#3` | EXECUTION | 2.Test | SUCCESS | Test group / stage 2 / wrote test artifact |
| 4 | `mosaictest-scripted#4` | EXECUTION | 2.Implementation | SUCCESS | Implementation group / stage 2 / wrote impl artifact |

**Run outcome:** COMPLETE. All stages exhausted, no deviations.

**Key observations:**
- Rows 1 and 2 are stage 1: Test before Implementation.
- Rows 3 and 4 are stage 2: Test before Implementation again.
- If group ordering were wrong (Implementation before Test), the chained input artifact would be missing and the run would fail with BLOCKED.
- The execution log carries the bare phase name in `Phase` (`EXECUTION`, never `EXECUTION.[Stage].Test`) and the iteration ID and group in `Iteration` (`1.Test`).

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| Run refuses to start, naming `Plan.md` | Plan fixture was not seeded, or the seed root was wrong. Check the seed path. |
| BLOCKED on the Implementation row, naming the Test artifact | Group ordering is wrong: Implementation dispatched before Test within a stage. The chained input did not exist yet. |
| All Test rows run before any Implementation row | The engine is not resetting group ordering per stage — it is running all of one group across all stages, then the other. |
| `Iteration` column shows `1` or `2` without a group name | The `EXECUTION.[Stage].{Group}` notation was not parsed. The engine may be treating rows as ungrouped. |
| `Phase` column shows `EXECUTION.[Stage].Test` (literal brackets) | The log wrote routing-table notation instead of the bare phase name and the iteration. |
| Only 2 workflow steps instead of 4 | Stage progression is broken — only one stage ran. Check Plan.md for two stages. |
| Orchestrator consulted mid-run (extra `orchestrator-script` entries in the dispatch log) | A deviation occurred. All rows should return SUCCESS in this workflow; a deviation means the stub script was not found or returned an unexpected status. |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-09-20 | MOSAIC | Initial version. Multi-group staged execution with Test/Implementation ordering. |
| 1.1 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16 notation: `EXECUTION.[Stage].Test` / `.Implementation` rows with `**Blocks:**` and `**Groups:**` tables, `{Stage}` fill-in, `COMPLETE` instead of `-` in `On Success`, unique `Subagent` cells through the mode qualifiers `test` and `impl`, `Plan.md` listed in `artifacts`. Expected Run uses the OrchestrationArtifactFormat 3.0 log values (`Phase` `EXECUTION`, `Iteration` `1.Test`). Dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A three-group variant (Test, Implementation, Review) to verify ordering with more than two groups.
- Adding HITL=TRUE on a specific stage in the plan to test per-stage HITL resolution combined with group ordering.
- A variant where one group has two rows per stage, to test intra-group row ordering alongside inter-group ordering.

**Dead ends (tried and rejected):**
- Using alphabetical group names (Alpha, Beta) to test ordering. The TDD approach defines a specific Test-before-Implementation order; alphabetical names would be ambiguous about what "correct" means.
