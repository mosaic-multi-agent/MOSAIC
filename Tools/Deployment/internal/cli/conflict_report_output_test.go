package cli_test

// conflict_report_output_test.go verifies how the CLI shows the catalog-stack conflict report:
// human output lists one line per entry with its severity before the summary, JSON output carries
// the report as an additive field of the single document, a run without shadowing prints what
// it printed before, and a report never changes the exit code.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

var (
	crUser    = domain.CatalogIdentity{ID: "user-a", Name: "User A", Version: "1.0.0"}
	crDefault = domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC", Version: "0.7.0"}
	crOther   = domain.CatalogIdentity{ID: "team-b", Name: "Team B", Version: "2.1"}
)

// crWarningReport is a report with an undeclared agent override that affects two workflows.
func crWarningReport() domain.ConflictReport {
	return domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:              domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "code-reviewer"},
		Winner:            crUser,
		Shadowed:          []domain.CatalogIdentity{crDefault},
		Severity:          domain.ConflictWarning,
		AffectedWorkflows: []string{"feature-dev", "quick-fix"},
	}}}
}

// crCommand is one summary-producing subcommand: its arguments and how to plant a summary in
// the spy service.
type crCommand struct {
	name string
	args func(ws string) []string
	set  func(s *spyService, sum domain.RunSummary)
	// summaryWord is text the human summary of a successful run contains.
	summaryWord string
}

func crCommands() []crCommand {
	return []crCommand{
		{"deploy", func(ws string) []string {
			return []string{"deploy", "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"}
		}, func(s *spyService, sum domain.RunSummary) { s.deployResp = sum }, "workspace:"},
		{"update", func(ws string) []string {
			return []string{"update", "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"}
		}, func(s *spyService, sum domain.RunSummary) { s.updateResp = sum }, "workspace:"},
		{"workflows", func(ws string) []string {
			return []string{"workflows", "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"}
		}, func(s *spyService, sum domain.RunSummary) { s.workflowsResp = sum }, "workspace:"},
		{"agents", func(ws string) []string {
			return []string{"agents", "--harness", "stub-harness", "--workspace", ws, "--yes"}
		}, func(s *spyService, sum domain.RunSummary) { s.deployAgentsResp = sum }, "workspace:"},
		{"hooks", func(ws string) []string {
			return []string{"hooks", "--harness", "stub-harness", "--workspace", ws, "--yes"}
		}, func(s *spyService, sum domain.RunSummary) { s.deployHooksResp = sum }, "workspace:"},
	}
}

// crRun executes the command with the given summary and output format.
func crRun(t *testing.T, c crCommand, sum domain.RunSummary, format string) (stdout string, code int) {
	t.Helper()
	svc := &spyService{}
	c.set(svc, sum)
	args := c.args(sum.WorkspacePath)
	if format != "" {
		args = append(args, "--output", format)
	}
	var out bytes.Buffer
	code = cli.Run(context.Background(), args, svc, &out, &bytes.Buffer{})
	return out.String(), code
}

// crLineWith returns the output line containing marker, failing the test when there is none.
func crLineWith(t *testing.T, output, marker string) string {
	t.Helper()
	for _, l := range strings.Split(output, "\n") {
		if strings.Contains(l, marker) {
			return l
		}
	}
	t.Fatalf("no output line contains %q; output:\n%s", marker, output)
	return ""
}

// TestHumanOutput_WarningLine_ShowsSeverityItemWinnerAndAffectedWorkflows verifies the
// content of a warning line for every summary-producing subcommand.
func TestHumanOutput_WarningLine_ShowsSeverityItemWinnerAndAffectedWorkflows(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			sum := successSummary(t.TempDir())
			sum.ConflictReport = crWarningReport()

			out, _ := crRun(t, c, sum, "")

			line := crLineWith(t, out, "agent:code-reviewer")
			for _, frag := range []string{
				"warning:",
				"from user-a 1.0.0 shadows mosaic-default 0.7.0",
				"affects workflows: feature-dev, quick-fix",
			} {
				if !strings.Contains(line, frag) {
					t.Errorf("report line %q lacks %q", line, frag)
				}
			}
		})
	}
}

