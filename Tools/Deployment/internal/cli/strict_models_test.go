package cli_test

// strict_models_test.go covers the CLI surface of the deploy subcommand's --strict-models
// flag: flag acceptance and request wiring, the exit code and stderr line of the strict
// failure, the one-document DeployFailure envelope under --output json, and the unchanged
// behaviour of every other path (default deploys, other service errors, success outcomes).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestUnresolvedErr returns the strict failure the service reports for one agent.
func newTestUnresolvedErr() *app.UnresolvedModelsError {
	return &app.UnresolvedModelsError{
		Agents: []app.UnresolvedAgentModel{{AgentKey: "test-runner", Tier: "HIGH"}},
	}
}

// runDeploy runs the deploy subcommand with extra flags against svc and returns the exit
// code, stdout and stderr.
func runDeploy(t *testing.T, svc *spyService, extra ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	args := append(minDeployArgs("stub-harness", t.TempDir()), extra...)
	code := cli.Run(context.Background(), args, svc, &out, &errOut)
	return code, out.String(), errOut.String()
}

// ---------------------------------------------------------------------------
// Flag acceptance and request wiring
// ---------------------------------------------------------------------------

// TestDeployCmd_StrictModelsFlagPresent_SetsStrictModelsInRequest verifies the flag is
// accepted by deploy and carried into the deploy request.
func TestDeployCmd_StrictModelsFlagPresent_SetsStrictModelsInRequest(t *testing.T) {
	svc := &spyService{deployResp: successSummary("ws")}

	code, _, errOut := runDeploy(t, svc, "--strict-models")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, cli.ExitSuccess, errOut)
	}
	if svc.deployReq == nil || !svc.deployReq.StrictModels {
		t.Errorf("DeployRequest.StrictModels is not true after passing --strict-models; request = %+v", svc.deployReq)
	}
}

// TestDeployCmd_StrictModelsFlagAbsent_StrictModelsIsFalse verifies the default is off.
func TestDeployCmd_StrictModelsFlagAbsent_StrictModelsIsFalse(t *testing.T) {
	svc := &spyService{deployResp: successSummary("ws")}

	code, _, _ := runDeploy(t, svc)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	if svc.deployReq == nil || svc.deployReq.StrictModels {
		t.Errorf("DeployRequest.StrictModels must be false without the flag; request = %+v", svc.deployReq)
	}
}

// TestDeployCmd_StrictModelsWithDryRun_BothReachRequest verifies the flag combines with
// --dry-run.
func TestDeployCmd_StrictModelsWithDryRun_BothReachRequest(t *testing.T) {
	svc := &spyService{deployResp: successSummary("ws")}

	code, _, errOut := runDeploy(t, svc, "--strict-models", "--dry-run")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, cli.ExitSuccess, errOut)
	}
	if svc.deployReq == nil || !svc.deployReq.StrictModels || !svc.deployReq.DryRun {
		t.Errorf("request = %+v, want StrictModels and DryRun both true", svc.deployReq)
	}
}

// TestUpdateCmd_StrictModelsFlag_IsUsageError verifies the flag is accepted only by deploy.
func TestUpdateCmd_StrictModelsFlag_IsUsageError(t *testing.T) {
	svc := &spyService{}
	var out, errOut bytes.Buffer

	code := cli.Run(context.Background(),
		[]string{"update", "--harness", "stub-harness", "--workspace", t.TempDir(), "--strict-models"},
		svc, &out, &errOut)

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d (ExitUsage): --strict-models belongs to deploy only", code, cli.ExitUsage)
	}
}

// ---------------------------------------------------------------------------
// Strict failure: exit code, stderr, stdout
// ---------------------------------------------------------------------------

