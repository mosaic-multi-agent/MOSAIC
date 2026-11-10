package app

// transform_run_harness.go resolves the first inputs of a transform run: the source and target
// harness modules, the workspace root and the plan.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/pathinput"
)

// resolveHarnesses resolves the source module, asks for the target harness when the request
// names none, and resolves the target module. A target equal to the source fails before any
// further question.
func (r *transformRun) resolveHarnesses(ctx context.Context) error {
	req := r.req
	if req.SourceHarnessID != "" && req.TargetHarnessID != "" && req.SourceHarnessID == req.TargetHarnessID {
		return ErrTransformSameHarness
	}
	src, err := r.s.deps.Registry.Resolve(req.SourceHarnessID)
	if err != nil {
		return fmt.Errorf("cannot resolve source harness %q: %w", req.SourceHarnessID, err)
	}
	r.src = src

	r.tgtID = req.TargetHarnessID
	if r.tgtID == "" {
		ans, askErr := r.s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
			Question: domain.Question{ID: domain.QTransformTargetHarness, Title: "Select target harness"},
			Options:  r.targetHarnessOptions(),
		})
		if askErr != nil || ans.Status != domain.Answered {
			return ErrSelectionCancelled
		}
		r.tgtID = ans.OptionID
	}
	if r.tgtID == req.SourceHarnessID {
		return ErrTransformSameHarness
	}
	tgt, err := r.s.deps.Registry.Resolve(r.tgtID)
	if err != nil {
		return fmt.Errorf("cannot resolve target harness %q: %w", r.tgtID, err)
	}
	r.tgt = tgt
	return nil
}

// targetHarnessOptions offers one option per registry harness. The source harness is included
// but disabled so the overlay is never empty on a single-harness registry.
func (r *transformRun) targetHarnessOptions() []domain.Option {
	harnesses := r.s.deps.Registry.List()
	opts := make([]domain.Option, 0, len(harnesses))
	for _, h := range harnesses {
		opt := domain.Option{ID: h.ID, Label: h.DisplayName}
		if h.ID == r.req.SourceHarnessID {
			opt.Disabled, opt.DisabledReason = true, "already the transform source"
		} else if !h.Usable {
			opt.Disabled, opt.DisabledReason = true, h.UnusableReason
		}
		opts = append(opts, opt)
	}
	return opts
}

// resolveWorkspace takes the workspace root from the request or asks for it, and makes it
// absolute. A cancelled or failed answer is a cancelled run.
func (r *transformRun) resolveWorkspace(ctx context.Context) error {
	ws := r.req.Workspace
	if ws == "" {
		ans, err := r.s.deps.Interaction.AskText(ctx, domain.TextQuestion{
			Question: domain.Question{ID: domain.QWorkspace, Title: "Workspace path"},
		})
		if err != nil || ans.Status != domain.Answered {
			return ErrSelectionCancelled
		}
		ws = pathinput.Unquote(strings.TrimSpace(ans.Text))
		if ws == "" {
			return ErrSelectionCancelled
		}
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return fmt.Errorf("cannot resolve workspace %q: %w", ws, err)
	}
	r.workspace = abs
	return nil
}

// makePlan computes the tool-mappings version stamped into the rebuilt agents and the whole
// plan. It asks nothing and writes nothing.
func (r *transformRun) makePlan() error {
	toolCfg, _ := r.s.deps.ToolConfig.Load()
	userCfg, err := r.s.deps.UserConfig.Load()
	if err != nil {
		return err
	}
	r.toolMappingsVersion = config.HashToolDestinations(toolCfg.ToolDestinations, userCfg.ToolDestinations)

	plan, err := r.s.planTransform(transformPlanInput{
		Workspace: r.workspace, Source: r.src, Target: r.tgt, Overwrite: r.req.Overwrite,
	})
	if err != nil {
		return err
	}
	r.plan = plan
	return nil
}
