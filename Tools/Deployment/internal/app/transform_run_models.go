package app

// transform_run_models.go resolves the target model of every source model group that has an
// agent to write: pre-answers first, then one question per remaining group.

import (
	"context"
	"fmt"

	"mosaic-deploy/internal/domain"
)

// titleForSourceModel returns the title of the model question of one source model group, so the
// user can tell which group is being mapped.
func titleForSourceModel(sourceModel string) string {
	if sourceModel == UnsetSourceModel {
		return "Select target model for agents with no model set (s to skip)"
	}
	return fmt.Sprintf("Select target model for source model %q (s to skip)", sourceModel)
}

// resolveModels fills r.groupModels with the target model of each source model group. An empty
// value means the group stays without a model. A cancelled or failed question aborts the run.
func (r *transformRun) resolveModels(ctx context.Context) error {
	r.groupModels = TransformModelMapping{}
	skippedAll := false
	for _, group := range r.plan.SourceModels() {
		if target, ok := r.req.ModelMap[group]; ok {
			r.groupModels[group] = target
			continue
		}
		if r.req.TargetModel != "" {
			r.groupModels[group] = r.req.TargetModel
			continue
		}
		if skippedAll || r.req.SkipAll[domain.QTransformTargetModel] {
			r.groupModels[group] = ""
			continue
		}
		target, latch, err := r.askModel(ctx, group)
		if err != nil {
			return err
		}
		r.groupModels[group] = target
		skippedAll = latch
	}
	return nil
}

// askModel asks the model question of one group. It returns the chosen model (empty when the
// group is skipped) and whether the user chose to skip this and every later group.
func (r *transformRun) askModel(ctx context.Context, group string) (string, bool, error) {
	ans, err := r.s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
		Question: domain.Question{
			ID: domain.QTransformTargetModel, Subject: group, Title: titleForSourceModel(group),
			AllowSkip: true, AllowSkipAll: true, AllowCustom: true,
		},
		Options: buildModelOptions(r.tgt.Descriptor().Models.IDs),
	})
	if err != nil {
		return "", false, ErrModelSelectionCancelled
	}
	switch ans.Status {
	case domain.Cancelled:
		return "", false, ErrModelSelectionCancelled
	case domain.SkippedAll:
		return "", true, nil
	case domain.Answered:
		if ans.Custom != "" {
			return ans.Custom, false, nil
		}
		return ans.OptionID, false, nil
	}
	return "", false, nil
}

// modelSelection is the rebuild input for a source model group.
func (r *transformRun) modelSelection(group string) domain.ModelSelection {
	id := r.groupModels[group]
	if id == "" {
		return domain.ModelSelection{Origin: domain.OriginUnresolved}
	}
	for _, known := range r.tgt.Descriptor().Models.IDs {
		if known == id {
			return domain.ModelSelection{ModelID: id, Origin: domain.OriginHarnessList}
		}
	}
	return domain.ModelSelection{ModelID: id, Origin: domain.OriginCustom}
}
