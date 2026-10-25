package agentdeploy_test

// Tests for the strict unresolved-model mode of Deploy: every deploy,
// real or dry run, asks the deployment tool to fail when an agent would be
// written without a resolved model, and a failure the tool reports through its
// failure envelope reaches the caller naming the agent and its tier.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-agent-test/internal/agentdeploy"
	"mosaic-agent-test/internal/domain"
)

const strictModelsFlag = "--strict-models"

// strictFailureEnvelope is the stdout document the deployment tool writes
// under --output json when a strict deploy finds an unresolved model.
const strictFailureEnvelope = `{"error": true, "reason": "unresolved-models",` +
	` "message": "strict models: 1 agent(s) have no resolved model: orchestrator (tier TEST_SUBJECT)",` +
	` "detail": "orchestrator (tier TEST_SUBJECT)", "exitCode": 1}`

// captureDeployArgs runs Deploy with the given request against a tool that
// succeeds, and returns the arguments the tool was invoked with.
func captureDeployArgs(t *testing.T, req domain.DeployRequest) []string {
	t.Helper()
	var captured []string
	d := newDeployer(agentdeploy.Options{
		Invoke: func(_ context.Context, _ string, args []string) ([]byte, []byte, int, error) {
			captured = append([]string(nil), args...)
			return deploySuccessJSON(), nil, exitSuccess, nil
		},
	})
	if _, err := d.Deploy(context.Background(), req); err != nil {
		t.Fatalf("Deploy returned unexpected error: %v", err)
	}
	return captured
}

// TestDeploy_RealRun_PassesStrictModelsFlag asserts a real (non dry-run) deploy
// asks the tool for strict model resolution. Without it a gap in the tier map
// is written as an agent with no model and the harness default silently runs.
func TestDeploy_RealRun_PassesStrictModelsFlag(t *testing.T) {
	req := minimalDeployRequest()
	req.DryRun = false

	args := captureDeployArgs(t, req)

	if flagIndex(args, strictModelsFlag) < 0 {
		t.Errorf("args = %v, want %s on every real deploy", args, strictModelsFlag)
	}
}

// TestDeploy_DryRun_PassesStrictModelsFlag asserts the preflight dry run is
// strict too, so a model gap is found before anything is spent.
func TestDeploy_DryRun_PassesStrictModelsFlag(t *testing.T) {
	req := minimalDeployRequest()
	req.DryRun = true

	args := captureDeployArgs(t, req)

	if flagIndex(args, strictModelsFlag) < 0 {
		t.Errorf("args = %v, want %s on every dry-run deploy", args, strictModelsFlag)
	}
}

// TestDeploy_StrictFlagIsNotExclusiveToPopulatedTierMaps asserts the flag does
// not depend on a tier map being supplied: an empty map is exactly the case
// strict mode exists to catch.
func TestDeploy_StrictFlagIsNotExclusiveToPopulatedTierMaps(t *testing.T) {
	req := minimalDeployRequest()
	req.TierModels = nil

	args := captureDeployArgs(t, req)

	if flagIndex(args, strictModelsFlag) < 0 {
		t.Errorf("args = %v, want %s even when no tier models are supplied", args, strictModelsFlag)
	}
}

// TestDeploy_StrictFailureEnvelope_ReasonDetailAndMessageSurface asserts that
// a non-zero exit whose stdout is the tool's failure envelope yields a
// *DeployError carrying the envelope's reason, detail and message.
func TestDeploy_StrictFailureEnvelope_ReasonDetailAndMessageSurface(t *testing.T) {
	d := newDeployer(agentdeploy.Options{
		Invoke: func(context.Context, string, []string) ([]byte, []byte, int, error) {
			return []byte(strictFailureEnvelope), []byte("error: strict models: ..."), exitFailure, nil
		},
	})

	_, err := d.Deploy(context.Background(), minimalDeployRequest())

	var de *agentdeploy.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("error = %v, want a *DeployError", err)
	}
	if de.Reason != agentdeploy.ReasonUnresolvedModels {
		t.Errorf("DeployError.Reason = %q, want %q", de.Reason, agentdeploy.ReasonUnresolvedModels)
	}
	if de.Detail != "orchestrator (tier TEST_SUBJECT)" {
		t.Errorf("DeployError.Detail = %q, want the envelope detail", de.Detail)
	}
	if !strings.Contains(de.ToolMessage, "no resolved model") {
		t.Errorf("DeployError.ToolMessage = %q, want the envelope message", de.ToolMessage)
	}
	if !errors.Is(err, agentdeploy.ErrDeployFailed) {
		t.Errorf("error = %v, want it to wrap ErrDeployFailed", err)
	}
}

// TestDeploy_StrictFailure_ErrorTextNamesAgentAndTier asserts the message the
// caller prints names the offending agent and its tier, so the user can see
// which catalog file to fix.
func TestDeploy_StrictFailure_ErrorTextNamesAgentAndTier(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		name := "real run"
		if dryRun {
			name = "dry run"
		}
		t.Run(name, func(t *testing.T) {
			d := newDeployer(agentdeploy.Options{
				Invoke: func(context.Context, string, []string) ([]byte, []byte, int, error) {
					return []byte(strictFailureEnvelope), nil, exitFailure, nil
				},
			})
			req := minimalDeployRequest()
			req.DryRun = dryRun

			_, err := d.Deploy(context.Background(), req)

			if err == nil {
				t.Fatal("Deploy returned nil error for a strict failure")
			}
			for _, want := range []string{"orchestrator", "TEST_SUBJECT"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error text %q does not name %q", err.Error(), want)
				}
			}
		})
	}
}

// TestDeploy_NonZeroExitWithoutEnvelope_DegradesToStderr asserts the envelope
// decoding does not replace the existing behaviour: stdout that is not a
// failure envelope leaves Reason empty and the message taken from stderr.
func TestDeploy_NonZeroExitWithoutEnvelope_DegradesToStderr(t *testing.T) {
	d := newDeployer(agentdeploy.Options{
		Invoke: func(context.Context, string, []string) ([]byte, []byte, int, error) {
			return []byte(`{"Actions":[],"Outcome":"failed"}`), []byte("boom"), exitFailure, nil
		},
	})

	_, err := d.Deploy(context.Background(), minimalDeployRequest())

	var de *agentdeploy.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("error = %v, want a *DeployError", err)
	}
	if de.Reason != "" || de.Detail != "" {
		t.Errorf("Reason/Detail = %q/%q, want both empty without a failure envelope", de.Reason, de.Detail)
	}
	if de.ToolMessage != "boom" {
		t.Errorf("ToolMessage = %q, want stderr text %q", de.ToolMessage, "boom")
	}
}