// TestHumanOutput_ReportLinesComeBeforeTheSummary verifies the ordering: conflict lines first,
// then the run summary.
func TestHumanOutput_ReportLinesComeBeforeTheSummary(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			sum := successSummary(t.TempDir())
			sum.ConflictReport = crWarningReport()

			out, _ := crRun(t, c, sum, "")

			report := strings.Index(out, "agent:code-reviewer")
			summary := strings.Index(out, c.summaryWord)
			if report < 0 || summary < 0 || report > summary {
				t.Errorf("report line at %d, summary at %d; want the report first in:\n%s", report, summary, out)
			}
		})
	}
}

// TestHumanOutput_InfoLine_ShowsInfoSeverity verifies that a declared override is labelled info.
func TestHumanOutput_InfoLine_ShowsInfoSeverity(t *testing.T) {
	sum := successSummary(t.TempDir())
	rep := crWarningReport()
	rep.Lines[0].Severity = domain.ConflictInfo

	sum.ConflictReport = rep
	out, _ := crRun(t, crCommands()[0], sum, "")

	line := crLineWith(t, out, "agent:code-reviewer")
	if !strings.Contains(line, "info:") || strings.Contains(line, "warning:") {
		t.Errorf("line %q, want severity info and not warning", line)
	}
}

// TestHumanOutput_PairLine_OmitsKeyAndListsEveryShadowedLayer verifies the orchestrator pair
// line (no key after the kind), several shadowed layers separated by a comma, and a line
// without affected workflows ending without the workflows clause.
func TestHumanOutput_PairLine_OmitsKeyAndListsEveryShadowedLayer(t *testing.T) {
	sum := successSummary(t.TempDir())
	sum.ConflictReport = domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:     domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair},
		Winner:   crUser,
		Shadowed: []domain.CatalogIdentity{crOther, crDefault},
		Severity: domain.ConflictWarning,
	}}}

	out, _ := crRun(t, crCommands()[0], sum, "")

	line := crLineWith(t, out, "orchestrator-pair")
	if strings.Contains(line, "orchestrator-pair:") {
		t.Errorf("line %q carries a key after the kind; the pair has none", line)
	}
	if !strings.Contains(line, "from user-a 1.0.0 shadows team-b 2.1, mosaic-default 0.7.0") {
		t.Errorf("line %q, want both shadowed layers, highest first, comma separated", line)
	}
	if strings.Contains(line, "affects workflows") {
		t.Errorf("line %q names workflows although none are affected", line)
	}
}

// TestHumanOutput_NoShadowing_OutputIsUnchanged verifies that a run without conflict lines
// prints exactly what it printed before the report existed.
func TestHumanOutput_NoShadowing_OutputIsUnchanged(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()

			out, code := crRun(t, c, successSummary(ws), "")

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
			}
			var want string
			switch c.name {
			case "agents":
				want = "agents deployment complete\nworkspace: " + ws + "\n"
			case "hooks":
				want = "hooks deployment complete\nworkspace: " + ws + "\n"
			default:
				want = "success: deployment complete\nworkspace: " + ws + "\n"
			}
			if out != want {
				t.Errorf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestHumanOutput_Warning_DoesNotChangeExitCode verifies that report lines never change the
// exit code: a successful run with a warning still exits 0.
func TestHumanOutput_Warning_DoesNotChangeExitCode(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			sum := successSummary(t.TempDir())
			sum.ConflictReport = crWarningReport()

			_, code := crRun(t, c, sum, "")

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want %d: a conflict warning must not change it", code, cli.ExitSuccess)
			}
		})
	}
}

// TestJSONOutput_Report_IsAdditiveFieldOfTheSingleDocument verifies that --output json writes
// one JSON document in which every existing key is still present and the key ConflictReport
// carries the report lines with their severity; no human report line leaks into stdout.
func TestJSONOutput_Report_IsAdditiveFieldOfTheSingleDocument(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			sum := successSummary(t.TempDir())
			sum.ConflictReport = crWarningReport()

			out, code := crRun(t, c, sum, "json")

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
			}
			for _, key := range []string{"Mode", "Outcome", "WorkspacePath", "Actions"} {
				if _, ok := doc[key]; !ok {
					t.Errorf("existing key %q missing from the document", key)
				}
			}
			var rep struct {
				Lines []struct {
					Item              struct{ Kind, Key string } `json:"item"`
					Winner            struct{ ID string }        `json:"winner"`
					Severity          string                     `json:"severity"`
					AffectedWorkflows []string                   `json:"affectedWorkflows"`
				} `json:"lines"`
			}
			if err := json.Unmarshal(doc["ConflictReport"], &rep); err != nil || len(rep.Lines) != 1 {
				t.Fatalf("ConflictReport = %s (decode err %v), want one line", doc["ConflictReport"], err)
			}
			l := rep.Lines[0]
			if l.Severity != "warning" || l.Item.Kind != "agent" || l.Item.Key != "code-reviewer" || l.Winner.ID != "user-a" {
				t.Errorf("line = %+v, want a warning for agent code-reviewer won by user-a", l)
			}
			if strings.Join(l.AffectedWorkflows, ",") != "feature-dev,quick-fix" {
				t.Errorf("affectedWorkflows = %v, want [feature-dev quick-fix]", l.AffectedWorkflows)
			}
			if strings.Contains(out, "warning:") {
				t.Errorf("stdout carries a human report line next to the document:\n%s", out)
			}
		})
	}
}

