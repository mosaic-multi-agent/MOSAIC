<Workflow type="core" name="fork-join-leaves" version="1.0">
## Fork Closed By A Join Whose On Success Leaves The Phase

Orchestrated (Mode 1) table. The fork's targets lie in its own phase, so the
fork is not the end of RESEARCH; the join routes to REPORT, so the join is,
although a later row (cleanup) carries the RESEARCH phase name again.

| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |
|-------|----------|:----:|------------|-------------|-----------|-------|--------|
| RESEARCH | scout | FALSE | research-a, research-b | - | - | - | a.md |
| RESEARCH | research-a | FALSE | merge | - | - | - | b.md |
| RESEARCH | research-b | FALSE | merge | - | - | - | c.md |
| RESEARCH | merge | FALSE | reporter | - | research-a, research-b | - | d.md |
| REPORT | reporter | FALSE | cleanup | - | - | - | e.md |
| RESEARCH | cleanup | FALSE | COMPLETE | - | - | - | f.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="phase-committer" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | PHASE_END | - | halt | Commit manager that fires at the end of a phase |
</InfrastructureAgent>
</InfrastructureAgents>
