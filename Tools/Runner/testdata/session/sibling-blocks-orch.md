<Workflow type="core" name="sibling-blocks" version="1.0">
## Two Sibling Blocks With Different Sources

Alpha reads AlphaPlan.md, produced by the planner. Beta reads BetaPlan.md,
produced by the bridge, a run-scope row between the blocks. Both schedules
sit in a section named Stages.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Alpha] | - | - | AlphaPlan.md |
| EXECUTION.[Alpha] | alpha-worker | FALSE | COMPLETE | - | AlphaPlan.md | alpha-{Alpha}.md |
| REVIEW | bridge | FALSE | [Beta] | - | - | BetaPlan.md |
| REVIEW.[Beta] | beta-worker | FALSE | COMPLETE | - | BetaPlan.md | beta-{Beta}.md |
| COMPLETION | finisher | FALSE | COMPLETE | - | - | done.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Alpha | AlphaPlan.md | Stages |
| Beta | BetaPlan.md | Stages |
</Workflow>
