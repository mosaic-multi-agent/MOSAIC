package integration_test

// Fixtures for the five supported workflows exercised end-to-end.

// planImplOnly is a single-stage Plan.md with Implementation-Only approach,
// used by two-group workflows (TwoGroup=true, needsApproach=true).
const planImplOnly = `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| 1 | Stage One | Run | - | FALSE | Implementation-Only |
`

// planSingleGroup is a single-stage Plan.md without an Approach column,
// used by single-group workflows (TwoGroup=false, needsApproach=false).
const planSingleGroup = `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL |
|-------|------|------|------------|:----:|
| 1 | Stage One | Run | - | FALSE |
`

const orchBrownfieldTDD = `			<Workflow type="core" name="brownfield-tdd" version="3.5">
## Brownfield TDD Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | tests-review-tdd | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
</Workflow>
`

const orchBrownfieldTDDBuildVerified = `			<Workflow type="core" name="brownfield-tdd-build-verified" version="2.1">
## Brownfield TDD Build-Verified Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | build-review(tests) | - | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | build-review(tests) | FALSE | tests-review-tdd | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-tests.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | build-review(impl) | - | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | build-review(impl) | FALSE | implementation-review | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-impl.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
</Workflow>
`

const orchGreenfieldTDD = `			<Workflow type="core" name="greenfield-tdd" version="3.4">
## Greenfield TDD Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | system-designer | requirements-refinement | Requirements.md | requirements-review.md |
| ARCHITECTURE | system-designer | TRUE | system-design-review | - | Requirements.md | SystemDesign.md |
| ARCHITECTURE | system-design-review | FALSE | planner-tdd-soft | system-designer | Requirements.md, SystemDesign.md | system-design-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Requirements.md, SystemDesign.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Requirements.md, Plan.md, Stage-*/Plan.md, SystemDesign.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | tests-review-tdd | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
</Workflow>
`

const orchImplementationOnly = `			<Workflow type="core" name="implementation-only" version="3.1">
## Implementation Only Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
`

const orchQuickFix = `			<Workflow type="core" name="quick-fix" version="3.0">
## Quick Fix Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | - | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | [Stage] | planner-tdd-soft | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
`

// integrationAgentResponse is a scripted SUCCESS response for a named agent.
type integrationAgentResponse struct {
	agent   string
	summary string
}

// integrationWorkflowCase describes one workflow run end-to-end.
type integrationWorkflowCase struct {
	name        string
	workflowID  string
	orchContent string
	plan        string
	agents      []string
	responses   []integrationAgentResponse
}