// TestJSONOutput_NoShadowing_ReportIsEmptyObject verifies that without shadowing the document
// still decodes and carries an empty report (an object without lines).
func TestJSONOutput_NoShadowing_ReportIsEmptyObject(t *testing.T) {
	for _, c := range crCommands() {
		t.Run(c.name, func(t *testing.T) {
			out, _ := crRun(t, c, successSummary(t.TempDir()), "json")

			var doc map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(doc["ConflictReport"])); got != "{}" {
				t.Errorf("ConflictReport = %s, want {}", got)
			}
		})
	}
}

// crRenderRun executes the render subcommand with the given result and output format.
func crRenderRun(t *testing.T, res app.RenderAgentResult, format string) (stdout string, code int) {
	t.Helper()
	svc := &spyService{renderAgentResp: res}
	args := minRenderArgs("stub-harness", "/agents/agent.md", "/out/agent.md")
	if format != "" {
		args = append(args, "--output", format)
	}
	var out bytes.Buffer
	code = cli.Run(context.Background(), args, svc, &out, &bytes.Buffer{})
	return out.String(), code
}

// TestRenderOutput_HumanShowsReportLines verifies that render lists the report lines with
// their severity and still exits 0.
func TestRenderOutput_HumanShowsReportLines(t *testing.T) {
	rep := crWarningReport()
	res := app.RenderAgentResult{DestinationPath: "/out/agent.md", AgentKey: "code-reviewer", ConflictReport: &rep}

	out, code := crRenderRun(t, res, "")

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	line := crLineWith(t, out, "agent:code-reviewer")
	if !strings.Contains(line, "warning:") || !strings.Contains(line, "from user-a 1.0.0 shadows mosaic-default 0.7.0") {
		t.Errorf("report line %q lacks the severity or the layers", line)
	}
}

// TestRenderOutput_JSONCarriesReportOnlyWhenPresent verifies the additive field of the render
// document: conflictReport appears with its lines when the result has a report, and is absent
// otherwise.
func TestRenderOutput_JSONCarriesReportOnlyWhenPresent(t *testing.T) {
	rep := crWarningReport()

	withOut, _ := crRenderRun(t, app.RenderAgentResult{DestinationPath: "/out/agent.md", ConflictReport: &rep}, "json")
	withoutOut, _ := crRenderRun(t, app.RenderAgentResult{DestinationPath: "/out/agent.md"}, "json")

	var with, without map[string]json.RawMessage
	if err := json.Unmarshal([]byte(withOut), &with); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, withOut)
	}
	if err := json.Unmarshal([]byte(withoutOut), &without); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, withoutOut)
	}
	if !strings.Contains(string(with["conflictReport"]), `"severity":"warning"`) {
		t.Errorf("conflictReport = %s, want a warning line", with["conflictReport"])
	}
	if _, ok := with["destinationPath"]; !ok {
		t.Error("existing key destinationPath missing from the document")
	}
	if _, ok := without["conflictReport"]; ok {
		t.Error("document without a report carries conflictReport, want the key absent")
	}
}

// TestRenderOutput_NoReport_HumanOutputUnchanged verifies that a render without a report prints
// no conflict lines.
func TestRenderOutput_NoReport_HumanOutputUnchanged(t *testing.T) {
	out, _ := crRenderRun(t, app.RenderAgentResult{DestinationPath: "/out/agent.md", AgentKey: "code-reviewer"}, "")

	if strings.Contains(out, "warning:") || strings.Contains(out, "info:") {
		t.Errorf("output carries a report line without a report:\n%s", out)
	}
}
