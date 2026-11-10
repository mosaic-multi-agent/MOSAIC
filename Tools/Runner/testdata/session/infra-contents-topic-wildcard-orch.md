<Workflow type="core" name="topic-wildcard" version="1.0">
## Topic Block With A Fill-In Wildcard Input And A Commit Agent

The synthesizer reads the findings files of its own Topic through the entry
*-{Topic}-Findings.md, and the product maps of every topic through
*-ProductMap.md (no fill-in).

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Topic] | - | - | TopicPlan.md |
| EXECUTION.[Topic] | researcher | FALSE | NEXT | - | TopicPlan.md | notes-{Topic}.md |
| EXECUTION.[Topic] | synthesizer | FALSE | COMPLETE | - | *-{Topic}-Findings.md, *-ProductMap.md | summary-{Topic}.md |

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
