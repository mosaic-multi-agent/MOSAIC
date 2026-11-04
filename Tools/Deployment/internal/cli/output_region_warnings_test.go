package cli_test

// output_region_warnings_test.go specifies the console output of deploy and update for the two
// warnings that used to reach only the TODO file: orchestrator content that was not delivered,
// and a role-forbidden region. The human output prints each in full, one line per warning, after
// the outcome and workspace lines; the outcome line and the exit code stay as they are, and the
// JSON document keeps its shape.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

const (
	undeliveredDetail = "orchestrator source has no OrchestratorHarnessConstraints marker; the harness orchestrator content was not deployed"
	roleForbiddenText = "agent test-runner (subagent) [role-forbidden-region]: region OrchestratorHarnessConstraints is not allowed for this role"
)

// regionWarningTodos returns TODO items holding both warnings plus unrelated items that must
// never be printed as warnings.
func regionWarningTodos() []domain.TodoItem {
	return []domain.TodoItem{
		{Category: domain.TodoModels, Subject: "test-runner", Detail: "no model resolved", Kind: domain.GapNoModel},
		{Category: domain.TodoManual, Subject: "orchestrator", Owner: "orchestrator", Detail: undeliveredDetail,
			Kind: domain.GapUndeliveredOrchestratorInjection},
		{Category: domain.TodoManual, Subject: "OrchestratorHarnessConstraints", Owner: "test-runner", Detail: roleForbiddenText,
			Kind: domain.GapRoleForbiddenRegion},
		{Category: domain.TodoManual, Subject: "manual", Detail: "a plain manual step"},
	}
}

// warningLines returns the output lines that start with "warning: ".
func warningLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "warning: ") {
			lines = append(lines, l)
		}
	}
	return lines
}

// runRegionCommand runs deploy or update (human output) against a service that returns summary.
func runRegionCommand(t *testing.T, command string, summary domain.RunSummary, extra ...string) (string, int) {
	t.Helper()
	svc := &spyService{deployResp: summary, updateResp: summary}
	args := append([]string{command, "--harness", "stub-harness", "--workspace", summary.WorkspacePath, "--auto-confirm"}, extra...)
	var out bytes.Buffer
	code := cli.Run(context.Background(), args, svc, &out, &bytes.Buffer{})
	return out.String(), code
}

func regionSummary(command string, outcome domain.Outcome) domain.RunSummary {
	s := successSummary("/ws/region-warnings")
	if command == "update" {
		s.Mode = domain.ModeUpdateWorkspace
	}
	s.Outcome = outcome
	s.Todos = regionWarningTodos()
	return s
}

func TestHumanOutput_PrintsBothRegionWarningsInFull(t *testing.T) {
	for _, command := range []string{"deploy", "update"} {
		t.Run(command, func(t *testing.T) {
			out, _ := runRegionCommand(t, command, regionSummary(command, domain.OutcomeSuccess))

			lines := warningLines(out)
			if len(lines) != 2 {
				t.Fatalf("warning lines = %d, want 2 (one per warning); output:\n%s", len(lines), out)
			}
			if !strings.Contains(out, "warning: "+undeliveredDetail+"\n") {
				t.Errorf("undelivered-orchestrator-content warning not printed in full; output:\n%s", out)
			}
			if !strings.Contains(out, "warning: "+roleForbiddenText+"\n") {
				t.Errorf("role-forbidden-region warning not printed in full; output:\n%s", out)
			}
		})
	}
}

func TestHumanOutput_RegionWarningsFollowOutcomeAndWorkspaceLines(t *testing.T) {
	for _, command := range []string{"deploy", "update"} {
		t.Run(command, func(t *testing.T) {
			out, _ := runRegionCommand(t, command, regionSummary(command, domain.OutcomeSuccess))

			ws := strings.Index(out, "workspace: /ws/region-warnings")
			first := strings.Index(out, "warning: ")
			if ws < 0 || first < 0 || first < ws {
				t.Errorf("warnings must follow the workspace line; output:\n%s", out)
			}
			if !strings.HasPrefix(out, "success: deployment complete\n") {
				t.Errorf("outcome line changed; output:\n%s", out)
			}
		})
	}
}

func TestHumanOutput_RegionWarningsKeepSummaryOrder(t *testing.T) {
	out, _ := runRegionCommand(t, "deploy", regionSummary("deploy", domain.OutcomeSuccess))

	lines := warningLines(out)
	if len(lines) != 2 || !strings.Contains(lines[0], "OrchestratorHarnessConstraints marker") ||
		!strings.Contains(lines[1], "role-forbidden-region") {
		t.Errorf("warnings not in summary order: %q", lines)
	}
}

