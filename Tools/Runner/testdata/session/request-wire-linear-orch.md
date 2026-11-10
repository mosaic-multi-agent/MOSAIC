<Workflow type="core" name="request-wire" version="1.0">
## Linear Workflow With Empty Input And Output Cells

Row 1 declares neither inputs nor outputs, row 2 only an output, row 3 only an
input.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | - |
| PLANNING | agent-b | FALSE | agent-c | - | - | result.md |
| PLANNING | agent-c | FALSE | COMPLETE | - | result.md | - |
</Workflow>
