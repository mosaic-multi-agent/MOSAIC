package resumecheck_test

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/resumecheck"
)

func TestCheckDeployment_Satisfiable_Passes(t *testing.T) {
	err := resumecheck.CheckDeployment(fixtureRunID, recordedState(), satisfiableDeployment())
	if err != nil {
		t.Fatalf("CheckDeployment = %v, want nil", err)
	}
}

// A workflow version that differs from the deployed one is the version-drift
// question's concern, and an unrecorded mode is an adoption: neither refuses.
func TestCheckDeployment_VersionMismatchAndAdoption_AreNotRefusals(t *testing.T) {
	state := recordedState()
	state.WorkflowVersion = "0.1"
	state.Mode = domain.ExecutionModeUnset

	if err := resumecheck.CheckDeployment(fixtureRunID, state, satisfiableDeployment()); err != nil {
		t.Fatalf("CheckDeployment = %v, want nil", err)
	}
}

func TestCheckDeployment_DisabledSettings_NeedNoAgents(t *testing.T) {
	state := recordedState()
	state.Checkpoints = false
	state.Commits = false
	state.CommitBranch = ""
	state.InfraClassSelections = nil
	state.InfrastructureOverrides = nil
	d := satisfiableDeployment()
	d.InfraAgents = nil

	if err := resumecheck.CheckDeployment(fixtureRunID, state, d); err != nil {
		t.Fatalf("CheckDeployment = %v, want nil", err)
	}
}

func TestCheckDeployment_RecordedWorkflowNoLongerDeclared_Refuses(t *testing.T) {
	d := satisfiableDeployment()
	d.Workflows = d.Workflows[:1]

	err := resumecheck.CheckDeployment(fixtureRunID, recordedState(), d)

	r := asRefusal(t, err)
	want := `workflow "build-flow" for run ` + fixtureRunID +
		` is not declared in the current orchestrator file; it may have been renamed or removed`
	if r.Message != want {
		t.Errorf("refusal text = %q, want %q", r.Message, want)
	}
}

func TestCheckDeployment_InvalidRecordedSelection_RefusesWithDomainText(t *testing.T) {
	state := recordedState()
	state.InfraClassSelections = map[string]string{"checkpoint": "cp-b", "commit": "no-such-agent"}
	domainErr := domain.ValidateInfraSelections(declaredAgents(), state.InfraClassSelections)
	if domainErr == nil {
		t.Fatal("fixture error: domain accepts the invalid selection")
	}

	err := resumecheck.CheckDeployment(fixtureRunID, state, satisfiableDeployment())

	r := asRefusal(t, err)
	if r.Message != domainErr.Error() {
		t.Errorf("refusal text = %q, want %q", r.Message, domainErr.Error())
	}
}

func TestCheckDeployment_MissingSelectionForMultiAgentClass_Refuses(t *testing.T) {
	state := recordedState()
	state.InfraClassSelections = nil
	domainErr := domain.ValidateInfraSelections(declaredAgents(), nil)
	if domainErr == nil {
		t.Fatal("fixture error: two checkpoint agents without a selection must be invalid")
	}

	err := resumecheck.CheckDeployment(fixtureRunID, state, satisfiableDeployment())

	r := asRefusal(t, err)
	if r.Message != domainErr.Error() {
		t.Errorf("refusal text = %q, want %q", r.Message, domainErr.Error())
	}
}

func TestCheckDeployment_RefusedOverride_RefusesWithSessionText(t *testing.T) {
	tests := []struct {
		name      string
		overrides []domain.InfrastructureOverride
		want      string
	}{
		{
			name:      "agent not declared",
			overrides: []domain.InfrastructureOverride{{AgentName: "ghost", Triggers: []domain.DeclaredInfraTrigger{{Trigger: domain.InfraTriggerManual}}}},
			want:      `infrastructure_overrides: agent "ghost" is not declared in the orchestrator file`,
		},
		{
			name:      "trigger not allowed for commit class",
			overrides: []domain.InfrastructureOverride{{AgentName: "git-commit", Triggers: []domain.DeclaredInfraTrigger{{Trigger: domain.InfraTriggerPhaseEnd}}}},
			want:      `infrastructure_overrides: trigger "PHASE_END" is not allowed for commit-class agent "git-commit"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := recordedState()
			state.InfrastructureOverrides = tc.overrides

			err := resumecheck.CheckDeployment(fixtureRunID, state, satisfiableDeployment())

			r := asRefusal(t, err)
			if r.Message != tc.want {
				t.Errorf("refusal text = %q, want %q", r.Message, tc.want)
			}
		})
	}
}

func TestCheckDeployment_CheckpointsWithoutCheckpointAgent_Refuses(t *testing.T) {
	state := recordedState()
	state.InfraClassSelections = map[string]string{"commit": "git-commit"}
	d := satisfiableDeployment()
	d.InfraAgents = []domain.DeclaredInfraAgent{{Name: "git-commit", Class: "commit", OnFailure: "halt"}}

	err := resumecheck.CheckDeployment(fixtureRunID, state, d)

	r := asRefusal(t, err)
	if want := "checkpoints requested but no checkpoint provider is available"; r.Message != want {
		t.Errorf("refusal text = %q, want %q", r.Message, want)
	}
}

func TestCheckDeployment_CommitsWithoutCommitAgent_Refuses(t *testing.T) {
	state := recordedState()
	state.InfraClassSelections = map[string]string{"checkpoint": "cp-b"}
	state.InfrastructureOverrides = nil
	d := satisfiableDeployment()
	d.InfraAgents = d.InfraAgents[:2]

	err := resumecheck.CheckDeployment(fixtureRunID, state, d)

	r := asRefusal(t, err)
	if want := "commits enabled but no commit-class infrastructure agent is declared"; r.Message != want {
		t.Errorf("refusal text = %q, want %q", r.Message, want)
	}
}

// When several conditions hold, the first in the pinned order is reported.
func TestCheckDeployment_SeveralProblems_ReportsFirstInOrder(t *testing.T) {
	bad := recordedState()
	bad.InfraClassSelections = map[string]string{"checkpoint": "cp-b", "commit": "no-such-agent"}
	bad.InfrastructureOverrides = []domain.InfrastructureOverride{{AgentName: "ghost"}}
	noAgents := satisfiableDeployment()
	noAgents.InfraAgents = nil
	noWorkflows := noAgents
	noWorkflows.Workflows = nil

	noSelections := bad
	noSelections.InfraClassSelections = nil
	noOverrides := noSelections
	noOverrides.InfrastructureOverrides = nil

	tests := []struct {
		name    string
		state   domain.ArtifactState
		d       resumecheck.Deployment
		wantSub string
	}{
		{"workflow before selections", bad, noWorkflows, "is not declared in the current orchestrator file"},
		{"selections before overrides", bad, satisfiableDeployment(), "no-such-agent"},
		{"overrides before checkpoints", noSelections, noAgents, "infrastructure_overrides:"},
		{"checkpoints before commits", noOverrides, noAgents, "checkpoints requested"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := resumecheck.CheckDeployment(fixtureRunID, tc.state, tc.d)

			r := asRefusal(t, err)
			if !strings.Contains(r.Message, tc.wantSub) {
				t.Errorf("refusal text = %q, want it to contain %q", r.Message, tc.wantSub)
			}
		})
	}
}
