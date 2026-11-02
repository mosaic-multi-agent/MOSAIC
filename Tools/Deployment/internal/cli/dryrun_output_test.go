package cli_test

// dryrun_output_test.go specifies the human output of --dry-run on the five write commands
// (deploy, update, workflows, agents, hooks): a run that reviewed a plan prints the dry-run
// report instead of its outcome line, never says "deployment complete", needs no confirmation
// flag, and leaves the JSON document exactly as it was.

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// dryRunCommand is one write command with the flags it needs and the summary mode it returns.
type dryRunCommand struct {
	name string
	args []string
	mode domain.RunMode
}

func dryRunCommands(ws string) []dryRunCommand {
	base := func(name string) []string {
		return []string{name, "--harness", "stub-harness", "--workspace", ws, "--dry-run"}
	}
	return []dryRunCommand{
		{"deploy", base("deploy"), domain.ModeDeployWorkspace},
		{"update", base("update"), domain.ModeUpdateWorkspace},
		{"workflows", base("workflows"), domain.ModeUpdateWorkflows},
		{"agents", base("agents"), domain.ModeDeployAgents},
		{"hooks", base("hooks"), domain.ModeDeployHooks},
	}
}

// dryRunPlan is a plan with every kind of line the report prints.
func dryRunPlan(ws string, mode domain.RunMode, withRemovalsAndReplacements bool) *domain.Plan {
	p := &domain.Plan{
		Mode:          mode,
		WorkspacePath: ws,
		Items: []domain.PlanItem{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "alpha"}, TargetPath: ".github/agents/alpha.agent.md",
				Action: domain.ActionCreate, Reason: "new artifact"},
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "beta"}, TargetPath: ".github/agents/beta.agent.md",
				Action: domain.ActionUpdate, Reason: "renamed from .github/agents/beta.md"},
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "gamma"}, TargetPath: ".github/agents/gamma.agent.md",
				Action: domain.ActionUnchanged, Reason: "up to date"},
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "delta"}, TargetPath: ".github/agents/delta.agent.md",
				Action: domain.ActionConflict, Reason: "locally modified"},
		},
	}
	if withRemovalsAndReplacements {
		p.Removals = []domain.Removal{
			{Path: ".github/agents/beta.md", DropEntry: true, Reason: "legacy name replaced by beta.agent.md"},
			{Path: ".github/agents/old.md", Reason: "superseded"},
		}
		p.Replacements = []domain.Replacement{
			{Path: ".github/agents/alpha.agent.md", Reason: "existing file is backed up and overwritten"},
		}
	}
	return p
}

func dryRunSummary(ws string, mode domain.RunMode, plan *domain.Plan) domain.RunSummary {
	s := successSummary(ws)
	s.Mode = mode
	s.DryRunPlan = plan
	return s
}

// runDryRun runs one command against a service that returns summary for every write method.
func runDryRun(t *testing.T, c dryRunCommand, summary domain.RunSummary, extra ...string) (string, string, int, *spyService) {
	t.Helper()
	svc := &spyService{deployResp: summary, updateResp: summary, workflowsResp: summary,
		deployAgentsResp: summary, deployHooksResp: summary}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), append(append([]string{}, c.args...), extra...), svc, &out, &errOut)
	return out.String(), errOut.String(), code, svc
}

