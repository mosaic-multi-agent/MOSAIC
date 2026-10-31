package app

// transform_run_exec.go writes the built agents through the deploy executor in one atomic run,
// which records the manifest and the TODO file.

import (
	"context"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

// collectGaps hands the gaps of the plan and of the rebuilds to the TODO collector.
func (r *transformRun) collectGaps(plan domain.Plan) {
	for _, g := range plan.Gaps {
		r.s.deps.Todo.AddGap(g)
	}
	for i := range r.agents {
		if r.agents[i].built && r.agents[i].active() {
			for _, g := range r.agents[i].result.Gaps {
				r.s.deps.Todo.AddGap(g)
			}
		}
	}
}

// execute writes the built agents. A failure reverses every file the run wrote and returns a
// *RevertedRunError.
func (r *transformRun) execute(ctx context.Context) (deploy.ExecResult, error) {
	plan := r.reviewPlan()
	r.collectGaps(plan)

	content := make(map[string][]byte)
	stamps := make(map[string]domain.VersionStamp)
	for i, pa := range r.plan.Agents {
		if r.agents[i].built && r.agents[i].active() {
			content[pa.DestinationPath] = r.agents[i].result.Output
			stamps[pa.DestinationPath] = r.agents[i].result.Stamps
		}
	}
	plan.Items = r.writeItems()
	r.addSkillContent(content, stamps)
	hooks := r.hookPlans(stamps)
	if err := r.checkHookDestinations(); err != nil {
		return deploy.ExecResult{}, &RevertedRunError{Cause: err}
	}

	res, err := r.s.deps.Executor.Execute(ctx, deploy.ExecRequest{
		Plan:       plan,
		MosaicRoot: r.s.deps.MosaicRoot,
		Content: func(item domain.PlanItem) ([]byte, error) {
			return content[item.TargetPath], nil
		},
		VersionStamps: stamps,
		Hooks:         hooks,
		TodoItems:     r.s.deps.Todo.Items,
		TodoMeta: todo.Meta{
			Harness: r.tgt.Ref().DisplayName, WorkspacePath: r.workspace, DeploymentRoot: r.workspace,
			GeneratedAt: r.s.now(), Mode: domain.ModeTransformHarness,
		},
		Atomic:      true,
		ToolVersion: r.s.deps.ToolVersion,
	})
	if err != nil {
		return res, err
	}
	if res.Reverted {
		unrestored := make([]string, 0, len(res.RevertFailures))
		for _, rf := range res.RevertFailures {
			unrestored = append(unrestored, rf.Path)
		}
		return res, &RevertedRunError{Cause: res.Partial, UnrestoredPaths: unrestored}
	}
	return res, res.Partial
}
