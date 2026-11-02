package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/ghcpcli"
)

// TestBuildAdapter_GHCPCLI_IsNotWrappedWithUsageIngest verifies that the GHCP CLI
// adapter built for a run is the bare CLI adapter: usage comes from the harness
// hook, so the Runner adds no decorator that reads Copilot OTel output, even
// when an OTel file and its environment variable are present.
func TestBuildAdapter_GHCPCLI_IsNotWrappedWithUsageIngest(t *testing.T) {
	// Arrange
	runFolder, workspace := mlRunFolder(t)
	otelFile := filepath.Join(workspace, "copilot-otel.jsonl")
	if err := os.WriteFile(otelFile, []byte(`{"type":"span"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", otelFile)
	bare := ghcpcli.NewGHCPCLIAdapterWithMode("copilot", time.Minute, domain.NopDebugLogger{}, commonharness.GHCPCLIModeBlanket)

	// Act
	got := buildAdapter(runFolder, commonharness.HarnessIDGHCPCLI, "copilot", string(commonharness.GHCPCLIModeBlanket), time.Minute)

	// Assert
	if reflect.TypeOf(got) != reflect.TypeOf(bare) {
		t.Fatalf("buildAdapter returned %T, want the undecorated %T", got, bare)
	}
}

// TestBuildAdapter_GHCPCLI_KeepsItsCapabilitiesWithoutUsageIngest verifies that
// the undecorated adapter still exposes the capabilities the Runner relies on.
func TestBuildAdapter_GHCPCLI_KeepsItsCapabilitiesWithoutUsageIngest(t *testing.T) {
	runFolder, _ := mlRunFolder(t)

	got := buildAdapter(runFolder, commonharness.HarnessIDGHCPCLI, "/opt/copilot", "", time.Minute)

	revealer, ok := got.(domain.ExecutableRevealer)
	if !ok || revealer.ExecutablePath() != "/opt/copilot" {
		t.Errorf("ExecutableRevealer missing or wrong path: %v", ok)
	}
	if _, ok := got.(domain.RawInvoker); !ok {
		t.Error("RawInvoker missing")
	}
	if reporter, ok := got.(runIDReporter); !ok || reporter.RunID() != mlRunID {
		t.Errorf("run id reporter missing or wrong: %v", ok)
	}
}
