package cli_test

// output_experimental_test.go specifies the human output of every writing command (deploy,
// update, workflows, agents, hooks, promote, transform) for a run that involves an experimental
// harness: the result carries the experimental harnesses and the renderer prints one
// "warning: <name> is experimental: <reason>" line per entry, first in the result's output,
// dry runs included. The CLI reads the result data only; it never looks at a harness id.
// The JSON, unchanged-output, render and failure-path expectations live in
// output_experimental_paths_test.go.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

var (
	expCodex = domain.ExperimentalHarness{
		HarnessID:   "exp-one",
		DisplayName: "Experimental One",
		Reason:      "agents deploy but there is no orchestration",
	}
	expMultiline = domain.ExperimentalHarness{
		HarnessID:   "exp-two",
		DisplayName: "Experimental Two",
		Reason:      "first line\n\tsecond   line",
	}
)

// resultKind selects the shape of the result the stub service returns.
type resultKind int

const (
	kindNormal resultKind = iota
	kindDryRun
	kindFailed
)

// writingCmd is one writing command: its minimal arguments and how to load the stub service
// with a result of a given kind that carries the given experimental harnesses.
type writingCmd struct {
	name    string
	args    []string
	jsonKey string
	arm     func(svc *spyService, exp []domain.ExperimentalHarness, kind resultKind)
}

// runSummaryFor builds the summary the RunSummary-based commands return for kind.
func runSummaryFor(mode domain.RunMode, exp []domain.ExperimentalHarness, kind resultKind) domain.RunSummary {
	s := successSummary("/ws/experimental")
	s.Mode = mode
	s.ExperimentalHarnesses = exp
	switch kind {
	case kindDryRun:
		s.DryRunPlan = &domain.Plan{Mode: mode, WorkspacePath: s.WorkspacePath}
	case kindFailed:
		s.Outcome = domain.OutcomeFailed
	}
	return s
}

func summaryCmd(name string, mode domain.RunMode, set func(*spyService, domain.RunSummary)) writingCmd {
	confirm := "--auto-confirm"
	if name == "agents" || name == "hooks" {
		confirm = "--yes"
	}
	return writingCmd{
		name:    name,
		args:    []string{name, "--harness", "stub-harness", "--workspace", "/ws/experimental", confirm},
		jsonKey: "ExperimentalHarnesses",
		arm: func(svc *spyService, exp []domain.ExperimentalHarness, kind resultKind) {
			set(svc, runSummaryFor(mode, exp, kind))
		},
	}
}

// writingCommands returns one case per writing command.
func writingCommands() []writingCmd {
	return []writingCmd{
		summaryCmd("deploy", domain.ModeDeployWorkspace, func(s *spyService, r domain.RunSummary) { s.deployResp = r }),
		summaryCmd("update", domain.ModeUpdateWorkspace, func(s *spyService, r domain.RunSummary) { s.updateResp = r }),
		summaryCmd("workflows", domain.ModeUpdateWorkflows, func(s *spyService, r domain.RunSummary) { s.workflowsResp = r }),
		summaryCmd("agents", domain.ModeDeployWorkspace, func(s *spyService, r domain.RunSummary) { s.deployAgentsResp = r }),
		summaryCmd("hooks", domain.ModeDeployWorkspace, func(s *spyService, r domain.RunSummary) { s.deployHooksResp = r }),
		{
			name:    "promote",
			args:    []string{"promote", "--file", "/path/to/agent.md", "--harness", "stub-harness"},
			jsonKey: "experimentalHarnesses",
			arm: func(svc *spyService, exp []domain.ExperimentalHarness, kind resultKind) {
				svc.promoteResp = app.PromoteResult{
					SourcePath: "/path/to/agent.md", DestinationPath: "/root/Subagents/agent.md",
					Key: "agent", NumericID: "42", Category: "Test",
					DryRun: kind == kindDryRun, ExperimentalHarnesses: exp,
				}
			},
		},
		{
			name:    "transform",
			args:    []string{"transform", "--harness", "src", "--target-harness", "tgt", "--workspace", "/ws/experimental"},
			jsonKey: "experimentalHarnesses",
			arm: func(svc *spyService, exp []domain.ExperimentalHarness, kind resultKind) {
				svc.transformResp = transformResultFor(exp, kind == kindDryRun)
			},
		},
	}
}

// transformResultFor builds a transform result with one transformed file.
func transformResultFor(exp []domain.ExperimentalHarness, dryRun bool) app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID: "src", TargetHarnessID: "tgt", Workspace: "/ws/experimental",
		DryRun: dryRun, Transformed: 1,
		Files: []app.TransformFileOutcome{{
			SourcePath: "/ws/experimental/a.md", DestinationPath: "/ws/experimental/a.tgt.md",
			Status: app.StatusTransformed,
		}},
		ExperimentalHarnesses: exp,
	}
}

