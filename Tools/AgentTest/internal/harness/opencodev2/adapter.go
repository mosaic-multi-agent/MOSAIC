package opencodev2

import (
	"errors"

	"mosaic-agent-test/internal/domain"
)

// ScopeProbe reads a configuration scope's raw document. It has no write method.
type ScopeProbe interface {
	Read(scope domain.ConfigScope) ([]byte, error)
}

// ErrScopeAbsent: the probed scope does not exist (as distinct from unreadable).
var ErrScopeAbsent = errors.New("opencode-v2: configuration scope absent")

// ErrMultipleRewriters: more than one plugin able to rewrite the intercepted call's
// input would be present in the sandbox.
var ErrMultipleRewriters = errors.New("opencode-v2: more than one plugin would rewrite the intercepted call's input")

// Options configures New.
type Options struct {
	// ScopeProbe reads configuration-scope documents. nil selects the real,
	// read-only filesystem probe.
	ScopeProbe ScopeProbe
}

// Adapter is the OpenCode V2 harness adapter.
type Adapter struct {
	opts Options
}

var _ domain.HarnessAdapter = (*Adapter)(nil)

// New constructs the adapter. It performs no I/O.
func New(opts Options) *Adapter { return &Adapter{opts: opts} }

// ID implements domain.HarnessAdapter.
func (a *Adapter) ID() string { return HarnessID }

// Capabilities implements domain.HarnessAdapter. The value is constant for the
// adapter's lifetime; ProducibleToolNames is a fresh slice per call.
//
// The declarations follow what the interceptor plugin can observe: a rewrite
// or rejection before the call runs, a launch-time acknowledgement after it,
// and the child's real reply from its first completion wake-up, which carries
// the dispatch's call id.
func (a *Adapter) Capabilities() domain.HarnessCapabilities {
	return domain.HarnessCapabilities{
		SupportsDirectSubstitution: false,
		SupportsPostInterception:   true,
		SupportsReplyRecovery:      true,
		CorrelationField:           "id",
		RegistrationModel:          domain.RegistrationDirectoryDrop,
		BridgeKind:                 domain.BridgeInProcess,
		ProducibleToolNames:        []string{domain.DispatchToolName},
	}
}
