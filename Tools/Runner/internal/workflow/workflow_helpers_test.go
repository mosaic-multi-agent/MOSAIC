package workflow_test

// Fixtures and helpers for the row-parsing tests of workflow.Parse on the
// catalog workflow shapes (workflow_quickfix_test.go,
// workflow_brownfield_test.go, workflow_columns_test.go,
// workflow_phasegroup_test.go, workflow_approach_test.go). Every fixture is
// WorkflowFormat 2.16 notation and is parsed by the strict parser.
//
// The 1.x fixtures and the transitional entry point's helper are in
// workflow_legacy_helpers_test.go.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

// asRefusalError asserts that err is (or wraps) a *domain.RefusalError.
// Calls t.Fatal on failure.
func asRefusalError(t *testing.T, err error) *domain.RefusalError {
	t.Helper()
	var re *domain.RefusalError
	if !errors.As(err, &re) {
		t.Fatalf("want *domain.RefusalError, got %T: %v", err, err)
	}
	return re
}

// ---- Fixture content: the shapes of the catalog's build workflows ----

// quickFixContent has two PLANNING rows, one ungrouped Stage block row and one
// REVIEW row. The planner row has no Input.
const quickFixContent = `## Quick Fix Workflow

**Use when:** Small changes, bug fixes, or well-understood modifications. Skips research and design.

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

// brownfieldContent has 13 rows: 7 before the Stage block (rows 1-7), then six
// grouped rows (Test, Implementation). build-review appears twice, told apart
// by mode qualifiers.
const brownfieldContent = `## Brownfield TDD Build-Verified Workflow

**Use when:** New features or significant changes to an **existing codebase** requiring test-first development where **compilation/build cannot be verified via standard terminal tools**.

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

// implOnlyContent starts with block rows (no row before the block): two Stage
// block rows and one REVIEW row.
const implOnlyContent = `## Implementation Only Workflow

**Use when:** Research, planning, and design already complete.

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

// onSuccessDashContent is a routing table where the On Success cell is "-".
// The strict parser refuses it: "-" is 1.x notation.
const onSuccessDashContent = `| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | TRUE | - | - | - | Plan.md |
`

// outputDashContent is a routing table where the Output cell is "-", used to
// verify that "-" in the Output column produces an empty OutputArtifacts slice,
// symmetric with the Input column behavior.
const outputDashContent = `| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | TRUE | COMPLETE | - | Plan.md | - |
`

// groupedPhasesContent has a plain row, two grouped rows of block Stage (Test
// and Implementation groups) and one ungrouped row of block Audit.
const groupedPhasesContent = `| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | agent-a | FALSE | NEXT | - | - | Plan.md |
| EXECUTION.[Stage].Test | agent-c | FALSE | NEXT | - | - | out-c.md |
| EXECUTION.[Stage].Implementation | agent-d | FALSE | NEXT | - | - | out-d.md |
| REVIEW.[Audit] | agent-b | FALSE | COMPLETE | - | - | out-b.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
| Audit | Audit.md | Items |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
`

// ---- Helpers ----

// mustParseFixture parses content with the strict parser and fails the test
// when it is refused.
func mustParseFixture(t *testing.T, content, id, version string) domain.RoutingTable {
	t.Helper()
	info := domain.WorkflowInfo{ID: domain.WorkflowID(id), Version: domain.WorkflowVersion(version)}
	table, err := workflow.Parse([]byte(content), info)
	if err != nil {
		t.Fatalf("Parse(%s): unexpected error: %v", id, err)
	}
	return table
}

func mustParseQuickFix(t *testing.T) domain.RoutingTable {
	t.Helper()
	return mustParseFixture(t, quickFixContent, "quick-fix", "4.0")
}

func mustParseBrownfield(t *testing.T) domain.RoutingTable {
	t.Helper()
	return mustParseFixture(t, brownfieldContent, "brownfield-tdd-build-verified", "4.0")
}

func mustParseImplOnly(t *testing.T) domain.RoutingTable {
	t.Helper()
	return mustParseFixture(t, implOnlyContent, "implementation-only", "4.0")
}

func mustParseGroupedPhases(t *testing.T) domain.RoutingTable {
	t.Helper()
	return mustParseFixture(t, groupedPhasesContent, "grouped-phases", "1.0")
}

// containsArtifact reports whether list holds path verbatim.
func containsArtifact(list []string, path string) bool {
	for _, a := range list {
		if a == path {
			return true
		}
	}
	return false
}
