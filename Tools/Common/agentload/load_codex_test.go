package agentload_test

// load_codex_test.go covers loading Codex TOML agent files: owned keys and
// instructions arrive in the standard form, and every declared setting (including
// tables, arrays, floats and commented keys) is available read-only with its value.

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/agentload"
)

const codexFormat = "codex-toml"

func TestLoad_Codex_OwnedKeysArriveInStandardForm(t *testing.T) {
	a := mustLoad(t, fixture("codex", "full.toml"), codexFormat)

	for key, want := range map[string]string{
		"name":         "reviewer",
		"description":  "Reviews code",
		"model":        "gpt-5",
		"sandbox_mode": "workspace-write",
	} {
		if got := scalar(t, a, key); got != want {
			t.Errorf("frontmatter %q = %q, want %q", key, got, want)
		}
	}
	if !strings.Contains(string(a.Document.Bytes()), "You review code.") {
		t.Errorf("standard form does not carry the developer instructions:\n%s", a.Document.Bytes())
	}
	if !strings.Contains(string(a.Canonical), "You review code.") {
		t.Errorf("Canonical does not carry the developer instructions")
	}
	if a.FormatID != codexFormat {
		t.Errorf("FormatID = %q, want %q", a.FormatID, codexFormat)
	}
}

func TestLoad_Codex_SettingsKeysInFileOrder(t *testing.T) {
	a := mustLoad(t, fixture("codex", "full.toml"), codexFormat)

	var keys []string
	for _, s := range a.Settings {
		keys = append(keys, s.Key)
	}
	want := []string{
		"name", "description", "model", "sandbox_mode", "web_search",
		"developer_instructions", "pi", "tags", "retries",
		"sandbox_workspace_write", "mcp_servers",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("Settings keys = %v, want %v", keys, want)
	}
}

func TestLoad_Codex_SettingsCarryTypedValues(t *testing.T) {
	a := mustLoad(t, fixture("codex", "full.toml"), codexFormat)
	got := map[string]any{}
	for _, s := range a.Settings {
		got[s.Key] = s.Value
	}

	want := map[string]any{
		"name":         "reviewer",
		"sandbox_mode": "workspace-write",
		"web_search":   "live",
		"pi":           3.14,
		"tags":         []any{"alpha", "beta"},
		"retries":      int64(3),
		"sandbox_workspace_write": map[string]any{
			"network_access": true,
		},
		"mcp_servers": map[string]any{
			"docs": map[string]any{"command": "docs-server"},
		},
	}
	for key, w := range want {
		if !reflect.DeepEqual(got[key], w) {
			t.Errorf("setting %q = %#v (%T), want %#v (%T)", key, got[key], got[key], w, w)
		}
	}
}

func TestLoad_Codex_MalformedTomlIsUndecodable(t *testing.T) {
	path := fixture("codex", "malformed.toml")

	a, err := agentload.Load(path, codexFormat)

	if err == nil {
		t.Fatalf("Load returned nil error for malformed TOML (agent %v)", a)
	}
	if a != nil {
		t.Error("Load returned a non-nil agent alongside an error")
	}
	if !errors.Is(err, agentload.ErrUndecodable) {
		t.Errorf("error %v does not wrap ErrUndecodable", err)
	}
	assertNamesFile(t, err, path)
}

func TestLoad_Codex_DoesNotModifyFile(t *testing.T) {
	path := copyFixture(t, "codex", "full.toml")
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)

	mustLoad(t, path, codexFormat)

	assertUnchanged(t, path, before, info)
}

func TestLoad_Codex_FailedLoadDoesNotModifyFile(t *testing.T) {
	path := copyFixture(t, "codex", "malformed.toml")
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)

	_, _ = agentload.Load(path, codexFormat)

	assertUnchanged(t, path, before, info)
}
