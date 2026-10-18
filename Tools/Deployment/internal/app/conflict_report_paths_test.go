package app_test

// conflict_report_paths_test.go verifies that every write-planning operation (deploy, update,
// workflows, agents, hooks) reports the catalog-stack conflict report on its run summary, that
// the report exists before anything is written (a dry run and a declined plan still carry it
// and leave the workspace unchanged), and that runs without shadowing carry an empty report.

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// TestConflictReport_EveryWritePath_ReportsUndeclaredOverride verifies that a two-layer stack
// in which the user layer shadows the default layer's agent without declaring it yields one
// warning line on the summary of each write operation, naming the winner and the shadowed layer.
func TestConflictReport_EveryWritePath_ReportsUndeclaredOverride(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if len(summary.ConflictReport.Lines) != 1 {
				t.Fatalf("report lines = %+v, want exactly the shadowed agent", summary.ConflictReport.Lines)
			}
			line := crLineFor(t, summary.ConflictReport, crAgentItem())
			if line.Severity != domain.ConflictWarning {
				t.Errorf("severity = %q, want %q for an undeclared override", line.Severity, domain.ConflictWarning)
			}
			if line.Winner.ID != crUserID {
				t.Errorf("winner = %q, want %q", line.Winner.ID, crUserID)
			}
			if len(line.Shadowed) != 1 || line.Shadowed[0].ID != crDefaultID {
				t.Errorf("shadowed = %+v, want the default layer only", line.Shadowed)
			}
		})
	}
}

// TestConflictReport_DeclaredOverride_IsInfo verifies that an override the winning layer
// declares in its identity is reported with info severity on every write operation.
func TestConflictReport_DeclaredOverride_IsInfo(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{declareAgent: true})

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			line := crLineFor(t, summary.ConflictReport, crAgentItem())
			if line.Severity != domain.ConflictInfo {
				t.Errorf("severity = %q, want %q for a declared override", line.Severity, domain.ConflictInfo)
			}
		})
	}
}

// TestConflictReport_SelectedWorkflowsAreAffected verifies that the workflows a run deploys
// that route to the shadowed agent are named on its line, and that operations that deploy no
// workflow name none.
func TestConflictReport_SelectedWorkflowsAreAffected(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{
				workflows: []domain.Workflow{crWorkflow("unrelated-flow", crOtherAgentKey)},
			})
			want := []string(nil)
			if op.selectsQuickFix {
				want = []string{"quick-fix"}
			}

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			got := crLineFor(t, summary.ConflictReport, crAgentItem()).AffectedWorkflows
			if !reflect.DeepEqual(got, want) {
				t.Errorf("AffectedWorkflows = %v, want %v", got, want)
			}
		})
	}
}

// TestConflictReport_ShadowedOrchestratorPair_NamesEveryRunWorkflow verifies that a shadowed
// orchestrator pair is one line that names every workflow the run deploys.
func TestConflictReport_ShadowedOrchestratorPair_NamesEveryRunWorkflow(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{
		orchestratorPair: true,
		workflows:        []domain.Workflow{crWorkflow("unrelated-flow", crOtherAgentKey)},
	})
	op := crOperations()[0] // deploy, selecting quick-fix

	summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	pair := crLineFor(t, summary.ConflictReport, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair})
	if !reflect.DeepEqual(pair.AffectedWorkflows, []string{"quick-fix"}) {
		t.Errorf("pair AffectedWorkflows = %v, want [quick-fix]", pair.AffectedWorkflows)
	}
}

// TestConflictReport_DryRun_ReportsAndWritesNothing verifies that a dry run still surfaces the
// report, hands the executor a dry-run request, and leaves the workspace unchanged.
func TestConflictReport_DryRun_ReportsAndWritesNothing(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
			spy := &crSpyExecutor{}
			deps.Executor = spy
			before := crSnapshotDir(t, ws)

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true, dryRun: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if !spy.dryRun {
				t.Error("executor was not asked for a dry run")
			}
			crLineFor(t, summary.ConflictReport, crAgentItem())
			if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
				t.Error("a dry run changed the workspace")
			}
		})
	}
}

// TestConflictReport_DeclinedPlan_StillReportsAndWritesNothing verifies that when the user
// declines the plan, the operation returns ErrPlanNotConfirmed together with a summary that
// carries the report, never runs the executor, and leaves the workspace unchanged.
func TestConflictReport_DeclinedPlan_StillReportsAndWritesNothing(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			stub := interactiontest.NewBuilder().AnswerConfirm(domain.QPlanConfirm, "", false).Build()
			deps, ws := crDeps(t, stub, crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
			spy := &crSpyExecutor{}
			deps.Executor = spy
			before := crSnapshotDir(t, ws)

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if !errors.Is(err, app.ErrPlanNotConfirmed) {
				t.Fatalf("err = %v, want ErrPlanNotConfirmed", err)
			}
			if spy.called {
				t.Error("executor ran although the plan was declined")
			}
			crLineFor(t, summary.ConflictReport, crAgentItem())
			if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
				t.Error("a declined plan changed the workspace")
			}
		})
	}
}

// TestConflictReport_ConfirmedRun_IsBuiltBeforeTheExecutorWrites verifies that on a confirmed
// run the report reflects the workspace as it was before the executor wrote: the executor
// replaces the deployed orchestrator (which embedded extra-flow) with one that embeds no
// workflow, and the report still names extra-flow, so it was built before execution.
func TestConflictReport_ConfirmedRun_IsBuiltBeforeTheExecutorWrites(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{
				workflows: []domain.Workflow{crWorkflow("extra-flow", crAgentKey)},
			})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix", "extra-flow")
			spy := &crSpyExecutor{onExecute: func() { crWriteDeployed(t, ws, "orchestrator.md") }}
			deps.Executor = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if !spy.called {
				t.Fatal("executor did not run on a confirmed run")
			}
			got := crLineFor(t, summary.ConflictReport, crAgentItem()).AffectedWorkflows
			if !reflect.DeepEqual(got, []string{"extra-flow", "quick-fix"}) {
				t.Errorf("AffectedWorkflows = %v, want [extra-flow quick-fix] from the pre-write workspace", got)
			}
		})
	}
}

// TestConflictReport_DefaultOnlyStack_ReportIsEmpty verifies that a stack of the default
// catalog alone has no shadow records, so every write operation reports nothing.
func TestConflictReport_DefaultOnlyStack_ReportIsEmpty(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
			mosaic := newSysMosaicRoot(t, sysLayer{id: crDefaultID, protocolVersion: "1.0", bundleVersion: "1.0.0"})
			crWriteAgent(t, filepath.Join(mosaic, catalogpaths.CatalogDirName), crAgentKey, "1.0")
			useResolvedStack(t, &deps, mosaic, defaultStackLayer(mosaic))

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if len(summary.ConflictReport.Lines) != 0 {
				t.Errorf("report lines = %+v, want none for a single layer", summary.ConflictReport.Lines)
			}
		})
	}
}

// TestConflictReport_PlainCatalog_ReportIsEmpty verifies that a catalog that is not a stack
// (no shadow records at all) yields an empty report and the run still completes.
func TestConflictReport_PlainCatalog_ReportIsEmpty(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if len(summary.ConflictReport.Lines) != 0 {
				t.Errorf("report lines = %+v, want none for a plain catalog", summary.ConflictReport.Lines)
			}
		})
	}
}