// newTestFiveWorkflowCases returns the five supported workflow scenarios.
func newTestFiveWorkflowCases() []integrationWorkflowCase {
	return []integrationWorkflowCase{
		{
			name:       "brownfield-tdd",
			workflowID: "brownfield-tdd",
			// Routing table matches brownfield-tdd v3.4.
			// Two-group: test group (test-writer-tdd, tests-review-tdd) +
			//            impl group (implementation-tdd, implementation-review).
			// Seven pre-exec rows; post-exec row: test-runner.
			orchContent: orchBrownfieldTDD,
			plan:        planImplOnly,
			agents: []string{
				"codebase-research", "requirements-refinement", "requirements-review",
				"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
				"test-writer-tdd", "tests-review-tdd",
				"implementation-tdd", "implementation-review",
				"test-runner",
			},
			// Dispatch order with Implementation-Only approach (impl group only):
			// 7 pre-exec + 2 exec (Stage-1) + 1 post-exec = 10
			responses: []integrationAgentResponse{
				{"codebase-research", "research done"},
				{"requirements-refinement", "requirements refined"},
				{"requirements-review", "requirements reviewed"},
				{"planner-tdd-soft", "plan created"},
				{"plan-review", "plan reviewed"},
				{"contracts-designer", "contracts designed"},
				{"contracts-review", "contracts reviewed"},
				{"implementation-tdd", "stage 1 implemented"},
				{"implementation-review", "stage 1 reviewed"},
				{"test-runner", "tests passed"},
			},
		},
		{
			name:       "brownfield-tdd-build-verified",
			workflowID: "brownfield-tdd-build-verified",
			// Two-group: test group (test-writer-tdd, build-review, tests-review-tdd) +
			//            impl group (implementation-tdd, build-review, implementation-review).
			// Seven pre-exec rows; no post-exec rows.
			orchContent: orchBrownfieldTDDBuildVerified,
			plan:        planImplOnly,
			agents: []string{
				"codebase-research", "requirements-refinement", "requirements-review",
				"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
				"test-writer-tdd", "build-review", "tests-review-tdd",
				"implementation-tdd", "implementation-review",
			},
			// Dispatch order with Implementation-Only (impl group: rows 10-12):
			// 7 pre-exec + 3 exec (Stage-1) = 10
			responses: []integrationAgentResponse{
				{"codebase-research", "research done"},
				{"requirements-refinement", "requirements refined"},
				{"requirements-review", "requirements reviewed"},
				{"planner-tdd-soft", "plan created"},
				{"plan-review", "plan reviewed"},
				{"contracts-designer", "contracts designed"},
				{"contracts-review", "contracts reviewed"},
				{"implementation-tdd", "stage 1 implemented"},
				{"build-review", "stage 1 build ok"},
				{"implementation-review", "stage 1 reviewed"},
			},
		},
		{
			name:       "greenfield-tdd",
			workflowID: "greenfield-tdd",
			// Two-group: test group (test-writer-tdd, tests-review-tdd) +
			//            impl group (implementation-tdd, implementation-review).
			// Eight pre-exec rows; post-exec row: test-runner.
			orchContent: orchGreenfieldTDD,
			plan:        planImplOnly,
			agents: []string{
				"requirements-refinement", "requirements-review",
				"system-designer", "system-design-review",
				"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
				"test-writer-tdd", "tests-review-tdd",
				"implementation-tdd", "implementation-review",
				"test-runner",
			},
			// Dispatch order with Implementation-Only (impl group rows 10-11):
			// 8 pre-exec + 2 exec (Stage-1) + 1 post-exec = 11
			responses: []integrationAgentResponse{
				{"requirements-refinement", "requirements refined"},
				{"requirements-review", "requirements reviewed"},
				{"system-designer", "system designed"},
				{"system-design-review", "system design reviewed"},
				{"planner-tdd-soft", "plan created"},
				{"plan-review", "plan reviewed"},
				{"contracts-designer", "contracts designed"},
				{"contracts-review", "contracts reviewed"},
				{"implementation-tdd", "stage 1 implemented"},
				{"implementation-review", "stage 1 reviewed"},
				{"test-runner", "tests passed"},
			},
		},
		{
			name:       "implementation-only",
			workflowID: "implementation-only",
			// Single-group: implementation-tdd → implementation-review.
			// No pre-exec rows; post-exec row: test-runner.
			orchContent: orchImplementationOnly,
			plan:        planSingleGroup,
			agents:      []string{"implementation-tdd", "implementation-review", "test-runner"},
			// Dispatch order: 2 exec (Stage-1) + 1 post-exec = 3
			responses: []integrationAgentResponse{
				{"implementation-tdd", "stage 1 implemented"},
				{"implementation-review", "stage 1 reviewed"},
				{"test-runner", "tests passed"},
			},
		},
		{
			name:       "quick-fix",
			workflowID: "quick-fix",
			// Single-group: implementation-tdd (EXECUTION).
			// Two pre-exec rows; post-exec row: test-runner.
			orchContent: orchQuickFix,
			plan:        planSingleGroup,
			agents:      []string{"planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner"},
			// Dispatch order: 2 pre-exec + 1 exec (Stage-1) + 1 post-exec = 4
			responses: []integrationAgentResponse{
				{"planner-tdd-soft", "plan created"},
				{"plan-review", "plan reviewed"},
				{"implementation-tdd", "stage 1 implemented"},
				{"test-runner", "tests passed"},
			},
		},
	}
}
