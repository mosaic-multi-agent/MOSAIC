package domain

import "context"

// ObservedModelForm says how a harness's logs identify a model.
type ObservedModelForm string

const (
	// ObservedModelBare is the model id as logged.
	ObservedModelBare ObservedModelForm = "bare"
	// ObservedModelProviderQualified is provider + "/" + model.
	ObservedModelProviderQualified ObservedModelForm = "provider_qualified"
)

// ObservedModelQuery identifies the run whose subject model is asked for.
// It deliberately carries no model selection, so no implementation can echo it.
type ObservedModelQuery struct {
	LogRoot  string // per-run folder
	LogsRoot string // OrchestrationLogs parent
	RunID    string
	Form     ObservedModelForm // "" = unknown
}

// NoEvidenceReason names why the subject model could not be determined.
type NoEvidenceReason string

const (
	NoEvidenceUnknownForm          NoEvidenceReason = "unknown_form"
	NoEvidenceToolUnavailable      NoEvidenceReason = "tool_unavailable"
	NoEvidenceNoData               NoEvidenceReason = "no_data"
	NoEvidenceToolFailed           NoEvidenceReason = "tool_failed"
	NoEvidenceUnparseable          NoEvidenceReason = "unparseable"
	NoEvidenceRunNotFound          NoEvidenceReason = "run_not_found"
	NoEvidenceAmbiguousRun         NoEvidenceReason = "ambiguous_run"
	NoEvidenceUnsupportedOutput    NoEvidenceReason = "unsupported_output"
	NoEvidenceUnattributedUsage    NoEvidenceReason = "unattributed_usage"
	NoEvidenceUnidentifiedRequests NoEvidenceReason = "unidentified_requests"
	NoEvidenceNoSubjectUsage       NoEvidenceReason = "no_subject_usage"
	NoEvidenceEmptyProvider        NoEvidenceReason = "empty_provider"
	NoEvidencePortUnavailable      NoEvidenceReason = "port_unavailable" // runner: no port wired
	NoEvidencePortError            NoEvidenceReason = "port_error"       // runner: port returned an error
)

// ObservedModelEvidence: exactly one of (len(Models) > 0) and (NoEvidence != "") holds.
type ObservedModelEvidence struct {
	Models     []string // distinct, sorted, non-empty strings in the selection's format
	NoEvidence NoEvidenceReason
	Detail     string // non-empty when NoEvidence != ""
}

// ObservedModelFormReporter is an optional HarnessAdapter capability declaring
// how the harness's logs identify a model. Absence means the form is unknown.
type ObservedModelFormReporter interface {
	ObservedModelForm() ObservedModelForm
}

// ObservedModelProvider reports which model(s) the subject session of one run
// actually used, determined from that run's logs.
type ObservedModelProvider interface {
	// ObservedSubjectModels never falls back to a selection or a default. Every
	// failure to determine the model is reported as evidence with NoEvidence set.
	// A non-nil error is a contract-level fault.
	ObservedSubjectModels(ctx context.Context, q ObservedModelQuery) (ObservedModelEvidence, error)
}
