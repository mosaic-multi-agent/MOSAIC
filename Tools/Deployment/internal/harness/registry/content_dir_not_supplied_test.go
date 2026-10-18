package registry_test

// content_dir_not_supplied_test.go verifies how the registry reports a built-in whose content
// directory resolver fails because no catalog layer supplies the harness injections: the
// harness stays listed but unusable with a reason naming the injection item, and Resolve
// returns an error that still reaches the resolver's typed cause.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// discoverNotSupplied registers a built-in that, like the real ones, asks the resolver for its
// content directory and fails construction on a resolver error, then discovers with a resolver
// that supplies nothing. It returns the registry and the harness id.
func discoverNotSupplied(t *testing.T) (registry.Registry, string) {
	t.Helper()
	id := uniqueID(t, "not-supplied")
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		if _, err := opts.ResolveContentDir(catalogpaths.HarnessDirNameClaudeCode); err != nil {
			return nil, err
		}
		return stubModule(id), nil
	})
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: makeRoot(t),
		BuiltinContentDir: func(harnessDirName string) (string, error) {
			return "", &injections.NotSuppliedError{HarnessDir: harnessDirName}
		},
	})
	if err != nil {
		t.Fatalf("Discover must tolerate a built-in whose injections are not supplied: %v", err)
	}
	return reg, id
}

func TestDiscover_BuiltinInjectionsNotSupplied_StaysListedAndNamesTheItem(t *testing.T) {
	reg, id := discoverNotSupplied(t)

	var ref *domain.HarnessRef
	for _, r := range reg.List() {
		if r.ID == id {
			r := r
			ref = &r
		}
	}

	if ref == nil {
		t.Fatalf("harness %q is missing from List(); a harness without supplied content is still listed", id)
	}
	if ref.Usable {
		t.Error("Usable = true, want false for a harness whose injections are not supplied")
	}
	item := catalogpaths.RelHarnessInjectionsDir + "/" + catalogpaths.HarnessDirNameClaudeCode
	if !strings.Contains(ref.UnusableReason, item) {
		t.Errorf("UnusableReason = %q, want it to name the injection item %q", ref.UnusableReason, item)
	}
	if !strings.HasPrefix(ref.UnusableReason, "failed to initialise built-in: ") {
		t.Errorf("UnusableReason = %q, want the prefix %q", ref.UnusableReason, "failed to initialise built-in: ")
	}
}

func TestResolve_BuiltinInjectionsNotSupplied_ErrorReachesTheNotSuppliedCause(t *testing.T) {
	reg, id := discoverNotSupplied(t)

	_, err := reg.Resolve(id)

	var notSupplied *injections.NotSuppliedError
	if !errors.As(err, &notSupplied) {
		t.Fatalf("Resolve error = %v; want an error wrapping *injections.NotSuppliedError", err)
	}
	if notSupplied.HarnessDir != catalogpaths.HarnessDirNameClaudeCode {
		t.Errorf("NotSuppliedError.HarnessDir = %q, want %q", notSupplied.HarnessDir, catalogpaths.HarnessDirNameClaudeCode)
	}
	if !errors.Is(err, injections.ErrNotSupplied) {
		t.Errorf("Resolve error %v does not match injections.ErrNotSupplied", err)
	}
	if want := `harness "` + id + `" is not usable: failed to initialise built-in: `; !strings.HasPrefix(err.Error(), want) {
		t.Errorf("Resolve error text = %q, want the prefix %q", err.Error(), want)
	}
}
