package app_test

// transform_unresolved_model_test.go specifies unresolved models in the whole-deployment
// transform: an agent whose model group is left unset is written without any model key (never
// an empty one) and is named in the result and in the TODO file, as deploy does.

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trPlanOrderKeys is the agent keys of the seeded agents in plan (file-name) order.
var trPlanOrderKeys = []string{
	"harness-only", "orchestrator-script", "orchestrator", "standalone", "subagent", "utility",
}

func TestTransformUnresolved_NoModelAnswered_WritesNoModelKeyAndNamesEveryAgent(t *testing.T) {
	r := trModelRun(t, nil)

	res := r.mustRun(r.request())

	for _, key := range trPlanOrderKeys {
		trAssertNoModel(t, r, key)
	}
	if !slices.Equal(res.UnresolvedModels, trPlanOrderKeys) {
		t.Errorf("UnresolvedModels = %q, want %q", res.UnresolvedModels, trPlanOrderKeys)
	}
}

func TestTransformUnresolved_OnlyTheSkippedGroupIsNamed(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOneCustom(domain.QTransformTargetModel, mSonnet, "my-org/s").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mUnset, "my-org/u").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mHaiku, "my-org/h").Build()
	r := trModelRun(t, script) // the opus group stays unanswered

	res := r.mustRun(r.request())

	want := []string{"orchestrator-script", "orchestrator"}
	if !slices.Equal(res.UnresolvedModels, want) {
		t.Errorf("UnresolvedModels = %q, want %q", res.UnresolvedModels, want)
	}
}

func TestTransformUnresolved_AllResolved_NamesNoAgent(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.TargetModel = "pre/answered"

	res := r.mustRun(req)

	if len(res.UnresolvedModels) != 0 {
		t.Errorf("UnresolvedModels = %q, want none", res.UnresolvedModels)
	}
	if got := trFile(t, res, "subagent").Model; got != "pre/answered" {
		t.Errorf("subagent outcome Model = %q, want %q", got, "pre/answered")
	}
}

func TestTransformUnresolved_AgentsAreNamedInTheTodoFile(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOneCustom(domain.QTransformTargetModel, mSonnet, "my-org/s").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mUnset, "my-org/u").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mHaiku, "my-org/h").Build()
	r := trModelRun(t, script)

	res := r.mustRun(r.request())

	if res.TodoPath == "" {
		t.Fatal("TodoPath is empty although agents were written without a model")
	}
	data, err := os.ReadFile(res.TodoPath)
	if err != nil {
		t.Fatalf("read TODO file: %v", err)
	}
	todo := string(data)
	if !strings.Contains(todo, "orchestrator-script") {
		t.Errorf("TODO file does not name the unresolved agent %q:\n%s", "orchestrator-script", todo)
	}
	// "orchestrator" is a prefix of "orchestrator-script", so match it not followed by "-script".
	if !regexp.MustCompile(`orchestrator([^-]|-[^s]|$)`).MatchString(todo) {
		t.Errorf("TODO file does not name the unresolved agent %q separately:\n%s", "orchestrator", todo)
	}
	// Agents whose model was resolved must not be named as unresolved.
	for _, key := range []string{"utility", "standalone"} {
		if strings.Contains(todo, key) {
			t.Errorf("TODO file names %q although its model was resolved:\n%s", key, todo)
		}
	}
}

func TestTransformUnresolved_DryRun_ReportsUnresolvedAgentsWithoutATodoFile(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if !slices.Equal(res.UnresolvedModels, trPlanOrderKeys) {
		t.Errorf("UnresolvedModels = %q, want %q", res.UnresolvedModels, trPlanOrderKeys)
	}
	if res.TodoPath != "" {
		t.Errorf("dry run reported a TODO file %q", res.TodoPath)
	}
}

func TestTransformUnresolved_OutcomeOfAnUnresolvedAgentHasNoModel(t *testing.T) {
	r := trModelRun(t, nil)

	res := r.mustRun(r.request())

	if got := trFile(t, res, "subagent").Model; got != "" {
		t.Errorf("subagent outcome Model = %q, want empty", got)
	}
	if res.Transformed != len(trAgentFixtures) {
		t.Errorf("Transformed = %d, want %d: unresolved models do not fail a file", res.Transformed, len(trAgentFixtures))
	}
}
