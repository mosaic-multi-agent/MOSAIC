# Plan: MosaicTest Infrastructure Checkpoint+Commit

> [WARN] **FIXTURE ARTIFACT** — pre-placed by hand before the run, not produced by a planner agent.
> This file exists so the runner has a block schedule to read in a workflow with no planning row.

## Overview
Two stages of fixture work. Two stages are needed so that ITERATION_END fires mid-run (at the stage 1/2 boundary), not only at run completion.

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| 1 | First infrastructure stage | Run the EXECUTION row once and trigger ITERATION_END for checkpoint and commit | - | FALSE | direct |
| 2 | Second infrastructure stage | Run the EXECUTION row again and trigger ITERATION_END for checkpoint and commit | 1 | FALSE | direct |

## Unresolved Questions
<!-- Empty = plan is complete. -->

## Fixture Notes

The `Approach` column is `direct` because the `Stage` block of this workflow has no groups. A block without groups must ignore the Approach column entirely; a value is present to guard against a regression where such blocks start honouring approaches.
