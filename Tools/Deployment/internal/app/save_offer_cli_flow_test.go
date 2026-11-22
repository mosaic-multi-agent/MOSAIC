package app_test

// save_offer_cli_flow_test.go specifies that a deploy driven by the non-interactive CLI
// interaction is never offered to remember a custom tool answer: nothing is saved and the
// run's TODO items are the same as without an offer, so the TODO file and exit code that
// Tools/AgentTest sees are unchanged. This is regression coverage for the CLI path; the offer
// itself is specified by the interactive tests.

import (
	"bytes"
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

func TestSaveOffer_NonInteractiveCLI_AnsweredToolIsNotOfferedAndTheTodoIsUnchanged(t *testing.T) {
	r := newOfferRun(t, "unused", offerSetup{})
	collector := todo.NewCollector()
	r.deps.Todo = collector
	r.deps.Interaction = cli.NewInteraction(cli.PreAnswers{Values: map[domain.QuestionID]map[string]string{
		domain.QCustomTool: {"bash": listFlowAnswer},
	}}, collector, &bytes.Buffer{})

	_, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: offerHarnessID, WorkspacePath: r.workDir, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
	})

	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	if len(r.user.puts) != 0 {
		t.Errorf("a non-interactive run saved %+v", r.user.puts)
	}
	// The skipped model questions are the only expected TODO items; the answered tool leaves none,
	// so any other item (such as a skipped offer) is new.
	items := collector.Items()
	if len(items) != 3 {
		t.Errorf("TODO items = %+v, want exactly the three no-model items of the agents", items)
	}
	for _, it := range items {
		if it.Kind != domain.GapNoModel {
			t.Errorf("unexpected TODO item: %+v", it)
		}
	}
}
