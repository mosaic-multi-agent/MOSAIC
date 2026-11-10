# Plan: MosaicTest Staged Multi-Group

> [WARN] **FIXTURE ARTIFACT** — pre-placed by hand before the run, not produced by a planner agent.
> This file exists so the runner has a block schedule to read in a workflow with no planning row.

## Overview
Two stages of fixture work with two groups (Test, Implementation). The TDD approach means Test group rows dispatch first within each stage, then Implementation group rows.

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| 1 | First stage | Run Test group then Implementation group | - | FALSE | TDD |
| 2 | Second stage | Run Test group then Implementation group again | 1 | FALSE | TDD |

## Unresolved Questions
<!-- Empty = plan is complete. -->

## Fixture Notes

The `Approach` column is `TDD` on both stages. This is the value the `**Groups:**` table maps to the order Test, then Implementation. It is the single most important value in this fixture — changing it changes the dispatch order, which is the assertion under test.

`Depends On` is set on stage 2 so the dependency parser has something non-trivial to read. IDs are unique and `Depends On` names only rows above it, both of which the runner validates.
