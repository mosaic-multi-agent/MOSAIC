<Workflow type="core" name="stage-grouped" version="1.0">
## Grouped Stage Block With A Commit Agent On ITERATION_END

A build workflow of Stage iterations with a Test and an Implementation group.
Plan.md is a seed read from the run folder. ContractsDesign.md is shared by
every iteration: it carries no fill-in, so a commit never receives it.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage].Test | test-writer | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review | FALSE | COMPLETE | test-writer | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementer | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | impl-review | FALSE | COMPLETE | implementer | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | halt | Git commit manager (fires at iteration end) |
</InfrastructureAgent>
</InfrastructureAgents>
