<Workflow type="core" name="seed-precedence" version="1.0">
## Seeded Backlog.md Also Output By A Row Before The Block

The first row lists Backlog.md as an Input, so Backlog.md is a seed. The same
row also outputs it, but a seed is not a produced Source: that row is not a
producer of the block's schedule.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Item] | - | Backlog.md | Backlog.md |
| EXECUTION.[Item] | worker | FALSE | COMPLETE | - | Backlog.md | done-{Item}.md |
| REVIEW | closer | FALSE | COMPLETE | - | - | closed.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Item | Backlog.md | Backlog |
</Workflow>
