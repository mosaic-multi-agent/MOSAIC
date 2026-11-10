<Workflow type="core" name="wildcard-block" version="1.0">
## Block Not Named Stage, Inputs With A Wildcard Of The Form *-Research.md

The block is named Feature and reads its schedule from FeaturePlan.md. A row of
the block and a row after it take the Input wildcard *-Research.md, which names
neither the block nor any schedule.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Feature] | - | - | FeaturePlan.md |
| EXECUTION.[Feature] | builder | FALSE | COMPLETE | - | FeaturePlan.md, *-Research.md | built-{Feature}.md |
| REVIEW | reviewer | FALSE | COMPLETE | - | *-Research.md | review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Feature | FeaturePlan.md | Stages |
</Workflow>
