package app

// promote_batch_types.go declares the contract of Service.PromoteBatch: its request and result
// types, the option IDs of the existing-agent question and the pre-run error sentinels.
// Behaviour is documented on Service.PromoteBatch (service.go) and below.

import "errors"

// PromoteBatchRequest drives Service.PromoteBatch. Every question of the batch is asked
// through Interaction; there are no pre-answers.
type PromoteBatchRequest struct {
	// HarnessID names the harness that produced the agent files. Required.
	HarnessID string
	// AgentsDir is the folder holding the harness agent files (not a workspace root).
	// Required; read non-recursively; never written.
	AgentsDir string
}

// PromoteBatchResult accounts for every agent selected in QPromoteAgents.
type PromoteBatchResult struct {
	// HarnessID echoes the request.
	HarnessID string
	// AgentsDir echoes the request.
	AgentsDir string
	// TargetCatalogID is the id of the layer that received the batch; empty for a plain
	// catalog, and empty when the batch was aborted before the layer was chosen.
	TargetCatalogID string
	// TargetLayer is the layer label of the receiving layer ("" when not chosen).
	TargetLayer string
	// Aborted reports that a question was cancelled (or Interaction/ctx failed) after the
	// selection; agents without a settled outcome at that point are NotProcessed.
	Aborted bool
	// Agents holds one entry per selected option, in option (selection) order.
	Agents []PromoteBatchAgentResult
}

// PromoteBatchStatus is the outcome of one selected agent.
type PromoteBatchStatus string

const (
	PromoteBatchPromoted     PromoteBatchStatus = "promoted"
	PromoteBatchSkipped      PromoteBatchStatus = "skipped"
	PromoteBatchFailed       PromoteBatchStatus = "failed"
	PromoteBatchNotProcessed PromoteBatchStatus = "not-processed"
)

// PromoteBatchAgentResult is the outcome of one selected agent.
type PromoteBatchAgentResult struct {
	// SourcePath is the absolute path of the selected file. Never modified.
	SourcePath string
	// Key is the agent key derived from the file name.
	Key string
	// Status is the outcome.
	Status PromoteBatchStatus
	// Reason is the human-readable skip reason (Skipped) or the error text (Failed);
	// empty for Promoted and NotProcessed.
	Reason string
	// Err is the error for Failed; nil otherwise. errors.Is works on it.
	Err error
	// ReplacedLayer is the layer label of the agent this one replaced; empty when the
	// promotion created a new agent.
	ReplacedLayer string
	// Result is the single-agent result for Promoted (same meaning as PromoteResult from
	// Service.Promote, DryRun false); zero value otherwise.
	Result PromoteResult
}

// Option IDs of QPromoteExistingAgent.
const (
	PromoteExistingSkip    = "skip"
	PromoteExistingReplace = "replace"
)

// ErrPromoteAgentsDirRequired reports a PromoteBatchRequest with an empty AgentsDir.
var ErrPromoteAgentsDirRequired = errors.New("promote requires an agents folder")

// ErrPromoteAgentsDirInvalid reports an AgentsDir that does not exist or is not a directory.
var ErrPromoteAgentsDirInvalid = errors.New("promote agents folder does not exist or is not a directory")

// ErrPromoteNoPromotableAgents reports an agents folder with no promotable agent. The
// wrapping error names the folder: "no promotable agents found in <folder>".
var ErrPromoteNoPromotableAgents = errors.New("no promotable agents found")
