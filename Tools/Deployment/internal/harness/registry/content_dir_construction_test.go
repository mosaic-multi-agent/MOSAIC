package registry_test

import (
	"errors"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// TestDiscover_BuiltinFactory_ReceivesMosaicRootContentDirResolver verifies that Discover
// supplies every built-in factory with a content-directory resolver that maps a harness
// directory name to <MosaicRoot>/Catalog/HarnessInjections/<harness dir>, so production
// output keeps reading today's content location.
func TestDiscover_BuiltinFactory_ReceivesMosaicRootContentDirResolver(t *testing.T) {
	root := makeRoot(t)
	id := uniqueID(t, "content-dir-default")

	var received registry.BuiltinOptions
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		received = opts
		return stubModule(id), nil
	})

	if _, err := registry.Discover(registry.Options{MosaicRoot: root}); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if received.ContentDir == nil {
		t.Fatal("factory received a nil BuiltinOptions.ContentDir; Discover must supply the content-directory resolver")
	}
	for _, name := range []string{
		catalogpaths.HarnessDirNameClaudeCode,
		catalogpaths.HarnessDirNameGhcpCLI,
		catalogpaths.HarnessDirNameOpenCode,
		catalogpaths.HarnessDirNameVSCodeGHCP,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := received.ContentDir(name)
			if err != nil {
				t.Fatalf("ContentDir(%q) returned error: %v", name, err)
			}
			want := filepath.Join(root, catalogpaths.MosaicRelHarnessInjectionsDir, name)
			if filepath.Clean(got) != filepath.Clean(want) {
				t.Errorf("ContentDir(%q) = %q, want %q", name, got, want)
			}
		})
	}
}

// TestDiscover_BuiltinContentDirOverride_ReachesFactory verifies that an explicit
// Options.BuiltinContentDir is the resolver every built-in factory receives, replacing the
// MOSAIC-root default, and that a resolver error is preserved by that resolver.
func TestDiscover_BuiltinContentDirOverride_ReachesFactory(t *testing.T) {
	root := makeRoot(t)
	id := uniqueID(t, "content-dir-override")
	overrideDir := t.TempDir()
	sentinel := errors.New("override resolver sentinel")

	var received registry.BuiltinOptions
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		received = opts
		return stubModule(id), nil
	})

	_, err := registry.Discover(registry.Options{
		MosaicRoot: root,
		BuiltinContentDir: func(name string) (string, error) {
			if name == catalogpaths.HarnessDirNameOpenCode {
				return "", sentinel
			}
			return filepath.Join(overrideDir, name), nil
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if received.ContentDir == nil {
		t.Fatal("factory received a nil BuiltinOptions.ContentDir")
	}
	got, err := received.ContentDir(catalogpaths.HarnessDirNameClaudeCode)
	if err != nil {
		t.Fatalf("ContentDir returned error: %v", err)
	}
	if want := filepath.Join(overrideDir, catalogpaths.HarnessDirNameClaudeCode); filepath.Clean(got) != filepath.Clean(want) {
		t.Errorf("ContentDir = %q, want the override path %q (not the MOSAIC-root default)", got, want)
	}
	if _, err := received.ContentDir(catalogpaths.HarnessDirNameOpenCode); !errors.Is(err, sentinel) {
		t.Errorf("override resolver error not preserved: got %v", err)
	}
}
