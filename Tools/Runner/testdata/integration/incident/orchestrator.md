<Workflow type="core" name="incident-pr-fix" version="1.0">
## Incident Workflow (shape of a brownfield pull request fix)

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | pull-request-comment-interface(retrieve) | FALSE | requirements-refinement | - | Requirements.md | PullRequestComments.md, PullRequestContext.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Requirements.md, PullRequestComments.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | codebase-research | requirements-refinement | Requirements.md | requirements-review.md |
| RESEARCH | codebase-research | FALSE | planner-tdd-soft | - | Requirements.md, PullRequestComments.md, PullRequestContext.md | Research.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md, PullRequestComments.md, PullRequestContext.md | Plan.md |
| PLANNING | plan-review | FALSE | [Stage] | planner-tdd-soft | Requirements.md, PullRequestComments.md, PullRequestContext.md, Plan.md | plan-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | tests-review-tdd | - | PullRequestContext.md, Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | PullRequestContext.md, Stage-{Stage}/Plan.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Impl | implementation-tdd | FALSE | implementation-review | - | PullRequestContext.md, Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Impl | implementation-review | FALSE | COMPLETE | implementation-tdd | PullRequestContext.md, Stage-{Stage}/Plan.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Impl |
| Stage | Implementation-First | Impl, Test |
| Stage | Implementation-Only | Impl |
| Stage | Tests-Only | Test |
</Workflow>