func TestHumanOutput_RegionWarnings_OutcomeAndExitCodeUnchanged(t *testing.T) {
	cases := []struct {
		outcome  domain.Outcome
		wantCode int
	}{
		{domain.OutcomeSuccess, cli.ExitSuccess},
		{domain.OutcomeCompletedWithGaps, cli.ExitWithSkips},
		{domain.OutcomeFailed, cli.ExitFailure},
	}
	for _, command := range []string{"deploy", "update"} {
		for _, c := range cases {
			t.Run(command+"/"+string(c.outcome), func(t *testing.T) {
				with := regionSummary(command, c.outcome)
				without := with
				without.Todos = nil

				outWith, codeWith := runRegionCommand(t, command, with)
				outWithout, codeWithout := runRegionCommand(t, command, without)

				if codeWith != c.wantCode || codeWithout != c.wantCode {
					t.Errorf("exit codes = %d (with warnings), %d (without), want %d", codeWith, codeWithout, c.wantCode)
				}
				if len(warningLines(outWith)) != 2 {
					t.Errorf("warnings missing for outcome %s; output:\n%s", c.outcome, outWith)
				}
				if !strings.HasPrefix(outWith, strings.TrimSuffix(outWithout, "\n")) {
					t.Errorf("output without warnings is not a prefix of the output with warnings:\n%q\nvs\n%q", outWithout, outWith)
				}
			})
		}
	}
}

func TestHumanOutput_OtherTodoItemsAreNotPrintedAsWarnings(t *testing.T) {
	summary := regionWarningTodos()[0:1]
	summary = append(summary, regionWarningTodos()[3])
	s := regionSummary("deploy", domain.OutcomeCompletedWithGaps)
	s.Todos = summary

	out, _ := runRegionCommand(t, "deploy", s)

	if lines := warningLines(out); len(lines) != 0 {
		t.Errorf("unrelated TODO items printed as warnings: %q", lines)
	}
}

func TestHumanOutput_NoRegionWarningsForOtherRunModes(t *testing.T) {
	s := regionSummary("deploy", domain.OutcomeSuccess)
	s.Mode = domain.ModeDeployAgents

	out, _ := runRegionCommand(t, "deploy", s)

	if lines := warningLines(out); len(lines) != 0 {
		t.Errorf("region warnings printed for a run mode other than deploy and update: %q", lines)
	}
}

func TestHumanOutput_EachWarningItemPrintsOneLine(t *testing.T) {
	// A further role-forbidden item for another agent adds exactly one more line.
	s := regionSummary("update", domain.OutcomeSuccess)
	s.Todos = append(s.Todos, domain.TodoItem{
		Category: domain.TodoManual, Subject: "OrchestratorHarnessConstraints", Owner: "other-agent",
		Detail: "agent other-agent (subagent) [role-forbidden-region]: region not allowed", Kind: domain.GapRoleForbiddenRegion,
	})

	out, _ := runRegionCommand(t, "update", s)

	if lines := warningLines(out); len(lines) != 3 {
		t.Errorf("warning lines = %d, want 3 (one per item); output:\n%s", len(lines), out)
	}
}

func TestJsonOutput_WithRegionWarnings_KeepsExistingFields(t *testing.T) {
	for _, command := range []string{"deploy", "update"} {
		t.Run(command, func(t *testing.T) {
			with := regionSummary(command, domain.OutcomeSuccess)
			without := with
			without.Todos = make([]domain.TodoItem, len(with.Todos))
			for i, it := range with.Todos {
				it.Kind = ""
				without.Todos[i] = it
			}

			outWith, _ := runRegionCommand(t, command, with, "--output", "json")
			outWithout, _ := runRegionCommand(t, command, without, "--output", "json")

			if outWith != outWithout {
				t.Errorf("JSON changed when the region kinds are present:\n%s\nvs\n%s", outWith, outWithout)
			}
			if strings.Contains(outWith, "warning: ") || strings.Contains(strings.ToLower(outWith), `"kind"`) {
				t.Errorf("JSON carries console warning text or a kind key:\n%s", outWith)
			}
			var doc struct {
				Todos []map[string]any `json:"Todos"`
			}
			if err := json.Unmarshal([]byte(outWith), &doc); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			for _, item := range doc.Todos {
				for _, key := range []string{"Category", "Subject", "Owner", "Detail", "Fragment"} {
					if _, ok := item[key]; !ok {
						t.Errorf("todo item lost field %q: %v", key, item)
					}
				}
				if len(item) != 5 {
					t.Errorf("todo item has %d keys, want the 5 existing ones: %v", len(item), item)
				}
			}
		})
	}
}
