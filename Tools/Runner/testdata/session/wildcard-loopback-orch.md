<Workflow type="core" name="wildcard-loopback" version="1.0">
## Creator And Reviewer With Wildcards On The Loop-Back

Two run-scope rows. The creator lists a wildcard Input and a concrete Input
that the wildcard also matches. The reviewer declares a wildcard Output and
routes findings back to the creator.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | notes/*.md, notes/n1.md | design.md |
| PLANNING | agent-b | FALSE | COMPLETE | agent-a | design.md | reviews/*.md |
</Workflow>
