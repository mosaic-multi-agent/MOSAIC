<Workflow type="core" name="kb-progress-block" version="1.0">
## Block Schedule From A Non-Plan Artifact

The bootstrapper outputs KBProgress.md, whose Generation section is the
schedule of the Generation block. The name of the Source does not end in
Plan.md, and the file holds another table before the schedule.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | bootstrapper | FALSE | [Generation] | - | - | KBProgress.md |
| EXECUTION.[Generation] | writer | FALSE | COMPLETE | - | KBProgress.md | docs/{Generation}.md |
| COMPLETION | indexer | FALSE | COMPLETE | - | - | index.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Generation | KBProgress.md | Generation |
</Workflow>
