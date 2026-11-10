<Workflow type="core" name="fork-all-leave" version="1.0">
## Fork Whose Every Target Leaves The Phase

Orchestrated (Mode 1) table. Both fork targets lie in REPORT, so the fork row
(scout) ends RESEARCH by its declared routing, although a later row (cleanup)
carries the RESEARCH phase name again.

| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |
|-------|----------|:----:|------------|-------------|-----------|-------|--------|
| RESEARCH | scout | FALSE | writer-a, writer-b | - | - | - | a.md |
| REPORT | writer-a | FALSE | publisher | - | - | - | b.md |
| REPORT | writer-b | FALSE | publisher | - | - | - | c.md |
| REPORT | publisher | FALSE | cleanup | - | writer-a, writer-b | - | d.md |
| RESEARCH | cleanup | FALSE | COMPLETE | - | - | - | e.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="phase-committer" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | PHASE_END | - | halt | Commit manager that fires at the end of a phase |
</InfrastructureAgent>
</InfrastructureAgents>
