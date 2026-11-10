package app

// transform_contracts.go declares the public contract for the harness-to-harness transform
// mode: request/result types and per-file status constants. The flow lives in
// transform_service_impl.go and the transform_run_*.go files.

import (
	"context"

	"mosaic-deploy/internal/domain"
)

// TransformModelMapping maps a source-harness model identifier to the target-harness
// model identifier to write for agents carrying that source model.
//
// The key UnsetSourceModel ("") is the group of agents whose source file declares no
// model at all. A present key with an empty value means "this group was answered:
// leave the model unset" and suppresses that group's question; an absent key
// means "not answered" and is resolved by the fallback rules.
type TransformModelMapping map[string]string

// TransformHarnessRequest drives the harness-to-harness transform mode. Following CD-6,
// any field that is set short-circuits its interactive question; an empty field triggers
// the question through domain.Interaction.
type TransformHarnessRequest struct {
	// SourceHarnessID is the harness the deployment was made for. Required.
	SourceHarnessID string
	// TargetHarnessID is the harness to produce. Empty triggers QTransformTargetHarness.
	TargetHarnessID string
	// Workspace is the workspace root whose whole deployment is transformed. Empty triggers
	// QWorkspace.
	Workspace string
	// AutoConfirmPlan skips the review confirmation of the transform plan (CLI --yes).
	AutoConfirmPlan bool
	// TargetModel is the default target model for source models not named in ModelMap.
	// Empty, with no matching ModelMap entry, triggers QTransformTargetModel for that
	// source model; a skipped answer leaves that group without a model.
	// SkipAll[QTransformTargetModel] suppresses all model questions entirely.
	TargetModel string
	// ModelMap pre-answers the per-source-model target selection. Each present key
	// suppresses that source model's question. Keys are source model identifiers;
	// UnsetSourceModel ("") addresses agents with no model set.
	ModelMap TransformModelMapping
	// Overwrite permits rebuilding an existing destination file (never a source file).
	// Without it, an existing destination is skipped and kept.
	Overwrite bool
	// DryRun plans, asks and builds, but shows no review and writes no file.
	DryRun bool
	// SkipAll suppresses the named questions. QTransformTargetModel and QCustomTool are
	// honoured; a target harness is a mandatory input and has no "none, don't ask" answer.
	SkipAll map[domain.QuestionID]bool
}

// TransformFileStatus is the per-file outcome kind.
type TransformFileStatus string

const (
	// StatusTransformed - written (or would be, under DryRun and on ErrPlanNotConfirmed).
	StatusTransformed TransformFileStatus = "transformed"
	// StatusSkipped - the item is not written; Skip and Reason say why.
	StatusSkipped TransformFileStatus = "skipped"
	// StatusFailed - the file could not be rebuilt; Reason says why.
	StatusFailed TransformFileStatus = "failed"
)

// TransformFileOutcome is the result for one planned item.
type TransformFileOutcome struct {
	Kind domain.ArtifactKind `json:"kind"` // agent | skill | hook
	Key  string              `json:"key,omitempty"`
	// SourcePath is the agent file or skill folder; empty for hooks.
	SourcePath string `json:"sourcePath,omitempty"`
	// DestinationPath is workspace-relative: agent file, skill folder or hook dir.
	DestinationPath string              `json:"destinationPath,omitempty"`
	Action          PlanAction          `json:"action"` // planned action: create | overwrite | skip
	Status          TransformFileStatus `json:"status"`
	Skip            TransformSkipKind   `json:"skip,omitempty"`
	// Reason is set for skipped and failed items.
	Reason string `json:"reason,omitempty"`
	// Role, Model, Tools and the tool and field lists below describe a rebuilt agent.
	Role          domain.AgentRole `json:"role,omitempty"`
	Model         string           `json:"model,omitempty"` // written model id; "" = none
	Tools         []string         `json:"tools,omitempty"`
	DroppedTools  []DroppedTool    `json:"droppedTools,omitempty"`
	CustomTools   []string         `json:"customTools,omitempty"`   // servers kept as custom tools
	UnmappedTools []string         `json:"unmappedTools,omitempty"` // generics whose custom-tool question was skipped
	DroppedFields []string         `json:"droppedFields,omitempty"`
	// StrippedFields reports source fields removed from the written agent: harness-specific
	// fields of a non-Markdown source and keys held in the source carriage container.
	StrippedFields []StrippedField `json:"strippedFields,omitempty"`
}

// TransformHarnessResult is the whole-run report, rendered by both frontends.
type TransformHarnessResult struct {
	SourceHarnessID string `json:"sourceHarnessId"`
	TargetHarnessID string `json:"targetHarnessId"`
	Workspace       string `json:"workspace"`
	// SourceAgentsDir is where transform looked for agents.
	SourceAgentsDir string `json:"sourceAgentsDir"`
	DryRun          bool   `json:"dryRun"`
	// Cancelled is true when a question was cancelled or failed or the plan was not
	// confirmed; nothing was written.
	Cancelled bool `json:"cancelled"`
	// TargetModel is the batch-wide target model when exactly one non-empty target model
	// was applied across the whole run; empty otherwise.
	TargetModel string `json:"targetModel,omitempty"`
	// AppliedModelMap records the resolved source-model to target-model mapping the run
	// applied, including groups resolved to no model. Nil when no source model was resolved.
	AppliedModelMap TransformModelMapping `json:"appliedModelMap,omitempty"`
	// UnresolvedModels are the agent keys written without a model key, in plan order.
	UnresolvedModels []string `json:"unresolvedModels,omitempty"`
	// Files lists agents (plan order), then skills, then hooks.
	Files []TransformFileOutcome `json:"files"`
	Notes []TransformNote        `json:"notes,omitempty"`
	// TodoPath is the TODO file written by the executor.
	TodoPath string `json:"todoPath,omitempty"`
	// Transformed, Skipped and Failed count Files by status.
	Transformed int `json:"transformed"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
	// Removals lists the files a GHCP-pair run removes, or would remove. Absent otherwise.
	Removals []TransformFileChange `json:"removals,omitempty"`
	// Replacements lists existing files a GHCP-pair run overwrites, or would overwrite, after
	// backing them up. Absent otherwise.
	Replacements []TransformFileChange `json:"replacements,omitempty"`
}

// TransformFileChange is one file a GHCP-pair transform removes or replaces.
type TransformFileChange struct {
	Kind   domain.ArtifactKind `json:"kind"` // agent | hook
	Key    string              `json:"key,omitempty"`
	Path   string              `json:"path"` // workspace-relative, forward slashes
	Reason string              `json:"reason,omitempty"`
	// BackupPath is the absolute backup file; set only after a real run that backed it up.
	BackupPath string `json:"backupPath,omitempty"`
}

// TransformHarness moves the deployment of a workspace from a source harness to a target
// harness. The flow is in transform_service_impl.go.
func (s *service) TransformHarness(ctx context.Context, req TransformHarnessRequest) (TransformHarnessResult, error) {
	return transformHarness(ctx, s, req)
}
