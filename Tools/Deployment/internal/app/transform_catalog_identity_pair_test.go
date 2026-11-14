package app_test

// transform_catalog_identity_pair_test.go specifies the same catalog identity copy for the GHCP
// pair, where the target entries are written over the shared agents and hooks folders: agent and
// hook file entries of the target harness carry the identity of the source harness's entries.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trSaveManifestValue saves m to the workspace manifest.
func trSaveManifestValue(t *testing.T, ws string, m domain.Manifest) {
	t.Helper()
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// trPairRecordCatalogs gives the recorded source agent and records the source hook files with
// catalog identities, all under the source harness.
func trPairRecordCatalogs(t *testing.T, r *trRun) {
	t.Helper()
	m := trLoadManifest(t, r.ws)
	for i := range m.Entries {
		if m.Entries[i].Ref.Kind == domain.ArtifactAgent {
			m.Entries[i].Catalog = trCatalogOf("pair-agent")
		}
	}
	for _, file := range []string{"logger.sh", trPairSourceOnlyFile} {
		rel := r.hooksDir(r.src) + "/" + file
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Harness: r.src, Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: trPairHookKey},
			TargetPath: rel, Version: trHookVersion, ContentHash: manifest.Hash([]byte(r.read(rel))),
			DeployedAt: trSourceTime, Catalog: trCatalogOf("pair-hook"),
		})
	}
	trSaveManifestValue(t, r.ws, m)
}

func TestTransformCatalogIdentity_GhcpPair_AgentAndHookEntriesCopyTheSourceIdentity(t *testing.T) {
	for _, agentRel := range []string{trPairInPlaceAgent, trPairLegacyAgent} {
		for _, pair := range trPairDirections {
			t.Run(agentRel+" "+pair[0]+"->"+pair[1], func(t *testing.T) {
				r, _ := trPairHookRunWithAgent(t, pair[0], pair[1], agentRel)
				trPairRecordCatalogs(t, r)

				r.mustRun(r.request())

				target := trLoadManifest(t, r.ws).ForHarness(r.tgt)
				agent, ok := trEntryAt(target, trPairInPlaceAgent)
				if !ok {
					t.Fatalf("target harness records no agent entry at %s", trPairInPlaceAgent)
				}
				if want := trCatalogOf("pair-agent"); agent.Catalog != want {
					t.Errorf("agent catalog = %+v, want the source entry's %+v", agent.Catalog, want)
				}
				hooks := trEntryByRef(target, domain.ArtifactHook, trPairHookKey)
				if len(hooks) == 0 {
					t.Fatalf("target harness records no hook file of %s", trPairHookKey)
				}
				for _, e := range hooks {
					if want := trCatalogOf("pair-hook"); e.Catalog != want {
						t.Errorf("%s: catalog = %+v, want the source bundle's %+v", e.TargetPath, e.Catalog, want)
					}
				}
			})
		}
	}
}
