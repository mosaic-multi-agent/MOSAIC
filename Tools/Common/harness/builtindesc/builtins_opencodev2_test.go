package builtindesc_test

// Tests for the opencode-v2 builtin descriptor facts that Common exposes and
// for the experimental reason carried by every builtin descriptor.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness/builtindesc"
)

func builtinByID(t *testing.T, id string) builtindesc.Descriptor {
	t.Helper()
	all, err := builtindesc.Builtins()
	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}
	for _, d := range all {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("builtin descriptor %q not found", id)
	return builtindesc.Descriptor{}
}

func TestBuiltins_ExperimentalReasonOnlyOnExperimentalHarnesses(t *testing.T) {
	all, err := builtindesc.Builtins()
	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}

	for _, d := range all {
		experimental := d.ID == "codex" || d.ID == "opencode-v2"
		if experimental && strings.TrimSpace(d.ExperimentalReason) == "" {
			t.Errorf("%s: ExperimentalReason is empty, want the descriptor's reason", d.ID)
		}
		if !experimental && d.ExperimentalReason != "" {
			t.Errorf("%s: ExperimentalReason = %q, want empty", d.ID, d.ExperimentalReason)
		}
	}
}

func TestBuiltins_CodexExperimentalReasonIsVerbatim(t *testing.T) {
	got := builtinByID(t, "codex").ExperimentalReason

	const prefix = "Codex agents run through mosaic-run only."
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("codex ExperimentalReason = %q, want it to start with %q", got, prefix)
	}
}

func TestBuiltins_OpenCodeV2ExperimentalReasonStatesSupportedButUntested(t *testing.T) {
	got := strings.ToLower(builtinByID(t, "opencode-v2").ExperimentalReason)

	if !strings.Contains(got, "supported") {
		t.Errorf("opencode-v2 ExperimentalReason = %q, want it to state that everything is supported", got)
	}
	if !strings.Contains(got, "not yet tested") {
		t.Errorf("opencode-v2 ExperimentalReason = %q, want it to state the harness is not yet tested", got)
	}
}

func TestBuiltins_OpenCodeV2ModelsMirrorOpenCodeWithVariantHint(t *testing.T) {
	v1 := builtinByID(t, "opencode")
	v2 := builtinByID(t, "opencode-v2")

	if v2.Models == nil || v1.Models == nil {
		t.Fatalf("models missing: v1=%v v2=%v", v1.Models, v2.Models)
	}
	if !reflect.DeepEqual(v2.Models.IDs, v1.Models.IDs) {
		t.Errorf("opencode-v2 model ids = %v, want the same list and order as opencode %v", v2.Models.IDs, v1.Models.IDs)
	}
	if v2.Models.FormatHint != "provider/model-id[#variant]" {
		t.Errorf("opencode-v2 FormatHint = %q, want %q", v2.Models.FormatHint, "provider/model-id[#variant]")
	}
}

func TestBuiltins_OpenCodeV2IsLastBuiltin(t *testing.T) {
	ids := builtindesc.IDs()

	if got := ids[len(ids)-1]; got != "opencode-v2" {
		t.Errorf("last builtin id = %q, want %q", got, "opencode-v2")
	}
}
