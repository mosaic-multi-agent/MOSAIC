package claudecode_test

// The subject is launched with an explicit model so a run never depends on the
// harness default: the plan carries `--model <selected>` next to `--agent`,
// and a subject without a model gets no plan at all.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
)

// argValue returns the element following flag in args, or false.
func argValue(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func TestSpawnPlan_ArgsCarryTheSelectedModelAlongsideTheAgent(t *testing.T) {
	a, _, prov := provisionedAdapter(t)
	subject := spawnTestSubject()
	subject.Model = "claude-sonnet-5-5"

	plan, err := a.SpawnPlan(testContext(), subject, prov)
	if err != nil {
		t.Fatalf("SpawnPlan: %v", err)
	}

	if got, ok := argValue(plan.Args, "--model"); !ok || got != "claude-sonnet-5-5" {
		t.Errorf("SpawnPlan: Args = %v, want %q followed by the selected model %q", plan.Args, "--model", "claude-sonnet-5-5")
	}
	if got, ok := argValue(plan.Args, "--agent"); !ok || got != subject.Identity {
		t.Errorf("SpawnPlan: Args = %v, want %q followed by the subject identity %q", plan.Args, "--agent", subject.Identity)
	}
}

func TestSpawnPlan_ModelArgumentFollowsTheSubjectModelExactly(t *testing.T) {
	a, _, prov := provisionedAdapter(t)

	for _, model := range []string{"haiku", "claude-opus-5-1"} {
		subject := spawnTestSubject()
		subject.Model = model

		plan, err := a.SpawnPlan(testContext(), subject, prov)
		if err != nil {
			t.Fatalf("SpawnPlan(%q): %v", model, err)
		}

		count := 0
		for _, arg := range plan.Args {
			if arg == "--model" {
				count++
			}
		}
		if got, _ := argValue(plan.Args, "--model"); got != model || count != 1 {
			t.Errorf("SpawnPlan: Args = %v, want exactly one --model argument with value %q", plan.Args, model)
		}
	}
}

func TestSpawnPlan_SubjectWithoutAModelYieldsNoPlan(t *testing.T) {
	a, _, prov := provisionedAdapter(t)
	subject := spawnTestSubject()
	subject.Model = ""

	plan, err := a.SpawnPlan(testContext(), subject, prov)

	if !errors.Is(err, domain.ErrNoSubjectModel) {
		t.Fatalf("SpawnPlan error = %v, want one wrapping domain.ErrNoSubjectModel", err)
	}
	if !reflect.DeepEqual(plan, domain.SpawnPlan{}) {
		t.Errorf("SpawnPlan returned a plan %+v alongside the error, want the zero plan", plan)
	}
}
