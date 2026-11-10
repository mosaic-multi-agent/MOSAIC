<Workflow type="core" name="sibling-blocks-cross-writer" version="1.0">
## Two Sibling Blocks, A Row Of One Block Also Writes The Other's Source

Alpha reads AlphaPlan.md, produced by the planner. Beta reads BetaPlan.md,
produced by the bridge. The beta-worker, a row of Beta, also outputs
AlphaPlan.md, the Source of the other block.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Alpha] | - | - | AlphaPlan.md |
| EXECUTION.[Alpha] | alpha-worker | FALSE | COMPLETE | - | AlphaPlan.md | alpha-{Alpha}.md |
| REVIEW | bridge | FALSE | [Beta] | - | - | BetaPlan.md |
| REVIEW.[Beta] | beta-worker | FALSE | COMPLETE | - | BetaPlan.md | beta-{Beta}.md, AlphaPlan.md |
| COMPLETION | finisher | FALSE | COMPLETE | - | - | done.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Alpha | AlphaPlan.md | Stages |
| Beta | BetaPlan.md | Stages |
</Workflow>
