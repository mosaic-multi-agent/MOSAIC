package main

// Tests that the TUI auto-selection pre-scan treats --out as a flag that takes
// a value, so the file name after it is not mistaken for a subcommand.

import "testing"

func TestHasPositionalArg_OutFlagSpaceSeparated_ReturnsFalse(t *testing.T) {
	if hasPositionalArg([]string{"--out", "runs.digest.json"}) {
		t.Error("hasPositionalArg([--out FILE]) = true, want false - FILE is the flag value")
	}
}

func TestHasPositionalArg_OutFlagEqualsSeparated_ReturnsFalse(t *testing.T) {
	if hasPositionalArg([]string{"--out=runs.digest.json"}) {
		t.Error("hasPositionalArg([--out=FILE]) = true, want false")
	}
}

func TestHasPositionalArg_ExportSubcommandWithOut_ReturnsTrue(t *testing.T) {
	if !hasPositionalArg([]string{"export", "--out", "runs.digest.json"}) {
		t.Error("hasPositionalArg([export --out FILE]) = false, want true - export is a subcommand")
	}
}

func TestHasPositionalArg_ForceIsBoolean_DoesNotConsumeNext(t *testing.T) {
	if !hasPositionalArg([]string{"--force", "export"}) {
		t.Error("hasPositionalArg([--force export]) = false, want true - --force takes no value")
	}
}

func TestWantsTUI_OutValueNotTreatedAsPositional_ConsultsSeam(t *testing.T) {
	seamCalled := false
	seam := func(fd uintptr) bool { seamCalled = true; return true }

	result := wantsTUI([]string{"--out", "runs.digest.json"}, seam)

	if !seamCalled {
		t.Error("wantsTUI should consult the terminal seam - the --out value is not a positional argument")
	}
	if !result {
		t.Error("wantsTUI([--out FILE], seam=true) = false, want true")
	}
}

func TestWantsTUI_ExportSubcommandWithOut_ForcesCLI(t *testing.T) {
	seamCalled := false
	seam := func(fd uintptr) bool { seamCalled = true; return true }

	result := wantsTUI([]string{"export", "--out", "runs.digest.json"}, seam)

	if result || seamCalled {
		t.Errorf("wantsTUI([export --out FILE]) = %v, seamCalled=%v; want CLI without consulting the seam", result, seamCalled)
	}
}
