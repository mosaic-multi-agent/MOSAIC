<Workflow type="core" name="seeded-backlog-first-block" version="1.0">
## Seeded Backlog.md, First Row Begins The Block

The first row is a block row, so Backlog.md is a seed. No row outputs it, and
its name does not end in Plan.md.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Item] | worker | FALSE | COMPLETE | - | Backlog.md | done-{Item}.md |
| REVIEW | closer | FALSE | COMPLETE | - | - | closed.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Item | Backlog.md | Backlog |
</Workflow>
