<Workflow type="core" name="linear" version="1.0">
## Three-step Linear Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | FALSE | agent-c | - | plan.md | b-out.md |
| PLANNING | agent-c | FALSE | COMPLETE | - | b-out.md | c-out.md |
</Workflow>
