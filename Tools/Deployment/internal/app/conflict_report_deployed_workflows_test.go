package app_test

// conflict_report_deployed_workflows_test.go verifies which workflows a conflict report line
// names as affected: the workflows the run deploys plus the workflows already embedded in the
// workspace's deployed orchestrator and script orchestrator files, filtered to the ones that
// route to the shadowed agent. The manifest never contributes workflows.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// crDeployedWorkflowOpts adds the three workflows the deployed-file tests embed: one that
// routes to the shadowed agent and one that does not. The test also embeds "ghost-flow", an id
// the catalog lacks, which must never be named.
func crDeployedWorkflowOpts() crStackOpts {
	return crStackOpts{workflows: []domain.Workflow{
		crWorkflow("legacy-flow", crAgentKey),
		crWorkflow("unrelated-flow", crOtherAgentKey),
	}}
}

// crWantAffected is the expected AffectedWorkflows when legacy-flow is deployed: the deployed
// one plus quick-fix for the operations that select it.
func crWantAffected(op crOperation) []string {
	if op.selectsQuickFix {
		return []string{"legacy-flow", "quick-fix"}
	}
	return []string{"legacy-flow"}
}

// TestConflictReport_DeployedOrchestratorWorkflows_NamedOnAgentLine verifies that a workflow
// embedded in an already-deployed orchestrator-role file, but not selected by the run, is named
// on the shadowed agent's line, for each write operation and for each of the two files.
func TestConflictReport_DeployedOrchestratorWorkflows_NamedOnAgentLine(t *testing.T) {
	for _, file := range []string{"orchestrator.md", "orchestrator-script.md"} {
		for _, op := range crOperations() {
			t.Run(op.name+"/"+file, func(t *testing.T) {
				deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crDeployedWorkflowOpts())
				embedded := []string{"legacy-flow", "unrelated-flow"}
				if op.name != "update" { // update refuses a deployed workflow the catalog lacks
					embedded = append(embedded, "ghost-flow")
				}
				crWriteDeployed(t, ws, file, embedded...)

				summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

				if err != nil {
					t.Fatalf("%s: %v", op.name, err)
				}
				got := crLineFor(t, summary.ConflictReport, crAgentItem()).AffectedWorkflows
				if !reflect.DeepEqual(got, crWantAffected(op)) {
					t.Errorf("AffectedWorkflows = %v, want %v (deployed in %s; unrelated and unknown ids excluded)",
						got, crWantAffected(op), file)
				}
			})
		}
	}
}

// TestConflictReport_WorkflowsSplitAcrossBothFiles_AreUnited verifies that the already-deployed
// set is the union of the orchestrator and the script orchestrator: one workflow in each file
// is named.
func TestConflictReport_WorkflowsSplitAcrossBothFiles_AreUnited(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			opts := crDeployedWorkflowOpts()
			opts.workflows = append(opts.workflows, crWorkflow("second-flow", crAgentKey))
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), opts)
			crWriteDeployed(t, ws, "orchestrator.md", "legacy-flow")
			crWriteDeployed(t, ws, "orchestrator-script.md", "second-flow")
			want := []string{"legacy-flow", "second-flow"}
			if op.selectsQuickFix {
				want = []string{"legacy-flow", "quick-fix", "second-flow"}
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

// TestConflictReport_NoDeployedOrchestrator_NamesOnlyRunSelection verifies that a workspace
// with no deployed orchestrator file contributes no workflows: only the run's own selection is
// named.
func TestConflictReport_NoDeployedOrchestrator_NamesOnlyRunSelection(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crDeployedWorkflowOpts())
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

// TestConflictReport_ManifestAlone_ContributesNoWorkflows verifies that a manifest that lists
// the orchestrator, with no deployed orchestrator file on disk, adds no workflow to the line.
// It guards the broad case only: a manifest-listed orchestrator must not be read as "every
// workflow is deployed". The manifest carries no workflow entries, so the source of the ids is
// pinned by the tests that write deployed orchestrator files.
func TestConflictReport_ManifestAlone_ContributesNoWorkflows(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crDeployedWorkflowOpts())
			deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{
				State: manifest.StatePresent,
				Manifest: domain.Manifest{
					SchemaVersion: "1", HarnessID: "stub-harness",
					Entries: []domain.ManifestEntry{
						{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "orchestrator"}, TargetPath: "orchestrator.md", Version: "1.0", DeployedAt: time.Unix(0, 0)},
						{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: crAgentKey}, TargetPath: "test-runner.md", Version: "1.0", DeployedAt: time.Unix(0, 0)},
					},
				},
			}}
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
				t.Errorf("AffectedWorkflows = %v, want %v (the manifest has no workflow entries)", got, want)
			}
		})
	}
}
