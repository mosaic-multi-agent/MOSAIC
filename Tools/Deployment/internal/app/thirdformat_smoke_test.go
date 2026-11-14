package app_test

// thirdformat_smoke_test.go proves the twin fake harnesses are usable: each one deploys a
// single agent through the Deploy agents flow. This is fixture coverage; format parity is
// specified elsewhere.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTwinHarness_DeployAgents_WritesOneDecodableAgent(t *testing.T) {
	twins := []struct {
		name   string
		format twinFormat
	}{
		{"test format", twinTestFormat},
		{"markdown", twinMarkdown},
	}
	for _, tc := range twins {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			mod := newTestTwinHarness(t, tc.format)
			svc := newTestTwinService(t, mod)
			ws := t.TempDir()
			rel, err := mod.TargetPath(domain.TargetPathRequest{
				Kind: domain.ArtifactAgent, Key: owRunner, Scope: domain.ScopeProject,
			})
			if err != nil {
				t.Fatalf("twin target path: %v", err)
			}
			req := app.DeployAgentsRequest{
				HarnessID: twinHarnessID, WorkspacePath: ws, Scope: domain.ScopeProject,
				SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{},
				InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
				AgentModels: map[string]string{owRunner: parityModel}, AutoConfirmPlan: true,
			}

			// Act
			_, err = svc.DeployAgents(context.Background(), req)

			// Assert
			if err != nil {
				t.Fatalf("DeployAgents on the %s twin: %v", tc.name, err)
			}
			tr, err := agentformat.LookupString(mod.Descriptor().AgentFormatID)
			if err != nil {
				t.Fatalf("twin format does not resolve: %v", err)
			}
			written := gamRead(t, ws, rel)
			if _, _, err := tr.Decode(written, agentformat.ArtifactContext{Kind: domain.ArtifactAgent}); err != nil {
				t.Errorf("written agent %s does not decode through the twin's translator: %v", rel, err)
			}
			if !twinHasAgentEntry(gamManifest(t, ws), owRunner, rel) {
				t.Errorf("manifest has no agent entry for %s at %s", owRunner, rel)
			}
		})
	}
}

func twinHasAgentEntry(m domain.Manifest, key, rel string) bool {
	for _, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key && filepath.ToSlash(e.TargetPath) == rel {
			return true
		}
	}
	return false
}
