package stackconfig_test

// malformed_test.go covers the structural invariants of a persisted stack, as seen by Read
// (manifest fixtures) and Validate (in-memory values). A stack that breaks one is rejected with
// an error wrapping ErrStoredStackMalformed, never reordered or repaired, and Validate rejects
// it before touching the disk.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// absNowhere returns an absolute path that does not exist.
func absNowhere(parts ...string) string {
	root, err := filepath.Abs(string(filepath.Separator) + "nowhere")
	if err != nil {
		panic(err)
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

// malformedCases are stacks that break an invariant. The paths do not exist, so a Validate that
// resolved layers before checking the invariants would report an unresolvable layer instead.
func malformedCases() map[string][]domain.PersistedCatalogLayer {
	return map[string][]domain.PersistedCatalogLayer{
		"default above a user layer": {defaultLayer, userLayer(absNowhere("a"), "cat-a")},
		"two default entries":        {userLayer(absNowhere("a"), "cat-a"), defaultLayer, defaultLayer},
		"default entry with a path":  {{Path: absNowhere("a"), Default: true}},
		"default entry with an id":   {{CatalogID: "cat-a", Default: true}},
		"user layer without a path":  {userLayer("", "cat-a")},
		"user layer relative path":   {userLayer(filepath.Join("rel", "cat"), "cat-a")},
		"user layer without an id":   {userLayer(absNowhere("a"), "")},
	}
}

func TestRead_MalformedStack_IsRejected(t *testing.T) {
	for name, layers := range malformedCases() {
		t.Run(name, func(t *testing.T) {
			ws := saveStack(t, layers...)

			s, ok, err := stackconfig.Read(ws)

			if !errors.Is(err, stackconfig.ErrStoredStackMalformed) {
				t.Fatalf("error = %v, want ErrStoredStackMalformed", err)
			}
			if ok || len(s.Layers) != 0 {
				t.Errorf("Read = (%+v, %v), want a zero Stored and ok false", s, ok)
			}
			if want := filepath.Join(ws, manifest.Dir, manifest.FileName); !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name the manifest %q", err, want)
			}
		})
	}
}

func TestValidate_MalformedStack_IsRejectedBeforeAnyLayerIsResolved(t *testing.T) {
	for name, layers := range malformedCases() {
		t.Run(name, func(t *testing.T) {
			err := stackconfig.Validate(stackconfig.Stored{Layers: layers}, t.TempDir())

			if !errors.Is(err, stackconfig.ErrStoredStackMalformed) {
				t.Fatalf("error = %v, want ErrStoredStackMalformed", err)
			}
			var unresolvable *stackconfig.UnresolvableLayerError
			if errors.As(err, &unresolvable) {
				t.Errorf("error is an UnresolvableLayerError (%v); the invariants are checked first", err)
			}
		})
	}
}

func TestRead_DefaultOnlyStack_IsWellFormed(t *testing.T) {
	ws := saveStack(t, defaultLayer)

	s, ok, err := stackconfig.Read(ws)

	if err != nil || !ok {
		t.Fatalf("Read = (_, %v, %v), want ok and no error", ok, err)
	}
	if d := s.Description(); !d.IncludeDefault || len(d.UserLayers) != 0 {
		t.Errorf("Description() = %+v, want the default layer alone", d)
	}
}
