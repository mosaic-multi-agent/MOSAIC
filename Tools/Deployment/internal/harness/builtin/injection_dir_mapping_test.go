package builtin_test

// injection_dir_mapping_test.go specifies that a registry reports, for each real built-in
// harness id, the HarnessInjections directory that built-in reads, so the mapping always follows
// the directory the built-in actually asks for.

import (
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/harness/registry"
)

// wantInjectionDirs maps each built-in harness id to its injection directory name.
var wantInjectionDirs = map[string]string{
	"claude-code": catalogpaths.HarnessDirNameClaudeCode,
	"opencode":    catalogpaths.HarnessDirNameOpenCode,
	"ghcp-cli":    catalogpaths.HarnessDirNameGhcpCLI,
	"vscode-ghcp": catalogpaths.HarnessDirNameVSCodeGHCP,
}

func assertBuiltinInjectionDirs(t *testing.T, reg registry.Registry) {
	t.Helper()
	rep, ok := reg.(registry.InjectionDirReporter)
	if !ok {
		t.Fatalf("%T does not implement registry.InjectionDirReporter", reg)
	}
	for id, want := range wantInjectionDirs {
		got, found := rep.InjectionDirName(id)
		if !found || got != want {
			t.Errorf("InjectionDirName(%q) = (%q, %v), want (%q, true)", id, got, found, want)
		}
	}
}

func TestInjectionDirName_RealBuiltins_Discover(t *testing.T) {
	// The resolver supplies nothing, so every built-in fails construction: the mapping must
	// still report the directory each one asked for.
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: t.TempDir(),
		BuiltinContentDir: func(dir string) (string, error) {
			return "", &injections.NotSuppliedError{HarnessDir: dir}
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	assertBuiltinInjectionDirs(t, reg)
}

func TestInjectionDirName_RealBuiltins_DiscoverForStackAndRebuild(t *testing.T) {
	forStack, err := registry.DiscoverForStack(registry.Options{MosaicRoot: t.TempDir()}, injections.Result{})
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	rebuilt, err := forStack.(registry.Rebuilder).RebuildForStack(injections.Result{})
	if err != nil {
		t.Fatalf("RebuildForStack: %v", err)
	}

	assertBuiltinInjectionDirs(t, forStack)
	assertBuiltinInjectionDirs(t, rebuilt)
}