// TestDryRunCommands_WithoutConfirmationFlag_PrintPlanReport verifies the report of every
// write command: dry-run statement first, the workspace line, and one line per plan item with
// action, target path and reason, in plan order.
func TestDryRunCommands_WithoutConfirmationFlag_PrintPlanReport(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			plan := dryRunPlan(ws, c.mode, false)

			out, errOut, code, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

			if code != cli.ExitSuccess || errOut != "" {
				t.Fatalf("exit code = %d, stderr = %q; want success without error output", code, errOut)
			}
			want := "dry run: nothing was written\n" +
				"workspace: " + ws + "\n" +
				"plan:\n" +
				"  create .github/agents/alpha.agent.md - new artifact\n" +
				"  update .github/agents/beta.agent.md - renamed from .github/agents/beta.md\n" +
				"  unchanged .github/agents/gamma.agent.md - up to date\n" +
				"  locally-modified .github/agents/delta.agent.md - locally modified\n"
			if !strings.HasPrefix(out, want) {
				t.Errorf("report does not start with the expected text.\nwant prefix:\n%s\ngot:\n%s", want, out)
			}
			if strings.Contains(out, "removals:") || strings.Contains(out, "replacements:") {
				t.Errorf("removals or replacements section printed for a plan without any:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_NeverPrintDeploymentComplete verifies no dry-run output of any outcome
// contains the completion wording of a real run.
func TestDryRunCommands_NeverPrintDeploymentComplete(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		for _, outcome := range []domain.Outcome{domain.OutcomeSuccess, domain.OutcomeCompletedWithGaps, domain.OutcomeFailed} {
			t.Run(c.name+"/"+string(outcome), func(t *testing.T) {
				s := dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, true))
				s.Outcome = outcome

				out, _, _, _ := runDryRun(t, c, s)

				if strings.Contains(out, "deployment complete") {
					t.Errorf("dry-run output says \"deployment complete\":\n%s", out)
				}
				if !strings.Contains(out, "dry run: nothing was written") {
					t.Errorf("dry-run output lacks the dry-run statement:\n%s", out)
				}
			})
		}
	}
}

// TestDryRunCommands_ListRemovalsAndReplacements verifies the removals and replacements
// sections follow the plan section, one line per entry with the reason.
func TestDryRunCommands_ListRemovalsAndReplacements(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, true)))

			want := "removals:\n" +
				"  remove .github/agents/beta.md - legacy name replaced by beta.agent.md\n" +
				"  remove .github/agents/old.md - superseded\n" +
				"replacements:\n" +
				"  replace .github/agents/alpha.agent.md - existing file is backed up and overwritten\n"
			if !strings.Contains(out, want) {
				t.Errorf("removals/replacements sections missing or malformed.\nwant:\n%s\ngot:\n%s", want, out)
			}
			if plan, removals := strings.Index(out, "plan:"), strings.Index(out, "removals:"); plan < 0 || removals < plan {
				t.Errorf("removals must follow the plan section:\n%s", out)
			}
			if rem, rep := strings.Index(out, "removals:"), strings.Index(out, "replacements:"); rep < rem {
				t.Errorf("replacements must follow the removals section:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_OnlyReplacements_PrintsNoRemovalsSection verifies each of the two
// sections is printed on its own condition.
func TestDryRunCommands_OnlyReplacements_PrintsNoRemovalsSection(t *testing.T) {
	ws := "/ws/dry-run"
	c := dryRunCommands(ws)[0]
	plan := dryRunPlan(ws, c.mode, true)
	plan.Removals = nil

	out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

	if strings.Contains(out, "removals:") || !strings.Contains(out, "replacements:\n  replace ") {
		t.Errorf("want a replacements section and no removals section:\n%s", out)
	}
}

// TestDryRunCommands_ItemWithoutReason_PrintsNoSeparator verifies the " - <reason>" part is
// left out when an item has no reason.
func TestDryRunCommands_ItemWithoutReason_PrintsNoSeparator(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			plan := &domain.Plan{Mode: c.mode, WorkspacePath: ws, Items: []domain.PlanItem{
				{TargetPath: "a.md", Action: domain.ActionCreate},
			}}

			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

			if !strings.Contains(out, "\n  create a.md\n") {
				t.Errorf("an item without reason must print as \"  create a.md\" alone:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_EmptyPlan_StillStatesDryRun verifies a plan without items prints the
// statement, the workspace and the plan heading, and exits through the outcome.
func TestDryRunCommands_EmptyPlan_StillStatesDryRun(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			plan := &domain.Plan{Mode: c.mode, WorkspacePath: ws}

			out, _, code, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
			}
			if !strings.HasPrefix(out, "dry run: nothing was written\nworkspace: "+ws+"\nplan:\n") {
				t.Errorf("empty-plan report malformed:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_ExitCodeStillFollowsOutcome verifies the report replaces the outcome
// line but not the exit code.
func TestDryRunCommands_ExitCodeStillFollowsOutcome(t *testing.T) {
	ws := "/ws/dry-run"
	cases := map[domain.Outcome]int{
		domain.OutcomeSuccess:         cli.ExitSuccess,
		domain.OutcomeCompletedWithGaps: cli.ExitWithSkips,
		domain.OutcomeFailed:          cli.ExitFailure,
	}
	for _, c := range dryRunCommands(ws) {
		for outcome, want := range cases {
			t.Run(c.name+"/"+string(outcome), func(t *testing.T) {
				s := dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, false))
				s.Outcome = outcome

				_, _, code, _ := runDryRun(t, c, s)

				if code != want {
					t.Errorf("exit code = %d, want %d", code, want)
				}
			})
		}
	}
}

// TestDryRunCommands_RequestCarriesDryRun verifies --dry-run alone (no confirmation flag) is
// accepted and reaches the request.
func TestDryRunCommands_RequestCarriesDryRun(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, svc := runDryRun(t, c, dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, false)))

			dry := false
			switch c.name {
			case "deploy":
				dry = svc.deployReq.DryRun
			case "update":
				dry = svc.updateReq.DryRun
			case "workflows":
				dry = svc.workflowsReq.DryRun
			case "agents":
				dry = svc.deployAgentsReq.DryRun
			case "hooks":
				dry = svc.deployHooksReq.DryRun
			}
			if !dry {
				t.Error("DryRun is not set on the request")
			}
		})
	}
}

