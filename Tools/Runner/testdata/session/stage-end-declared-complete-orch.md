<Workflow type="core" name="declared-complete" version="1.0">
## Block Whose First Row Declares COMPLETE

Orchestrated (Mode 1) table. The worker declares COMPLETE, so by its declared
routing it ends its iteration; the checker below it is the last row of the
scope. Requires Plan.md in RunFolder.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| BUILD.[Stage] | worker | FALSE | COMPLETE | - | - | - |
| BUILD.[Stage] | checker | FALSE | NEXT | - | - | - |
| REPORT | reporter | FALSE | COMPLETE | - | - | - |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | STAGE_END | - | halt | Git commit manager (fires at iteration end) |
</InfrastructureAgent>
</InfrastructureAgents>
