package session_test

// Tests of infrastructure_overrides validation on resume: a trigger outside
// the closed vocabulary is refused before any dispatch (the commit setup
// dispatch included) and before any artifact write, and a commit-class agent
// may only be overridden to ITERATION_END.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// overrideOf builds an override of agent with the given trigger names.
func overrideOf(agent string, triggers ...string) domain.InfrastructureOverride {
	ov := domain.InfrastructureOverride{AgentName: agent}
	for _, tr := range triggers {
		ov.Triggers = append(ov.Triggers, domain.DeclaredInfraTrigger{Trigger: tr})
	}
	return ov
}

// resumeCommitSetupPendingWith resumes a run with commits enabled and no
// commit_branch recorded, whose artifact records the given override. A run
// that is wrongly allowed to proceed would dispatch the commit setup first.
func resumeCommitSetupPendingWith(t *testing.T, ov domain.InfrastructureOverride) (domain.RunOutcome, error, *harness.MockAdapter, *memStore) {
	t.Helper()
	ses, f, store, orchPath := newCommitSession(t)
	store.state = commitResumeState()
	store.state.InfrastructureOverrides = []domain.InfrastructureOverride{ov}
	store.exists = true
	queueSuccessfulSetupAndWorkflow(f, "mosaic/run/override-test")
	cfg := baseCommitConfig(orchPath)
	markResume(&cfg)
	cfg.Supplied.CommitBranchVariant = true

	got, err := ses.Start(context.Background(), cfg)
	return got, err, f, store
}

// resumeCommitBranchRecordedWith resumes a run whose commit setup is done and
// whose artifact records the given override.
func resumeCommitBranchRecordedWith(t *testing.T, ov domain.InfrastructureOverride) (domain.RunOutcome, error, *harness.MockAdapter) {
	t.Helper()
	ses, f, store, orchPath := newCommitSession(t)
	store.state = commitResumeState()
	store.state.CommitBranch = "mosaic/run/test-run-id"
	store.state.InfrastructureOverrides = []domain.InfrastructureOverride{ov}
	store.exists = true
	f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-a#1", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
	}})
	f.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#2", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
	}})
	cfg := baseCommitConfig(orchPath)
	markResume(&cfg)

	got, err := ses.Start(context.Background(), cfg)
	return got, err, f
}

// resumeNonCommitClassWith resumes a run whose artifact records the given
// override on the checkpoint-class agent. No commit-class restriction applies,
// so only the trigger vocabulary check can refuse a bad trigger. Both workflow
// agents have scripted SUCCESS responses so that a wrongly allowed run
// completes.
func resumeNonCommitClassWith(t *testing.T, ov domain.InfrastructureOverride) (domain.RunOutcome, error, *harness.MockAdapter, *memStore) {
	t.Helper()
	ses, f, store, orchPath := newIntervalAgentSession(t)
	store.state = domain.ArtifactState{
		Workflow:                "linear",
		WorkflowVersion:         "1.0",
		Task:                    "test task",
		RunSettings:             domain.RunSettings{Mode: domain.ExecutionModeAuto},
		InfrastructureOverrides: []domain.InfrastructureOverride{ov},
	}
	store.exists = true
	queueWorkflowAgents(f)
	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)

	got, err := ses.Start(context.Background(), cfg)
	return got, err, f, store
}

