<Workflow type="core" name="consulted" version="1.0">
## Declared Route Leaves The Phase

Orchestrated (Mode 1) table. The scout declares the reporter next, a row of
another phase, so it ends RESEARCH by its declared routing. The analyst is a
later RESEARCH row that the orchestrator may run next anyway.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | scout | FALSE | reporter | - | - | a.md |
| RESEARCH | analyst | FALSE | NEXT | - | - | b.md |
| REPORT | reporter | FALSE | COMPLETE | - | - | c.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="phase-committer" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | PHASE_END | - | halt | Commit manager that fires at the end of a phase |
</InfrastructureAgent>
</InfrastructureAgents>
