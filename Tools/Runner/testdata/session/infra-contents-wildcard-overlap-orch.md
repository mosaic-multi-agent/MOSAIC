<Workflow type="core" name="wildcard-overlap" version="1.0">
## Topic Block Whose Concrete Input Is Also Matched By A Fill-In Wildcard

The synthesizer names A-{Topic}-Findings.md explicitly and also reads every
findings file of its Topic through *-{Topic}-Findings.md, so the concrete file
is reached twice. The summary it writes follows the wildcard in table order.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Topic] | - | - | TopicPlan.md |
| EXECUTION.[Topic] | researcher | FALSE | NEXT | - | TopicPlan.md | notes-{Topic}.md |
| EXECUTION.[Topic] | synthesizer | FALSE | COMPLETE | - | A-{Topic}-Findings.md, *-{Topic}-Findings.md | summary-{Topic}.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Topic | TopicPlan.md | Topics |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | halt | Git commit manager (fires at iteration end) |
</InfrastructureAgent>
</InfrastructureAgents>
