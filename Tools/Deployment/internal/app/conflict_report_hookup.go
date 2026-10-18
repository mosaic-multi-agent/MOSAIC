package app

// conflict_report_hookup.go connects the catalog-stack conflict report to the write-planning
// paths and to render. The report is built before anything is written, from the stack's shadow
// records, the workflows the run deploys, and the workflows already embedded in the workspace's
// deployed orchestrator files. In an auto-confirmed run each warning line is also recorded as a
// catalog-shadowing gap so the TODO output keeps it.

import (
	"fmt"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/app/conflictreport"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/plan"
)

// conflictReportFor builds the report for a run that deploys selected workflows into a workspace
// that already embeds deployed ones. When autoConfirm is set, every warning line adds one gap to
// the run's TODO collector, in report order. A catalog that is not a stack has no shadow records.
// A run with a target harnessID reports harness-injection shadows only for that harness's
// injection directory; an empty harnessID, or a registry that cannot name the directory, leaves
// the report unscoped.
func (s *service) conflictReportFor(harnessID string, selected, deployed []string, autoConfirm bool) domain.ConflictReport {
	st, ok := s.deps.Catalog.(stack.Catalog)
	if !ok {
		return domain.ConflictReport{}
	}
	in := conflictreport.Input{
		Shadows:           st.Shadows(),
		Workflow:          s.deps.Catalog.Workflow,
		SelectedWorkflows: selected,
		DeployedWorkflows: deployed,
	}
	if rep, ok := s.deps.Registry.(registry.InjectionDirReporter); ok && harnessID != "" {
		in.HarnessScoped = true
		in.InjectionDir, _ = rep.InjectionDirName(harnessID)
	}
	report := conflictreport.Build(in)
	if autoConfirm {
		for _, line := range report.Lines {
			if line.Severity == domain.ConflictWarning {
				s.deps.Todo.AddGap(shadowingGap(line))
			}
		}
	}
	return report
}

// withConflictReport returns summary carrying report.
func withConflictReport(summary domain.RunSummary, report domain.ConflictReport) domain.RunSummary {
	summary.ConflictReport = report
	return summary
}

// renderConflictReport returns the report of a render that embeds the given workflows, or nil
// when nothing is shadowed. Render reads no workspace, so it has no deployed workflows.
func (s *service) renderConflictReport(harnessID string, rendered []string) *domain.ConflictReport {
	report := s.conflictReportFor(harnessID, rendered, nil, false)
	if len(report.Lines) == 0 {
		return nil
	}
	return &report
}

// shadowingGap converts a warning line into its run-level TODO gap.
func shadowingGap(line domain.ConflictLine) domain.Gap {
	subject := string(line.Item.Kind)
	if line.Item.Key != "" {
		subject += ":" + line.Item.Key
	}
	shadowed := make([]string, len(line.Shadowed))
	for i, id := range line.Shadowed {
		shadowed[i] = id.ID + " " + id.Version
	}
	detail := fmt.Sprintf("from %s %s shadows %s", line.Winner.ID, line.Winner.Version, strings.Join(shadowed, ", "))
	if len(line.AffectedWorkflows) > 0 {
		detail += "; affects workflows: " + strings.Join(line.AffectedWorkflows, ", ")
	}
	return domain.Gap{Kind: domain.GapCatalogShadowing, Subject: subject, Detail: detail + "; undeclared override"}
}

// orchestratorTargetPaths returns the target paths of the orchestrator and, when the catalog has
// one, the script orchestrator. includeOrchestrator false leaves out the first.
func (s *service) orchestratorTargetPaths(module domain.HarnessModule, scope domain.Scope, includeOrchestrator bool) []string {
	agents := []domain.Agent{}
	if includeOrchestrator {
		agents = append(agents, s.deps.Catalog.Orchestrator())
	}
	if script, ok := s.deps.Catalog.OrchestratorScript(); ok {
		agents = append(agents, script)
	}
	var paths []string
	for _, a := range agents {
		p, err := module.TargetPath(domain.TargetPathRequest{
			Kind: domain.ArtifactAgent, Key: a.Key, FileName: filepath.Base(a.SourcePath), Scope: scope, GOOS: s.deps.GOOS,
		})
		if err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

// deployedWorkflowIDs reads the workflow ids embedded in the workspace's deployed script
// orchestrator and, when includeOrchestrator is set, the deployed orchestrator. A missing file
// contributes nothing. The manifest is never consulted.
func (s *service) deployedWorkflowIDs(module domain.HarnessModule, workspace string, scope domain.Scope, includeOrchestrator bool) []string {
	var ids []string
	for _, p := range s.orchestratorTargetPaths(module, scope, includeOrchestrator) {
		state := probeDeployedArtifact(workspace, p, module.Descriptor().Frontmatter.ModelKey)
		ids = unionPreserveOrder(ids, state.Workflows.IDs())
	}
	return ids
}

// plannedWorkflowIDs returns the workflow ids embedded in the deployed orchestrator files among
// the planned paths, taken from the deployed state already probed for the run.
func (s *service) plannedWorkflowIDs(paths plan.PlannedPaths, state map[string]domain.DeployedArtifactState) []string {
	keys := []string{s.deps.Catalog.Orchestrator().Key}
	if script, ok := s.deps.Catalog.OrchestratorScript(); ok {
		keys = append(keys, script.Key)
	}
	var ids []string
	for _, k := range keys {
		if p, ok := paths.Path(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: k}); ok {
			ids = unionPreserveOrder(ids, state[p].Workflows.IDs())
		}
	}
	return ids
}
