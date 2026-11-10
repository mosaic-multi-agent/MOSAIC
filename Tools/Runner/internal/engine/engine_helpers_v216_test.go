package engine_test

// WorkflowFormat 2.16 counterparts of the workflow fixtures in
// engine_helpers_test.go, with the same agents in the same row order. Block
// rows use the [Stage] block, grouped rows the Test and Implementation groups;
// a row that repeats an agent carries a mode qualifier. They are parsed with
// the strict parser only, so a fixture can never pass through the 1.x
// transitional path.

import (
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

const v216Columns = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

const v216StageBlock = `
**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
`

const v216StageGroups = `
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
`

// mustParseAndAdmitV216 parses a 2.16 region with the strict parser and admits
// it in auto mode.
func mustParseAndAdmitV216(t *testing.T, content, id, version string) domain.AdmittedWorkflow {
	t.Helper()
	info := domain.WorkflowInfo{ID: domain.WorkflowID(id), Version: domain.WorkflowVersion(version)}
	table, err := workflow.Parse([]byte(content), info)
	if err != nil {
		t.Fatalf("workflow.Parse(%s): %v", id, err)
	}
	aw, err := compat.Admit(table, domain.ExecutionModeAuto)
	if err != nil {
		t.Fatalf("compat.Admit(%s): %v", id, err)
	}
	return aw
}

// quickFixV216Content: planner, plan review, one ungrouped Stage block row,
// then a run-scope test runner.
const quickFixV216Content = v216Columns +
	`| PLANNING | planner-tdd-soft | TRUE | plan-review | - | - | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | [Stage] | planner-tdd-soft | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |
` + v216StageBlock

// brownfieldTDDV216Content: research, planning and design, then the grouped
// Stage block (Test group rows 8-9, Implementation group rows 10-11), then a
// run-scope test runner.
const brownfieldTDDV216Content = v216Columns +
	`| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |
` + v216StageBlock + v216StageGroups

// brownfieldBuildVerifiedV216Content: the grouped Stage block holds a build
// review in each group (told apart by mode qualifiers) and the table ends with
// the block.
const brownfieldBuildVerifiedV216Content = v216Columns +
	`| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | build-review(tests) | FALSE | NEXT | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-tests.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-tests.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | build-review(impl) | FALSE | NEXT | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-impl.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-impl.md | Stage-{Stage}/implementation-review.md |
` + v216StageBlock + v216StageGroups

// greenfieldTDDV216Content: as brownfield, with an architecture phase and no
// codebase research.
const greenfieldTDDV216Content = v216Columns +
	`| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | system-designer | requirements-refinement | Requirements.md | requirements-review.md |
| ARCHITECTURE | system-designer | TRUE | system-design-review | - | Requirements.md | SystemDesign.md |
| ARCHITECTURE | system-design-review | FALSE | planner-tdd-soft | system-designer | Requirements.md, SystemDesign.md | system-design-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Requirements.md, SystemDesign.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Requirements.md, Plan.md, Stage-*/Plan.md, SystemDesign.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |
` + v216StageBlock + v216StageGroups

// implOnlyV216Content: the table starts with the ungrouped Stage block, so
// its schedule source Plan.md is a seed.
const implOnlyV216Content = v216Columns +
	`| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |
` + v216StageBlock
