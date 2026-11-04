package app

// transform_plan.go holds the planning seam of transform.

import (
	"errors"
	"fmt"
	"os"

	"mosaic-deploy/internal/domain"
)

// errTransformPlanNotImplemented is retained for callers that distinguish it from a
// workspace-read failure.
var errTransformPlanNotImplemented = errors.New("transform planning is not implemented")

// transformPlanInput is the input of planTransform.
type transformPlanInput struct {
	Workspace string // absolute workspace root
	Source    domain.HarnessModule
	Target    domain.HarnessModule
	Overwrite bool
}

// planTransform decides every source, destination and action of a transform run. It reads
// the filesystem, writes nothing and asks nothing.
func (s *service) planTransform(in transformPlanInput) (TransformPlan, error) {
	plan := TransformPlan{Workspace: in.Workspace}
	if info, err := os.Stat(in.Workspace); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return plan, fmt.Errorf("read workspace %s: %w", in.Workspace, err)
	}
	if err := s.planAgents(in, &plan); err != nil {
		return plan, fmt.Errorf("plan agents: %w", err)
	}
	if err := s.planSkills(in, &plan); err != nil {
		return plan, fmt.Errorf("plan skills: %w", err)
	}
	s.planHooks(in, &plan)
	return plan, nil
}
