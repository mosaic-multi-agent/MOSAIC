package builtindesc

import (
	"errors"
	"os"
	"testing"
)

func TestParseEmbedded_IDDifferentFromEmbeddingIDIsRejected(t *testing.T) {
	src, err := os.ReadFile("testdata/valid_cli.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	_, err = parseEmbedded("other-id", src)

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	if ve.Field != "id" || ve.Origin != "builtin:other-id" {
		t.Errorf("got Origin %q Field %q, want builtin:other-id / id", ve.Origin, ve.Field)
	}
}

func TestParseEmbedded_MatchingIDIsAccepted(t *testing.T) {
	src, err := os.ReadFile("testdata/valid_cli.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	d, err := parseEmbedded("fixture", src)

	if err != nil || d.ID != "fixture" {
		t.Fatalf("got %+v, %v; want fixture descriptor, nil", d, err)
	}
}
