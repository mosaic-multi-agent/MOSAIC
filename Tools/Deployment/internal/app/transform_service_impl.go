package app

// transform_service_impl.go implements Service.TransformHarness: it plans the whole
// deployment of a workspace root before asking or writing anything, asks the model and
// custom-tool questions for the files that will be written, rebuilds every agent in memory,
// shows the plan for review and writes through the deploy executor in one atomic run.
// Obsolete behaviour (single files, loose folders, empty model keys) does not exist here.

import (
	"context"
	"errors"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// transformRun is the state of one transform run.
type transformRun struct {
	s   *service
	req TransformHarnessRequest

	src, tgt            domain.HarnessModule
	srcID, tgtID        string
	workspace           string
	toolMappingsVersion string
	plan                TransformPlan
	dropped             []domain.DroppedEntry // manifest entries dropped before planning
	targetManifest      manifest.Snapshot     // reconciled manifest narrowed to the target harness
	sourceManifest      manifest.Snapshot     // reconciled manifest narrowed to the source harness

	groupModels     TransformModelMapping // target model per source model group; "" = unresolved
	customTools     map[string]string     // custom-tool answers: generic -> server
	skippedTools    map[string]bool       // generics whose custom-tool question was skipped
	declaredServers []string              // servers declared in the source harness's MCP configuration
	mcpSplits       map[string]string     // answered and pre-answered MCP name splits: source name -> server
	agents          []agentState          // parallel to plan.Agents
	inspections     []DeployedAgentInspection
	skills          []skillState // parallel to plan.Skills
	hooks           []hookState  // parallel to plan.Hooks
}

// close releases the harness modules the run resolved.
func (r *transformRun) close() {
	if r.src != nil {
		r.src.Close()
	}
	if r.tgt != nil {
		r.tgt.Close()
	}
}

// transformHarness is the implementation of Service.TransformHarness.
func transformHarness(ctx context.Context, s *service, req TransformHarnessRequest) (TransformHarnessResult, error) {
	r := &transformRun{s: s, req: req, srcID: req.SourceHarnessID}
	defer r.close()
	res, err := r.run(ctx)
	if err == nil {
		return res, nil
	}
	return r.failureResult(err), err
}

// run executes the steps of the flow in their observable order.
func (r *transformRun) run(ctx context.Context) (TransformHarnessResult, error) {
	if err := r.resolveHarnesses(ctx); err != nil {
		return TransformHarnessResult{}, err
	}
	if err := r.resolveWorkspace(ctx); err != nil {
		return TransformHarnessResult{}, err
	}
	if err := r.refuseBothGhcpTargets(); err != nil {
		return TransformHarnessResult{}, err
	}
	r.reconcileManifest()
	if err := r.makePlan(); err != nil {
		return TransformHarnessResult{}, err
	}
	if err := r.resolveModels(ctx); err != nil {
		return TransformHarnessResult{}, err
	}
	r.prepareAgents()
	if err := r.resolveCustomTools(ctx); err != nil {
		return TransformHarnessResult{}, err
	}
	if err := r.resolveMCPSplits(ctx); err != nil {
		return TransformHarnessResult{}, err
	}
	r.buildAgents()
	r.prepareSkills()
	r.prepareHooks()

	if r.req.DryRun || r.writeCount() == 0 {
		return r.result(""), nil
	}
	if !r.req.AutoConfirmPlan {
		if err := r.confirmPlan(ctx); err != nil {
			return TransformHarnessResult{}, err
		}
	}
	exec, err := r.execute(ctx)
	if err != nil {
		return TransformHarnessResult{}, err
	}
	res := r.result(exec.TodoFilePath)
	res.recordBackups(exec)
	return res, nil
}

// failureResult is the result that accompanies a run-level error. Errors before a run exists
// (same harness, unknown harness, unreadable workspace) return the zero result; the cancel
// errors and a failed write carry the identifiers known; a declined plan also carries the
// plan that was shown.
func (r *transformRun) failureResult(err error) TransformHarnessResult {
	switch {
	case errors.Is(err, ErrPlanNotConfirmed):
		res := r.result("")
		res.Cancelled = true
		return res
	case errors.Is(err, ErrSelectionCancelled), errors.Is(err, ErrModelSelectionCancelled):
		res := r.identifiers()
		res.Cancelled = true
		return res
	case errors.Is(err, ErrRunReverted):
		return r.identifiers()
	}
	return TransformHarnessResult{}
}
