package app_test

// custom_tool_list_flows_test.go specifies the custom tool question of the deploy-family flows
// (deploy, deploy agents, change workflows): the question teaches the list syntax with a mixed
// example and asks for no harness-specific names, an answer that does not fit the syntax is
// refused through the question's validation hook, every entry of an answer reaches the tools of
// every agent that declares the generic tool, and the tool is then reported unmapped nowhere
// (plan, TODO file, summary). A single bare server answer keeps its result.

import (
	"context"
	"slices"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// listFlowAnswer is the mixed answer of the flow tests: one tool of a server and a whole server.
const listFlowAnswer = "github/get_pr, jira"

// listFlowEntries is what listFlowAnswer means.
var listFlowEntries = []domain.MCPEntry{{Server: "github", Tool: "get_pr"}, {Server: "jira"}}

// customToolSpy records the QCustomTool questions asked and delegates the answer to the stub.
type customToolSpy struct {
	*interactiontest.Stub
	questions []domain.TextQuestion
}

func (s *customToolSpy) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	if q.ID == domain.QCustomTool {
		s.questions = append(s.questions, q)
	}
	return s.Stub.AskText(ctx, q)
}

// listFlowRun is a deploy-family run in which two agents declare the unmapped generic tool
// "bash" and the user answers its question with answer.
type listFlowRun struct {
	*customToolRun
	spy  *customToolSpy
	reqs *[]domain.ToolRequest // every ToolRequest the harness module received
}

func newListFlowRun(t *testing.T, answer string) *listFlowRun {
	t.Helper()
	agents := []domain.Agent{toolAgent("test-runner", "bash"), toolAgent("second-runner", "bash")}
	return newListFlowRunWith(t, agents, []string{"bash"}, map[string]string{"bash": answer})
}

// newListFlowRunWith runs the same wiring for the given agents: every tool in unmapped is
// unmapped for the harness, and answers holds the answer per generic tool (a tool without an
// entry is skipped).
func newListFlowRunWith(t *testing.T, agents []domain.Agent, unmapped []string, answers map[string]string) *listFlowRun {
	t.Helper()
	r := newCustomToolRun(t, agents, unmapped...)
	b := interactiontest.NewBuilder()
	for generic, answer := range answers {
		b = b.AnswerText(domain.QCustomTool, generic, answer)
	}
	script := b.
		AnswerReview(true).
		Build()
	spy := &customToolSpy{Stub: script}
	r.deps.Interaction = spy
	var reqs []domain.ToolRequest
	reg := r.deps.Registry.(*stubRegistry)
	reg.modules["stub-harness"] = &capturingToolsModule{
		base:    reg.modules["stub-harness"],
		onTools: func(req domain.ToolRequest) { reqs = append(reqs, req) },
	}
	return &listFlowRun{customToolRun: r, spy: spy, reqs: &reqs}
}

// listFlow is one deploy-family flow driven to completion.
type listFlow struct {
	name string
	run  func(t *testing.T, r *listFlowRun) domain.RunSummary
}

var listSkipModels = map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true}

