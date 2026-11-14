package plan

// build_agent.go classifies one agent into a PlanItem.

import (
	"mosaic-deploy/internal/domain"
)

// classifyAgentItem classifies one agent into a PlanItem with the appropriate action.
// The classification contract (see ContractsDesign.md Classification Contract):
//  1. No deployed file → ActionCreate
//  2. No usable manifest → ActionConflict (ManifestMissing: true)
//  3. No manifest entry at current target path → ActionConflict (ManifestMissing: true)
//  4. Version staleness check against deployed-file versions
//  5a. For orchestrator-role agents: workflow staleness check; merged into deltas
//  5b. For agents whose role carries version markers: protocol staleness check; merged into deltas.
//      Excluded for RoleUtility and RoleStandalone: those roles receive no protocol marker by design.
//  5c. For agents whose role carries version markers and is not orchestrator: bundle staleness check;
//      merged into deltas. Excluded for RoleUtility and RoleStandalone alongside RoleOrchestrator.
//  6. Stale or no version info → ActionUpdate
//  7. → ActionUnchanged
//
// selectedWorkflows is the full list of workflows resolved from plan.Input.WorkflowIDs for
// this run. It is only consulted for agents with Role == domain.RoleOrchestrator; it may
// be nil or empty for other agents with no effect.
//
// toolMappingsVersion is the current effective mapping hash from plan.Input.ToolMappingsVersion.
// When it differs from deployed.ToolMappingsVersion, a "tool_mappings_version" staleness
// delta is produced so the destination field is regenerated on the next update.
//
// protocolVersion is the current protocol source version from plan.Input.ProtocolVersion.
// Protocol staleness applies to agents whose role carries version markers (RoleSubagent and
// RoleOrchestrator). It is suppressed for RoleUtility and RoleStandalone, which receive harness
// transformation only — the absence of a protocol marker in those files is by design.
//
// bundleVersion is the current bundle source version from plan.Input.BundleVersion.
// Bundle staleness applies to subagent-role agents whose role carries version markers;
// the orchestrator and marker-free roles (utility, standalone) are excluded.
func classifyAgentItem(
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
	ref := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: agent.Key}
	item := domain.PlanItem{
		Ref:           ref,
		SourcePath:    agent.SourcePath,
		TargetPath:    targetPath,
		Model:         model,
		SourceVersion: agent.Version,
	}

	// Step 1: No deployed file — short-circuit to create before consulting the manifest.
	if !deployed.Present {
		item.Action = domain.ActionCreate
		item.AddReason("new artifact; no file at target path")
		return item
	}

	// Step 1b: File exists but frontmatter could not be parsed. Classify as CONFLICT with a
	// distinct reason before reaching the manifest-availability or hash-mismatch checks.
	// Without this early exit, the file would fall through to the manifest-availability check
	// and version-staleness steps with incomplete state.
	if deployed.ParseFailed {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		reason := "deployed file could not be read/parsed"
		if deployed.ParseProblem != "" {
			reason += ": " + deployed.ParseProblem
		}
		item.AddReason(reason)
		return item
	}

	// Step 2: No usable manifest — any file on disk could have been placed there manually.
	if !manifestUsable {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("deployed file found but no manifest is available; cannot confirm it was written by this tool")
		return item
	}

	// Step 3: Target-path-aware manifest lookup. An entry for the same Kind+Key at a
	// different target path (e.g. a different harness) does not apply to this run.
	entry, inManifest := m.LookupAt(ref, targetPath)
	if !inManifest {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("file exists on disk but has no manifest record for this target path; cannot confirm origin")
		return item
	}

	// Step 3b: A local edit of Deploy-owned content makes the item an update with a backup,
	// whatever the versions say. Its reason comes before the version reasons.
	edited := markOwnedContentEdit(&item, entry, deployed)

	// Step 4: Compare version fields against the deployed-file versions.
	stamps := agentVersionStamps(agent, desc, toolMappingsVersion)
	deltas := AgentStaleness(deployed, agent, stamps)

	// Step 5a: For orchestrator-role agents, also compare the deployed workflow set
	// against the newly selected workflows. Workflow drift composes with version-field
	// deltas into a single update item; never produces a separate plan item.
	var drift WorkflowDrift
	if agent.Role == domain.RoleOrchestrator {
		drift = WorkflowStaleness(deployed.Workflows, selectedWorkflows)
		deltas = append(deltas, drift.Deltas()...)
	}

	// Step 5a (continued): infrastructure declaration drift, orchestrator-role agents only,
	// evaluated against this item's own deployed declarations.
	infraDrift := orchestratorInfrastructureDrift(agent, deployed, infra)
	deltas = append(deltas, infraDrift.Deltas()...)

	// Steps 5b/5c precondition: evaluate once for all role-gated staleness checks.
	// Utility and standalone agents receive harness transformation only; neither a protocol
	// marker nor a bundle stamp is written to their deployed files by design. Reporting either
	// as staleness would cause a perpetual-update loop.
	markersApply := domain.RoleCarriesVersionMarkers(agent.Role)

	// Step 5b: For agents whose role carries version markers, compare the deployed protocol
	// version against the source version. Suppressed for RoleUtility and RoleStandalone.
	var protocolDrift ProtocolDrift
	if markersApply {
		protocolDrift = ProtocolStaleness(deployed, protocolVersion)
		if delta, ok := protocolDrift.Delta(); ok {
			deltas = append(deltas, delta)
		}
	}

	// Step 5c: For agents whose role carries version markers and is not the orchestrator,
	// compare the deployed bundle version against the source version. Bundle blocks apply to
	// subagent-role agents; the orchestrator and marker-free roles are excluded.
	bundleApplies := markersApply && agent.Role != domain.RoleOrchestrator
	bundleDrift := BundleStaleness(deployed, bundleVersion, bundleApplies)
	if delta, ok := bundleDrift.Delta(); ok {
		deltas = append(deltas, delta)
	}

	// Step 6: Stale or no version info → update.
	catDrift := entryCatalogDrift(supplier, ref, entry)
	item.AddWarning(catDrift.Warning())
	if edited || len(deltas) > 0 || !deployed.HasVersionInfo() || catDrift.Stale() {
		item.Action = domain.ActionUpdate
		addAgentUpdateReasons(&item, deltas, !deployed.HasVersionInfo(), drift, protocolDrift, bundleDrift)
		if infraDrift.IsStale() {
			item.AddReason(infraDrift.Reason())
		}
		item.Stale = withCatalogDrift(&item, catDrift, deltas)
		return item
	}

	// Step 7: All checks passed — unchanged.
	item.Action = domain.ActionUnchanged
	return item
}
