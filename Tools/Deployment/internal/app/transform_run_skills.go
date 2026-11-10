package app

// transform_run_skills.go carries the planned skills of a transform run: each skill folder is
// read in full before anything is written and then handed to the deploy executor as one plan
// item per file, with the source bytes and the source version.

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// writes reports whether a planned action writes files.
func writes(a PlanAction) bool { return a == PlanWriteCreate || a == PlanWriteOverwrite }

// skillState is the working state of one planned skill during a run.
type skillState struct {
	// files holds the source bytes of every file of the skill, by skill-relative slash path.
	files map[string][]byte
	// failReason is set when a source file could not be read.
	failReason string
}

// active reports whether the skill is still part of the write set.
func (s *skillState) active() bool { return s.failReason == "" }

// prepareSkills reads every file of the skills the plan writes. A skill with an unreadable file
// is marked failed and leaves the write set.
func (r *transformRun) prepareSkills() {
	r.skills = make([]skillState, len(r.plan.Skills))
	for i, sk := range r.plan.Skills {
		if !writes(sk.Action) {
			continue
		}
		st := &r.skills[i]
		st.files = make(map[string][]byte, len(sk.Files))
		for _, f := range sk.Files {
			data, err := os.ReadFile(filepath.Join(sk.SourceDir, filepath.FromSlash(f)))
			if err != nil {
				st.failReason = fmt.Sprintf("cannot read %s: %v", f, err)
				break
			}
			st.files[f] = data
		}
	}
}

// skillWrites is the number of skills that will be written.
func (r *transformRun) skillWrites() int {
	n := 0
	for i, sk := range r.plan.Skills {
		if writes(sk.Action) && r.skills[i].active() {
			n++
		}
	}
	return n
}

// skillItems returns one plan item per file of every skill that will be written.
func (r *transformRun) skillItems() []domain.PlanItem {
	var items []domain.PlanItem
	for i, sk := range r.plan.Skills {
		if !writes(sk.Action) || !r.skills[i].active() {
			continue
		}
		action, reason := domain.ActionCreate, "transform from "+r.srcID
		if sk.Action == PlanWriteOverwrite {
			action, reason = domain.ActionUpdate, "transform from "+r.srcID+" (overwrites the existing file)"
		}
		for _, f := range sk.Files {
			items = append(items, domain.PlanItem{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: sk.Key},
				SourcePath: filepath.Join(sk.SourceDir, filepath.FromSlash(f)),
				TargetPath: slashJoin(sk.DestinationDir, f),
				Action:     action, Reason: reason,
			})
		}
	}
	return items
}

// addSkillContent adds the bytes and version stamp of every skill file to the executor inputs,
// keyed by target path.
func (r *transformRun) addSkillContent(content map[string][]byte, stamps map[string]domain.VersionStamp) {
	for i, sk := range r.plan.Skills {
		if !writes(sk.Action) || !r.skills[i].active() {
			continue
		}
		for _, f := range sk.Files {
			target := slashJoin(sk.DestinationDir, f)
			content[target] = r.skills[i].files[f]
			stamps[target] = domain.VersionStamp{Version: sk.Version}
		}
	}
}

// skillGaps names every skill that is not carried for a reason other than a kept or
// source destination, which the plan already reports as unchanged.
func (r *transformRun) skillGaps() []domain.Gap {
	var gaps []domain.Gap
	for _, sk := range r.plan.Skills {
		if sk.Action == PlanSkip && sk.Skip != SkipDestinationExists && sk.Skip != SkipDestinationIsSource {
			gaps = append(gaps, domain.Gap{
				Kind: domain.GapSkippedFile, Subject: "skill " + sk.Key,
				Detail: "skill " + sk.Key + " was not carried: " + sk.Reason,
			})
		}
	}
	return gaps
}

// skillOutcome is the outcome of one planned skill.
func (r *transformRun) skillOutcome(i int) TransformFileOutcome {
	sk, st := r.plan.Skills[i], r.skills[i]
	out := TransformFileOutcome{
		Kind: domain.ArtifactSkill, Key: sk.Key, SourcePath: sk.SourceDir,
		DestinationPath: sk.DestinationDir, Action: sk.Action,
	}
	switch {
	case sk.Action == PlanSkip:
		out.Status, out.Skip, out.Reason = StatusSkipped, sk.Skip, sk.Reason
	case !st.active():
		out.Status, out.Reason = StatusFailed, st.failReason
	default:
		out.Status = StatusTransformed
	}
	return out
}
