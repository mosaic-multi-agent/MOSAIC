package transform

import (
	"strings"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// resolveTools determines the tool frontmatter output for the transformation. It handles
// two distinct cases:
//
//   - Placeholder case: the source declares tools as a scalar placeholder (e.g.
//     `tools: {tool-permissions}`). The placeholder is expanded using the descriptor's
//     PlaceholderExpansion list. Report.Tools is an empty, non-nil slice because the
//     expansion is not decomposed into per-tool resolutions at this stage.
//
//   - List case: the source declares a concrete list of generic tool names. These are
//     passed to Module.Tools, which maps them to harness-specific names and builds the
//     frontmatter field(s). Report.Tools mirrors the resolution list from the module.
//
// If the source has no "tools" field, or if the descriptor declares no tools_key, a safe
// empty result is returned. All harness-specific tool knowledge flows through Module.Tools
// or through the descriptor's PlaceholderExpansion; no harness name appears here (AC8.4).
func resolveTools(req Request, fm *docformat.Frontmatter, desc *domain.HarnessDescriptor) (domain.ToolResult, error) {
	v, ok := fm.Get("tools")
	if !ok {
		// Source has no tools field — return empty, non-nil Resolutions for a safe report.
		return domain.ToolResult{Resolutions: make([]domain.ToolResolution, 0)}, nil
	}

	if v.Kind == domain.KindScalar {
		// Placeholder case: route through module.Tools() with Placeholder set so the module
		// controls the output format. Built-in modules may render the placeholder expansion
		// differently (e.g., Claude Code uses a comma-separated scalar while GHCP CLI uses a
		// flow-style single-quoted list). The Placeholder field carries the verbatim source
		// placeholder text (e.g., "{tool-permissions}").
		toolReq := domain.ToolRequest{
			AgentKey:     req.Key,
			Placeholder:  v.Scalar,
			CustomNames:  req.CustomTools,
			SkippedTools: req.SkippedTools,
		}
		result, err := req.Module.Tools(toolReq)
		if err != nil {
			return domain.ToolResult{}, err
		}
		if result.Resolutions == nil {
			result.Resolutions = make([]domain.ToolResolution, 0)
		}
		return result, nil
	}

	// List case: extract generic tool names and call Module.Tools.
	var genericTools []string
	if v.Kind == domain.KindList {
		genericTools = make([]string, len(v.Items))
		for i, item := range v.Items {
			genericTools[i] = item.Scalar
		}
	}

	toolReq := domain.ToolRequest{
		AgentKey:     req.Key,
		Generic:      genericTools,
		CustomNames:  req.CustomTools,
		SkippedTools: req.SkippedTools,
	}
	result, err := req.Module.Tools(toolReq)
	if err != nil {
		return domain.ToolResult{}, err
	}
	if result.Resolutions == nil {
		result.Resolutions = make([]domain.ToolResolution, 0)
	}
	return result, nil
}

// applyFrontmatter applies the descriptor-driven frontmatter plan (removes, adds, and
// key order) together with the model selection, version stamps, and resolved tool fields.
// It mutates fm in place and returns:
//   - fieldChanges: one FieldChange per frontmatter key that was added, overwritten, or
//     removed (satisfying AC8.5).
//   - gaps: domain.Gap entries produced during the transformation (e.g. GapNoModel).
//
// Step order is significant: removes happen before adds so that a key that is dropped and
// re-added with a different value under the same name works correctly. Version stamps and
// tool fields are applied after the descriptor plan so that the plan's key-order list can
// include them in the desired position.
func applyFrontmatter(
	fm *docformat.Frontmatter,
	plan domain.FrontmatterPlan,
	toolResult domain.ToolResult,
	req Request,
	desc *domain.HarnessDescriptor,
) ([]FieldChange, []domain.Gap) {
	var changes []FieldChange
	var gaps []domain.Gap

	// Frontmatter computation below consults req.Deployed only; when the deployed frontmatter
	// is to be ignored, the original is kept solely for the carried bundle stamp.
	frontmatterDeployed := req.Deployed
	if req.IgnoreDeployedFrontmatter {
		req.Deployed = nil
	}

	// touchedKeys records every frontmatter key that any transform step explicitly
	// manages (sets or removes). Used by the deployed-field preservation pass (Step 5c)
	// to determine which deployed-file keys are user-owned and should be copied into
	// the output when the transform has not touched them.
	touchedKeys := make(map[string]bool)

	// Step 1: Remove fields declared in the descriptor drop list.
	for _, key := range plan.Remove {
		touchedKeys[key] = true
		if v, ok := fm.Get(key); ok {
			changes = append(changes, FieldChange{
				Key:    key,
				Before: renderValue(v),
				After:  "",
				Reason: "descriptor drop",
			})
			fm.Remove(key)
		}
	}

	// Step 2: Add or overwrite fields from the descriptor add list.
	for _, field := range plan.Set {
		touchedKeys[field.Key] = true
		before := ""
		if v, ok := fm.Get(field.Key); ok {
			before = renderValue(v)
		}
		changes = append(changes, FieldChange{
			Key:    field.Key,
			Before: before,
			After:  renderValue(field.Value),
			Reason: "descriptor add",
		})
		fm.Set(field.Key, field.Value)
	}

	// Step 3: Set the model field from the resolved model selection.
	modelKey := desc.Frontmatter.ModelKey
	if modelKey != "" {
		touchedKeys[modelKey] = true
		if req.Model.Resolved() {
			before := ""
			if v, ok := fm.Get(modelKey); ok {
				before = renderValue(v)
			}
			changes = append(changes, FieldChange{
				Key:    modelKey,
				Before: before,
				After:  req.Model.ModelID,
				Reason: "model selection",
			})
			fm.Set(modelKey, domain.ScalarValue(req.Model.ModelID, domain.QuotePlain))
		} else {
			// No model resolved: remove the placeholder field and record a gap so the
			// TODO checklist can surface the missing model for the user.
			if v, ok := fm.Get(modelKey); ok {
				changes = append(changes, FieldChange{
					Key:    modelKey,
					Before: renderValue(v),
					After:  "",
					Reason: "model unresolved",
				})
				fm.Remove(modelKey)
			}
			gaps = append(gaps, domain.Gap{
				Kind:    domain.GapNoModel,
				Subject: req.Key,
				Detail:  "no model selected for this agent",
			})
		}
	}

	// Step 4: Stamp version fields under their mosaic_-prefixed deployed names, derived from
	// the agentfields registry. No literal prefixed key names appear here; all names come from
	// agentfields.ByDeployedName (which looks up by the legacy name) and uses field.Deployed.
	// The harness version stamp is written as mosaic_harness_version (via the new registry
	// entry with Legacy="harness_version"). The old mosaic_transform_version key is no longer
	// written here; it is stripped by the migration step below if present from a legacy file.
	tvField, _ := agentfields.ByDeployedName("harness_version")
	if c := applyVersionStamp(fm, tvField.Deployed, desc.TransformVersion); c != nil {
		touchedKeys[tvField.Deployed] = true
		changes = append(changes, *c)
	}
	// injections_version is no longer stamped as a frontmatter field; it is written as a
	// version attribute on InjectionHarness-class region tags by applyHarnessRegion instead.
	// The ivField variable is retained here for the migration strip below, which removes
	// legacy mosaic_injections_version fields from pre-migration deployed files.
	ivField, _ := agentfields.ByDeployedName("injections_version")
	touchedKeys[ivField.Deployed] = true
	// tool_mappings_version is a content hash of the effective tool_destinations
	// mappings for this harness (combined project + user config). It lets `update`
	// detect stale tool mappings without re-diffing tool lists. See "The
	// tool_mappings_version stamp" in Tools/Deployment/docs/configuration.md.
	tmvField, _ := agentfields.ByDeployedName("tool_mappings_version")
	if c := applyVersionStamp(fm, tmvField.Deployed, req.ToolMappingsVersion); c != nil {
		touchedKeys[tmvField.Deployed] = true
		changes = append(changes, *c)
	} else if req.Deployed != nil {
		// ToolMappingsVersion is empty (no new stamp to write). Check whether the deployed
		// file carries a zombie stamp left over from a previous run when mappings were
		// active. Actively remove it and mark it touched so the Step 5c preservation pass
		// does not re-inject it, allowing the staleness loop to converge.
		if deployedDoc, deployedParseErr := docformat.Parse(req.Deployed); deployedParseErr == nil {
			deployedFM := deployedDoc.Frontmatter()
			if deployedStamp, ok := deployedFM.Get(tmvField.Deployed); ok && renderValue(deployedStamp) != "" {
				touchedKeys[tmvField.Deployed] = true
				fm.Remove(tmvField.Deployed)
				changes = append(changes, FieldChange{
					Key:    tmvField.Deployed,
					Before: renderValue(deployedStamp),
					After:  "",
					Reason: "zombie stamp removal",
				})
			}
		}
	}

	// Step 4b: Stamp bundle_version for roles that receive bundle blocks. The stamp enables
	// staleness detection on subsequent runs. Written only when AppliesToRole is true, so the
	// orchestrator (which today receives no bundle blocks) is never stamped.
	bvField, _ := agentfields.ByDeployedName("bundle_version")
	if req.PreserveDeployedManaged && frontmatterDeployed != nil {
		touchedKeys[bvField.Deployed] = true
		if c := carryDeployedBundleStamp(fm, frontmatterDeployed); c != nil {
			changes = append(changes, *c)
		}
	} else if req.Bundle.AppliesToRole(req.Role) && req.Bundle.Version != "" {
		touchedKeys[bvField.Deployed] = true
		before := ""
		if v, ok := fm.Get(bvField.Deployed); ok {
			before = renderValue(v)
		}
		fm.Set(bvField.Deployed, domain.ScalarValue(req.Bundle.Version, domain.QuotePlain))
		changes = append(changes, FieldChange{
			Key:    bvField.Deployed,
			Before: before,
			After:  req.Bundle.Version,
			Reason: "bundle version stamp",
		})
	}

	// Step 4c: Rename the generic "id" field to its mosaic_-prefixed deployed name. The
	// generic source carries "id"; deployed files must carry the prefixed form. The
	// agentfields registry supplies both names; no prefixed literal appears here.
	idField, _ := agentfields.ByGeneric("id")
	touchedKeys[idField.Legacy] = true
	touchedKeys[idField.Deployed] = true
	if v, ok := fm.Get(idField.Legacy); ok {
		fm.Remove(idField.Legacy)
		fm.Set(idField.Deployed, v)
		changes = append(changes, FieldChange{
			Key:    idField.Deployed,
			Before: "",
			After:  renderValue(v),
			Reason: "id field rename to deployed form",
		})
	}

	// Step 4d: Rename the generic "role" field to its mosaic_-prefixed deployed name. The
	// generic source carries "role"; deployed files must carry the prefixed form. This is
	// parallel to the id rename above; the agentfields registry supplies both names.
	roleField, _ := agentfields.ByGeneric("role")
	touchedKeys[roleField.Legacy] = true
	touchedKeys[roleField.Deployed] = true
	if v, ok := fm.Get(roleField.Legacy); ok {
		fm.Remove(roleField.Legacy)
		fm.Set(roleField.Deployed, v)
		changes = append(changes, FieldChange{
			Key:    roleField.Deployed,
			Before: "",
			After:  renderValue(v),
			Reason: "role field rename to deployed form",
		})
	}

	// Step 4e: Rename the generic "version" field to its mosaic_-prefixed deployed name.
	// The generic source carries "version"; deployed files must carry the prefixed form
	// (mosaic_version). This is parallel to the id and role renames above; the agentfields
	// registry supplies both names so no prefixed literal appears here.
	versionField, _ := agentfields.ByGeneric("version")
	touchedKeys[versionField.Legacy] = true
	touchedKeys[versionField.Deployed] = true
	if v, ok := fm.Get(versionField.Legacy); ok {
		fm.Remove(versionField.Legacy)
		fm.Set(versionField.Deployed, v)
		changes = append(changes, FieldChange{
			Key:    versionField.Deployed,
			Before: "",
			After:  renderValue(v),
			Reason: "version field rename to deployed form",
		})
	}

	// Migration: strip legacy fields that have been renamed or relocated.
	// All lookups go through the registry — no literal mosaic_* strings.
	// mosaic_transform_version: the harness version stamp is now written as mosaic_harness_version.
	// mosaic_injections_version: will be relocated to region tag attributes in Stage 2; Stage 1
	// pre-strips it so that deployed files are already free of it when Stage 2 takes effect.
	// mosaic_orchestrator_injections_version: same — stripped here to prepare for Stage 2.
	oldTVField, _ := agentfields.ByDeployedName("transform_version")
	touchedKeys[oldTVField.Deployed] = true
	if fm.Remove(oldTVField.Deployed) {
		changes = append(changes, FieldChange{
			Key:    oldTVField.Deployed,
			Before: "(present)",
			After:  "",
			Reason: "migration strip",
		})
	}
	if fm.Remove(ivField.Deployed) {
		changes = append(changes, FieldChange{
			Key:    ivField.Deployed,
			Before: "(present)",
			After:  "",
			Reason: "migration strip",
		})
	}
	oivField, _ := agentfields.ByDeployedName("orchestrator_injections_version")
	touchedKeys[oivField.Deployed] = true
	if fm.Remove(oivField.Deployed) {
		changes = append(changes, FieldChange{
			Key:    oivField.Deployed,
			Before: "(present)",
			After:  "",
			Reason: "migration strip",
		})
	}

	// Steps 5 and 5b: tool fields, appended tools, and the merge with the deployed tools field.
	toolChanges, _ := writeToolFields(req, fm, desc, toolResult, touchedKeys)
	changes = append(changes, toolChanges...)

	// Step 5c: Preserve user-owned fields from the deployed file. After all managed-field
	// steps have run, any key in the deployed file's frontmatter that was not touched by
	// this transform is considered user-owned and is copied verbatim into the output. This
	// implements the general preservation policy for arbitrary fields like mcpServers (when
	// coming from a prior deploy) or custom fields the user has manually added.
	if req.Deployed != nil {
		deployedDoc, deployedParseErr := docformat.Parse(req.Deployed)
		if deployedParseErr == nil {
			deployedFM := deployedDoc.Frontmatter()
			for _, k := range deployedFM.Keys() {
				if touchedKeys[k] {
					continue
				}
				v, _ := deployedFM.Get(k)
				fm.Set(k, v)
				changes = append(changes, FieldChange{
					Key:    k,
					Before: "",
					After:  renderValue(v),
					Reason: "preserved from deployed file",
				})
			}
		}
	}

	// Step 6: Accumulate gaps for unresolved tool mappings. ToolUnmapped and ToolSkipped
	// resolutions are never emitted into the output; each produces a GapUnmappedTool so the
	// TODO checklist can surface the missing tool mapping. ToolCustom resolutions are resolved
	// and must not produce a gap.
	for _, res := range toolResult.Resolutions {
		if res.Outcome == domain.ToolUnmapped || res.Outcome == domain.ToolSkipped {
			gaps = append(gaps, domain.Gap{
				Kind:    domain.GapUnmappedTool,
				Subject: res.Generic,
				Owner:   req.Key,
				Detail:  "unmapped tool: " + res.Generic + " for agent " + req.Key,
			})
		}
	}

	// Step 7: Reorder the frontmatter keys according to the descriptor's key_order.
	// Unlisted keys keep their current relative order and are appended after listed keys.
	if len(plan.KeyOrder) > 0 {
		fm.Reorder(plan.KeyOrder)
	}

	return changes, gaps
}

// applyVersionStamp sets a single version stamp field in the frontmatter and returns the
// corresponding FieldChange. Returns nil when value is empty (nothing to stamp).
func applyVersionStamp(fm *docformat.Frontmatter, key, value string) *FieldChange {
	if value == "" {
		return nil
	}
	before := ""
	if v, ok := fm.Get(key); ok {
		before = renderValue(v)
	}
	fm.Set(key, domain.ScalarValue(value, domain.QuotePlain))
	return &FieldChange{
		Key:    key,
		Before: before,
		After:  value,
		Reason: "version stamp",
	}
}

// renderValue converts a domain.FieldValue to a concise string for use in FieldChange
// Before/After fields. The rendering is for human-readable audit purposes only; it is
// not a YAML round-trip serialisation. Callers must not parse the returned string.
func renderValue(v domain.FieldValue) string {
	switch v.Kind {
	case domain.KindScalar:
		return v.Scalar
	case domain.KindList:
		items := make([]string, len(v.Items))
		for i, item := range v.Items {
			items[i] = renderValue(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case domain.KindMapping:
		pairs := make([]string, len(v.Pairs))
		for i, p := range v.Pairs {
			pairs[i] = p.Key + ": " + renderValue(p.Value)
		}
		return "{" + strings.Join(pairs, ", ") + "}"
	default:
		return ""
	}
}
