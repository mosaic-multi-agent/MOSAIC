package main

// Tests for the not-supported outcome of the composition root: a catalog
// harness this tool does not support (Codex today) is refused with an outcome
// distinct from the unknown-id outcome, is never offered, is never mapped to
// another harness, and no adapter or decoder is built for it. Assertions are
// on the kind of outcome and on the harness being named, not on wording.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/cli"
)

func TestUnsupportedHarnesses_IsCatalogMinusSupported_InCatalogOrder(t *testing.T) {
	supported := make(map[string]bool)
	for _, e := range supportedHarnesses() {
		supported[e.ID] = true
	}
	var want []commonharness.CLIHarness
	for _, e := range commonharness.CLIHarnesses() {
		if !supported[e.ID] {
			want = append(want, e)
		}
	}

	got := unsupportedHarnesses()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("unsupportedHarnesses() = %v, want catalog minus supported %v", got, want)
	}
}

func TestUnsupportedHarnesses_ContainsCodex_SupportedDoesNot(t *testing.T) {
	var inUnsupported, inSupported bool
	for _, e := range unsupportedHarnesses() {
		if e.ID == commonharness.HarnessIDCodex {
			inUnsupported = true
		}
	}
	for _, e := range supportedHarnesses() {
		if e.ID == commonharness.HarnessIDCodex {
			inSupported = true
		}
	}
	if !inUnsupported {
		t.Error("unsupportedHarnesses() does not contain codex")
	}
	if inSupported {
		t.Error("supportedHarnesses() offers codex; it must stay unsupported")
	}
}

func TestUnsupportedHarnesses_DoesNotContainOpenCodeV2(t *testing.T) {
	for _, e := range unsupportedHarnesses() {
		if e.ID == commonharness.HarnessIDOpenCodeV2 {
			t.Error("unsupportedHarnesses() still names opencode-v2; it is supported")
		}
	}
	if _, err := newAdapter(commonharness.HarnessIDOpenCodeV2, adapterOptions{}); errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("newAdapter(opencode-v2) error = %v, want it not to be the not-supported refusal", err)
	}
	if _, err := decoderFor(commonharness.HarnessIDOpenCodeV2); errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("decoderFor(opencode-v2) error = %v, want it not to be the not-supported refusal", err)
	}
}

func TestNewAdapter_Codex_IsUnsupportedNotUnknown_NamesCodex(t *testing.T) {
	adapter, err := newAdapter(commonharness.HarnessIDCodex, adapterOptions{})

	if err == nil {
		t.Fatal("newAdapter(codex) returned a nil error")
	}
	if !errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("newAdapter(codex) error = %v, want it to wrap ErrUnsupportedHarness", err)
	}
	if errors.Is(err, ErrUnknownHarness) {
		t.Errorf("newAdapter(codex) error = %v, must not be the unknown-harness outcome", err)
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("newAdapter(codex) error = %q, want it to name codex", err.Error())
	}
	if adapter != nil {
		t.Errorf("newAdapter(codex) built an adapter %v; none may be built, and codex must not map to another harness", adapter)
	}
}

func TestDecoderFor_Codex_IsUnsupportedNotUnknown_NamesCodex(t *testing.T) {
	dec, err := decoderFor(commonharness.HarnessIDCodex)

	if err == nil {
		t.Fatal("decoderFor(codex) returned a nil error")
	}
	if !errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("decoderFor(codex) error = %v, want it to wrap ErrUnsupportedHarness", err)
	}
	if errors.Is(err, ErrUnknownHarness) {
		t.Errorf("decoderFor(codex) error = %v, must not be the unknown-harness outcome", err)
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("decoderFor(codex) error = %q, want it to name codex", err.Error())
	}
	if dec != nil {
		t.Error("decoderFor(codex) returned a decoder; none may be built")
	}
}

