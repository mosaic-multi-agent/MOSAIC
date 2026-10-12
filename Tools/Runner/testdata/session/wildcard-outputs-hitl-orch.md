<Workflow type="core" name="wildcard-outputs-hitl" version="1.0">
## One Row Needing Approval With A Wildcard Output

The row declares Report-*/notes.md and the concrete summary.md as Outputs. The
approval of what it wrote is decided over the files that match.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | TRUE | COMPLETE | - | - | Report-*/notes.md, summary.md |
</Workflow>
