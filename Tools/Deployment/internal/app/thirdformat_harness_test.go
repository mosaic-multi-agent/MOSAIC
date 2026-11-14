package app_test

// thirdformat_harness_test.go holds the twin fake harnesses: two harnesses that are identical
// except for the agent format and the agent file extension. One uses the test-only JSON agent
// format, the other Markdown. Both declare a permission-shaped tools key with deny-able tools,
// modelled on the built-in OpenCode descriptor. The descriptors are built in Go, so they bypass
// the descriptor loader (which rejects the test-only format).

import (
	"fmt"
	"testing"

	"mosaic-deploy/internal/agentformat/testjson"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// twinFormat selects one of the twin harnesses.
type twinFormat int

const (
	twinTestFormat twinFormat = iota // agent format testjson.ID, agent extension ".json"
	twinMarkdown                     // agent format "" (Markdown), agent extension ".md"
)

// twinHarnessID is the harness id both twins share, so manifest entries compare equal.
const twinHarnessID = "thirdformat"

// registerTestFormat registers the test-only format in this test binary (idempotent).
func registerTestFormat(t testing.TB) {
	t.Helper()
	testjson.Register()
}

// twinModule is a harness module that answers from the twin descriptor and delegates
// everything else to the built-in OpenCode module.
type twinModule struct {
	domain.HarnessModule
	ref  domain.HarnessRef
	desc *domain.HarnessDescriptor
}

func (m *twinModule) Ref() domain.HarnessRef                { return m.ref }
func (m *twinModule) Descriptor() *domain.HarnessDescriptor { return m.desc }

// TargetPath resolves agent paths from the twin descriptor (so they carry the twin's
// extension) and delegates every other kind.
func (m *twinModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	if req.Kind == domain.ArtifactAgent {
		return descriptor.ResolveTargetPath(m.desc, req)
	}
	return m.HarnessModule.TargetPath(req)
}

// newTestTwinHarness returns the twin module for f. It calls registerTestFormat.
func newTestTwinHarness(t *testing.T, f twinFormat) domain.HarnessModule {
	t.Helper()
	registerTestFormat(t)
	inner := rbModule(t, hOpen)
	desc := *inner.Descriptor()
	desc.ID = twinHarnessID
	desc.DisplayName = "Third Format"
	desc.Extensions = map[domain.ArtifactKind]string{}
	for kind, ext := range inner.Descriptor().Extensions {
		desc.Extensions[kind] = ext
	}
	switch f {
	case twinTestFormat:
		desc.AgentFormatID = string(testjson.ID)
		desc.Extensions[domain.ArtifactAgent] = ".json"
	case twinMarkdown:
		desc.AgentFormatID = ""
		desc.Extensions[domain.ArtifactAgent] = ".md"
	default:
		t.Fatalf("unknown twin format %d", f)
	}
	ref := inner.Ref()
	ref.ID = twinHarnessID
	ref.DisplayName = desc.DisplayName
	return &twinModule{HarnessModule: inner, ref: ref, desc: &desc}
}

// newTestTwinService wires a service like parityServiceWith, over the parity sample catalog,
// with a registry holding the given twin module plus any modules added through
// withTwinRegistryModules. opts are applied in order after the registry is built.
func newTestTwinService(t *testing.T, mod domain.HarnessModule, opts ...func(*app.Deps)) app.Service {
	t.Helper()
	useTwinRegistry := func(d *app.Deps) {
		d.Registry = &stubRegistry{
			list:    []domain.HarnessRef{mod.Ref()},
			modules: map[string]domain.HarnessModule{mod.Ref().ID: mod},
		}
	}
	return parityServiceWith(t, append([]func(*app.Deps){useTwinRegistry}, opts...)...)
}

// withTwinRegistryModules is a newTestTwinService option that adds mods to the registry
// newTestTwinService built, in argument order. A module whose id equals twinHarnessID or an
// already registered id is a test setup error.
func withTwinRegistryModules(mods ...domain.HarnessModule) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		for _, m := range mods {
			id := m.Ref().ID
			if _, dup := reg.modules[id]; dup {
				panic(fmt.Sprintf("withTwinRegistryModules: harness id %q is already registered", id))
			}
			reg.list = append(reg.list, m.Ref())
			reg.modules[id] = m
		}
	}
}

// twinCounterpart returns the counterpart harness for flows between a twin and a second
// harness: the built-in Claude Code module, whose tools field is list-shaped.
func twinCounterpart(t *testing.T) domain.HarnessModule {
	t.Helper()
	return rbModule(t, hClaude)
}
