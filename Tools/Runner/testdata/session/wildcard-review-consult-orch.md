<Workflow type="core" name="wildcard-review-consult" version="1.0">
## Two Run-Scope Rows With Wildcard Inputs And A Review-Class Agent

A review-class infrastructure agent fires after every workflow step and
triggers a routing consultation in Mode 2. The consultant dispatches the second
row, whose defaults hold Input wildcards.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | FALSE | COMPLETE | - | Stage-*/Plan.md, *-Research.md | result.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="review-agent" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| review | INVOCATION_INTERVAL | 1 | continue | Review agent |
</InfrastructureAgent>
</InfrastructureAgents>