// TestDeployCmd_StrictFailure_Text_ExitsOneWithErrorLine verifies the text-mode failure:
// exit 1, one stderr line carrying the service message, nothing on stdout.
func TestDeployCmd_StrictFailure_Text_ExitsOneWithErrorLine(t *testing.T) {
	uerr := newTestUnresolvedErr()
	svc := &spyService{deployErr: uerr}

	code, out, errOut := runDeploy(t, svc, "--strict-models")

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d (ExitFailure)", code, cli.ExitFailure)
	}
	want := "error: " + uerr.Error() + "\n"
	if errOut != want {
		t.Errorf("stderr = %q, want exactly %q", errOut, want)
	}
	for _, frag := range []string{"test-runner", "HIGH"} {
		if !strings.Contains(errOut, frag) {
			t.Errorf("stderr %q does not name %q", errOut, frag)
		}
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty for a text-mode strict failure", out)
	}
}

// TestDeployCmd_StrictFailure_DryRun_ExitsOne verifies a dry run fails the same way.
func TestDeployCmd_StrictFailure_DryRun_ExitsOne(t *testing.T) {
	svc := &spyService{deployErr: newTestUnresolvedErr()}

	code, _, errOut := runDeploy(t, svc, "--strict-models", "--dry-run")

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d (ExitFailure)", code, cli.ExitFailure)
	}
	if !strings.Contains(errOut, "test-runner") || !strings.Contains(errOut, "HIGH") {
		t.Errorf("stderr %q must name the agent and its tier", errOut)
	}
}

// ---------------------------------------------------------------------------
// Strict failure: JSON envelope
// ---------------------------------------------------------------------------

// TestDeployCmd_StrictFailure_JSON_WritesOneDeployFailureDocument verifies stdout under
// --output json is exactly one DeployFailure document and no RunSummary.
func TestDeployCmd_StrictFailure_JSON_WritesOneDeployFailureDocument(t *testing.T) {
	uerr := newTestUnresolvedErr()
	svc := &spyService{deployErr: uerr}

	code, out, errOut := runDeploy(t, svc, "--strict-models", "--output", "json")

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d (ExitFailure)", code, cli.ExitFailure)
	}
	dec := json.NewDecoder(strings.NewReader(out))
	var env cli.DeployFailure
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("stdout is not a DeployFailure document: %v\nstdout: %q", err, out)
	}
	if dec.More() {
		t.Errorf("stdout holds more than one JSON document: %q", out)
	}
	if !env.Error {
		t.Error("envelope error = false, want true")
	}
	if env.Reason != "unresolved-models" {
		t.Errorf("envelope reason = %q, want \"unresolved-models\"", env.Reason)
	}
	if env.ExitCode != cli.ExitFailure {
		t.Errorf("envelope exitCode = %d, want %d", env.ExitCode, cli.ExitFailure)
	}
	if wantMsg := strings.TrimSuffix(strings.TrimPrefix(errOut, "error: "), "\n"); env.Message != wantMsg || env.Message == "" {
		t.Errorf("envelope message = %q, want the stderr text without \"error: \" (%q)", env.Message, wantMsg)
	}
	if !strings.Contains(env.Detail, "test-runner (tier HIGH)") {
		t.Errorf("envelope detail = %q, want it to contain \"test-runner (tier HIGH)\"", env.Detail)
	}
	if !strings.HasPrefix(errOut, "error: ") {
		t.Errorf("stderr = %q, want the \"error: \" line to be kept under --output json", errOut)
	}
}

// TestDeployCmd_StrictFailure_JSON_UsesWireKeys verifies the stable cross-tool key names.
func TestDeployCmd_StrictFailure_JSON_UsesWireKeys(t *testing.T) {
	svc := &spyService{deployErr: newTestUnresolvedErr()}

	_, out, _ := runDeploy(t, svc, "--strict-models", "--output", "json")

	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\nstdout: %q", err, out)
	}
	for _, key := range []string{"error", "reason", "message", "detail", "exitCode"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("envelope lacks key %q; got %v", key, raw)
		}
	}
	if _, ok := raw["Outcome"]; ok {
		t.Error("stdout carries a RunSummary field \"Outcome\"; a strict failure writes no RunSummary")
	}
}

