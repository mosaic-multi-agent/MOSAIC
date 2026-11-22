package main

// Composition of the Codex harness: buildAdapter returns the Codex adapter for
// "codex" (never the mock), with the default executable "codex" or the
// --executable-path override, and without the OpenCode-style safety net.

import (
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/harness/codex"
)

func TestBuildAdapter_Codex_ReturnsCodexAdapter(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDCodex, "/custom/codex", "", nonHITLTimeouts(45*time.Minute), nil)

	if _, isMock := h.(*harness.MockAdapter); isMock {
		t.Fatal("buildAdapter(codex) returned *harness.MockAdapter: the composition root has no codex case")
	}
	if _, ok := h.(*codex.CodexAdapter); !ok {
		t.Errorf("buildAdapter(codex) returned %T, want *codex.CodexAdapter", h)
	}
}

func TestBuildAdapter_Codex_DefaultExecutableIsCodex(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDCodex, "", "", nonHITLTimeouts(30*time.Minute), nil)

	rev := requireExecutableRevealer(t, h)
	if got := rev.ExecutablePath(); got != "codex" {
		t.Errorf("default ExecutablePath() = %q, want %q", got, "codex")
	}
}

func TestBuildAdapter_Codex_OverrideWinsOverDefault(t *testing.T) {
	const override = "/opt/codex/bin/codex-custom"
	h := buildAdapter("", commonharness.HarnessIDCodex, override, "", nonHITLTimeouts(30*time.Minute), nil)

	rev := requireExecutableRevealer(t, h)
	if got := rev.ExecutablePath(); got != override {
		t.Errorf("ExecutablePath() = %q, want the override %q", got, override)
	}
}

func TestBuildAdapter_Codex_ZeroTimeoutStillBuildsCodexAdapter(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDCodex, "codex", "", nonHITLTimeouts(0), nil)

	if _, ok := h.(*codex.CodexAdapter); !ok {
		t.Errorf("buildAdapter(codex, timeout=0) returned %T, want *codex.CodexAdapter", h)
	}
}

func TestBuildAdapter_Codex_SupportsRawConsultationAndRunID(t *testing.T) {
	runFolder, _ := mlRunFolder(t)

	h := buildAdapter(runFolder, commonharness.HarnessIDCodex, "codex", "", nonHITLTimeouts(time.Minute), nil)

	if _, ok := h.(domain.RawInvoker); !ok {
		t.Error("Codex adapter lost domain.RawInvoker: routing consultation would silently degrade")
	}
	rep, ok := h.(runIDReporter)
	if !ok || rep.RunID() != mlRunID {
		t.Errorf("runIDReporter = %v (ok=%v), want run id %s", rep, ok, mlRunID)
	}
}
