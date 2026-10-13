package session_test

// Tests that a trigger outside the closed vocabulary in the orchestrator's
// infrastructure declaration region makes the Runner refuse to start or resume
// before any agent is dispatched and before any artifact is created or
// written. Orchestrator files are built in the test so each case states its own
// declaration.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const trigRefusalWorkflow = `<Workflow type="core" name="linear" version="1.0">
## Linear Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | FALSE | COMPLETE | - | plan.md | result.md |
</Workflow>
`

// trigTableHeader is a declaration table header with a Trigger column.
const trigTableHeader = "| Class | Trigger | Param | On Failure | Description |\n" +
	"|-------|---------|-------|------------|-------------|\n"

// trigAgentRegion renders one declared infrastructure agent with a single row.
func trigAgentRegion(name, class, triggerCell string) string {
	return "<InfrastructureAgent type=\"core\" name=\"" + name + "\" version=\"1.0.0\">\n" +
		trigTableHeader +
		"| " + class + " | " + triggerCell + " | - | halt | row |\n" +
		"</InfrastructureAgent>\n"
}

// newTriggerRefusalRig builds a session over a linear workflow whose
// infrastructure declaration region holds the given agent regions. Every
// agent of the workflow and every declared agent has an agent file, and the
// workflow agents have scripted SUCCESS responses so that a run that is
// wrongly allowed to start completes instead of failing for another reason.
func newTriggerRefusalRig(t *testing.T, regions ...string) (session.Session, *harness.MockAdapter, *memStore, string) {
	t.Helper()
	dir := t.TempDir()
	content := trigRefusalWorkflow + "\n<InfrastructureAgents type=\"managed\">\n" +
		strings.Join(regions, "") + "</InfrastructureAgents>\n"
	orchPath := filepath.Join(dir, "orchestrator.md")
	if err := os.WriteFile(orchPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write orchestrator file: %v", err)
	}
	for _, a := range []string{"agent-a", "agent-b", "commit-manager-git", "checkpoint-manager-git", "restore-manager-git"} {
		writeAgentFile(t, dir, a)
	}
	f := harness.NewMockAdapter()
	for _, a := range []string{"agent-a", "agent-b", "commit-manager-git", "checkpoint-manager-git", "restore-manager-git"} {
		f.Queue(a, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: a + "#queued", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
	store := &memStore{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    store,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
		Manual:   &scriptedRoutingConsultant{},
	})
	return ses, f, store, orchPath
}

// requireNothingDispatchedOrCreated asserts the run left no trace: no agent was
// invoked, no artifact was created and nothing was recorded or rewritten.
func requireNothingDispatchedOrCreated(t *testing.T, f *harness.MockAdapter, store *memStore) {
	t.Helper()
	if n := len(f.Invocations()); n != 0 {
		t.Errorf("want zero invocations (including commit setup), got %d", n)
	}
	if store.exists {
		t.Error("want no artifact created or stored")
	}
	if len(store.Applied) != 0 || store.AdoptWrites != 0 || len(store.BranchCalls) != 0 {
		t.Errorf("want no artifact write, got applied=%d adopt=%d branch=%v",
			len(store.Applied), store.AdoptWrites, store.BranchCalls)
	}
}

// ===== New run =====

func TestSession_Start_NewRun_UnknownTriggerInDeclaration_RefusedNamingAgentAndTrigger(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", "WHENEVER"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	msg := requireRefused(t, got, err)
	for _, token := range []string{"commit-manager-git", "WHENEVER"} {
		if !strings.Contains(msg, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	requireNothingDispatchedOrCreated(t, f, store)
}

func TestSession_Start_NewRun_InvalidDeclarationTriggers_Refused(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
	}{
		{"legacy stage end", "STAGE_END"},
		{"lowercase vocabulary name", "iteration_end"},
		{"blank trigger cell", "  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", tc.trigger))

			got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, "commit-manager-git") {
				t.Errorf("refusal %q must name the agent", msg)
			}
			requireNothingDispatchedOrCreated(t, f, store)
		})
	}
}

func TestSession_Start_NewRun_DeclarationTableWithoutTriggerColumn_Refused(t *testing.T) {
	region := "<InfrastructureAgent type=\"core\" name=\"commit-manager-git\" version=\"1.0.0\">\n" +
		"| Class | Param | On Failure | Description |\n" +
		"|-------|-------|------------|-------------|\n" +
		"| commit | - | halt | row |\n" +
		"</InfrastructureAgent>\n"
	ses, f, store, orchPath := newTriggerRefusalRig(t, region)

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	msg := requireRefused(t, got, err)
	if !strings.Contains(msg, "commit-manager-git") {
		t.Errorf("refusal %q must name the agent", msg)
	}
	requireNothingDispatchedOrCreated(t, f, store)
}

func TestSession_Start_NewRun_UnknownTriggerWithCommitsEnabled_RefusedBeforeSetupDispatch(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", "WHENEVER"))
	cfg := baseCommitConfig(orchPath)

	got, err := ses.Start(context.Background(), cfg)

	requireRefused(t, got, err)
	requireNothingDispatchedOrCreated(t, f, store)
}

func TestSession_Start_NewRun_UnknownTriggerOnRestoreClassAgent_Refused(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("restore-manager-git", "restore", "AT_DAWN"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	msg := requireRefused(t, got, err)
	for _, token := range []string{"restore-manager-git", "AT_DAWN"} {
		if !strings.Contains(msg, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	requireNothingDispatchedOrCreated(t, f, store)
}

func TestSession_Start_NewRun_UnknownTriggerOnAgentExcludedByFilter_Refused(t *testing.T) {
	filters := map[string][]string{
		"filter naming only another agent": {"checkpoint-manager-git"},
		"filter naming no agent":           {},
	}
	for name, filter := range filters {
		t.Run(name, func(t *testing.T) {
			ses, f, store, orchPath := newTriggerRefusalRig(t,
				trigAgentRegion("checkpoint-manager-git", "checkpoint", "PHASE_END"),
				trigAgentRegion("commit-manager-git", "commit", "WHENEVER"))
			cfg := baseLinearConfig(orchPath)
			cfg.InfrastructureFilter = filter

			got, err := ses.Start(context.Background(), cfg)

			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, "commit-manager-git") || !strings.Contains(msg, "WHENEVER") {
				t.Errorf("refusal %q must name the excluded agent and its trigger", msg)
			}
			requireNothingDispatchedOrCreated(t, f, store)
		})
	}
}

func TestSession_Start_NewRun_StageEndInDeclaration_RefusalGuidesToIterationEndAndRedeploy(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", "STAGE_END"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	msg := strings.ToLower(requireRefused(t, got, err))
	for _, token := range []string{"stage_end", "iteration_end", "redeploy"} {
		if !strings.Contains(msg, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	requireNothingDispatchedOrCreated(t, f, store)
}

// ===== Resume =====

func TestSession_Start_Resume_UnknownTriggerInDeclaration_RefusedBeforeAdoptionAndDispatch(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", "WHENEVER"))
	store.state = nativeCreatedState()
	store.exists = true
	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)
	cfg.Mode = domain.ExecutionModeAuto
	cfg.Supplied = domain.SuppliedSettings{Mode: true}

	got, err := ses.Start(context.Background(), cfg)

	msg := requireRefused(t, got, err)
	for _, token := range []string{"commit-manager-git", "WHENEVER"} {
		if !strings.Contains(msg, token) {
			t.Errorf("refusal %q must contain %q", msg, token)
		}
	}
	requireNoDispatchOrWrite(t, f, store)
	if len(store.AdoptCalls) != 0 {
		t.Errorf("want no adoption call before the refusal, got %d", len(store.AdoptCalls))
	}
}

func TestSession_Start_Resume_UnknownTriggerWithCommitSetupPending_RefusedBeforeSetupDispatch(t *testing.T) {
	ses, f, store, orchPath := newTriggerRefusalRig(t, trigAgentRegion("commit-manager-git", "commit", "STAGE_END"))
	store.state = commitResumeState()
	store.exists = true
	cfg := baseCommitConfig(orchPath)
	markResume(&cfg)
	cfg.Supplied.CommitBranchVariant = true

	got, err := ses.Start(context.Background(), cfg)

	requireRefused(t, got, err)
	requireNoDispatchOrWrite(t, f, store)
	if len(store.BranchCalls) != 0 {
		t.Errorf("want no commit_branch write, got %v", store.BranchCalls)
	}
}

func TestSession_Start_Resume_InvalidDeclarationTriggers_Refused(t *testing.T) {
	cases := []struct {
		name   string
		region string
	}{
		{"blank trigger cell", trigAgentRegion("commit-manager-git", "commit", "  ")},
		{"lowercase vocabulary name", trigAgentRegion("commit-manager-git", "commit", "iteration_end")},
		{"missing trigger column", "<InfrastructureAgent type=\"core\" name=\"commit-manager-git\" version=\"1.0.0\">\n" +
			"| Class | Param | On Failure | Description |\n" +
			"|-------|-------|------------|-------------|\n" +
			"| commit | - | halt | row |\n" +
			"</InfrastructureAgent>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ses, f, store, orchPath := newTriggerRefusalRig(t, tc.region)
			store.state = nativeCreatedState()
			store.exists = true
			cfg := baseLinearConfig(orchPath)
			markResume(&cfg)
			cfg.Mode = domain.ExecutionModeAuto
			cfg.Supplied = domain.SuppliedSettings{Mode: true}

			got, err := ses.Start(context.Background(), cfg)

			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, "commit-manager-git") {
				t.Errorf("refusal %q must name the agent", msg)
			}
			requireNoDispatchOrWrite(t, f, store)
			if len(store.AdoptCalls) != 0 {
				t.Errorf("want no adoption call before the refusal, got %d", len(store.AdoptCalls))
			}
		})
	}
}
