package cli_test

// output_experimental_paths_test.go continues output_experimental_test.go: JSON mode stays one
// clean document that carries the experimental field, output of non-experimental runs is
// unchanged, render prints no experimental warning, and results rendered on failed, cancelled or
// declined paths show the warning once while error paths that render no result stay as they were.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

func TestExperimentalJSON_SingleDocumentWithFieldAndNoWarningLine(t *testing.T) {
	for _, c := range writingCommands() {
		for _, kind := range []resultKind{kindNormal, kindDryRun} {
			t.Run(c.name, func(t *testing.T) {
				svc := &spyService{}
				c.arm(svc, []domain.ExperimentalHarness{expCodex, expMultiline}, kind)
				var extra []string
				if kind == kindDryRun {
					extra = []string{"--dry-run"}
				}

				out, _, _ := runCmd(t, c, svc, "json", extra...)

				dec := json.NewDecoder(strings.NewReader(out))
				var doc map[string]any
				if err := dec.Decode(&doc); err != nil {
					t.Fatalf("output is not a JSON document: %v\n%s", err, out)
				}
				if dec.More() || strings.Count(strings.TrimSuffix(out, "\n"), "\n") > 0 {
					t.Errorf("JSON mode wrote more than one document or extra lines:\n%s", out)
				}
				entries, _ := doc[c.jsonKey].([]any)
				if len(entries) != 2 {
					t.Errorf("%s has %d entries, want 2; document: %v", c.jsonKey, len(entries), doc)
				}
				if strings.Contains(out, "warning:") {
					t.Errorf("JSON mode printed a warning line:\n%s", out)
				}
			})
		}
	}
}

func TestExperimentalJSON_NonExperimentalDocumentHasNoField(t *testing.T) {
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, nil, kindNormal)

			out, _, _ := runCmd(t, c, svc, "json")

			var doc map[string]any
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("output is not a JSON document: %v\n%s", err, out)
			}
			if _, present := doc[c.jsonKey]; present {
				t.Errorf("%s present for a non-experimental run: %s", c.jsonKey, out)
			}
		})
	}
}

// The expected strings are the output of each command before experimental support existed.
func TestExperimentalHuman_NonExperimentalOutputIsUnchanged(t *testing.T) {
	want := map[string]string{
		"deploy":    "success: deployment complete\nworkspace: /ws/experimental\n",
		"update":    "success: deployment complete\nworkspace: /ws/experimental\n",
		"workflows": "success: deployment complete\nworkspace: /ws/experimental\n",
		"agents":    "agents deployment complete\nworkspace: /ws/experimental\n",
		"hooks":     "hooks deployment complete\nworkspace: /ws/experimental\n",
		"promote":   "promoted: /path/to/agent.md\ndestination: /root/Subagents/agent.md\nkey: agent\nid: 42\n",
	}
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, nil, kindNormal)

			out, _, _ := runCmd(t, c, svc, "")

			if c.name == "transform" {
				if !strings.HasPrefix(out, "transform plan\nsource: src -> tgt\nworkspace: /ws/experimental\n") ||
					strings.Contains(out, "warning:") {
					t.Errorf("transform output changed for a non-experimental run:\n%s", out)
				}
				return
			}
			if out != want[c.name] {
				t.Errorf("output = %q, want %q", out, want[c.name])
			}
		})
	}
}

func TestExperimentalRender_PrintsNoWarningInEitherMode(t *testing.T) {
	refs := []domain.HarnessRef{{ID: "stub-harness", DisplayName: "Stub", ExperimentalReason: "not finished"}}
	for _, format := range []string{"", "json"} {
		t.Run("format_"+format, func(t *testing.T) {
			svc := &spyService{
				harnessRefs:     refs,
				renderAgentResp: app.RenderAgentResult{DestinationPath: "/out/agent.md"},
			}
			args := minRenderArgs("stub-harness", "/agents/agent.md", "/out/agent.md")
			if format != "" {
				args = append(args, "--output", format)
			}
			var out, errOut bytes.Buffer

			cli.Run(context.Background(), args, svc, &out, &errOut)

			if svc.renderAgentReq == nil {
				t.Fatal("render did not reach the service")
			}
			all := strings.ToLower(out.String() + errOut.String())
			if strings.Contains(all, "experimental") || strings.Contains(all, "warning:") {
				t.Errorf("render printed an experimental warning:\n%s", all)
			}
		})
	}
}

