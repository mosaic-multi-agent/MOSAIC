package domain

import (
	"strings"

	"mosaic-common/interaction"
)

// Guidance is the run's standing guidance: text appended to engine-routed
// dispatches' task_description and constraints. Either field may be empty.
type Guidance struct {
	TaskDescription string
	Constraints     string
}

// Normalized returns g with "\r\n" and lone "\r" replaced by "\n", and trailing
// "\n" characters removed from each field. Leading and inner text is unchanged.
func (g Guidance) Normalized() Guidance {
	return Guidance{
		TaskDescription: normalizeGuidanceField(g.TaskDescription),
		Constraints:     normalizeGuidanceField(g.Constraints),
	}
}

func normalizeGuidanceField(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

// GuidanceChange is the change field of session.guidance.changed.
type GuidanceChange string

const (
	GuidanceLoaded      GuidanceChange = "loaded" // read from the file at session start
	GuidanceAccepted    GuidanceChange = "accepted"
	GuidanceEdited      GuidanceChange = "edited" // edited at the run-start review
	GuidanceRegenerated GuidanceChange = "regenerated"
	GuidanceEditedInRun GuidanceChange = "edited-in-run"
)

// GuidanceStore reads and writes a run's standing-guidance file.
// Implementations never panic.
type GuidanceStore interface {
	// Load reads the guidance file of runFolder. found is false, with a nil
	// error, when the file does not exist. A file that exists but cannot be
	// read or parsed returns a non-nil error that names the file.
	Load(runFolder string) (g Guidance, found bool, err error)

	// Save replaces the guidance file of runFolder with g, written in the
	// guidance file format. The replacement is atomic: a concurrent Load never
	// sees a partial file. The returned error names the file.
	Save(runFolder string, g Guidance) error
}

// Question IDs of the run-start guidance review.
const (
	QuestionGuidanceReview      interaction.QuestionID = "guidance.review"
	QuestionGuidanceTask        interaction.QuestionID = "guidance.task_description"
	QuestionGuidanceConstraints interaction.QuestionID = "guidance.constraints"
)

// Option IDs of the guidance review question.
const (
	GuidanceOptionAccept     = "accept"     // label "Accept"
	GuidanceOptionEdit       = "edit"       // label "Edit"
	GuidanceOptionRegenerate = "regenerate" // label "Regenerate"; offered only for saved guidance
)
