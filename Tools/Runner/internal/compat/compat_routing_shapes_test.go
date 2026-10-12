package compat_test

// Catalog-shaped 2.16 tables (inline copies of the shapes of quick-fix,
// implementation-only, brownfield-tdd and brownfield-pr-audit) must be admitted
// in every mode by the routing-shape, schedule-source and cycle checks, with the
// schedule source classified in the admitted model.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestAdmitRoutingShapes_QuickFixShape_AdmittedAndProduced(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "planner-tdd-soft", output: "Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md"},
		{phase: "PLANNING", agent: "plan-review", onFindings: "planner-tdd-soft"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd"},
		{phase: "REVIEW", agent: "test-runner", onFindings: "planner-tdd-soft"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
	})
}

func TestAdmitRoutingShapes_ImplementationOnlyShape_AdmittedAndSeeded(t *testing.T) {
	rows := []rrow{
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-review", onFindings: "implementation-tdd"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, nil)
	})
}

func TestAdmitRoutingShapes_BrownfieldTDDShape_AdmittedAndProduced(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "codebase-research"},
		{phase: "RESEARCH", agent: "requirements-refinement"},
		{phase: "RESEARCH", agent: "requirements-review", onFindings: "requirements-refinement"},
		{phase: "PLANNING", agent: "planner-tdd-soft", output: "Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md"},
		{phase: "PLANNING", agent: "plan-review", onFindings: "planner-tdd-soft"},
		{phase: "DESIGN", agent: "contracts-designer"},
		{phase: "DESIGN", agent: "contracts-review", onFindings: "contracts-designer"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer-tdd"},
		{phase: "EXECUTION.[Stage].Test", agent: "tests-review-tdd", onFindings: "test-writer-tdd"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementation-review", onFindings: "implementation-tdd"},
		{phase: "REVIEW", agent: "test-runner", onFindings: "planner-tdd-soft"},
	}
	table := parseRows(t, rows, stageBlockTable()+groupsSection(
		"| Stage | TDD | Test, Implementation |",
		"| Stage | Implementation-Only | Implementation |",
	))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{3}, nil)
	})
}

func TestAdmitRoutingShapes_BrownfieldPRAuditShape_AdmittedAndProducedUnderANonPlanName(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "pull-request-comment-interface(retrieve)", input: "Requirements.md"},
		{phase: "RESEARCH", agent: "pr-requirements-analyzer"},
		{phase: "RESEARCH", agent: "codebase-research"},
		{phase: "PLANNING", agent: "planner-audit", output: "AuditPlan.md, Stage-*/AuditPlan.md, Stage-*/AuditProgress.md"},
		{phase: "RESEARCH", agent: "codebase-research(architecture)"},
		{phase: "RESEARCH", agent: "codebase-research(contracts)", onSuccess: "[Stage]"},
		{phase: "EXECUTION.[Stage].Tests", agent: "tests-audit"},
		{phase: "EXECUTION.[Stage].Tests", agent: "audit-to-pull-request(tests)", onSuccess: "COMPLETE"},
		{phase: "EXECUTION.[Stage].Architecture", agent: "architecture-audit"},
		{phase: "EXECUTION.[Stage].Architecture", agent: "audit-review(architecture)", onFindings: "architecture-audit"},
		{phase: "EXECUTION.[Stage].Architecture", agent: "audit-to-pull-request(architecture)", onSuccess: "COMPLETE"},
		{phase: "REVIEW", agent: "audit-response-merger"},
		{phase: "COMPLETION", agent: "pull-request-comment-interface(post)"},
	}
	tables := blocksSection("| Stage | AuditPlan.md | Stages |") + groupsSection(
		"| Stage | Full | Tests, Architecture |",
		"| Stage | Tests-Only | Tests |",
	)
	table := parseRows(t, rows, tables)

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{3}, nil)
	})
}
