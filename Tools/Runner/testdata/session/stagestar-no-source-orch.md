<Workflow type="core" name="stagestar-no-source" version="1.0">
## Output With Stage-* That Produces No Schedule Source

The planner outputs a path containing Stage-*, which does not produce the
block's Source (Plan.md). No row outputs Plan.md and the first row lists it
as an Input, so it is a seed.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Stage] | - | Plan.md | Stage-*/Notes.md |
| EXECUTION.[Stage] | worker | FALSE | COMPLETE | - | Plan.md | out-{Stage}.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