// queueWorkflowAgents scripts a SUCCESS response for both workflow agents.
func queueWorkflowAgents(f *harness.MockAdapter) {
	for _, a := range []string{"agent-a", "agent-b"} {
		f.Queue(a, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: a + "#queued", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
}

// ===== Unknown trigger in an override =====

func TestSession_Start_Resume_OverrideWithUnknownTrigger_RefusedBeforeCommitSetup(t *testing.T) {
	got, err, f, store := resumeCommitSetupPendingWith(t, overrideOf("commit-manager-git", "WHENEVER"))

	msg := requireRefused(t, got, err)

	for _, token := range []string{"commit-manager-git", "WHENEVER"} {
		if !strings.Contains(msg, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	requireNoDispatchOrWrite(t, f, store)
	if len(store.BranchCalls) != 0 {
		t.Errorf("want no commit_branch write, got %v", store.BranchCalls)
	}
}

func TestSession_Start_Resume_OverrideWithInvalidTriggerSpelling_RefusedBeforeDispatch(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
	}{
		{"lowercase vocabulary name", "iteration_end"},
		{"quote wrapped vocabulary name", "\"ITERATION_END\""},
		{"blank trigger", ""},
		{"unknown trigger", "WHENEVER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err, f, store := resumeNonCommitClassWith(t, overrideOf("checkpoint-manager-git", tc.trigger))

			msg := requireRefused(t, got, err)

			if !strings.Contains(msg, "checkpoint-manager-git") {
				t.Errorf("refusal %q must name the agent", msg)
			}
			requireNoDispatchOrWrite(t, f, store)
		})
	}
}

func TestSession_Start_Resume_OverrideWithUnknownTriggerOnNonCommitClass_RefusalNamesTrigger(t *testing.T) {
	got, err, f, store := resumeNonCommitClassWith(t, overrideOf("checkpoint-manager-git", "WHENEVER"))

	msg := requireRefused(t, got, err)

	if !strings.Contains(msg, "WHENEVER") {
		t.Errorf("refusal %q must name the trigger", msg)
	}
	requireNoDispatchOrWrite(t, f, store)
}

func TestSession_Start_Resume_OneBadTriggerAmongValidOnes_RefusedOnNonCommitClass(t *testing.T) {
	got, err, f, store := resumeNonCommitClassWith(t,
		overrideOf("checkpoint-manager-git", "ITERATION_END", "WHENEVER"))

	msg := requireRefused(t, got, err)

	if !strings.Contains(msg, "WHENEVER") {
		t.Errorf("refusal %q must name the offending trigger", msg)
	}
	requireNoDispatchOrWrite(t, f, store)
}

func TestSession_Start_Resume_OverrideWithUnknownTriggerOnNativeArtifact_RefusedBeforeAdoption(t *testing.T) {
	ses, f, store, orchPath := newIntervalAgentSession(t)
	store.state = nativeCreatedState()
	store.state.InfrastructureOverrides = []domain.InfrastructureOverride{overrideOf("checkpoint-manager-git", "WHENEVER")}
	store.exists = true
	queueWorkflowAgents(f)
	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)
	cfg.Mode = domain.ExecutionModeAuto
	cfg.Supplied = domain.SuppliedSettings{Mode: true}

	got, err := ses.Start(context.Background(), cfg)

	msg := requireRefused(t, got, err)
	if !strings.Contains(msg, "checkpoint-manager-git") || !strings.Contains(msg, "WHENEVER") {
		t.Errorf("refusal %q must name the agent and the trigger", msg)
	}
	requireNoDispatchOrWrite(t, f, store)
	if len(store.AdoptCalls) != 0 {
		t.Errorf("want no adoption call before the refusal, got %d", len(store.AdoptCalls))
	}
}

func TestSession_Start_Resume_OverrideWithUnknownAgent_RefusedBeforeCommitSetup(t *testing.T) {
	got, err, f, store := resumeCommitSetupPendingWith(t, overrideOf("no-such-agent", "ITERATION_END"))

	msg := requireRefused(t, got, err)

	if !strings.Contains(msg, "no-such-agent") {
		t.Errorf("refusal %q must name the agent", msg)
	}
	requireNoDispatchOrWrite(t, f, store)
}

// ===== STAGE_END guidance =====

func TestSession_Start_Resume_OverrideNamingStageEnd_RefusalSaysOverrideMustNameIterationEnd(t *testing.T) {
	got, err, f, store := resumeNonCommitClassWith(t, overrideOf("checkpoint-manager-git", "STAGE_END"))

	msg := requireRefused(t, got, err)

	lower := strings.ToLower(msg)
	for _, token := range []string{"checkpoint-manager-git", "stage_end", "iteration_end", "override"} {
		if !strings.Contains(lower, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	if strings.Contains(lower, "redeploy") {
		t.Errorf("redeploying does not fix an override; refusal %q must not suggest it", msg)
	}
	requireNoDispatchOrWrite(t, f, store)
}

// ===== Commit-class restriction =====

func TestSession_Start_Resume_CommitClassOverrideNamingIterationEnd_Accepted(t *testing.T) {
	got, err, _ := resumeCommitBranchRecordedWith(t, overrideOf("commit-manager-git", "ITERATION_END"))

	requireRunStatus(t, got, err, domain.RunCompleted)
}

func TestSession_Start_Resume_CommitClassOverrideNamingAnotherValidTrigger_Refused(t *testing.T) {
	for _, trigger := range []string{"INVOCATION_INTERVAL", "PHASE_END", "MANUAL"} {
		t.Run(trigger, func(t *testing.T) {
			got, err, f := resumeCommitBranchRecordedWith(t, overrideOf("commit-manager-git", trigger))

			msg := requireRefused(t, got, err)

			if !strings.Contains(msg, "commit-manager-git") || !strings.Contains(msg, trigger) {
				t.Errorf("refusal %q must name the agent and the trigger", msg)
			}
			if n := len(f.Invocations()); n != 0 {
				t.Errorf("want zero invocations, got %d", n)
			}
		})
	}
}

func TestSession_Start_Resume_CommitClassOverrideMixingIterationEndWithAnother_Refused(t *testing.T) {
	got, err, _ := resumeCommitBranchRecordedWith(t, overrideOf("commit-manager-git", "ITERATION_END", "MANUAL"))

	msg := requireRefused(t, got, err)

	if !strings.Contains(msg, "MANUAL") {
		t.Errorf("refusal %q must name the offending trigger", msg)
	}
}

func TestSession_Start_Resume_NonCommitClassOverride_AcceptsEveryVocabularyTrigger(t *testing.T) {
	for _, trigger := range []string{"INVOCATION_INTERVAL", "ITERATION_END", "PHASE_END", "MANUAL"} {
		t.Run(trigger, func(t *testing.T) {
			ses, f, store, orchPath := newIntervalAgentSession(t)
			store.state = domain.ArtifactState{
				Workflow:        "linear",
				WorkflowVersion: "1.0",
				Task:            "test task",
				RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeAuto},
				InfrastructureOverrides: []domain.InfrastructureOverride{
					overrideOf("checkpoint-manager-git", trigger),
				},
			}
			store.exists = true
			queueWorkflowAgents(f)
			cfg := baseLinearConfig(orchPath)
			markResume(&cfg)

			got, err := ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunCompleted)
		})
	}
}
