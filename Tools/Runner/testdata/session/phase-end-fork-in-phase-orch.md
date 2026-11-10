<Workflow type="core" name="fork-in-phase" version="1.0">
## Fork With One Target In Its Own Phase

Orchestrated (Mode 1) table. The fork row (scout) is the last RESEARCH row by
position, but one of its targets (prep) is a RESEARCH row, so the fork is not
the end of RESEARCH. The planner jumps over prep, which runs only as a fork
target and routes to REPORT: prep is the end of RESEARCH.

| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |
|-------|----------|:----:|------------|-------------|-----------|-------|--------|
| PLANNING | planner | FALSE | scout | - | - | - | a.md |
| RESEARCH | prep | FALSE | merge | - | - | - | b.md |
| RESEARCH | scout | FALSE | writer, prep | - | - | - | c.md |
| REPORT | writer | FALSE | merge | - | - | - | d.md |
| REPORT | merge | FALSE | COMPLETE | - | prep, writer | - | e.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="phase-committer" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | PHASE_END | - | halt | Commit manager that fires at the end of a phase |
</InfrastructureAgent>
</InfrastructureAgents>
