<Workflow type="core" name="linear" version="1.0">
## Three-step Linear Workflow With HITL On The Middle Step

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | TRUE | agent-c | - | plan.md | b-out.md |
| PLANNING | agent-c | FALSE | COMPLETE | - | b-out.md | c-out.md |
</Workflow>
