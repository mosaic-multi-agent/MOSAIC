package opencode_test

// The subject is launched with an explicit provider/model value so a run never
// depends on the harness default, and a subject without a model gets no plan.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencode"
)

func TestSpawnPlan_ArgsCarryTheSelectedProviderQualifiedModel(t *testing.T) {
	a := opencode.New(opencode.Options{})
	subject := spawnTestSubject()
	subject.Model = "anthropic/claude-sonnet-5-5"

	plan, err := a.SpawnPlan(testContext(), subject, mustProvisioning(t))
	if err != nil {
		t.Fatalf("SpawnPlan: %v", err)
	}

	found := false
	for i := 0; i+1 < len(plan.Args); i++ {
		if plan.Args[i] == "--model" && plan.Args[i+1] == "anthropic/claude-sonnet-5-5" {
			found = true
		}
	}
	if !found {
		t.Errorf("SpawnPlan: Args = %v, want %q followed by the selected model", plan.Args, "--model")
	}
}

func TestSpawnPlan_SubjectWithoutAModelYieldsNoPlan(t *testing.T) {
	a := opencode.New(opencode.Options{})
	subject := spawnTestSubject()
	subject.Model = ""

	plan, err := a.SpawnPlan(testContext(), subject, mustProvisioning(t))

	if !errors.Is(err, domain.ErrNoSubjectModel) {
		t.Fatalf("SpawnPlan error = %v, want one wrapping domain.ErrNoSubjectModel", err)
	}
	if !reflect.DeepEqual(plan, domain.SpawnPlan{}) {
		t.Errorf("SpawnPlan returned a plan %+v alongside the error, want the zero plan", plan)
	}
}
