package codextoml_test

// read_settings_test.go covers the read-side capability of the Codex translator:
// every top-level key of a deployed file is returned with its decoded value, in file
// order, including values Decode only carries as key names in
// mosaic_carriage_markers (tables, arrays, floats, commented keys).

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-common/agentformat"
	"mosaic-common/agentformat/formatid"
)

const settingsFile = `name = "reviewer"
model = "gpt-5"
developer_instructions = "Review."
pi = 3.14
tags = ["a", "b"]
# keep this at three
retries = 3

[sandbox_workspace_write]
network_access = true

[mcp_servers.docs]
command = "docs-server"
`

func settingsReader(t *testing.T) agentformat.SettingsReader {
	t.Helper()
	tr, err := agentformat.Lookup(formatid.CodexTOML)
	if err != nil {
		t.Fatalf("Lookup(codex-toml): %v", err)
	}
	r, ok := tr.(agentformat.SettingsReader)
	if !ok {
		t.Fatalf("codex-toml translator %T does not implement agentformat.SettingsReader", tr)
	}
	return r
}

func TestReadSettings_ReturnsEveryKeyInFileOrderWithValues(t *testing.T) {
	got, err := settingsReader(t).ReadSettings([]byte(settingsFile))
	if err != nil {
		t.Fatalf("ReadSettings error: %v", err)
	}

	want := []agentformat.Setting{
		{Key: "name", Value: "reviewer"},
		{Key: "model", Value: "gpt-5"},
		{Key: "developer_instructions", Value: "Review."},
		{Key: "pi", Value: 3.14},
		{Key: "tags", Value: []any{"a", "b"}},
		{Key: "retries", Value: int64(3)},
		{Key: "sandbox_workspace_write", Value: map[string]any{"network_access": true}},
		{Key: "mcp_servers", Value: map[string]any{"docs": map[string]any{"command": "docs-server"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSettings =\n%#v\nwant\n%#v", got, want)
	}
}

func TestReadSettings_StampBlockIsNotASetting(t *testing.T) {
	file := "# mosaic_id: reviewer\n" + settingsFile

	got, err := settingsReader(t).ReadSettings([]byte(file))
	if err != nil {
		t.Fatalf("ReadSettings error: %v", err)
	}
	for _, s := range got {
		if s.Key == "mosaic_id" || s.Key == "# mosaic_id" {
			t.Errorf("stamp surfaced as setting %q", s.Key)
		}
	}
	if len(got) != 8 {
		t.Errorf("len(settings) = %d, want 8", len(got))
	}
}

func TestReadSettings_MalformedTomlIsErrMalformedDeployed(t *testing.T) {
	_, err := settingsReader(t).ReadSettings([]byte("name = \"x\nmodel = = 1\n"))

	if !errors.Is(err, agentformat.ErrMalformedDeployed) {
		t.Fatalf("error %v does not wrap ErrMalformedDeployed", err)
	}
	var ae *agentformat.ArtifactError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not an *ArtifactError", err)
	}
	if ae.Phase != "decode" {
		t.Errorf("Phase = %q, want decode", ae.Phase)
	}
}

func TestReadSettings_DoesNotModifyInput(t *testing.T) {
	input := []byte(settingsFile)
	before := append([]byte(nil), input...)

	_, _ = settingsReader(t).ReadSettings(input)

	if string(input) != string(before) {
		t.Error("ReadSettings modified its input")
	}
}
