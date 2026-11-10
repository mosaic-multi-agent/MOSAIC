<Workflow type="core" name="fork-join" version="1.0">
## Fork Join Workflow

Orchestrated (Mode 1) table: agent-a forks to agent-b and agent-c, and agent-d
joins both.

| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |
|-------|----------|:----:|------------|-------------|-----------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b, agent-c | - | - | - | a.md |
| PLANNING | agent-b | FALSE | agent-d | - | - | a.md | b.md |
| PLANNING | agent-c | FALSE | agent-d | - | - | a.md | c.md |
| PLANNING | agent-d | FALSE | COMPLETE | - | agent-b, agent-c | b.md, c.md | d.md |
</Workflow>
