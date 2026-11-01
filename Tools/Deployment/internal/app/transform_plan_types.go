package app

// transform_plan_types.go declares the data contract of transform planning: the plan of
// every source agent, skill and hook bundle with its destination, action, skip reason and the
// notes the run carries. Planning reads the workspace, writes nothing and asks nothing.

import "mosaic-deploy/internal/domain"

// PlanAction is what transform will do with one planned item.
type PlanAction string

const (
	PlanWriteCreate    PlanAction = "create"
	PlanWriteOverwrite PlanAction = "overwrite"
	PlanSkip           PlanAction = "skip"
)

// TransformSkipKind names why a planned item is not written.
type TransformSkipKind string

const (
	SkipNotAgent            TransformSkipKind = "not-agent"
	SkipTargetExtension     TransformSkipKind = "target-extension"
	SkipDuplicateKey        TransformSkipKind = "duplicate-key"
	SkipDestinationIsSource TransformSkipKind = "destination-is-source"
	SkipDestinationExists   TransformSkipKind = "destination-exists"
	SkipUnsupported         TransformSkipKind = "unsupported"
	SkipNoTargetVariant     TransformSkipKind = "no-target-variant"
	SkipSharedHooksFolder   TransformSkipKind = "shared-hooks-folder"
	// SkipNotRecorded - GHCP pair only: the agent file has no manifest record, so the pair
	// transform neither writes, replaces nor removes it.
	SkipNotRecorded TransformSkipKind = "not-recorded"
)

// TransformNoteKind names a run-level note of the plan.
type TransformNoteKind string

const (
	NoteBothForms          TransformNoteKind = "both-forms"
	NoteNewerTargetContent TransformNoteKind = "newer-target-content"
	NoteNoAgentsFound      TransformNoteKind = "no-agents-found"
)

// TransformNote is one run-level note.
type TransformNote struct {
	Kind    TransformNoteKind `json:"kind"`
	Message string            `json:"message"`
	Paths   []string          `json:"paths,omitempty"`
}

// PlannedAgent is one .md file of the source agents folder with its planned outcome.
type PlannedAgent struct {
	SourcePath      string
	FileName        string
	Key             string
	DestinationPath string
	Action          PlanAction
	Skip            TransformSkipKind
	Reason          string
	SourceModel     string
}

// PlannedSkill is one skill folder of the source deployment.
type PlannedSkill struct {
	Key            string
	SourceDir      string
	DestinationDir string
	Files          []string
	Version        string
	Action         PlanAction
	Skip           TransformSkipKind
	Reason         string
}

// PlannedHookBundle is one hook bundle present in the source deployment.
type PlannedHookBundle struct {
	Key    string
	Plan   domain.HookPlan
	Action PlanAction
	Skip   TransformSkipKind
	Reason string
}

// TransformPlan is everything a transform run would do, decided before any question.
type TransformPlan struct {
	Workspace       string
	SourceAgentsDir string
	Agents          []PlannedAgent
	Skills          []PlannedSkill
	Hooks           []PlannedHookBundle
	Notes           []TransformNote
	// Removals and Replacements are filled for the GHCP pair only; nil otherwise.
	Removals     []domain.Removal
	Replacements []domain.Replacement
}

// SourceModels returns the distinct SourceModel values of agents whose action writes, in
// first-seen order.
func (p TransformPlan) SourceModels() []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range p.Agents {
		if a.Action != PlanWriteCreate && a.Action != PlanWriteOverwrite {
			continue
		}
		if !seen[a.SourceModel] {
			seen[a.SourceModel] = true
			out = append(out, a.SourceModel)
		}
	}
	return out
}
