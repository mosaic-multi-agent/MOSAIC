package plan

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// planner is the production implementation of Planner.
type planner struct{}

// Build derives the complete plan from the given inputs. It performs no file writes.
func (p *planner) Build(ctx context.Context, in Input) (domain.Plan, error) {
	// Resolve the complete artifact set from all selections, including standalone agents.
	set, err := ResolveArtifactsFrom(in.Catalog, Selection{
		WorkflowIDs:            in.WorkflowIDs,
		KeptWorkflowIDs:        in.KeptWorkflowIDs,
		SubagentIDs:           in.SubagentIDs,
		UtilityAgentIDs:        in.UtilityAgentIDs,
		InfrastructureAgentIDs: in.InfrastructureAgentIDs,
		StandaloneAgentIDs:     in.StandaloneAgentIDs,
		HookIDs:                in.HookIDs,
		ScannedAgentKeys:       in.ScannedAgentKeys,
		ExcludeOrchestrator:    OrchestratorExcludedFor(in.Mode),
	})
	if err != nil {
		return domain.Plan{}, err
	}

	// Compute all target paths once; path logic is centralised in EnumerateTargetPaths so that
	// Build and the app-layer probe use exactly the same path computation (D2).
	paths, err := EnumerateTargetPaths(set, in.Module, in.Scope, in.GOOS)
	if err != nil {
		return domain.Plan{}, err
	}

	desc := in.Module.Descriptor()
	manifestUsable := in.Manifest.State == manifest.StatePresent
	// Classification sees only the entries of the harness the run is for.
	runManifest := in.Manifest.Manifest.ForHarness(in.Module.Ref().ID)

	var items []domain.PlanItem
	var gaps []domain.Gap
	var registrations []domain.RegistrationStep

	// Resolve the selected workflows from the catalog once, so classifyAgentItem can apply
	// workflow staleness to orchestrator-role agents without duplicating this lookup.
	// WorkflowIDs are guaranteed to be valid: ResolveArtifacts already returned ErrUnknownWorkflow
	// for any unrecognised ID, so Catalog.Workflow(id) always succeeds here.
	selectedWorkflows := catalogWorkflowsFor(in.Catalog.Workflow, in.WorkflowIDs, in.KeptWorkflowIDs)

	// Build plan items for each agent.
	for _, agent := range set.Agents {
		ref := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: agent.Key}
		targetPath, _ := paths.Path(ref) // guaranteed present; EnumerateTargetPaths already errored if not

		model := in.Models[agent.Key]
		deployed := in.DeployedState[targetPath]
		deployed.Workflows = deployedWithoutKept(deployed.Workflows, in.KeptWorkflowIDs)

		// Classify before evaluating gaps: the GapNoModel emission rule depends on the action.
		item := classifyAgentItemLegacyAware(in.LegacyAgentPaths[targetPath], agent, targetPath, model, manifestUsable, runManifest, deployed, descriptorForAgent(desc, agent, in.OrchestratorRegionUndeliverable), selectedWorkflows, in.ToolMappingsVersion, in.ProtocolVersion, in.BundleVersion, infrastructureInput{Catalog: in.Catalog, Intent: in.InfrastructureDeclarations}, supplierOf(in.Catalog))
		items = append(items, item)

		// Surface a GapNoModel gap only when the file write that would occur requires a model
		// and neither the incoming selection nor the deployed file supplies one.
		//
		//   ActionUnchanged: no rewrite occurs; the deployed file's embedded model stands — never emit.
		//   ActionCreate:    a new file will be written — emit when the incoming model is unresolved.
		//   ActionUpdate:    the file will be regenerated — emit only when neither the incoming model
		//                    is resolved nor the deployed file carries an embedded ModelID that
		//                    regeneration will preserve.
		//   ActionConflict:  same rule as ActionUpdate.
		switch item.Action {
		case domain.ActionCreate:
			if !model.Resolved() {
				gaps = append(gaps, domain.Gap{
					Kind:    domain.GapNoModel,
					Subject: agent.Key,
					Detail:  "no model has been selected for agent " + agent.Key,
				})
			}
		case domain.ActionUpdate, domain.ActionConflict:
			if !model.Resolved() && deployed.ModelID == "" {
				gaps = append(gaps, domain.Gap{
					Kind:    domain.GapNoModel,
					Subject: agent.Key,
					Detail:  "no model has been selected for agent " + agent.Key,
				})
			}
		// ActionUnchanged: no gap emitted — file is not rewritten.
		}

		// Surface a gap for each generic tool the harness cannot map.
		// Suppressed for ActionUnchanged: the file will not be rewritten, so the current
		// tool destination in the deployed file stands and no resolution action is needed.
		if len(agent.Tools) > 0 && item.Action != domain.ActionUnchanged {
			toolResult, toolErr := in.Module.Tools(domain.ToolRequest{
				AgentKey:    agent.Key,
				Generic:     agent.Tools,
				Placeholder: agent.ToolsPlaceholder,
			})
			if toolErr == nil {
				for _, res := range toolResult.Resolutions {
					if res.Outcome == domain.ToolUnmapped {
						gaps = append(gaps, domain.Gap{
							Kind:    domain.GapUnmappedTool,
							Subject: res.Generic,
							Owner:   agent.Key,
							Detail: fmt.Sprintf("generic tool %q has no harness mapping for agent %q; add a tool_destinations entry in tool-config.yaml (project scope) or user-config.yaml (user scope) for a durable fix — manually editing the deployed agent file does not persist", res.Generic, agent.Key),
						})
					}
				}
				gaps = append(gaps, unknownHarnessToolGaps(desc, agent.Key, toolResult)...)
			}
		}
	}

	// Build plan items for each skill: one PlanItem per file.
	for _, skill := range set.Skills {
		ref := domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: skill.Key}
		targetPaths := paths.PathsFor(ref) // one entry per file in skill.Files order

		files := skill.Files
		if len(files) == 0 {
			files = []string{skill.EntryFile}
		}

		for i, file := range files {
			var targetPath string
			if i < len(targetPaths) {
				targetPath = targetPaths[i]
			}
			sourcePath := filepath.Join(skill.SourceDir, file)
			isEntryFile := file == skill.EntryFile
			deployed := in.DeployedState[targetPath]
			item := classifySkillItem(skill, sourcePath, targetPath, isEntryFile, manifestUsable, runManifest, deployed, supplierOf(in.Catalog))
			items = append(items, item)
		}
	}

	// Build plan items for each hook bundle.
	for _, hook := range set.Hooks {
		ref := domain.ArtifactRef{Kind: domain.ArtifactHook, Key: hook.Key}
		targetPaths := paths.PathsFor(ref) // one entry per hook file, in HookPlan file order
		hookPlan, hookErr := in.Module.HookPlan(domain.HookPlanRequest{
			Bundle: hook,
			Scope:  in.Scope,
		})
		if hookErr != nil || !hookPlan.Supported {
			// Hook not supported by this harness — skip plan items.
			continue
		}

		// Check registration steps for conflicts with existing workspace files.
		for _, step := range hookPlan.Registration {
			registrations = append(registrations, step)
			if step.TargetPath != "" && registrationTargetExists(in, step.TargetPath) {
				// Registration target already exists in the workspace.
				gaps = append(gaps, domain.Gap{
					Kind:     domain.GapHookRegistration,
					Subject:  step.ID,
					Detail:   fmt.Sprintf("registration target %q already exists in workspace", step.TargetPath),
					Fragment: step.Fragment,
				})
			}
		}

		for i, file := range hookPlan.Files {
			if i >= len(targetPaths) {
				break
			}
			targetPath := targetPaths[i]
			deployed := in.DeployedState[targetPath]
			items = append(items, classifyHookItem(hook, file.SourcePath, targetPath, manifestUsable, runManifest, deployed, supplierOf(in.Catalog)))
		}
	}

	// Sort items deterministically: agents first, then skills, then hooks; within each kind by key.
	sort.Slice(items, func(i, j int) bool {
		oi := artifactKindOrder(items[i].Ref.Kind)
		oj := artifactKindOrder(items[j].Ref.Kind)
		if oi != oj {
			return oi < oj
		}
		return items[i].Ref.Key < items[j].Ref.Key
	})

	return domain.Plan{
		Mode:          in.Mode,
		Harness:       in.Module.Ref(),
		WorkspacePath: in.WorkspacePath,
		Scope:         in.Scope,
		Items:         items,
		Gaps:          gaps,
		Registrations: registrations,
		Workflows:     in.WorkflowIDs,
	}, nil
}

// registrationTargetExists reports whether the workspace file at the given relative path
// exists. When Input.WorkspaceFileExists is non-nil, that injected function is used,
// keeping Build free of direct file-system access. When nil, the target is treated as
// absent and no GapHookRegistration gap is surfaced for it.
func registrationTargetExists(in Input, relPath string) bool {
	if in.WorkspaceFileExists != nil {
		return in.WorkspaceFileExists(relPath)
	}
	return false
}

// artifactKindOrder returns a sort key for artifact kinds so that agents sort before skills
// and skills sort before hooks.
func artifactKindOrder(k domain.ArtifactKind) int {
	switch k {
	case domain.ArtifactAgent:
		return 0
	case domain.ArtifactSkill:
		return 1
	case domain.ArtifactHook:
		return 2
	default:
		return 99
	}
}
