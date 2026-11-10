package claudecode

import "mosaic-agent-test/internal/domain"

// ObservedModelForm declares that Claude Code logs the bare model id.
func (a *Adapter) ObservedModelForm() domain.ObservedModelForm {
	return domain.ObservedModelBare
}

var _ domain.ObservedModelFormReporter = (*Adapter)(nil)