func TestWiringFailureExitCode_Unsupported_IsUsage_UnknownStillUsage(t *testing.T) {
	cases := map[string]error{
		"unsupported":         fmt.Errorf("wrapped: %w", ErrUnsupportedHarness),
		"unknown (unchanged)": fmt.Errorf("wrapped: %w", ErrUnknownHarness),
	}
	for name, err := range cases {
		if got := wiringFailureExitCode(err); got != cli.ExitUsage {
			t.Errorf("%s: wiringFailureExitCode = %d, want ExitUsage (%d)", name, got, cli.ExitUsage)
		}
	}
	if got := wiringFailureExitCode(errors.New("something else")); got != cli.ExitFailure {
		t.Errorf("other error: wiringFailureExitCode = %d, want ExitFailure (%d)", got, cli.ExitFailure)
	}
}

func TestBuildDeps_CodexHarnessID_IsErrUnsupportedHarness(t *testing.T) {
	cfg := WiringConfig{
		HarnessID:       commonharness.HarnessIDCodex,
		FixtureRoot:     "C:/fixtures",
		WorkspaceRoot:   "C:/workspaces",
		SelfPath:        "C:/bin/mosaic-agent-test.exe",
		LoggerBundleDir: "C:/bundles/logger",
		CostToolPath:    "C:/bin/mosaic-log-analyzer.exe",
		CostTimeout:     30 * time.Second,
		Diag:            new(discardWriter),
	}

	d, err := buildDeps(cfg)

	if err == nil {
		t.Fatal("buildDeps with HarnessID codex returned a nil error")
	}
	if !errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("buildDeps error = %v, want it to wrap ErrUnsupportedHarness", err)
	}
	if errors.Is(err, ErrUnknownHarness) {
		t.Errorf("buildDeps error = %v, must not be the unknown-harness outcome", err)
	}
	if !reflect.ValueOf(d).IsZero() {
		t.Errorf("buildDeps returned a non-zero Deps %+v for an unsupported harness", d)
	}
}

// TestHarnessFactory_Bundle_Codex_IsUnsupportedNotUnknown verifies the per-run
// selection path refuses codex with the not-supported outcome and builds nothing.
func TestHarnessFactory_Bundle_Codex_IsUnsupportedNotUnknown(t *testing.T) {
	factory := newHarnessFactory(adapterOptions{})

	bundle, err := factory.Bundle(context.Background(), commonharness.HarnessIDCodex)

	if !errors.Is(err, ErrUnsupportedHarness) {
		t.Errorf("Bundle(codex) error = %v, want it to wrap ErrUnsupportedHarness", err)
	}
	if errors.Is(err, ErrUnknownHarness) {
		t.Errorf("Bundle(codex) error = %v, must not be the unknown-harness outcome", err)
	}
	if bundle.Adapter != nil {
		t.Error("Bundle(codex) returned an adapter; none may be built")
	}
}

// TestCLIOptions_UnsupportedHarnesses_FilledAndNeverOffered verifies the
// composition root fills UnsupportedHarnesses wherever it fills Harnesses,
// and that no unsupported harness is also offered.
func TestCLIOptions_UnsupportedHarnesses_FilledAndNeverOffered(t *testing.T) {
	opts := cliOptions(Deps{}, io.Discard, io.Discard)

	if !reflect.DeepEqual(opts.UnsupportedHarnesses, unsupportedHarnesses()) {
		t.Errorf("cliOptions.UnsupportedHarnesses = %v, want unsupportedHarnesses() %v", opts.UnsupportedHarnesses, unsupportedHarnesses())
	}
	var hasCodex bool
	for _, h := range opts.UnsupportedHarnesses {
		if h.ID == commonharness.HarnessIDCodex {
			hasCodex = true
		}
	}
	if !hasCodex {
		t.Error("cliOptions.UnsupportedHarnesses does not contain codex")
	}
	for _, h := range opts.Harnesses {
		if h.ID == commonharness.HarnessIDCodex {
			t.Error("cliOptions.Harnesses offers codex")
		}
	}
}
