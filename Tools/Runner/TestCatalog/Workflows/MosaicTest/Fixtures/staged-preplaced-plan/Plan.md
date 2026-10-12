# Plan: MosaicTest Staged Pre-placed Plan

> [WARN] **FIXTURE ARTIFACT** — pre-placed by hand before the run, not produced by a planner agent.
> This file exists so the runner has a block schedule to read in a workflow with no planning row.

## Overview
Two stages of fixture work, so that stage progression can be distinguished from row progression in the execution log.

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| 1 | First fixture stage | Run both EXECUTION rows once and write Stage-1/MosaicTestStage.md | - | FALSE | TDD |
| 2 | Second fixture stage | Run both EXECUTION rows again and write Stage-2/MosaicTestStage.md | 1 | FALSE | TDD |

## Unresolved Questions
<!-- Empty = plan is complete. -->

## Fixture Notes

The `Approach` column is populated deliberately even though the `Stage` block of `staged-preplaced-plan` has no groups. A block without groups must ignore this column entirely. Observing that both stages run their rows in table order — rather than doing anything TDD-shaped — is one of the assertions of this fixture.

`Depends On` is set on stage 2 so the dependency parser has something non-trivial to read. IDs are unique and `Depends On` names only rows above it, both of which the runner validates.