// runCmd runs the command with the given output format and optional extra arguments.
func runCmd(t *testing.T, c writingCmd, svc *spyService, format string, extra ...string) (out, errOut string, code int) {
	t.Helper()
	args := append([]string{}, c.args...)
	args = append(args, extra...)
	if format != "" {
		args = append(args, "--output", format)
	}
	var o, e bytes.Buffer
	code = cli.Run(context.Background(), args, svc, &o, &e)
	return o.String(), e.String(), code
}

// experimentalLines returns the output lines that announce an experimental harness.
func experimentalLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, " is experimental:") {
			lines = append(lines, l)
		}
	}
	return lines
}

// assertWarnedOnceFirst checks that out holds exactly the warning line of each entry of exp,
// in order, as its first lines.
func assertWarnedOnceFirst(t *testing.T, out string, exp ...domain.ExperimentalHarness) {
	t.Helper()
	lines := experimentalLines(out)
	if len(lines) != len(exp) {
		t.Fatalf("experimental warning lines = %d, want %d; output:\n%s", len(lines), len(exp), out)
	}
	var want strings.Builder
	for i, e := range exp {
		if lines[i] != "warning: "+e.Message() {
			t.Errorf("warning line %d = %q, want %q", i, lines[i], "warning: "+e.Message())
		}
		want.WriteString("warning: " + e.Message() + "\n")
	}
	if !strings.HasPrefix(out, want.String()) {
		t.Errorf("warning lines are not the first lines of the output:\n%s", out)
	}
}

func TestExperimentalWarning_HumanOutputPrintsReasonOnceFirst(t *testing.T) {
	for _, c := range writingCommands() {
		for _, kind := range []resultKind{kindNormal, kindDryRun} {
			name := c.name
			if kind == kindDryRun {
				name += "_dry_run"
			}
			t.Run(name, func(t *testing.T) {
				svc := &spyService{}
				c.arm(svc, []domain.ExperimentalHarness{expCodex}, kind)
				var extra []string
				if kind == kindDryRun {
					extra = []string{"--dry-run"}
				}

				out, _, _ := runCmd(t, c, svc, "", extra...)

				assertWarnedOnceFirst(t, out, expCodex)
			})
		}
	}
}

func TestExperimentalWarning_MultiLineReasonStaysOneLine(t *testing.T) {
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, []domain.ExperimentalHarness{expMultiline}, kindNormal)

			out, _, _ := runCmd(t, c, svc, "")

			assertWarnedOnceFirst(t, out, expMultiline)
			if !strings.Contains(out, "warning: Experimental Two is experimental: first line second line\n") {
				t.Errorf("reason whitespace was not collapsed to one line; output:\n%s", out)
			}
		})
	}
}

func TestExperimentalWarning_OneLinePerHarnessInSliceOrder(t *testing.T) {
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, []domain.ExperimentalHarness{expCodex, expMultiline}, kindNormal)

			out, _, _ := runCmd(t, c, svc, "")

			assertWarnedOnceFirst(t, out, expCodex, expMultiline)
		})
	}
}

// A transform whose only experimental harness is the source still warns, once.
func TestExperimentalWarning_TransformSourceOnlyPrintsOnce(t *testing.T) {
	svc := &spyService{transformResp: transformResultFor([]domain.ExperimentalHarness{expCodex}, false)}
	svc.transformResp.SourceHarnessID = expCodex.HarnessID
	svc.transformResp.TargetHarnessID = "plain-target"
	c := writingCommands()[6]

	out, _, _ := runCmd(t, c, svc, "")

	assertWarnedOnceFirst(t, out, expCodex)
}

// The warning precedes the dry-run report, the promote summary and the transform plan.
func TestExperimentalWarning_PrecedesTheResultBody(t *testing.T) {
	markers := map[string]string{
		"deploy": "dry run: nothing was written", "update": "dry run: nothing was written",
		"workflows": "dry run: nothing was written", "agents": "dry run: nothing was written",
		"hooks": "dry run: nothing was written", "promote": "dry run: promote validated",
		"transform": "dry run: transform plan",
	}
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, []domain.ExperimentalHarness{expCodex}, kindDryRun)

			out, _, _ := runCmd(t, c, svc, "", "--dry-run")

			warnAt := strings.Index(out, "warning: ")
			bodyAt := strings.Index(out, markers[c.name])
			if warnAt < 0 || bodyAt < 0 || warnAt > bodyAt {
				t.Errorf("warning must come before %q; output:\n%s", markers[c.name], out)
			}
		})
	}
}
