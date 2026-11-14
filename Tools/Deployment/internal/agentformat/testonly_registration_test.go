package agentformat_test

// testonly_registration_test.go covers RegisterTestOnly: a test-only translator is
// resolvable through LookupString while the production vocabulary and registry stay
// exactly the production formats.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentformat/formatid"

	_ "mosaic-deploy/internal/agentformat/all"
)

// probeTranslator is a comparable stub translator; identity distinguishes instances.
type probeTranslator struct{ name string }

func (*probeTranslator) Encode(c []byte, _ agentformat.ArtifactContext) ([]byte, agentformat.Report, error) {
	return c, agentformat.Report{}, nil
}

func (*probeTranslator) Decode(d []byte, _ agentformat.ArtifactContext) ([]byte, agentformat.Report, error) {
	return d, agentformat.Report{}, nil
}

func TestRegisterTestOnly_ResolvesThroughLookupString(t *testing.T) {
	const id = formatid.ID("probe-format-resolves")
	want := &probeTranslator{name: "resolves"}

	agentformat.RegisterTestOnly(id, want)
	got, err := agentformat.LookupString(string(id))

	if err != nil {
		t.Fatalf("LookupString(%q) after RegisterTestOnly: %v", id, err)
	}
	if got != agentformat.Translator(want) {
		t.Errorf("LookupString returned a different translator than the one registered")
	}
}

func TestRegisterTestOnly_DoesNotEnterVocabularyOrProductionRegistry(t *testing.T) {
	const id = formatid.ID("probe-format-isolated")
	agentformat.RegisterTestOnly(id, &probeTranslator{name: "isolated"})

	for _, known := range formatid.All() {
		if known == id {
			t.Errorf("formatid.All() contains %q", id)
		}
	}
	for _, registered := range agentformat.RegisteredIDs() {
		if registered == id {
			t.Errorf("RegisteredIDs() contains %q", id)
		}
	}
	if _, err := formatid.Parse(string(id)); !errors.Is(err, formatid.ErrUnknownFormatID) {
		t.Errorf("formatid.Parse(%q) error = %v, want ErrUnknownFormatID", id, err)
	}
	if _, err := agentformat.Lookup(id); !errors.Is(err, agentformat.ErrUnknownFormat) {
		t.Errorf("Lookup(%q) error = %v, want ErrUnknownFormat", id, err)
	}
}

// Order-independent: other tests may add ids to the shared registry, so only the
// probe id's absence and the continued registration of every vocabulary id are checked.
func TestRegisterTestOnly_KeepsEveryVocabularyIDRegisteredAndProbeOut(t *testing.T) {
	const probe = formatid.ID("probe-format-vocab")
	agentformat.RegisterTestOnly(probe, &probeTranslator{name: "vocab"})

	registered := map[formatid.ID]bool{}
	for _, id := range agentformat.RegisteredIDs() {
		registered[id] = true
	}
	for _, id := range formatid.All() {
		if !registered[id] {
			t.Errorf("vocabulary id %q is not registered", id)
		}
	}
	if registered[probe] {
		t.Errorf("RegisteredIDs() contains the test-only id %q", probe)
	}
}

func TestRegisterTestOnly_SecondRegistrationIsNoOpAndFirstTranslatorStays(t *testing.T) {
	const id = formatid.ID("probe-format-twice")
	first := &probeTranslator{name: "first"}
	second := &probeTranslator{name: "second"}

	agentformat.RegisterTestOnly(id, first)
	agentformat.RegisterTestOnly(id, second)
	got, err := agentformat.LookupString(string(id))

	if err != nil {
		t.Fatalf("LookupString: %v", err)
	}
	if got != agentformat.Translator(first) {
		t.Errorf("the second registration replaced the first translator")
	}
}

func TestRegisterTestOnly_PanicsForProductionVocabularyIDs(t *testing.T) {
	for _, id := range formatid.All() {
		id := id
		t.Run(string(id), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterTestOnly(%q) did not panic", id)
				}
			}()

			agentformat.RegisterTestOnly(id, &probeTranslator{name: "forbidden"})
		})
	}
}

func TestRegisterTestOnly_ProductionLookupIsUnchanged(t *testing.T) {
	agentformat.RegisterTestOnly("probe-format-prod", &probeTranslator{name: "prod"})

	for _, id := range []string{"", string(formatid.Markdown), string(formatid.CodexTOML)} {
		tr, err := agentformat.LookupString(id)
		if err != nil || tr == nil {
			t.Errorf("LookupString(%q) = (%v, %v), want a translator", id, tr, err)
		}
		if _, isProbe := tr.(*probeTranslator); isProbe {
			t.Errorf("LookupString(%q) resolved to the test-only translator", id)
		}
	}
}

func TestLookupString_UnregisteredUnknownIDKeepsErrUnknownFormatID(t *testing.T) {
	agentformat.RegisterTestOnly("probe-format-other", &probeTranslator{name: "other"})

	_, err := agentformat.LookupString("never-registered-format")

	if !errors.Is(err, formatid.ErrUnknownFormatID) {
		t.Errorf("error = %v, want one wrapping formatid.ErrUnknownFormatID", err)
	}
}
