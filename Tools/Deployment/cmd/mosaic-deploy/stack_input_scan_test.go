package main

// stack_input_scan_test.go verifies scanStackInput: how the raw arguments become the
// ordered user layers and the default-layer choice.

import (
	"path/filepath"
	"reflect"
	"testing"
)

func absOf(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", p, err)
	}
	return abs
}

func TestScanStackInput_NoStackFlags_NotGiven(t *testing.T) {
	got := scanStackInput([]string{"deploy", "--harness", "h", "--workspace", "w"})

	if got.Given || len(got.Layers) != 0 || got.IncludeDefault {
		t.Errorf("scanStackInput without stack flags = %+v, want the zero value (no stack input)", got)
	}
}

func TestScanStackInput_CatalogFolder_BothFormsGiveTheSameLayer(t *testing.T) {
	dir := t.TempDir()

	space := scanStackInput([]string{"--catalog-folder", dir, "deploy"})
	equals := scanStackInput([]string{"--catalog-folder=" + dir, "deploy"})

	want := stackInput{Layers: []string{absOf(t, dir)}, Given: true}
	if !reflect.DeepEqual(space, want) {
		t.Errorf("space form = %+v, want %+v", space, want)
	}
	if !reflect.DeepEqual(equals, want) {
		t.Errorf("equals form = %+v, want %+v", equals, want)
	}
}

func TestScanStackInput_CatalogFolder_RepeatedKeepsArgumentOrderHighestFirst(t *testing.T) {
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()

	got := scanStackInput([]string{"--catalog-folder", a, "deploy", "--catalog-folder=" + b, "--catalog-folder", c})

	want := []string{absOf(t, a), absOf(t, b), absOf(t, c)}
	if !reflect.DeepEqual(got.Layers, want) {
		t.Errorf("Layers = %v, want %v (argument order, first is highest)", got.Layers, want)
	}
}

func TestScanStackInput_CatalogFolder_ValueIsTrimmedUnquotedAndMadeAbsolute(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("some", "relative", "catalog")

	got := scanStackInput([]string{"--catalog-folder", `  "` + dir + `"  `, "--catalog-folder=" + rel})

	want := []string{absOf(t, dir), absOf(t, rel)}
	if !reflect.DeepEqual(got.Layers, want) {
		t.Errorf("Layers = %v, want %v (trimmed, unquoted, absolute against the working directory)", got.Layers, want)
	}
}

func TestScanStackInput_CatalogFolder_TrailingFlagWithoutValueAddsNoLayer(t *testing.T) {
	got := scanStackInput([]string{"deploy", "--catalog-folder"})

	if len(got.Layers) != 0 || got.Given {
		t.Errorf("scanStackInput with a trailing --catalog-folder = %+v, want no layer and no stack input", got)
	}
}

func TestScanStackInput_IncludeDefault_BareIsTrueAndCountsAsStackInput(t *testing.T) {
	got := scanStackInput([]string{"deploy", "--include-default-catalog"})

	if !got.Given || !got.IncludeDefault || len(got.Layers) != 0 {
		t.Errorf("scanStackInput = %+v, want Given, IncludeDefault and no user layers", got)
	}
}

func TestScanStackInput_IncludeDefault_BareDoesNotConsumeTheNextToken(t *testing.T) {
	dir := t.TempDir()

	got := scanStackInput([]string{"--include-default-catalog", "deploy", "--catalog-folder", dir})

	want := stackInput{Layers: []string{absOf(t, dir)}, IncludeDefault: true, Given: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scanStackInput = %+v, want %+v (the token after the bare flag is never its value)", got, want)
	}
}

func TestScanStackInput_IncludeDefault_EqualsFormsFollowCobraBoolForms(t *testing.T) {
	cases := []struct {
		arg  string
		want bool
	}{
		{"--include-default-catalog=true", true},
		{"--include-default-catalog=1", true},
		{"--include-default-catalog=false", false},
		{"--include-default-catalog=0", false},
	}
	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			got := scanStackInput([]string{"deploy", tc.arg})

			if !got.Given {
				t.Errorf("%s did not count as stack input", tc.arg)
			}
			if got.IncludeDefault != tc.want {
				t.Errorf("%s: IncludeDefault = %v, want %v", tc.arg, got.IncludeDefault, tc.want)
			}
		})
	}
}

func TestScanStackInput_IncludeDefault_InvalidValueIsNotStackInput(t *testing.T) {
	got := scanStackInput([]string{"deploy", "--include-default-catalog=banana"})

	if got.Given || got.IncludeDefault {
		t.Errorf("scanStackInput = %+v, want a value cobra rejects not to count as stack input", got)
	}
}

func TestScanStackInput_IncludeDefault_LastValidOccurrenceWins(t *testing.T) {
	got := scanStackInput([]string{
		"--include-default-catalog=false", "--include-default-catalog=banana", "--include-default-catalog",
		"--include-default-catalog=false",
	})

	if !got.Given || got.IncludeDefault {
		t.Errorf("scanStackInput = %+v, want the last valid occurrence (=false) to win", got)
	}
}

func TestScanStackInput_EmptyValueArgumentsDoNotDisturbTheScan(t *testing.T) {
	dir := t.TempDir()

	got := scanStackInput([]string{"deploy", "--workflows", "", "--auto-confirm", "--catalog-folder", dir})

	if !reflect.DeepEqual(got.Layers, []string{absOf(t, dir)}) {
		t.Errorf("Layers = %v, want the one catalog folder after an empty --workflows value", got.Layers)
	}
}

// The pre-scan must not change what scanGlobalFlags reports for the TUI reload path.
func TestScanStackInput_ScanGlobalFlagsStillReturnsLastCatalogFolderUnchanged(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()

	_, catalogFolder, _, _ := scanGlobalFlags([]string{"--catalog-folder", a, "--catalog-folder", b, "deploy"})

	if catalogFolder != b {
		t.Errorf("scanGlobalFlags catalogFolder = %q, want %q (raw last value, unchanged)", catalogFolder, b)
	}
}
