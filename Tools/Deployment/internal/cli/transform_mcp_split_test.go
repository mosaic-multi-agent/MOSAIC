package cli_test

// transform_mcp_split_test.go covers the CLI side of the MCP name split question of the
// transform command: the repeatable --mcp-split <source-name>=<server> flag pre-answers the
// question on the request, malformed values are usage errors, and the non-interactive
// interaction answers the question from pre-answers through its select-one lookup (an answer
// naming no offered split is a recorded skip).

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// runTransformWith runs the transform command with the extra arguments and returns the exit
// code, the service spy and stderr.
func runTransformWith(t *testing.T, extra ...string) (int, *spyService, *bytes.Buffer) {
	t.Helper()
	svc := &spyService{transformResp: app.TransformHarnessResult{SourceHarnessID: "src", TargetHarnessID: "tgt"}}
	errBuf := &bytes.Buffer{}
	args := append([]string{"transform", "--harness", "src", "--target-harness", "tgt", "--workspace", "/ws"}, extra...)
	code := cli.Run(context.Background(), args, svc, &bytes.Buffer{}, errBuf)
	return code, svc, errBuf
}

func TestTransformCmd_McpSplitFlag_SinglePair_SetsMCPSplits(t *testing.T) {
	code, svc, _ := runTransformWith(t, "--mcp-split", "azure-devops_get_pull_request=azure-devops")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess (%d)", code, cli.ExitSuccess)
	}
	if svc.transformReq == nil {
		t.Fatal("svc.TransformHarness was not called")
	}
	if got := svc.transformReq.MCPSplits["azure-devops_get_pull_request"]; got != "azure-devops" {
		t.Errorf("MCPSplits = %v, want azure-devops_get_pull_request -> azure-devops", svc.transformReq.MCPSplits)
	}
}

func TestTransformCmd_McpSplitFlag_RepeatedFlag_SetsEveryPair(t *testing.T) {
	code, svc, _ := runTransformWith(t,
		"--mcp-split", "a-b_c_d=a-b",
		"--mcp-split", "tracker_get_issue=tracker_get")

	if code != cli.ExitSuccess || svc.transformReq == nil {
		t.Fatalf("exit code = %d, request = %v; want a successful dispatch", code, svc.transformReq)
	}
	got := svc.transformReq.MCPSplits
	if len(got) != 2 || got["a-b_c_d"] != "a-b" || got["tracker_get_issue"] != "tracker_get" {
		t.Errorf("MCPSplits = %v, want both pairs", got)
	}
}

func TestTransformCmd_McpSplitFlag_Absent_LeavesMCPSplitsEmpty(t *testing.T) {
	code, svc, _ := runTransformWith(t)

	if code != cli.ExitSuccess || svc.transformReq == nil {
		t.Fatalf("exit code = %d, want a successful dispatch", code)
	}
	if len(svc.transformReq.MCPSplits) != 0 {
		t.Errorf("MCPSplits = %v, want none without the flag", svc.transformReq.MCPSplits)
	}
}

func TestTransformCmd_McpSplitFlag_Malformed_ReturnsExitUsageNamingTheValue(t *testing.T) {
	for _, bad := range []string{"no-separator", "tracker_get_issue=", "=tracker"} {
		code, svc, errBuf := runTransformWith(t, "--mcp-split", bad)

		if code != cli.ExitUsage {
			t.Errorf("%q: exit code = %d, want ExitUsage (%d)", bad, code, cli.ExitUsage)
		}
		if svc.transformReq != nil {
			t.Errorf("%q: svc.TransformHarness was called despite a malformed --mcp-split", bad)
		}
		if !strings.Contains(errBuf.String(), bad) {
			t.Errorf("%q: stderr must name the offending value, got %q", bad, errBuf.String())
		}
	}
}

func TestTransformCmd_HelpText_DocumentsMcpSplitFlag(t *testing.T) {
	errBuf := &bytes.Buffer{}

	cli.Run(context.Background(), []string{"transform", "--help"}, &spyService{}, &bytes.Buffer{}, errBuf)

	if !strings.Contains(errBuf.String(), "--mcp-split") {
		t.Errorf("transform --help must document --mcp-split, got:\n%s", errBuf.String())
	}
}

// splitQuestion is the question the transform asks for an ambiguous OpenCode name.
func splitQuestion(subject string) domain.ChoiceQuestion {
	return domain.ChoiceQuestion{
		Question: domain.Question{ID: domain.QMCPNameSplit, Subject: subject, AllowSkip: true, AllowSkipAll: true},
		Options: []domain.Option{
			{ID: "tracker", Label: "tracker / get_issue"},
			{ID: "tracker_get", Label: "tracker_get / issue"},
		},
	}
}

func TestNewInteraction_SelectOne_McpNameSplit_PreAnswerIsAnswered(t *testing.T) {
	answers := cli.PreAnswers{Values: map[domain.QuestionID]map[string]string{
		domain.QMCPNameSplit: {"tracker_get_issue": "tracker_get"},
	}}
	ix := cli.NewInteraction(answers, &stubTodo{}, &bytes.Buffer{})

	ans, err := ix.SelectOne(context.Background(), splitQuestion("tracker_get_issue"))

	if err != nil {
		t.Fatalf("SelectOne returned error: %v", err)
	}
	if ans.Status != domain.Answered || ans.OptionID != "tracker_get" {
		t.Errorf("answer = %+v, want Answered with tracker_get", ans)
	}
}

func TestNewInteraction_SelectOne_McpNameSplit_PreAnswerNamingNoOfferedSplitIsARecordedSkip(t *testing.T) {
	todos := &stubTodo{}
	answers := cli.PreAnswers{Values: map[domain.QuestionID]map[string]string{
		domain.QMCPNameSplit: {"tracker_get_issue": "nonsense"},
	}}
	ix := cli.NewInteraction(answers, todos, &bytes.Buffer{})

	ans, err := ix.SelectOne(context.Background(), splitQuestion("tracker_get_issue"))

	if err != nil {
		t.Fatalf("SelectOne returned error: %v", err)
	}
	if ans.Status != domain.SkippedOne {
		t.Errorf("Status = %q, want skipped", ans.Status)
	}
	if len(todos.gaps) == 0 {
		t.Error("a skipped question must be recorded as a gap")
	}
}

func TestNewInteraction_SelectOne_McpNameSplit_WithoutPreAnswerIsARecordedSkip(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(cli.PreAnswers{}, todos, &bytes.Buffer{})

	ans, err := ix.SelectOne(context.Background(), splitQuestion("tracker_get_issue"))

	if err != nil {
		t.Fatalf("SelectOne returned error: %v", err)
	}
	if ans.Status != domain.SkippedOne || len(todos.gaps) == 0 {
		t.Errorf("answer = %+v gaps = %v, want a recorded skip", ans, todos.gaps)
	}
}
