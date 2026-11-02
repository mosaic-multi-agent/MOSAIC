package app_test

// rerun_persisted_tier_test.go verifies that a tier-to-model mapping persisted in user config
// replaces the deployed model on deploy but not on agents, and that a tier mapping the run
// itself supplies replaces it on both. Covered through real deploy and agents runs on
// all four built-in harnesses.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
	"path/filepath"
)

const rpPersistedModel = "persisted-model-4"

// rpServiceWithPersistedTiers wires a service over the catalog copy whose user config already
// holds a persisted mapping of every tier the sample agents use for the given harness.
func rpServiceWithPersistedTiers(t *testing.T, catRoot, harness string) app.Service {
	t.Helper()
	abs, err := filepath.Abs(catRoot)
	if err != nil {
		t.Fatalf("resolve catalog copy: %v", err)
	}
	cat, err := catalog.Load(t.TempDir(), abs)
	if err != nil {
		t.Fatalf("load catalog copy: %v", err)
	}
	return parityServiceWith(t, func(d *app.Deps) {
		d.Catalog = cat
		d.UserConfig = &spyUserConfig{cfg: config.UserConfig{
			TierModels: map[string]map[domain.Tier]string{
				harness: {"LOW": rpPersistedModel, "MEDIUM": rpPersistedModel, "HIGH": rpPersistedModel},
			},
		}}
	})
}

func TestDeployRerun_PersistedTierMappingReplacesDeployedModel(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpBumpAll(t, cat)

			req := rpDeployReq(h, ws)
			if _, err := rpServiceWithPersistedTiers(t, cat, h).DeployNew(context.Background(), req); err != nil {
				t.Fatalf("deploy over an existing deployment: %v", err)
			}

			for _, key := range rpRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertModel(t, key, got, rpPersistedModel)
			}
		})
	}
}

func TestAgentsRerun_PersistedTierMappingReplacesDeployedModel(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpBumpAll(t, cat)

			req := rpAgentsReq(h, ws)
			if _, err := rpServiceWithPersistedTiers(t, cat, h).DeployAgents(context.Background(), req); err != nil {
				t.Fatalf("agents over an existing deployment: %v", err)
			}

			for _, key := range rpAgentsRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertModel(t, key, got, rpDeployedModel)
			}
		})
	}
}

func TestDeployRerun_RunTierModelBeatsPersistedMapping(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpBumpAll(t, cat)

			req := rpDeployReq(h, ws)
			req.TierModels = map[domain.Tier]string{"LOW": rpTierModel}
			if _, err := rpServiceWithPersistedTiers(t, cat, h).DeployNew(context.Background(), req); err != nil {
				t.Fatalf("deploy over an existing deployment: %v", err)
			}

			rpAssertModel(t, "parity-helper", rpReadAgent(t, ws, h, "parity-helper"), rpTierModel)
			rpAssertModel(t, "parity-runner", rpReadAgent(t, ws, h, "parity-runner"), rpPersistedModel)
		})
	}
}
