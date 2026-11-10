package session

// A block's schedule is read from the Source and Section the workflow declares
// for it. The admitted model records, per block, whether its Source is a seed
// and which rows produce or write it; the session reads those facts and never
// derives them again.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/planstages"
)

// blockSchedulePath is the path of block b's declared Source in the run folder.
func blockSchedulePath(runFolder string, b domain.AdmittedBlock) string {
	return filepath.Join(runFolder, filepath.FromSlash(b.Source))
}

// readBlockSchedule reads block b's schedule from its declared Source and
// Section in the run folder.
func readBlockSchedule(runFolder string, b domain.AdmittedBlock) (domain.BlockSchedule, error) {
	return planstages.ReadBlockSchedule(planstages.ScheduleRequest{
		Path:       blockSchedulePath(runFolder, b),
		Source:     b.Source,
		Section:    b.Section,
		Block:      b.Name,
		Grouped:    b.Grouped,
		Approaches: b.Approaches.Approaches(),
	})
}

// fileExists reports whether path names an existing file or directory.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readStartSchedules reads every block's schedule from its declared Source,
// at run start once seeds are applied and at resume. A block classified
// seeded must have a readable, valid schedule: a defect is returned as a
// *domain.RefusalError. Any other block is read when its Source exists; a
// defective read is a warning and leaves no schedule for it.
func (s *sessionImpl) readStartSchedules(ctx context.Context, rs *runStartCtx) (domain.BlockSchedules, error) {
	var out domain.BlockSchedules
	for _, b := range rs.admitted.Blocks {
		path := blockSchedulePath(rs.config.RunFolder, b)
		if b.Seed != domain.SeedSeeded && !fileExists(path) {
			continue
		}
		sched, err := readBlockSchedule(rs.config.RunFolder, b)
		switch {
		case err == nil:
		case b.Seed == domain.SeedSeeded:
			return nil, seededScheduleRefusal(rs.config, b, err)
		case b.Seed == domain.SeedProduced && !rs.config.IsNewRun:
			handled, refusal := rs.noteResumedDefect(b, err)
			if refusal != nil {
				return nil, refusal
			}
			if !handled {
				s.warnScheduleReread(ctx, b, err)
			}
			continue
		default:
			s.warnScheduleReread(ctx, b, err)
			continue
		}
		if out == nil {
			out = domain.BlockSchedules{}
		}
		out[b.Name] = sched
	}
	return out, nil
}

// seededScheduleRefusal is the refusal of a defective seeded schedule. It
// names the rule, the offending token, the block and the source, and wraps the
// schedule defect. The resource is the run for a resume and the workflow for a
// new run.
func seededScheduleRefusal(config domain.RunConfig, b domain.AdmittedBlock, cause error) *domain.RefusalError {
	return scheduleDefectRefusal(config, b, cause, "seeded schedule")
}

// scheduleDefectRefusal is the refusal of a defective schedule of block b; what
// names the kind of schedule in the reason.
func scheduleDefectRefusal(config domain.RunConfig, b domain.AdmittedBlock, cause error, what string) *domain.RefusalError {
	resource := string(config.WorkflowID)
	if !config.IsNewRun {
		resource = config.RunID
	}
	refusal := &domain.RefusalError{
		Component: "session",
		Resource:  resource,
		Reason: fmt.Sprintf("%s of block %s (source %s, section %q) is defective: %v",
			what, b.Name, b.Source, b.Section, cause),
		Cause: cause,
		Row:   domain.NoWorkflowRow,
	}
	var defect *domain.ScheduleDefectError
	if errors.As(cause, &defect) {
		refusal.Rule = defect.Rule
		refusal.Token = defect.Token
	}
	return refusal
}

// rereadSchedules re-reads the schedules affected by the completed or
// completing row rowIdx and returns the schedules in force. changed is false,
// and stages is returned as it is, when nothing was re-read successfully.
//
// A block is re-read when rowIdx is one of the producing or in-block writer
// rows its admitted entry records. A defective re-read is a warning and the
// previous schedule stays in use.
func (s *sessionImpl) rereadSchedules(
	ctx context.Context,
	admitted domain.AdmittedWorkflow,
	runFolder string,
	rowIdx int,
	stages domain.BlockSchedules,
) (current domain.BlockSchedules, changed bool) {
	res := s.rereadBlocks(ctx, admitted, runFolder, rowIdx, stages, false)
	return res.schedules, res.changed
}

