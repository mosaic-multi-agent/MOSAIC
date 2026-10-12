<Workflow type="core" name="wildcard-inputs" version="1.0">
## Two Run-Scope Rows, The Second Takes Wildcard Inputs

No block is declared. The first row declares a wildcard Output, which reaches
the subagent as written. The second row declares Input wildcards of three
forms and one that matches nothing in the run folder.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md, Notes-*/draft.md |
| PLANNING | agent-b | FALSE | COMPLETE | - | Stage-*/Plan.md, *-Research.md, Absent-*/Notes.md | result.md |
</Workflow>