var listFlows = []listFlow{
	{"deploy", func(t *testing.T, r *listFlowRun) domain.RunSummary {
		s, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
			HarnessID: "stub-harness", WorkspacePath: r.workDir, Scope: domain.ScopeProject,
			WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		if err != nil {
			t.Fatalf("DeployNew: %v", err)
		}
		return s
	}},
	{"deploy agents", func(t *testing.T, r *listFlowRun) domain.RunSummary {
		s, err := app.New(r.deps).DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: "stub-harness", WorkspacePath: r.workDir, Scope: domain.ScopeProject,
			SubagentIDs: []string{"test-runner", "second-runner"}, UtilityAgentIDs: []string{},
			InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
			SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		if err != nil {
			t.Fatalf("DeployAgents: %v", err)
		}
		return s
	}},
	{"change workflows", func(t *testing.T, r *listFlowRun) domain.RunSummary {
		s, err := app.New(r.deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: "stub-harness", WorkspacePath: r.workDir, WorkflowIDs: []string{"quick-fix"},
			SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		if err != nil {
			t.Fatalf("UpdateWorkflows: %v", err)
		}
		return s
	}},
}

// customEntriesByAgent returns, per agent key, the entries of every ToolRequest that carried an
// answer for "bash".
func (r *listFlowRun) customEntriesByAgent() map[string][][]domain.MCPEntry {
	return r.entriesFor("bash")
}

// entriesFor returns, per agent key, the entries of every ToolRequest that carried an answer
// for the given generic tool.
func (r *listFlowRun) entriesFor(generic string) map[string][][]domain.MCPEntry {
	out := map[string][][]domain.MCPEntry{}
	for _, req := range *r.reqs {
		if e := req.CustomNames[generic]; len(e) > 0 {
			out[req.AgentKey] = append(out[req.AgentKey], e)
		}
	}
	return out
}

func assertEveryAgentGets(t *testing.T, r *listFlowRun, want []domain.MCPEntry) {
	t.Helper()
	got := r.customEntriesByAgent()
	for _, key := range []string{"test-runner", "second-runner"} {
		if len(got[key]) == 0 {
			t.Errorf("agent %q received no custom tool entries for bash", key)
		}
		for _, entries := range got[key] {
			if !slices.Equal(entries, want) {
				t.Errorf("agent %q received entries %+v, want %+v", key, entries, want)
			}
		}
	}
}

func assertNotInSummary(t *testing.T, summary domain.RunSummary, generic string) {
	t.Helper()
	for _, it := range summary.Todos {
		if it.Category == domain.TodoToolMappings && it.Subject == generic {
			t.Errorf("summary still lists %q as an open tool mapping: %+v", generic, it)
		}
	}
}

func TestCustomToolQuestion_TeachesTheListSyntaxWithAMixedExample(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newListFlowRun(t, listFlowAnswer)

			flow.run(t, r)

			if len(r.spy.questions) == 0 {
				t.Fatal("QCustomTool was not asked")
			}
			assertListSyntaxQuestion(t, r.spy.questions[0])
		})
	}
}

func TestCustomToolQuestion_ValidateRefusesAnswersThatDoNotFitTheSyntax(t *testing.T) {
	r := newListFlowRun(t, listFlowAnswer)
	listFlows[0].run(t, r)
	if len(r.spy.questions) == 0 {
		t.Fatal("QCustomTool was not asked")
	}
	validate := r.spy.questions[0].Validate
	if validate == nil {
		t.Fatal("the question carries no Validate hook")
	}

	assertListSyntaxValidate(t, validate)
}

func TestCustomToolQuestion_MixedAnswer_ReachesEveryAgentAndLeavesNoUnmappedGap(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newListFlowRun(t, listFlowAnswer)

			summary := flow.run(t, r)

			// Only the entries assertion is evidence for the new behavior; the gap assertions
			// are support (custom_tool_list_gaps_test.go holds the discriminating ones).
			assertEveryAgentGets(t, r, listFlowEntries)
			assertToolGapReporting(t, r.customToolRun, []string{"bash"}, nil)
			assertNotInSummary(t, summary, "bash")
			if n := r.spy.Stub.CountAsked(domain.QCustomTool, "bash"); n != 1 {
				t.Errorf("QCustomTool asked %d times for bash, want once for both agents", n)
			}
		})
	}
}

func TestCustomToolQuestion_AnswerForms_BecomeTheEntriesTheyName(t *testing.T) {
	cases := []struct {
		answer string
		want   []domain.MCPEntry
	}{
		{"my-bash-server", []domain.MCPEntry{{Server: "my-bash-server"}}}, // today's single bare server
		{"  github/* ", []domain.MCPEntry{{Server: "github"}}},
		{"github/get_pr,jira,github/get_pr", listFlowEntries},
	}
	for _, c := range cases {
		t.Run(c.answer, func(t *testing.T) {
			r := newListFlowRun(t, c.answer)

			summary := listFlows[0].run(t, r)

			assertEveryAgentGets(t, r, c.want)
			assertToolGapReporting(t, r.customToolRun, []string{"bash"}, nil)
			assertNotInSummary(t, summary, "bash")
		})
	}
}

func TestCustomToolQuestion_PreAnsweredEntries_AreUsedWithoutAskingAndLeaveNoGap(t *testing.T) {
	r := newListFlowRun(t, "unused")

	_, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: "stub-harness", WorkspacePath: r.workDir, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
		CustomTools: domain.CustomToolAnswers{"bash": listFlowEntries},
	})

	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	if len(r.spy.questions) != 0 {
		t.Error("QCustomTool was asked although the generic tool was pre-answered")
	}
	assertEveryAgentGets(t, r, listFlowEntries)
	assertToolGapReporting(t, r.customToolRun, []string{"bash"}, nil)
}