func TestExperimentalWarning_FailedOutcomeSummaryPrintsOnce(t *testing.T) {
	for _, c := range writingCommands()[:5] {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, []domain.ExperimentalHarness{expCodex}, kindFailed)

			out, _, code := runCmd(t, c, svc, "")

			if code != cli.ExitFailure {
				t.Errorf("exit code = %d, want ExitFailure (%d)", code, cli.ExitFailure)
			}
			assertWarnedOnceFirst(t, out, expCodex)
		})
	}
}

// transformErrorCases are runs the transform command renders as a result next to an error.
func transformErrorCases() map[string]struct {
	result app.TransformHarnessResult
	err    error
} {
	cancelled := app.TransformHarnessResult{
		SourceHarnessID: "src", Cancelled: true,
		ExperimentalHarnesses: []domain.ExperimentalHarness{expCodex},
	}
	return map[string]struct {
		result app.TransformHarnessResult
		err    error
	}{
		"declined_plan":       {transformResultFor([]domain.ExperimentalHarness{expCodex}, false), app.ErrPlanNotConfirmed},
		"cancelled_selection": {cancelled, app.ErrSelectionCancelled},
		"reverted_run":        {transformResultFor([]domain.ExperimentalHarness{expCodex}, false), app.ErrRunReverted},
	}
}

func TestExperimentalWarning_TransformErrorPathWithResultPrintsOnce(t *testing.T) {
	c := writingCommands()[6]
	for name, tc := range transformErrorCases() {
		t.Run(name, func(t *testing.T) {
			svc := &spyService{transformResp: tc.result, transformErr: tc.err}

			out, errOut, code := runCmd(t, c, svc, "")

			if code != cli.ExitFailure {
				t.Errorf("exit code = %d, want ExitFailure (%d)", code, cli.ExitFailure)
			}
			assertWarnedOnceFirst(t, out, expCodex)
			if !strings.Contains(errOut, tc.err.Error()) || strings.Contains(errOut, "is experimental") {
				t.Errorf("stderr must carry the error and no warning; got %q", errOut)
			}
		})
	}
}

func TestExperimentalJSON_TransformErrorPathEncodesField(t *testing.T) {
	c := writingCommands()[6]
	for name, tc := range transformErrorCases() {
		t.Run(name, func(t *testing.T) {
			svc := &spyService{transformResp: tc.result, transformErr: tc.err}

			out, _, _ := runCmd(t, c, svc, "json")

			var doc map[string]any
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
			}
			if entries, _ := doc["experimentalHarnesses"].([]any); len(entries) != 1 {
				t.Errorf("experimentalHarnesses = %v, want one entry", doc["experimentalHarnesses"])
			}
		})
	}
}

// Error paths that render no result keep their output exactly: the summary or result is not
// rendered, so a warning must not appear even if the value happens to carry the field.
func TestExperimentalWarning_ErrorPathsWithoutResultAreUnchanged(t *testing.T) {
	exp := []domain.ExperimentalHarness{expCodex}
	for _, c := range writingCommands() {
		t.Run(c.name, func(t *testing.T) {
			svc := &spyService{}
			c.arm(svc, exp, kindNormal)
			failWith := errors.New("service exploded")
			svc.deployErr, svc.updateErr, svc.workflowsErr = failWith, failWith, failWith
			svc.deployAgentsErr, svc.deployHooksErr, svc.promoteErr = failWith, failWith, failWith
			svc.transformErr = failWith
			if c.name == "transform" {
				svc.transformResp = app.TransformHarnessResult{ExperimentalHarnesses: exp}
			}

			out, errOut, code := runCmd(t, c, svc, "")

			if code != cli.ExitFailure {
				t.Errorf("exit code = %d, want ExitFailure (%d)", code, cli.ExitFailure)
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty (no result is rendered)", out)
			}
			if strings.Contains(errOut, "is experimental") || !strings.Contains(errOut, "service exploded") {
				t.Errorf("stderr must be the error alone; got %q", errOut)
			}
		})
	}
}

func TestExperimentalWarning_DeclinedPlanWithEmptySummaryIsUnchanged(t *testing.T) {
	c := writingCommands()[0]
	svc := &spyService{deployResp: domain.RunSummary{}, deployErr: app.ErrPlanNotConfirmed}

	out, errOut, code := runCmd(t, c, svc, "")

	if code != cli.ExitFailure || out != "" {
		t.Errorf("code = %d, stdout = %q; want ExitFailure and empty stdout", code, out)
	}
	if !strings.Contains(errOut, app.ErrPlanNotConfirmed.Error()) || strings.Contains(errOut, "experimental") {
		t.Errorf("stderr = %q, want the plain error", errOut)
	}
}
