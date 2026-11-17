package opencodev2

import "mosaic-agent-test/internal/domain"

var _ domain.ObservedModelFormReporter = (*Adapter)(nil)

// ObservedModelForm declares that V2 logs the model as provider/model.
func (a *Adapter) ObservedModelForm() domain.ObservedModelForm {
	return domain.ObservedModelProviderQualified
}
