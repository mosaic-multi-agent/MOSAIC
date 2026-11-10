package resumecheck_test

// A damaged run artifact is not an early refusal while its frontmatter still
// names the workflow: the run is repaired once it starts. Without a readable
// frontmatter there is nothing to resume the run as, and the refusal names the
// damage.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/resumecheck"
)

func damageWithFrontmatter(state domain.ArtifactState) *domain.ArtifactDamageError {
	return &domain.ArtifactDamageError{
		Resource:    "Orchestration.md",
		Problems:    []domain.ArtifactDamageProblem{{Section: "ExecutionLog", Line: 12, Problem: "row has 3 cells, the header has 8"}},
		Frontmatter: &state,
	}
}

func damageWithoutFrontmatter() *domain.ArtifactDamageError {
	return &domain.ArtifactDamageError{
		Resource: "Orchestration.md",
		Problems: []domain.ArtifactDamageProblem{{Section: "frontmatter", Line: 4, Problem: "value of 'workflow' opens [ without closing ]"}},
	}
}

func TestCheckArtifact_DamagedWithRecordedWorkflow_PassesTheEarlyCheck(t *testing.T) {
	readErr := damageWithFrontmatter(recordedState())

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	if err != nil {
		t.Fatalf("CheckArtifact = %v, want nil: a damaged run with a recorded workflow is repaired at start", err)
	}
}

func TestCheckArtifact_DamagedWithRecordedWorkflow_WrappedDamageStillPasses(t *testing.T) {
	readErr := fmt.Errorf("reading artifact: %w", damageWithFrontmatter(recordedState()))

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	if err != nil {
		t.Fatalf("CheckArtifact = %v, want nil for a wrapped damage error", err)
	}
}

func TestCheckArtifact_DamagedFrontmatterWithoutRecordedState_RefusesNamingTheDamage(t *testing.T) {
	readErr := damageWithoutFrontmatter()

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	r := asRefusal(t, err)
	if !strings.Contains(r.Message, "damaged") {
		t.Errorf("refusal text = %q, want it to name the damage", r.Message)
	}
	if !strings.Contains(r.Message, "last-good") {
		t.Errorf("refusal text = %q, want it to say no usable last-good copy records the workflow", r.Message)
	}
	var damage *domain.ArtifactDamageError
	if !errors.As(r, &damage) {
		t.Errorf("refusal does not carry the *domain.ArtifactDamageError as its cause: %v", r.Cause)
	}
}

func TestCheckArtifact_DamagedFrontmatterWithRecordedStateFromTheLastGoodCopy_PassesTheEarlyCheck(t *testing.T) {
	readErr := damageWithoutFrontmatter()

	err := resumecheck.CheckArtifact(fixtureRunID, recordedState(), readErr)

	if err != nil {
		t.Fatalf("CheckArtifact = %v, want nil: the last-good copy records the workflow and the run is repaired at start", err)
	}
}

func TestCheckArtifact_DamagedFrontmatterWithWrappedErrorAndRecordedState_PassesTheEarlyCheck(t *testing.T) {
	readErr := fmt.Errorf("reading artifact: %w", damageWithoutFrontmatter())

	err := resumecheck.CheckArtifact(fixtureRunID, recordedState(), readErr)

	if err != nil {
		t.Fatalf("CheckArtifact = %v, want nil for a wrapped damage error with a recorded state", err)
	}
}

func TestCheckArtifact_DamagedFrontmatterWithRecordedStateRecordingNoWorkflow_RefusesAsNoWorkflow(t *testing.T) {
	state := recordedState()
	state.Workflow = ""

	err := resumecheck.CheckArtifact(fixtureRunID, state, damageWithoutFrontmatter())

	r := asRefusal(t, err)
	if !strings.Contains(r.Message, "records no workflow") {
		t.Errorf("refusal text = %q, want the no-workflow refusal: the recorded state names no workflow", r.Message)
	}
}

func TestCheckArtifact_DamagedWithFrontmatterRecordingNoWorkflow_RefusesAsNoWorkflow(t *testing.T) {
	state := recordedState()
	state.Workflow = ""
	readErr := damageWithFrontmatter(state)

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	r := asRefusal(t, err)
	if !strings.Contains(r.Message, "records no workflow") {
		t.Errorf("refusal text = %q, want the no-workflow refusal: the frontmatter records no workflow", r.Message)
	}
}
