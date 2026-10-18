package cli_test

// promote_layer_flag_test.go covers the "promote --target-catalog" flag, which names the
// stack layer that receives a new item.

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
)

// TestPromoteCmd_TargetCatalogFlag_SetsTargetCatalogID verifies the flag value reaches the
// service request unchanged.
func TestPromoteCmd_TargetCatalogFlag_SetsTargetCatalogID(t *testing.T) {
	// Arrange
	svc := &spyService{promoteResp: app.PromoteResult{Key: "agent", NumericID: "5"}}

	// Act
	code := cli.Run(context.Background(),
		[]string{"promote", "--file", "/p/agent.md", "--harness", "stub-harness", "--target-catalog", "team-catalog"},
		svc, &bytes.Buffer{}, &bytes.Buffer{})

	// Assert
	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)", code, cli.ExitSuccess)
	}
	if svc.promoteReq == nil {
		t.Fatal("svc.Promote was not called")
	}
	if svc.promoteReq.TargetCatalogID != "team-catalog" {
		t.Errorf("PromoteRequest.TargetCatalogID = %q, want team-catalog", svc.promoteReq.TargetCatalogID)
	}
}

// TestPromoteCmd_TargetCatalogFlagOmitted_LeavesTargetCatalogIDEmpty verifies the flag is
// optional at the CLI: the choice rule is enforced by the service, not by flag parsing.
func TestPromoteCmd_TargetCatalogFlagOmitted_LeavesTargetCatalogIDEmpty(t *testing.T) {
	// Arrange
	svc := &spyService{promoteResp: app.PromoteResult{Key: "agent", NumericID: "5"}}

	// Act
	code := cli.Run(context.Background(),
		[]string{"promote", "--file", "/p/agent.md", "--harness", "stub-harness"},
		svc, &bytes.Buffer{}, &bytes.Buffer{})

	// Assert
	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)", code, cli.ExitSuccess)
	}
	if svc.promoteReq == nil {
		t.Fatal("svc.Promote was not called")
	}
	if svc.promoteReq.TargetCatalogID != "" {
		t.Errorf("PromoteRequest.TargetCatalogID = %q, want empty", svc.promoteReq.TargetCatalogID)
	}
}

// TestPromoteCmd_TargetCatalogFlag_ServiceErrorIsFailure verifies a service refusal (for
// example a missing or unknown layer) maps to ExitFailure.
func TestPromoteCmd_TargetCatalogFlag_ServiceErrorIsFailure(t *testing.T) {
	// Arrange
	svc := &spyService{promoteErr: errors.New("no target layer chosen")}

	// Act
	code := cli.Run(context.Background(),
		[]string{"promote", "--file", "/p/agent.md", "--harness", "stub-harness", "--target-catalog", "nope"},
		svc, &bytes.Buffer{}, &bytes.Buffer{})

	// Assert
	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want ExitFailure (%d)", code, cli.ExitFailure)
	}
}
