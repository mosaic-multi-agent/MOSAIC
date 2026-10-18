package registry_test

// injection_dir_test.go verifies that every registry the package produces reports, per harness
// id, the HarnessInjections directory name its built-in asked its content resolver for, and
// reports nothing for any other kind of entry.

import (
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// reporterOf returns the registry as an InjectionDirReporter and fails the test when it is not one.
func reporterOf(t *testing.T, reg registry.Registry) registry.InjectionDirReporter {
	t.Helper()
	rep, ok := reg.(registry.InjectionDirReporter)
	if !ok {
		t.Fatalf("%T does not implement registry.InjectionDirReporter", reg)
	}
	return rep
}

// registerAskingFor registers a built-in that asks the resolver for dirName (when not empty)
// and returns its id. The factory fails when the resolver does.
func registerAskingFor(t *testing.T, suffix, dirName string) string {
	t.Helper()
	id := uniqueID(t, suffix)
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		if dirName != "" {
			if _, err := opts.ResolveContentDir(dirName); err != nil {
				return nil, err
			}
		}
		return stubModule(id), nil
	})
	return id
}

func TestInjectionDirName_BuiltinThatAskedForDir_ReportsIt(t *testing.T) {
	id := registerAskingFor(t, "asks", catalogpaths.HarnessDirNameOpenCode)
	reg, err := registry.Discover(registry.Options{MosaicRoot: makeRoot(t)})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	got, ok := reporterOf(t, reg).InjectionDirName(id)

	if !ok || got != catalogpaths.HarnessDirNameOpenCode {
		t.Errorf("InjectionDirName(%q) = (%q, %v), want (%q, true)", id, got, ok, catalogpaths.HarnessDirNameOpenCode)
	}
}

func TestInjectionDirName_BuiltinWhoseConstructionFailed_StillReportsTheAskedDir(t *testing.T) {
	id := registerAskingFor(t, "fails", catalogpaths.HarnessDirNameGhcpCLI)
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: makeRoot(t),
		BuiltinContentDir: func(dir string) (string, error) {
			return "", &injections.NotSuppliedError{HarnessDir: dir}
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	got, ok := reporterOf(t, reg).InjectionDirName(id)

	if !ok || got != catalogpaths.HarnessDirNameGhcpCLI {
		t.Errorf("InjectionDirName(%q) = (%q, %v), want (%q, true) although construction failed", id, got, ok, catalogpaths.HarnessDirNameGhcpCLI)
	}
}

func TestInjectionDirName_BuiltinWithoutResolverCall_ReportsNothing(t *testing.T) {
	id := registerAskingFor(t, "silent", "")
	reg, err := registry.Discover(registry.Options{MosaicRoot: makeRoot(t)})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got, ok := reporterOf(t, reg).InjectionDirName(id); ok {
		t.Errorf("InjectionDirName(%q) = (%q, true), want ok == false for a built-in that asked for no directory", id, got)
	}
}

func TestInjectionDirName_UnknownID_ReportsNothing(t *testing.T) {
	reg, err := registry.Discover(registry.Options{MosaicRoot: makeRoot(t)})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got, ok := reporterOf(t, reg).InjectionDirName("no-such-harness"); ok {
		t.Errorf("InjectionDirName = (%q, true), want ok == false for an unknown id", got)
	}
}

func TestInjectionDirName_DescriptorOnlyHarness_ReportsNothing(t *testing.T) {
	root := makeRoot(t)
	addDescriptorOnly(t, root, "desc-only-dir", "Descriptor Only")
	reg, err := registry.Discover(registry.Options{MosaicRoot: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got, ok := reporterOf(t, reg).InjectionDirName("desc-only-dir"); ok {
		t.Errorf("InjectionDirName = (%q, true), want ok == false for a descriptor-only harness", got)
	}
}

func TestInjectionDirName_ExternalHarness_ReportsNothing(t *testing.T) {
	root := makeRoot(t)
	addExternal(t, root, "external-dir", "External")
	reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: true})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got, ok := reporterOf(t, reg).InjectionDirName("external-dir"); ok {
		t.Errorf("InjectionDirName = (%q, true), want ok == false for an external harness", got)
	}
}

func TestInjectionDirName_DescriptorOverridingBuiltin_ReportsNothing(t *testing.T) {
	id := registerAskingFor(t, "overridden", catalogpaths.HarnessDirNameClaudeCode)
	root := makeRoot(t)
	addDescriptorOnly(t, root, id, "Overrides built-in")
	reg, err := registry.Discover(registry.Options{MosaicRoot: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got, ok := reporterOf(t, reg).InjectionDirName(id); ok {
		t.Errorf("InjectionDirName(%q) = (%q, true), want ok == false once a descriptor-only entry replaces the built-in", id, got)
	}
}

func TestInjectionDirName_DiscoverForStackAndRebuild_ReportTheAskedDir(t *testing.T) {
	id := registerAskingFor(t, "stack", catalogpaths.HarnessDirNameVSCodeGHCP)
	root := makeRoot(t)

	forStack, err := registry.DiscoverForStack(registry.Options{MosaicRoot: root}, injections.Result{})
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	rebuilt, err := forStack.(registry.Rebuilder).RebuildForStack(injections.Result{})
	if err != nil {
		t.Fatalf("RebuildForStack: %v", err)
	}

	for name, reg := range map[string]registry.Registry{"DiscoverForStack": forStack, "RebuildForStack": rebuilt} {
		got, ok := reporterOf(t, reg).InjectionDirName(id)
		if !ok || got != catalogpaths.HarnessDirNameVSCodeGHCP {
			t.Errorf("%s: InjectionDirName(%q) = (%q, %v), want (%q, true)", name, id, got, ok, catalogpaths.HarnessDirNameVSCodeGHCP)
		}
	}
}
