package opencode

import "mosaic-agent-test/internal/domain"

// ObservedModelForm declares that OpenCode logs the model as provider/model.
func (a *Adapter) ObservedModelForm() domain.ObservedModelForm {
	return domain.ObservedModelProviderQualified
}

var _ domain.ObservedModelFormReporter = (*Adapter)(nil)
