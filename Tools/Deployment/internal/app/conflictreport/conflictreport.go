// Package conflictreport builds the catalog-stack conflict report from shadow records.
//
// Export rule: this package exports only Input and Build (well within the ~15-symbol package
// guideline); everything else is unexported.
//
// Deployed workflows: "deployed workflows" means the union of (a) the workflows this run deploys
// and (b) the workflows already deployed in the target workspace that exist in the resolved
// catalog. The source of (b) is the deployed orchestrator files (Workflow:<id> managed regions),
// never the manifest, which has no workflow entries. Ids in (b) that the resolved catalog does not
// contain are dropped, because no routes can be computed for them. Build takes both id sets as
// plain input and reads neither the workspace nor any deployed file itself.
package conflictreport

import (
	"slices"
	"sort"

	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// Input is everything Build needs; callers supply plain values.
type Input struct {
	// Shadows are the stack's shadow records (stack.Catalog.Shadows()); nil for a plain catalog.
	Shadows []stackrec.Shadow
	// Workflow looks up a resolved workflow by id. Ids it does not know are dropped.
	Workflow func(id string) (domain.Workflow, bool)
	// SelectedWorkflows are the workflow ids this run deploys.
	SelectedWorkflows []string
	// DeployedWorkflows are workflow ids already deployed in the workspace's orchestrator files.
	DeployedWorkflows []string

	// HarnessScoped is true for a run with a target harness. Then a shadow whose Item.Kind is
	// domain.CatalogItemHarnessInjections is kept only when Item.Key == InjectionDir. False
	// keeps every harness-injection shadow (runs without a target harness).
	HarnessScoped bool
	// InjectionDir is the target harness's HarnessInjections directory name; "" when the
	// harness reads no stack injections, which drops every harness-injection shadow when
	// HarnessScoped is true. Ignored when HarnessScoped is false.
	InjectionDir string
}

// Build returns one line per shadow record, ordered by item kind and then key. Severity is
// info when the winner declared the override, warning otherwise. Shadowed agents and the
// orchestrator pair name the deployed workflows they affect.
func Build(in Input) domain.ConflictReport {
	if len(in.Shadows) == 0 {
		return domain.ConflictReport{}
	}
	deployed := resolveDeployed(in)
	lines := make([]domain.ConflictLine, 0, len(in.Shadows))
	for _, s := range in.Shadows {
		if outOfHarnessScope(s, in) {
			continue
		}
		lines = append(lines, buildLine(s, deployed))
	}
	if len(lines) == 0 {
		return domain.ConflictReport{}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		ri, rj := kindRank(lines[i].Item.Kind), kindRank(lines[j].Item.Kind)
		if ri != rj {
			return ri < rj
		}
		return lines[i].Item.Key < lines[j].Item.Key
	})
	return domain.ConflictReport{Lines: lines}
}

// outOfHarnessScope reports whether s is a harness-injection shadow of a directory other than
// the target harness's. Every other shadow kind is always in scope.
func outOfHarnessScope(s stackrec.Shadow, in Input) bool {
	return in.HarnessScoped &&
		s.Item.Kind == domain.CatalogItemHarnessInjections &&
		s.Item.Key != in.InjectionDir
}

func buildLine(s stackrec.Shadow, deployed map[string]domain.Workflow) domain.ConflictLine {
	shadowed := make([]domain.CatalogIdentity, len(s.Shadowed))
	for i, l := range s.Shadowed {
		shadowed[i] = l.Identity
	}
	severity := domain.ConflictWarning
	if s.Declared {
		severity = domain.ConflictInfo
	}
	return domain.ConflictLine{
		Item:              s.Item,
		Winner:            s.Winner.Identity,
		Shadowed:          shadowed,
		Severity:          severity,
		AffectedWorkflows: affected(s.Item, deployed),
	}
}

// resolveDeployed returns the union of selected and already-deployed ids that the catalog knows.
func resolveDeployed(in Input) map[string]domain.Workflow {
	out := map[string]domain.Workflow{}
	if in.Workflow == nil {
		return out
	}
	for _, ids := range [][]string{in.SelectedWorkflows, in.DeployedWorkflows} {
		for _, id := range ids {
			if wf, ok := in.Workflow(id); ok {
				out[id] = wf
			}
		}
	}
	return out
}

func affected(item domain.CatalogItem, deployed map[string]domain.Workflow) []string {
	var ids []string
	switch item.Kind {
	case domain.CatalogItemOrchestratorPair:
		for id := range deployed {
			ids = append(ids, id)
		}
	case domain.CatalogItemAgent:
		for id, wf := range deployed {
			if slices.Contains(wf.ReferencedAgents, item.Key) {
				ids = append(ids, id)
			}
		}
	default:
		return nil
	}
	sort.Strings(ids)
	return ids
}

func kindRank(k domain.CatalogItemKind) int {
	switch k {
	case domain.CatalogItemOrchestratorPair:
		return 0
	case domain.CatalogItemSystemFile:
		return 1
	case domain.CatalogItemHarnessInjections:
		return 2
	case domain.CatalogItemAgent:
		return 3
	case domain.CatalogItemWorkflow:
		return 4
	case domain.CatalogItemSkill:
		return 5
	case domain.CatalogItemHook:
		return 6
	}
	return 7
}
