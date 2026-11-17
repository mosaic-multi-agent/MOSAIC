package main

// Composition of the opencode-v2 harness: buildAdapter returns the V2 adapter
// for "opencode-v2" (never the mock), with the default executable "opencode" or
// the --executable-path override.

import (
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/harness/opencode"
	"mosaic-run/internal/harness/opencodev2"
)

func TestBuildAdapter_OpenCodeV2_ReturnsV2AdapterNotV1(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDOpenCodeV2, "/custom/opencode", "", nonHITLTimeouts(45*time.Minute), nil)

	if _, isMock := h.(*harness.MockAdapter); isMock {
		t.Fatal("buildAdapter(opencode-v2) returned *harness.MockAdapter: the composition root has no opencode-v2 case")
	}
	if _, isV1 := h.(*opencode.OpenCodeAdapter); isV1 {
		t.Fatal("buildAdapter(opencode-v2) returned the V1 adapter")
	}
	if _, ok := h.(*opencodev2.OpenCodeV2Adapter); !ok {
		t.Errorf("buildAdapter(opencode-v2) returned %T, want *opencodev2.OpenCodeV2Adapter", h)
	}
}

func TestBuildAdapter_OpenCodeV2_DefaultExecutableIsOpenCode(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDOpenCodeV2, "", "", nonHITLTimeouts(30*time.Minute), nil)

	rev := requireExecutableRevealer(t, h)
	if got := rev.ExecutablePath(); got != "opencode" {
		t.Errorf("default ExecutablePath() = %q, want %q", got, "opencode")
	}
}

func TestBuildAdapter_OpenCodeV2_OverrideWinsOverDefault(t *testing.T) {
	const override = "/opt/opencode/bin/opencode-next"
	h := buildAdapter("", commonharness.HarnessIDOpenCodeV2, override, "", nonHITLTimeouts(30*time.Minute), nil)

	rev := requireExecutableRevealer(t, h)
	if got := rev.ExecutablePath(); got != override {
		t.Errorf("ExecutablePath() = %q, want the override %q", got, override)
	}
}

func TestBuildAdapter_OpenCodeV2_ZeroTimeoutStillBuildsV2Adapter(t *testing.T) {
	h := buildAdapter("", commonharness.HarnessIDOpenCodeV2, "opencode", "", nonHITLTimeouts(0), nil)

	if _, ok := h.(*opencodev2.OpenCodeV2Adapter); !ok {
		t.Errorf("buildAdapter(opencode-v2, timeout=0) returned %T, want *opencodev2.OpenCodeV2Adapter", h)
	}
}

func TestBuildAdapter_OpenCodeV2_SupportsRawConsultationAndRunID(t *testing.T) {
	runFolder, _ := mlRunFolder(t)

	h := buildAdapter(runFolder, commonharness.HarnessIDOpenCodeV2, "opencode", "", nonHITLTimeouts(time.Minute), nil)

	if _, ok := h.(domain.RawInvoker); !ok {
		t.Error("opencode-v2 adapter lost domain.RawInvoker: routing consultation would silently degrade")
	}
	rep, ok := h.(runIDReporter)
	if !ok || rep.RunID() != mlRunID {
		t.Errorf("runIDReporter = %v (ok=%v), want run id %s", rep, ok, mlRunID)
	}
}
