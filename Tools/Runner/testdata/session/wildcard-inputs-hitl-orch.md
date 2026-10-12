<Workflow type="core" name="wildcard-inputs-hitl" version="1.0">
## Second Row Needs Approval And Takes Wildcard Inputs

The second row declares an Input wildcard and needs human approval of its
Output, so a rejected attempt is dispatched again with the same Inputs.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | TRUE | COMPLETE | - | Stage-*/Plan.md | result.md |
</Workflow>
