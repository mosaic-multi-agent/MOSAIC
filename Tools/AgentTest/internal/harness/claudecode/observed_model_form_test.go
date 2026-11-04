package claudecode_test

// The adapter declares how this harness's logs identify a model, so the
// observed-model provider knows how to read them. Claude Code logs the bare
// model id.

import (
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/claudecode"
)

func TestAdapter_DeclaresBareObservedModelForm(t *testing.T) {
	var adapter domain.HarnessAdapter = claudecode.New(claudecode.Options{})

	reporter, ok := adapter.(domain.ObservedModelFormReporter)
	if !ok {
		t.Fatalf("adapter does not implement domain.ObservedModelFormReporter; every run would have no model evidence")
	}
	if got := reporter.ObservedModelForm(); got != domain.ObservedModelBare {
		t.Errorf("ObservedModelForm() = %q, want %q", got, domain.ObservedModelBare)
	}
}
