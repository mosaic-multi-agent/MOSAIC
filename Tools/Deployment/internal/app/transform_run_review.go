package app

// transform_run_review.go builds the domain.Plan of a transform run, which the user reviews
// and the executor writes.

import (
	"context"
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// writeItems returns the plan items that will be written: agents, then skills, then hooks.
func (r *transformRun) writeItems() []domain.PlanItem {
	items := r.agentWriteItems()
	items = append(items, r.skillItems()...)
	return append(items, r.hookItems()...)
}

// agentWriteItems returns the plan items of the agents that will be written, in plan order.
func (r *transformRun) agentWriteItems() []domain.PlanItem {
	var items []domain.PlanItem
	for i, pa := range r.plan.Agents {
		st := r.agents[i]
		if !isWrite(pa) || !st.active() {
			continue
		}
		action, reason := domain.ActionCreate, "transform from "+r.srcID
		if pa.Action == PlanWriteOverwrite {
			action, reason = domain.ActionUpdate, "transform from "+r.srcID+" (overwrites the existing file)"
		}
		items = append(items, domain.PlanItem{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: st.result.RefKey},
			SourcePath: pa.SourcePath, TargetPath: pa.DestinationPath, Action: action, Reason: reason,
			Model:         r.modelSelection(pa.SourceModel),
			SourceVersion: st.result.Stamps.Version,
		})
	}
	return items
}

// reviewPlan is the plan shown to the user: the files to write, the files kept unchanged and
// the gaps for skipped files and manual steps.
func (r *transformRun) reviewPlan() domain.Plan {
	p := domain.Plan{
		Mode: domain.ModeTransformHarness, Harness: r.tgt.Ref(),
		WorkspacePath: r.workspace, Scope: domain.ScopeProject,
		Items: r.writeItems(),
	}
	for _, pa := range r.plan.Agents {
		if pa.Action != PlanSkip {
			continue
		}
		switch pa.Skip {
		case SkipDestinationExists, SkipDestinationIsSource:
			p.Items = append(p.Items, domain.PlanItem{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: pa.Key},
				SourcePath: pa.SourcePath, TargetPath: pa.DestinationPath,
				Action: domain.ActionUnchanged, Reason: pa.Reason,
			})
		default:
			p.Gaps = append(p.Gaps, domain.Gap{Kind: domain.GapSkippedFile, Subject: pa.FileName, Detail: pa.Reason})
		}
	}
	p.Gaps = append(p.Gaps, r.skillGaps()...)
	p.Gaps = append(p.Gaps, r.hookGaps()...)
	for _, n := range r.plan.Notes {
		if n.Kind == NoteBothForms {
			p.Gaps = append(p.Gaps, domain.Gap{Kind: domain.GapManualStep, Subject: noteSubject(n), Detail: n.Message})
		}
	}
	return p
}

// noteSubject names a note by the files it concerns.
func noteSubject(n TransformNote) string {
	var subject string
	for _, p := range n.Paths {
		if subject != "" {
			subject += ", "
		}
		subject += filepath.Base(p)
	}
	return subject
}

// confirmPlan shows the plan for review. It returns ErrPlanNotConfirmed when the user cancels
// or declines.
func (r *transformRun) confirmPlan(ctx context.Context) error {
	ans, err := r.s.deps.Interaction.Review(ctx, r.reviewPlan())
	if err != nil {
		return err
	}
	if ans.Status == domain.Cancelled || !ans.Confirm {
		return ErrPlanNotConfirmed
	}
	return nil
}
