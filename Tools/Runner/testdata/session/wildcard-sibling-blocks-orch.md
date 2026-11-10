<Workflow type="core" name="wildcard-sibling-blocks" version="1.0">
## Two Sibling Blocks, Rows With Stage-*/Plan.md Inputs

Alpha reads AlphaPlan.md and Beta reads BetaPlan.md. A row of Alpha and the row
after both blocks take the Input wildcard Stage-*/Plan.md, which refers to
neither schedule.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Alpha] | - | - | AlphaPlan.md |
| EXECUTION.[Alpha] | alpha-worker | FALSE | COMPLETE | - | AlphaPlan.md, Stage-*/Plan.md | alpha-{Alpha}.md |
| REVIEW | bridge | FALSE | [Beta] | - | - | BetaPlan.md |
| REVIEW.[Beta] | beta-worker | FALSE | COMPLETE | - | BetaPlan.md | beta-{Beta}.md |
| COMPLETION | finisher | FALSE | COMPLETE | - | Stage-*/Plan.md | done.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Alpha | AlphaPlan.md | Stages |
| Beta | BetaPlan.md | Stages |
</Workflow>
