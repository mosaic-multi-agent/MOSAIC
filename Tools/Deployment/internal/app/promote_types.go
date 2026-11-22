package app

// promote_types.go declares the request and result types of Service.Promote.

import "mosaic-deploy/internal/domain"

// PromoteRequest carries the caller's pre-answers for the promote flow. A set field is
// used directly without asking; an unset field causes the flow to ask through Interaction.
type PromoteRequest struct {
	// FilePath is the harness-only agent file to promote. Required; there is no
	// interactive fallback for it.
	FilePath string
	// Category is the destination placement. An empty value causes QPromoteCategory to
	// be asked. A value of PromoteCategoryUtility places the file under
	// Catalog/UtilityAgents/; any other value names a subdirectory under
	// Catalog/Subagents/.
	Category string
	// Overwrite is explicit consent to replace an existing file at the destination
	// path. Without it a collision is refused.
	Overwrite bool
	// DryRun computes and validates everything but writes no file.
	DryRun bool
	// HarnessID names the harness that produced the deployed file being promoted. It is
	// required and has no interactive fallback: the TUI supplies its already-selected
	// harness and the CLI supplies the --harness flag. An empty value is
	// ErrPromoteHarnessRequired; a value naming no registered harness is
	// ErrPromoteHarnessUnresolvable. Neither writes a file.
	HarnessID string
	// TargetCatalogID names the stack layer that receives a NEW item (no resolved source).
	// Empty means ask (interactive) or error (non-interactive) when the stack has two or
	// more layers.
	TargetCatalogID string
	// EmptyProjectRegions empties every project region of the generated generic instead of
	// keeping its content (the default). Never asked; the CLI sets it from
	// --empty-project-regions.
	EmptyProjectRegions bool
}

// PromoteResult describes what a successful promote produced. It is returned populated only
// when Service.Promote returns a nil error; on any failure it is the zero value.
type PromoteResult struct {
	// SourcePath is the harness-only file that was read. It is never modified or deleted.
	SourcePath string `json:"sourcePath"`
	// DestinationPath is the generic file written, relative to the MOSAIC root.
	DestinationPath string `json:"destinationPath"`
	// Key is the agent key the catalog will derive from the written file.
	Key string `json:"key"`
	// NumericID is the id assigned to the new agent.
	NumericID string `json:"numericId"`
	// Category is the resolved placement, either PromoteCategoryUtility or a
	// subdirectory name under Catalog/Subagents/.
	Category string `json:"category"`
	// DryRun reports that nothing was written.
	DryRun bool `json:"dryRun"`
	// HarnessID echoes the harness the promotion was interpreted against, so a caller
	// can confirm the outcome without re-reading the file.
	HarnessID string `json:"harnessId"`
	// Tools is the generic tools list written to the promoted file, in written order.
	// Empty when the source declared no harness-side tool entries.
	Tools []string `json:"tools,omitempty"`
	// VerbatimTools names the harness-side tool entries that reached the generic tools
	// list unchanged because no reverse mapping existed and no generic name was supplied.
	// A non-empty value is the documented limitation of the autonomous path, not an error.
	VerbatimTools []string `json:"verbatimTools,omitempty"`
	// RecoveredFields names the generic-only frontmatter fields the user supplied during
	// this run, in ask order. Fields left absent are not listed.
	RecoveredFields []string `json:"recoveredFields,omitempty"`
	// StrippedFields lists frontmatter fields removed from the generated generic agent
	// because they could not be recovered — unmappable diverted tool values and fields
	// unknown to MOSAIC. Empty when nothing was stripped.
	StrippedFields []StrippedField `json:"strippedFields,omitempty"`
	// DivertedTools lists the harness-side tool entries recovered from diverted
	// frontmatter fields (as opposed to the harness's main tools key), in extraction
	// order. Reported separately from Tools so the user can see the recovery happened.
	DivertedTools []string `json:"divertedTools,omitempty"`
	// TargetCatalogID is the id of the layer that received the file; empty for a plain catalog.
	TargetCatalogID string `json:"targetCatalogId,omitempty"`
	// ExperimentalHarnesses holds the harness the promote was interpreted against when it is
	// experimental. Absent from JSON otherwise.
	ExperimentalHarnesses []domain.ExperimentalHarness `json:"experimentalHarnesses,omitempty"`
	// RemovedCustomRegions names every custom region of the source, at any depth and in
	// document order, removed from the generated generic. Empty when the source had none.
	RemovedCustomRegions []string `json:"removedCustomRegions,omitempty"`
}
