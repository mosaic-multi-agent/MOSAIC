package stackerr_test

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack/stackerr"
)

func TestLayerError_UnwrapWithCause_ExposesKindAndCause(t *testing.T) {
	cause := errors.New("underlying cause")
	e := &stackerr.LayerError{Root: "/r", CatalogID: "cid", Kind: stackerr.ErrLayerLoad, Err: cause}

	if !errors.Is(e, stackerr.ErrLayerLoad) || !errors.Is(e, cause) {
		t.Errorf("errors.Is must reach both Kind and Err for %v", e)
	}
	if errors.Is(e, stackerr.ErrLayerIdentity) {
		t.Error("errors.Is matched an unrelated sentinel")
	}
}

func TestLayerError_UnwrapWithoutCause_ReturnsOnlyKind(t *testing.T) {
	e := &stackerr.LayerError{Root: "/r", Kind: stackerr.ErrInvalidLayer}

	got := e.Unwrap()

	if len(got) != 1 || got[0] != stackerr.ErrInvalidLayer {
		t.Errorf("Unwrap() = %v, want exactly [ErrInvalidLayer] and no nil entry", got)
	}
	if !errors.Is(e, stackerr.ErrInvalidLayer) {
		t.Error("errors.Is(e, ErrInvalidLayer) = false")
	}
}

func TestLayerError_Error_NamesIDRootAndCause(t *testing.T) {
	cause := errors.New("underlying cause")
	e := &stackerr.LayerError{Root: "/some/root", CatalogID: "cid", Kind: stackerr.ErrLayerLoad, Err: cause}

	msg := e.Error()

	for _, want := range []string{"cid", "/some/root", stackerr.ErrLayerLoad.Error(), "underlying cause"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}

func TestLayerError_Error_EmptyID_NamesRootWithoutPlaceholder(t *testing.T) {
	cause := errors.New("underlying cause")
	e := &stackerr.LayerError{Root: "/some/root", Kind: stackerr.ErrLayerIdentity, Err: cause}

	msg := e.Error()

	for _, want := range []string{"/some/root", stackerr.ErrLayerIdentity.Error(), "underlying cause"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
	for _, bad := range []string{"?", "catalog layer  (", "catalog layer ? ("} {
		if strings.Contains(msg, bad) {
			t.Errorf("Error() = %q, must not contain %q", msg, bad)
		}
	}
}

func TestLayerError_Error_EmptyIDAndEmptyRoot_StillNamesNoPlaceholder(t *testing.T) {
	e := &stackerr.LayerError{Kind: stackerr.ErrInvalidLayer, Err: errors.New("root is empty")}

	msg := e.Error()

	for _, want := range []string{stackerr.ErrInvalidLayer.Error(), "root is empty"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
	for _, bad := range []string{"?", "catalog layer  (", "catalog layer ? (", "()"} {
		if strings.Contains(msg, bad) {
			t.Errorf("Error() = %q, must not contain %q", msg, bad)
		}
	}
}

func TestLayerError_Error_KnownID_KeepsIDRootFormat(t *testing.T) {
	e := &stackerr.LayerError{Root: "/some/root", CatalogID: "cid", Kind: stackerr.ErrLayerLoad}

	want := "catalog layer cid (/some/root): " + stackerr.ErrLayerLoad.Error()
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
