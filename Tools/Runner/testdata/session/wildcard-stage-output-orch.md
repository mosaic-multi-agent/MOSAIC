<Workflow type="core" name="wildcard-stage-output" version="1.0">
## One Row Needing Approval With A Stage-Star Output

The row declares Stage-*/notes.md and the concrete summary.md as Outputs. The
workflow has no schedule: the files that match are found in the run folder.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | TRUE | COMPLETE | - | - | Stage-*/notes.md, summary.md |
</Workflow>
