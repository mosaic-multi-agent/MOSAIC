package engine

import (
	"errors"
	"fmt"

	"mosaic-run/internal/domain"
)

// Sentinel causes of ResolveRowDefaultsForIteration iteration argument errors.
var (
	ErrRowDefaultsStageRequired       = errors.New("staged row requires a stage")
	ErrRowDefaultsStageNotInSet       = errors.New("stage is not in the stage set")
	ErrRowDefaultsStageOnNonStagedRow = errors.New("non-staged row cannot take a stage")
)

// ResolveRowDefaultsForIteration resolves the default dispatch payload of row
// at iteration, exactly as an engine-routed dispatch does: inputs and outputs
// through ResolveArtifactsForIteration, HITL = row.HITL || the iteration's HITL
// in the schedule of the row's block. iteration is "" for a row outside every
// block. Paths are bare (no run prefix).
//
// The row's block is row.Block(). The iteration is looked up in that block's
// schedule ignoring case and is used in paths exactly as passed.
func ResolveRowDefaultsForIteration(
	row domain.RoutingRow,
	iteration domain.IterationID,
	sc ScheduleContext,
) (domain.DispatchDefaults, error) {
	block, inBlock := row.Block()
	if inBlock {
		if iteration == "" {
			return domain.DispatchDefaults{}, ErrRowDefaultsStageRequired
		}
		sched, ok := sc.Schedules[block]
		if !ok {
			return domain.DispatchDefaults{}, ErrRowDefaultsStageNotInSet
		}
		if _, found := sched.Find(iteration); !found {
			return domain.DispatchDefaults{}, ErrRowDefaultsStageNotInSet
		}
	} else if iteration != "" {
		return domain.DispatchDefaults{}, ErrRowDefaultsStageOnNonStagedRow
	}
	return resolveDefaults(row, block, iteration, sc)
}

// resolveDefaults is the one routine behind both the engine's dispatch step and
// ResolveRowDefaultsForIteration. It tolerates a missing schedule the way the
// engine always has. block is "" for a row outside every block.
func resolveDefaults(
	row domain.RoutingRow,
	block domain.BlockName,
	iteration domain.IterationID,
	sc ScheduleContext,
) (domain.DispatchDefaults, error) {
	hitl := row.HITL
	if block != "" && iteration != "" {
		if sched, ok := sc.Schedules[block]; ok {
			if it, found := sched.Find(iteration); found {
				hitl = hitl || it.HITL
			}
		}
	}
	inputs, err := ResolveArtifactsForIteration(row.InputArtifacts, block, iteration, sc, true)
	if err != nil {
		return domain.DispatchDefaults{}, fmt.Errorf("row for agent %q: input artifacts: %w", row.Agent, err)
	}
	outputs, err := ResolveArtifactsForIteration(row.OutputArtifacts, block, iteration, sc, false)
	if err != nil {
		return domain.DispatchDefaults{}, fmt.Errorf("row for agent %q: output artifacts: %w", row.Agent, err)
	}
	return domain.DispatchDefaults{InputArtifacts: inputs, OutputArtifacts: outputs, HITL: hitl}, nil
}
