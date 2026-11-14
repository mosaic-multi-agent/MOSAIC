package all_test

// all_test.go models the production wiring: a binary that imports the wiring package
// registers exactly the production formats and does not know the test-only format.
// This binary never calls testjson.Register; importing the package for its ID must
// not register anything.

import (
	"errors"
	"sort"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentformat/formatid"
	"mosaic-deploy/internal/agentformat/testjson"

	_ "mosaic-deploy/internal/agentformat/all"
)

func TestProductionWiring_TestFormatIsNotRegistered(t *testing.T) {
	_, err := agentformat.LookupString(string(testjson.ID))

	if !errors.Is(err, formatid.ErrUnknownFormatID) {
		t.Errorf("LookupString(%q) error = %v, want ErrUnknownFormatID: the production wiring must not resolve the test format", testjson.ID, err)
	}
}

func TestProductionWiring_RegistryHoldsExactlyTheProductionVocabulary(t *testing.T) {
	vocab := formatid.All()
	sort.Slice(vocab, func(i, j int) bool { return vocab[i] < vocab[j] })

	registered := agentformat.RegisteredIDs()

	if len(vocab) != len(registered) {
		t.Fatalf("vocabulary %v != registered %v", vocab, registered)
	}
	for i := range vocab {
		if vocab[i] != registered[i] {
			t.Errorf("vocabulary[%d]=%q != registered[%d]=%q", i, vocab[i], i, registered[i])
		}
	}
}

func TestProductionWiring_EveryProductionFormatResolves(t *testing.T) {
	for _, id := range formatid.All() {
		if tr, err := agentformat.LookupString(string(id)); err != nil || tr == nil {
			t.Errorf("LookupString(%q) = (%v, %v), want a translator", id, tr, err)
		}
	}
}
