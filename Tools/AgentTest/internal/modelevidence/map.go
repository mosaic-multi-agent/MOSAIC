package modelevidence

import (
	"fmt"
	"sort"

	"mosaic-agent-test/internal/domain"
)

// The log-analysis tool's exit code for "no data" (see its exit code table).
const exitNoData = 2

// MapInput is everything Map needs; Report is the decoded stdout.
type MapInput struct {
	ExecutablePath string
	RunID          string
	Form           domain.ObservedModelForm
	ExitCode       int
	InvokeErr      error
	ParseErr       error
	Report         ReportDoc
}

// ReportDoc mirrors the subset of the log-analysis report JSON that is read.
type ReportDoc struct {
	Runs           []RunDoc `json:"runs"`
	Unattributable *RunDoc  `json:"unattributable"`
}

// RunDoc is one run of the report. Only the orchestrator actor (the subject
// session) is read; agent actors are stubs.
type RunDoc struct {
	RunID        string   `json:"run_id"`
	Orchestrator ActorDoc `json:"orchestrator"`
}

// ActorDoc is one actor of a run.
type ActorDoc struct {
	Models               []string       `json:"models"`
	ModelKeys            *[]ModelKeyDoc `json:"model_keys"` // nil = key absent (older analyser)
	RequestsWithoutModel int            `json:"requests_without_model"`
}

// ModelKeyDoc is one (provider, model) pair as logged.
type ModelKeyDoc struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func validForm(f domain.ObservedModelForm) bool {
	return f == domain.ObservedModelBare || f == domain.ObservedModelProviderQualified
}

func none(in MapInput, reason domain.NoEvidenceReason, format string, args ...any) domain.ObservedModelEvidence {
	return domain.ObservedModelEvidence{
		NoEvidence: reason,
		Detail:     fmt.Sprintf("observed model: run %q: ", in.RunID) + fmt.Sprintf(format, args...),
	}
}

// Map is the pure outcome mapping. The first matching rule decides; no rule
// ever produces a model value from anything but the subject's logged models.
func Map(in MapInput) domain.ObservedModelEvidence {
	if !validForm(in.Form) {
		return none(in, domain.NoEvidenceUnknownForm, "model form %q is not known", string(in.Form))
	}
	if in.InvokeErr != nil {
		return none(in, domain.NoEvidenceToolUnavailable, "log-analysis tool %q failed to run: %v", in.ExecutablePath, in.InvokeErr)
	}
	if in.ExitCode == exitNoData {
		return none(in, domain.NoEvidenceNoData, "the log-analysis tool found no data")
	}
	if in.ExitCode != 0 {
		return none(in, domain.NoEvidenceToolFailed, "the log-analysis tool exited with code %d", in.ExitCode)
	}
	if in.ParseErr != nil {
		return none(in, domain.NoEvidenceUnparseable, "report output could not be decoded: %v", in.ParseErr)
	}

	var found *RunDoc
	for i := range in.Report.Runs {
		if in.Report.Runs[i].RunID != in.RunID {
			continue
		}
		if found != nil {
			return none(in, domain.NoEvidenceAmbiguousRun, "the report lists the run more than once")
		}
		found = &in.Report.Runs[i]
	}
	if found == nil {
		return none(in, domain.NoEvidenceRunNotFound, "the report does not contain the run")
	}

	actor := found.Orchestrator
	if actor.ModelKeys == nil {
		return none(in, domain.NoEvidenceUnsupportedOutput, "the report carries no model_keys for the subject (older analyser)")
	}
	if u := in.Report.Unattributable; u != nil && hasUsage(u.Orchestrator) {
		return none(in, domain.NoEvidenceUnattributedUsage, "usage exists that could not be attributed to any run")
	}
	if actor.RequestsWithoutModel > 0 {
		return none(in, domain.NoEvidenceUnidentifiedRequests, "%d subject request(s) logged no model", actor.RequestsWithoutModel)
	}
	keys := *actor.ModelKeys
	if len(keys) == 0 {
		return none(in, domain.NoEvidenceNoSubjectUsage, "no subject usage was logged")
	}
	return modelsOf(in, keys)
}

func hasUsage(a ActorDoc) bool {
	return (a.ModelKeys != nil && len(*a.ModelKeys) > 0) || a.RequestsWithoutModel > 0
}

// modelsOf renders the logged keys in the form's shape: distinct and sorted.
func modelsOf(in MapInput, keys []ModelKeyDoc) domain.ObservedModelEvidence {
	seen := map[string]bool{}
	for _, k := range keys {
		if k.Model == "" {
			return none(in, domain.NoEvidenceUnidentifiedRequests, "a subject request logged an empty model")
		}
		name := k.Model
		if in.Form == domain.ObservedModelProviderQualified {
			if k.Provider == "" {
				return none(in, domain.NoEvidenceEmptyProvider, "model %q was logged without a provider", k.Model)
			}
			name = k.Provider + "/" + k.Model
		}
		seen[name] = true
	}
	models := make([]string, 0, len(seen))
	for m := range seen {
		models = append(models, m)
	}
	sort.Strings(models)
	return domain.ObservedModelEvidence{Models: models}
}
