package plan

// build_legacy_agent.go classifies an agent whose deployed state was read from its
// manifest-recorded legacy file (GHCP X.md awaiting migration to X.agent.md). The agent is
// compared against the manifest record of the legacy path and reported as a rename plus any
// staleness, never as a new artifact or a conflict.

import (
	"mosaic-deploy/internal/domain"
)

// classifyAgentItemLegacyAware classifies an agent like classifyAgentItem. When legacyPath is
// non-empty, the agent's deployed state is present and parsed, and the manifest records the
// legacy path, the item is an update reported as a rename from legacyPath. In every other
// case the result is exactly that of classifyAgentItem.
func classifyAgentItemLegacyAware(
	legacyPath string,
	agent domain.Agent,
	targetPath string,
	model domain.ModelSelection,
	manifestUsable bool,
	m domain.Manifest,
	deployed domain.DeployedArtifactState,
	desc *domain.HarnessDescriptor,
	selectedWorkflows []domain.Workflow,
	toolMappingsVersion string,
	protocolVersion string,
	bundleVersion string,
	infra infrastructureInput,
	supplier supplierLookup,
) domain.PlanItem {
	classify := func(manifest domain.Manifest) domain.PlanItem {
		return classifyAgentItem(agent, targetPath, model, manifestUsable, manifest, deployed, desc, selectedWorkflows, toolMappingsVersion, protocolVersion, bundleVersion, infra, supplier)
	}
	if legacyPath == "" || !manifestUsable || !deployed.Present || deployed.ParseFailed {
		return classify(m)
	}
	ref := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: agent.Key}
	if _, recorded := m.LookupAt(ref, legacyPath); !recorded {
		return classify(m)
	}

	item := classify(manifestRecordedAt(m, ref, legacyPath, targetPath))
	item.Action = domain.ActionUpdate
	item.Conflict = nil
	item.RenamedFrom = legacyPath
	item.AddReason("renamed from " + legacyPath)
	return item
}

// manifestRecordedAt returns a copy of m in which the entry of ref recorded at fromPath is
// recorded at toPath, so the standard classification finds the legacy record at the target.
func manifestRecordedAt(m domain.Manifest, ref domain.ArtifactRef, fromPath, toPath string) domain.Manifest {
	moved := m
	moved.Entries = make([]domain.ManifestEntry, len(m.Entries))
	copy(moved.Entries, m.Entries)
	for i, e := range moved.Entries {
		if e.Ref == ref && e.TargetPath == fromPath {
			moved.Entries[i].TargetPath = toPath
		}
	}
	return moved
}