// TestDeployCmd_StrictFailure_JSON_WrappedErrorStillProducesEnvelope verifies the mapping
// follows errors.Is, not a concrete type assertion on the top-level error.
func TestDeployCmd_StrictFailure_JSON_WrappedErrorStillProducesEnvelope(t *testing.T) {
	wrapped := fmt.Errorf("deploy: %w", newTestUnresolvedErr())
	if !errors.Is(wrapped, app.ErrUnresolvedModels) {
		t.Fatal("test setup: wrapped error does not match ErrUnresolvedModels")
	}
	svc := &spyService{deployErr: wrapped}

	code, out, _ := runDeploy(t, svc, "--strict-models", "--output", "json")

	var env cli.DeployFailure
	if err := json.Unmarshal([]byte(out), &env); err != nil || env.Reason != cli.DeployReasonUnresolvedModels {
		t.Errorf("stdout = %q (decode error %v), want a DeployFailure with reason unresolved-models", out, err)
	}
	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d", code, cli.ExitFailure)
	}
}

// ---------------------------------------------------------------------------
// Everything else is unchanged
// ---------------------------------------------------------------------------

// TestDeployCmd_OtherServiceError_JSON_WritesNoEnvelope verifies that only the strict
// failure writes the envelope: any other service error keeps writing nothing to stdout.
func TestDeployCmd_OtherServiceError_JSON_WritesNoEnvelope(t *testing.T) {
	svc := &spyService{deployErr: errors.New("planner error: unknown workflow")}

	code, out, errOut := runDeploy(t, svc, "--output", "json")

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d", code, cli.ExitFailure)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty for a non-strict service error", out)
	}
	if !strings.Contains(errOut, "planner error") {
		t.Errorf("stderr = %q, want the service error text", errOut)
	}
}

// TestDeployCmd_NoStrictFlag_UnresolvedModelOutcomeKeepsExitCode is the default-behaviour
// regression: a deploy that completed with a skipped (no-model) gap still exits 2 and
// writes its summary.
func TestDeployCmd_NoStrictFlag_UnresolvedModelOutcomeKeepsExitCode(t *testing.T) {
	svc := &spyService{deployResp: skippedSummary("ws")}

	code, out, _ := runDeploy(t, svc)

	if code != cli.ExitWithSkips {
		t.Errorf("exit code = %d, want %d (ExitWithSkips)", code, cli.ExitWithSkips)
	}
	if !strings.Contains(strings.ToLower(out), "skip") {
		t.Errorf("stdout = %q, want the completed-with-skips summary", out)
	}
	if svc.deployReq == nil || svc.deployReq.StrictModels {
		t.Errorf("StrictModels must be false without the flag; request = %+v", svc.deployReq)
	}
}

// TestDeployCmd_StrictFlag_SuccessOutcome_JSONIsUnchangedRunSummary verifies that with the
// flag and a successful run the output and exit code equal today's: a plain RunSummary.
func TestDeployCmd_StrictFlag_SuccessOutcome_JSONIsUnchangedRunSummary(t *testing.T) {
	svc := &spyService{deployResp: successSummary("ws")}

	code, out, _ := runDeploy(t, svc, "--strict-models", "--output", "json")

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\nstdout: %q", err, out)
	}
	if _, ok := raw["Outcome"]; !ok {
		t.Errorf("stdout lacks the RunSummary \"Outcome\" field: %v", raw)
	}
	if _, ok := raw["error"]; ok {
		t.Errorf("stdout carries the failure envelope key \"error\" on a successful run: %v", raw)
	}
}

// TestDeployCmd_StrictFlag_SuccessOutcome_TextIsUnchanged verifies the text-mode success.
func TestDeployCmd_StrictFlag_SuccessOutcome_TextIsUnchanged(t *testing.T) {
	svc := &spyService{deployResp: successSummary("ws")}

	code, out, errOut := runDeploy(t, svc, "--strict-models")

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	if !strings.Contains(out, "success") || errOut != "" {
		t.Errorf("stdout = %q, stderr = %q, want the usual success summary and empty stderr", out, errOut)
	}
}