// blockDefect is a defective schedule read of one block that a producing or
// in-block writer row's completed step caused.
type blockDefect struct {
	block domain.AdmittedBlock
	err   error
}

// rereadResult is the outcome of rereadBlocks: the schedules in force, whether
// any block was re-read successfully or dropped, and the defects judged.
type rereadResult struct {
	schedules domain.BlockSchedules
	changed   bool
	defects   []blockDefect
}

// rereadBlocks is rereadSchedules with judgement. With judge set, a defective
// re-read of a block classified produced or seeded after one of its recorded
// producing or in-block writer rows is returned in defects and the block's
// previous schedule is dropped, so it is never used. Otherwise a defective
// re-read is a warning and the previous schedule stays in use.
func (s *sessionImpl) rereadBlocks(
	ctx context.Context,
	admitted domain.AdmittedWorkflow,
	runFolder string,
	rowIdx int,
	stages domain.BlockSchedules,
	judge bool,
) rereadResult {
	res := rereadResult{schedules: stages}
	own := func() {
		if res.changed {
			return
		}
		res.schedules = make(domain.BlockSchedules, len(stages)+1)
		for name, held := range stages {
			res.schedules[name] = held
		}
		res.changed = true
	}
	for _, b := range admitted.Blocks {
		if !rereadsAfterRow(b, rowIdx) {
			continue
		}
		sched, err := readBlockSchedule(runFolder, b)
		switch {
		case err == nil:
			own()
			res.schedules[b.Name] = sched
		case judge && judgedBlock(b, rowIdx):
			own()
			delete(res.schedules, b.Name)
			res.defects = append(res.defects, blockDefect{block: b, err: err})
		default:
			s.warnScheduleReread(ctx, b, err)
		}
	}
	return res
}

// judgedBlock reports whether a defective re-read of block b after row rowIdx
// is acted on: the admitted model records the row as producing or writing its
// Source.
func judgedBlock(b domain.AdmittedBlock, rowIdx int) bool {
	return slices.Contains(b.ProducingRows, rowIdx) || slices.Contains(b.WriterRows, rowIdx)
}

// rereadsAfterRow reports whether row rowIdx causes a re-read of block b's
// schedule: it is a producing or in-block writer row of the block.
func rereadsAfterRow(b domain.AdmittedBlock, rowIdx int) bool {
	return judgedBlock(b, rowIdx)
}

// warnScheduleReread tells the user that a block's schedule could not be
// re-read; the previous schedule, if any, stays in use.
func (s *sessionImpl) warnScheduleReread(ctx context.Context, b domain.AdmittedBlock, err error) {
	msg := fmt.Sprintf("failed to re-read the schedule of block %s from %s: %v", b.Name, b.Source, err)
	s.deps.Interact.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeWarning,
		Message: msg,
	})
}

// stageSourcesFor describes, per declared block, where its schedule is looked
// for, so a stop for a missing schedule can name it.
func stageSourcesFor(admitted domain.AdmittedWorkflow, runFolder string) []domain.StageSource {
	out := make([]domain.StageSource, 0, len(admitted.Blocks))
	for _, b := range admitted.Blocks {
		out = append(out, domain.StageSource{
			Block:  b.Name,
			Source: b.Source,
			Path:   blockSchedulePath(runFolder, b),
			Seeded: b.Seed == domain.SeedSeeded,
		})
	}
	return out
}

// scheduleRefusal turns the error of a start-time schedule read into the
// run's refusal. A refusal of a seeded schedule travels as the outcome's
// cause so frontends can reach the rule, token and defect behind it.
func (s *sessionImpl) scheduleRefusal(err error) domain.RunOutcome {
	var refusal *domain.RefusalError
	if errors.As(err, &refusal) {
		return s.refusalCaused(err.Error(), err)
	}
	return s.refusal(err.Error())
}