// TestDryRunCommands_ReportReplacesTheOutcomeLine verifies the dry-run statement appears
// exactly once and the per-command outcome wording (including the former "dry run: " prefixed
// agents and hooks lines) is not printed beside it.
func TestDryRunCommands_ReportReplacesTheOutcomeLine(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, false)))

			if n := strings.Count(out, "dry run:"); n != 1 {
				t.Errorf("\"dry run:\" appears %d times, want exactly once:\n%s", n, out)
			}
			if n := strings.Count(out, "workspace: "); n != 1 {
				t.Errorf("workspace line appears %d times, want once:\n%s", n, out)
			}
			if strings.Contains(out, "success:") || strings.Contains(out, "deployment complete") {
				t.Errorf("outcome wording printed in a dry run:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_WithoutPlan_KeepsOutcomeOutput verifies a summary without DryRunPlan is
// rendered as before, even when the flag was given (for example a service error path).
func TestDryRunCommands_WithoutPlan_KeepsOutcomeOutput(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, nil))

			if strings.Contains(out, "plan:") || strings.Contains(out, "nothing was written") {
				t.Errorf("plan report printed without a plan:\n%s", out)
			}
		})
	}
}

// TestDryRunCommands_JsonOutputIsUnchanged verifies the JSON document is byte-identical with
// and without a plan on the summary, and holds no plan key.
func TestDryRunCommands_JsonOutputIsUnchanged(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			with := dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, true))
			without := dryRunSummary(ws, c.mode, nil)

			outWith, _, _, _ := runDryRun(t, c, with, "--output", "json")
			outWithout, _, _, _ := runDryRun(t, c, without, "--output", "json")

			if outWith != outWithout {
				t.Errorf("JSON changed by the dry-run plan:\n%s\nvs\n%s", outWith, outWithout)
			}
			if strings.Contains(strings.ToLower(outWith), "dryrunplan") || strings.Contains(outWith, "dry run:") {
				t.Errorf("JSON carries the dry-run plan or report text:\n%s", outWith)
			}
		})
	}
}

// TestDeployDryRunAutoConfirmStrictJson_KeepsDocumentFields verifies the invocation the
// agent-test tool decodes keeps its top-level JSON fields.
func TestDeployDryRunAutoConfirmStrictJson_KeepsDocumentFields(t *testing.T) {
	ws := "/ws/dry-run"
	summary := dryRunSummary(ws, domain.ModeDeployWorkspace, dryRunPlan(ws, domain.ModeDeployWorkspace, true))
	svc := &spyService{deployResp: summary}
	var out bytes.Buffer

	code := cli.Run(context.Background(), []string{"deploy", "--harness", "stub-harness", "--workspace", ws,
		"--auto-confirm", "--dry-run", "--strict-models", "--output", "json"}, svc, &out, &bytes.Buffer{})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out.String())
	}
	var keys []string
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"Actions", "ConflictReport", "DeploymentRoot", "External", "Fallback", "Harness", "LogDegraded",
		"Logs", "Mode", "Outcome", "StackInEffect", "TodoFilePath", "Todos", "WorkspacePath"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("JSON top-level fields = %v, want %v", keys, want)
	}
}
