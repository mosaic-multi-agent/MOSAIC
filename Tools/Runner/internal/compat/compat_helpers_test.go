package compat_test

import (
	"errors"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

// asRefusalError asserts that err is (or wraps) a *domain.RefusalError.
func asRefusalError(t *testing.T, err error) *domain.RefusalError {
	t.Helper()
	var re *domain.RefusalError
	if !errors.As(err, &re) {
		t.Fatalf("want *domain.RefusalError, got %T: %v", err, err)
	}
	return re
}

// ---- Workflow shape fixtures (WorkflowFormat 2.16 notation) ----
//
// Inline copies of the shapes of the catalog's build workflows; no test reads
// a workflow file from the catalog.

// brownfieldBuildVerifiedContent has 13 rows: 7 before the Stage block, Test
// rows 8-10 and Implementation rows 11-13. build-review appears twice, told
// apart by mode qualifiers.
const brownfieldBuildVerifiedContent = `## Brownfield TDD Build-Verified Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md, Research.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | build-review(tests) | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | build-review(tests) | FALSE | tests-review-tdd | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-tests.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-tests.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | build-review(impl) | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | build-review(impl) | FALSE | implementation-review | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-impl.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-impl.md | Stage-{Stage}/implementation-review.md |

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
`

// greenFieldTDDContent has 13 rows: 8 before the Stage block, Test rows 9-10,
// Implementation rows 11-12, 1 REVIEW row after the block.
const greenFieldTDDContent = `## Greenfield TDD Workflow

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
`

// brownfieldTDDContent has 12 rows: 7 before the Stage block, Test rows 8-9,
// Implementation rows 10-11, 1 REVIEW row after the block.
const brownfieldTDDContent = `## Brownfield TDD Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md, Research.md | requirements-review.md |
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
`

// quickFixContent has 4 rows: 2 PLANNING rows, 1 ungrouped Stage block row
// (row 3), 1 REVIEW row.
const quickFixContent = `## Quick Fix Workflow

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
`

// implOnlyContent has 3 rows: 2 ungrouped Stage block rows and 1 REVIEW row.
// No row comes before the block; the block's Source is a seed.
const implOnlyContent = `## Implementation Only Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
`

// threeGroupsContent is a synthetic workflow with three named groups (Alpha,
// Beta, Gamma) and a single-row approach table. No fixed-vocabulary agent
// names appear.
const threeGroupsContent = `## Three Groups Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | any-planner | TRUE | NEXT | - | - | Plan.md |
| EXECUTION.[Stage].Alpha | agent-a | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Stage-{Stage}/alpha.md |
| EXECUTION.[Stage].Beta | agent-b | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Stage-{Stage}/beta.md |
| EXECUTION.[Stage].Gamma | agent-c | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Stage-{Stage}/gamma.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | AllGroups | Alpha, Beta, Gamma |
`

// genericAgentGroupsContent is a synthetic workflow whose agent identifiers
// appear in no hardcoded list. The group partition comes entirely from the
// Phase column.
const genericAgentGroupsContent = `## Generic Agent Groups

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | any-planner | TRUE | NEXT | - | - | Plan.md |
| EXECUTION.[Stage].Analysis | completely-unknown-agent-x | FALSE | another-unknown-agent-y | - | Stage-{Stage}/Plan.md | Stage-{Stage}/x.md |
| EXECUTION.[Stage].Analysis | another-unknown-agent-y | FALSE | COMPLETE | - | Stage-{Stage}/x.md | Stage-{Stage}/y.md |
| EXECUTION.[Stage].Synthesis | yet-another-unknown-z | FALSE | COMPLETE | - | Stage-{Stage}/y.md | Stage-{Stage}/z.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | Forward | Analysis, Synthesis |
`

// ---- Helper functions ----

// mustParseShape parses a 2.16 fixture with the strict parser and fails the
// test when it is refused.
func mustParseShape(t *testing.T, content string, id, version string) domain.RoutingTable {
	t.Helper()
	info := domain.WorkflowInfo{ID: domain.WorkflowID(id), Version: domain.WorkflowVersion(version)}
	table, err := workflow.Parse([]byte(content), info)
	if err != nil {
		t.Fatalf("workflow.Parse(%s): %v", id, err)
	}
	return table
}

// mustAdmit admits table in Mode 2 and fails the test on a refusal.
func mustAdmit(t *testing.T, table domain.RoutingTable) domain.AdmittedWorkflow {
	t.Helper()
	aw, err := compat.Admit(table, domain.ExecutionModeAuto)
	if err != nil {
		t.Fatalf("Admit(%s): unexpected error: %v", table.Info.ID, err)
	}
	return aw
}

// stageBlock returns the one admitted block of a one-block workflow.
func stageBlock(t *testing.T, aw domain.AdmittedWorkflow) domain.AdmittedBlock {
	t.Helper()
	b, ok := aw.BlockByName("Stage")
	if !ok {
		t.Fatalf(`BlockByName("Stage"): want found=true, blocks: %+v`, aw.Blocks)
	}
	return b
}
