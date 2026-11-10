<Workflow type="core" name="request-wire-restore" version="1.0">
## Workflow Whose Second Row Is A Restore-Class Agent

The restore-class agent has no automatic trigger path; a consultant or a manual
dispatch reaches it through its routing row, which declares no inputs or
outputs.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | checkpoint-restore-git | - | - | - |
| PLANNING | checkpoint-restore-git | FALSE | COMPLETE | - | - | - |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="checkpoint-restore-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| restore | MANUAL | - | halt | Git checkpoint restore |
</InfrastructureAgent>
</InfrastructureAgents>
