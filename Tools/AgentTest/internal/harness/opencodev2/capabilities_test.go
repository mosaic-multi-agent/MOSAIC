package opencodev2_test

// Tests for the declared capabilities. They must be the honest ones for V2:
// execute.before can rewrite or reject a call but nothing can hand the parent
// a fabricated child reply; execute.after exists but fires at launch with a
// running acknowledgement; the real reply is recovered from the child's first
// completion wake-up, which carries the dispatch's call id itself.

import (
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func newAdapterForCaps() *opencodev2.Adapter { return opencodev2.New(opencodev2.Options{}) }

func TestCapabilities_DirectSubstitutionUnsupported(t *testing.T) {
	if newAdapterForCaps().Capabilities().SupportsDirectSubstitution {
		t.Errorf("SupportsDirectSubstitution = true, want false: a hook can rewrite or reject a call but not fabricate the child's reply")
	}
}

func TestCapabilities_PostInterceptionSupported(t *testing.T) {
	if !newAdapterForCaps().Capabilities().SupportsPostInterception {
		t.Errorf("SupportsPostInterception = false, want true: execute.after exists")
	}
}

func TestCapabilities_ReplyRecoverySupported(t *testing.T) {
	if !newAdapterForCaps().Capabilities().SupportsReplyRecovery {
		t.Errorf("SupportsReplyRecovery = false, want true: the first completion wake-up carries the real reply")
	}
}

func TestCapabilities_CorrelationIsTheToolCallIDOnEveryPhase(t *testing.T) {
	caps := newAdapterForCaps().Capabilities()

	if caps.CorrelationField != "id" {
		t.Errorf("CorrelationField = %q, want %q", caps.CorrelationField, "id")
	}
	if caps.CompletionCorrelationField != "" {
		t.Errorf("CompletionCorrelationField = %q, want empty: the completion payload carries the call id itself, no agent-start phase", caps.CompletionCorrelationField)
	}
}

func TestCapabilities_RegistrationModelIsDirectoryDrop(t *testing.T) {
	if got := newAdapterForCaps().Capabilities().RegistrationModel; got != domain.RegistrationDirectoryDrop {
		t.Errorf("RegistrationModel = %q, want %q", got, domain.RegistrationDirectoryDrop)
	}
}

func TestCapabilities_BridgeKindIsInProcess(t *testing.T) {
	if got := newAdapterForCaps().Capabilities().BridgeKind; got != domain.BridgeInProcess {
		t.Errorf("BridgeKind = %q, want %q", got, domain.BridgeInProcess)
	}
}

func TestCapabilities_ProducibleToolNamesIsTheNormalisedDispatchName(t *testing.T) {
	got := newAdapterForCaps().Capabilities().ProducibleToolNames
	want := []string{domain.DispatchToolName}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProducibleToolNames = %v, want %v", got, want)
	}
}

func TestCapabilities_ConstantAcrossCallsAndSliceIsFresh(t *testing.T) {
	a := newAdapterForCaps()
	first := a.Capabilities()
	second := a.Capabilities()

	if !reflect.DeepEqual(first, second) {
		t.Errorf("Capabilities differ between calls: %+v vs %+v", first, second)
	}
	if len(first.ProducibleToolNames) == 0 {
		t.Fatalf("ProducibleToolNames is empty")
	}
	first.ProducibleToolNames[0] = "mutated"
	if got := a.Capabilities().ProducibleToolNames[0]; got != domain.DispatchToolName {
		t.Errorf("mutating a returned ProducibleToolNames changed the adapter's declaration to %q", got)
	}
}
