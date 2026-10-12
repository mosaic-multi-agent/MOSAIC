---
version: "1.1"
name: "MosaicTest Staged Pre-placed Plan Workflow"
description: "Harness conformance fixture — staged execution driven by a pre-placed Plan.md, with no planning row. Isolates block-schedule reading and {Stage} fill-in from planner behaviour."
hint: "Harness test — iteration progression and {Stage} fill-in from a fixture plan"
author: MOSAIC
id: staged-preplaced-plan
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/stage-write.md
  - MosaicTestScript/stage-echo.md
  - Stage-{Stage}/MosaicTestStage.md
  - Plan.md
modes:
  - auto
  - auto-review
---

<Workflow type="core" name="staged-preplaced-plan" version="1.1">
## MosaicTest Staged Pre-placed Plan Workflow

**Use when:** Verifying that the runner reads a block schedule, progresses through its iterations in order, and fills `{Stage}` into artifact paths — without any planner agent involved.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | mosaictest-scripted(write) | FALSE | NEXT | - | MosaicTestScript/stage-write.md | Stage-{Stage}/MosaicTestStage.md |
| EXECUTION.[Stage] | mosaictest-scripted(echo) | FALSE | COMPLETE | - | MosaicTestScript/stage-echo.md, Stage-{Stage}/MosaicTestStage.md | - |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Notes:**
- **`Plan.md` is a pre-placed fixture, not produced by the run.** It and the scripts must be seeded into the run folder at run creation. Seed `Fixtures/staged-preplaced-plan` — the whole directory, not anything inside it — as the single seed path. See `Fixtures/README.md`.
- **No row produces `Plan.md`.** The block's schedule source is therefore a seed (WorkflowFormat 2.16 sec. 6.7), which is deliberate: it is what lets the plan be pre-placed. No row outputs the declared Source, so the schedule is read once and never re-read.
- The `Stage` block runs both rows for every row of the `## Stages` table, in table order. `NEXT` moves from the first row to the second and `COMPLETE` ends the iteration.
- The two rows run the same agent, so their `Subagent` cells are told apart by the mode qualifiers `write` and `echo`. Dispatch, instance ids and logs use the bare id `mosaictest-scripted`.
- The block has no groups and there is no `**Groups:**` table. The fixture plan carries an `Approach` column anyway, which a block without groups must ignore entirely — that non-effect is itself an assertion of this fixture.

</Workflow>

---

## Design Rationale

Two rows and two stages give four invocations, which is the smallest run that can distinguish "stages advanced" from "rows advanced." A single row would leave the two indistinguishable in the log; a third row adds nothing new.

The second row takes the first row's output as its input. That chains the stage artifact within a stage and proves `{Stage}` filled in the same value for both rows — a substitution bug that produced `Stage-1/` for one row and `Stage-2/` for the next would otherwise pass unnoticed, because each file would still be written somewhere plausible.

**Why the plan is pre-placed.** `staged-single-group` (not yet authored) covers the other half of this: a stub planner that writes the block schedule itself, with an Output that produces the block's declared Source `Plan.md`, which makes the runner re-read the schedule once that row has run. Splitting the two means a schedule-reading failure here cannot be blamed on planner behaviour, and vice versa.

**Why the fixture plan carries an `Approach` column.** A block without groups ignores it. That is documented behaviour (WorkflowFormat 2.16 sec. 8.1), so a fixture that supplies a value and observes no effect is worth having — it guards against a regression where blocks without groups start honouring approaches.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-03 | MOSAIC | Initial version |
| 1.1 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16 notation: `EXECUTION.[Stage]` with a `**Blocks:**` table over `Plan.md`, `{Stage}` fill-in, `NEXT` and `COMPLETE` instead of `-` in `On Success`, unique `Subagent` cells through the mode qualifiers `write` and `echo`, `Plan.md` listed in `artifacts`. Notes and rationale describe the pre-placed schedule as a seed. Dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A three-stage variant to check that stage progression is not hardcoded to two.
- Setting stage 2's HITL to TRUE in the fixture plan to assert per-stage HITL resolution (effective HITL is the row flag OR the stage flag).

**Dead ends (tried and rejected):**
- Declaring `Stage-{Stage}/Plan.md` as a row input. It would require per-stage plan files in the fixture tree for no added coverage — the `{Stage}` fill-in is already proven by the output path.
