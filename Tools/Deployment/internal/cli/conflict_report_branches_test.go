package cli_test

// conflict_report_branches_test.go verifies that the catalog-stack conflict report is also shown
// on the dry-run branch of every summary-producing subcommand and of render, and when the
// service declines the plan (ErrPlanNotConfirmed returned together with a summary that carries
// the report) without the report changing the exit code.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// crSetErr plants the service error a subcommand's spy returns next to its summary.
func crSetErr(c crCommand, s *spyService, err error) {
	switch c.name {
	case "deploy":
		s.deployErr = err
	case "update":
		s.updateErr = err
	case "workflows":
		s.workflowsErr = err
	case "agents":
		s.deployAgentsErr = err
	case "hooks":
		s.deployHooksErr = err
	}
}

// crRunWith executes the command with a summary, extra arguments and a service error, and
// returns stdout, stderr and the exit code.
func crRunWith(t *testing.T, c crCommand, sum domain.RunSummary, extra []string, err error) (stdout, stderr string, code int) {
	t.Helper()
	svc := &spyService{}
	c.set(svc, sum)
	crSetErr(c, svc, err)
	var out, errOut bytes.Buffer
	code = cli.Run(context.Background(), append(c.args(sum.WorkspacePath), extra...), svc, &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestHumanOutput_DryRun_ShowsTheReportBeforeTheSummary verifies that a dry run prints the
// warning line, ahead of the summary, for every summary-producing subcommand.
func TestHumanOutput_DryRun_ShowsTheReportBeforeTheSummary(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			sum := successSummary(t.TempDir())
			sum.ConflictReport = crWarningReport()

			out, _, code := crRunWith(t, c, sum, []string{"--dry-run"}, nil)

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
			}
			line := crLineWith(t, out, "agent:code-reviewer")
			if !strings.Contains(line, "warning:") || !strings.Contains(line, "affects workflows: feature-dev, quick-fix") {
				t.Errorf("dry-run report line %q lacks the severity or the affected workflows", line)
			}
			report := strings.Index(out, "agent:code-reviewer")
			summary := strings.Index(out, c.summaryWord)
			if summary < 0 || report > summary {
				t.Errorf("report at %d, summary at %d; want the report first in:\n%s", report, summary, out)
			}
		})
	}
}

// TestHumanOutput_DeclinedPlan_ShowsTheReportAndKeepsTheExitCode verifies that when the service
// returns ErrPlanNotConfirmed with a summary carrying the report, the warning line reaches
// stdout and the exit code equals the one the same declined plan gives without a report.
func TestHumanOutput_DeclinedPlan_ShowsTheReportAndKeepsTheExitCode(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			withReport := successSummary(ws)
			withReport.ConflictReport = crWarningReport()
			_, _, plainCode := crRunWith(t, c, domain.RunSummary{WorkspacePath: ws}, nil, app.ErrPlanNotConfirmed)

			out, _, code := crRunWith(t, c, withReport, nil, app.ErrPlanNotConfirmed)

			line := crLineWith(t, out, "agent:code-reviewer")
			if !strings.Contains(line, "warning:") {
				t.Errorf("declined-plan report line %q lacks the severity", line)
			}
			if code != plainCode {
				t.Errorf("exit code = %d with a report, %d without: the report must not change it", code, plainCode)
			}
		})
	}
}

// TestRenderOutput_DryRun_ShowsReportLines verifies that a dry-run render prints the warning
// line.
func TestRenderOutput_DryRun_ShowsReportLines(t *testing.T) {
	rep := crWarningReport()
	res := app.RenderAgentResult{DestinationPath: "/out/agent.md", AgentKey: "code-reviewer", DryRun: true, ConflictReport: &rep}

	out, code := crRenderRun(t, res, "")

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	line := crLineWith(t, out, "agent:code-reviewer")
	if !strings.Contains(line, "warning:") {
		t.Errorf("dry-run render report line %q lacks the severity", line)
	}
}

// TestRenderOutput_ReportLinesComeBeforeTheResult verifies that render prints the conflict
// lines ahead of its result, which names the destination path.
func TestRenderOutput_ReportLinesComeBeforeTheResult(t *testing.T) {
	rep := crWarningReport()
	res := app.RenderAgentResult{DestinationPath: "/out/agent.md", AgentKey: "code-reviewer", ConflictReport: &rep}

	out, _ := crRenderRun(t, res, "")

	report := strings.Index(out, "agent:code-reviewer")
	result := strings.Index(out, "/out/agent.md")
	if report < 0 || result < 0 || report > result {
		t.Errorf("report line at %d, result at %d; want the report first in:\n%s", report, result, out)
	}
}
