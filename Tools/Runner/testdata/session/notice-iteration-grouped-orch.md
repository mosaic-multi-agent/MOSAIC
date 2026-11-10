<Workflow type="core" name="notice-iteration-grouped" version="1.0">
## Grouped Block Rows, Seeded Plan.md

The first row is a block row of group Test, so Plan.md is a seed. A reporter
row outside every block follows the block.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage].Test | test-writer | FALSE | NEXT | - | Plan.md | out-1-{Stage}.md |
| EXECUTION.[Stage].Test | tests-review | FALSE | COMPLETE | test-writer | Plan.md | out-2-{Stage}.md |
| REPORT | reporter | FALSE | COMPLETE | - | - | report.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | Tests-Only | Test |
</Workflow>
