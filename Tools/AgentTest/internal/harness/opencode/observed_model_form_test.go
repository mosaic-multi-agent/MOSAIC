package opencode_test

// The adapter declares how this harness's logs identify a model, so the
// observed-model provider knows how to read them. OpenCode logs a provider and
// a model, reported as provider/model.

import (
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencode"
)

func TestAdapter_DeclaresProviderQualifiedObservedModelForm(t *testing.T) {
	var adapter domain.HarnessAdapter = opencode.New(opencode.Options{})

	reporter, ok := adapter.(domain.ObservedModelFormReporter)
	if !ok {
		t.Fatalf("adapter does not implement domain.ObservedModelFormReporter; every run would have no model evidence")
	}
	if got := reporter.ObservedModelForm(); got != domain.ObservedModelProviderQualified {
		t.Errorf("ObservedModelForm() = %q, want %q", got, domain.ObservedModelProviderQualified)
	}
}
