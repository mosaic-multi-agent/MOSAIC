package codextoml_test

// capability_settings_decode_test.go covers decoding the web_search key and the
// sandbox_workspace_write table of a deployed Codex agent file: both are MOSAIC-owned, so they
// appear in the canonical frontmatter (not in the user-key carriage), and a value of the wrong
// type is a malformed file naming the key.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-common/agentformat"
	"mosaic-common/agentformat/codextoml"
	"mosaic-common/mosaic"
)

func TestDecode_WebSearch_AppearsInCanonicalFrontmatter(t *testing.T) {
	// Arrange / Act
	fm := capFrontmatter(t, capDecode(t, capCurrentFile))

	// Assert
	if got := capScalar(t, fm, "web_search"); got != "live" {
		t.Errorf("web_search = %q, want live", got)
	}
}

func TestDecode_SandboxWorkspaceWrite_AppearsAsMapping(t *testing.T) {
	// Arrange / Act
	fm := capFrontmatter(t, capDecode(t, capCurrentFile))

	// Assert
	v, ok := capPair(t, fm, "network_access")
	if !ok || v.Kind != mosaic.KindScalar || v.Scalar != "true" {
		t.Errorf("sandbox_workspace_write.network_access = %+v (present=%v), want scalar true", v, ok)
	}
}

func TestDecode_CapabilitySettings_AreNotCarriedAsUserKeys(t *testing.T) {
	// Arrange / Act
	fm := capFrontmatter(t, capDecode(t, capCurrentFile))

	// Assert: the owned settings are frontmatter keys; the container names no user key but
	// the genuinely user-owned ones.
	for _, key := range []string{agentformat.CarriageValuesKey, agentformat.CarriageMarkersKey} {
		v, ok := fm.Get(key)
		if !ok {
			continue
		}
		text := mosaicText(v)
		for _, owned := range []string{"web_search", "sandbox_workspace_write"} {
			if strings.Contains(text, owned) {
				t.Errorf("%s names owned key %q: %s", key, owned, text)
			}
		}
	}
}

func TestDecode_PreChangeFile_HasNeitherSetting(t *testing.T) {
	// Arrange / Act
	fm := capFrontmatter(t, capDecode(t, capPreChangeFile))

	// Assert
	if slices.Contains(fm.Keys(), "web_search") || slices.Contains(fm.Keys(), "sandbox_workspace_write") {
		t.Errorf("decoded keys %v include a capability setting the file does not carry", fm.Keys())
	}
}

func TestDecode_TableLeafTypes_BoolStringIntegerAndStringArray(t *testing.T) {
	// Arrange
	file := `name = "cap-agent"
sandbox_mode = "workspace-write"
developer_instructions = "Body.\n"

[sandbox_workspace_write]
network_access = false
writable_roots = ["/a", "/b"]
exclude_slash_tmp = true
extra_label = "x"
extra_count = 3
`

	// Act
	fm := capFrontmatter(t, capDecode(t, file))

	// Assert
	if v, _ := capPair(t, fm, "network_access"); v.Scalar != "false" {
		t.Errorf("network_access = %+v, want false", v)
	}
	roots, _ := capPair(t, fm, "writable_roots")
	if roots.Kind != mosaic.KindList || len(roots.Items) != 2 || roots.Items[0].Scalar != "/a" || roots.Items[1].Scalar != "/b" {
		t.Errorf("writable_roots = %+v, want list [/a /b]", roots)
	}
	if v, _ := capPair(t, fm, "extra_label"); v.Scalar != "x" {
		t.Errorf("extra_label = %+v, want x", v)
	}
	if v, _ := capPair(t, fm, "extra_count"); v.Scalar != "3" {
		t.Errorf("extra_count = %+v, want 3", v)
	}
}

func TestDecode_CapabilitySettings_WrongTypes_AreMalformedNamingTheKey(t *testing.T) {
	cases := []struct {
		name string
		body string
		key  string
	}{
		{"web_search_integer", "web_search = 3\n", "web_search"},
		{"web_search_boolean", "web_search = true\n", "web_search"},
		{"web_search_table", "[web_search]\nmode = \"live\"\n", "web_search"},
		{"table_is_string", "sandbox_workspace_write = \"yes\"\n", "sandbox_workspace_write"},
		{"table_leaf_float", "[sandbox_workspace_write]\nratio = 1.5\n", "sandbox_workspace_write"},
		{"table_leaf_nested_table", "[sandbox_workspace_write.inner]\nflag = true\n", "sandbox_workspace_write"},
		{"table_leaf_mixed_array", "[sandbox_workspace_write]\nwritable_roots = [\"/a\", 2]\n", "sandbox_workspace_write"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange: root keys must precede any table in the file.
			file := "name = \"cap-agent\"\nsandbox_mode = \"workspace-write\"\ndeveloper_instructions = \"Body.\\n\"\n"
			if tc.body[0] != '[' {
				file += tc.body
			} else {
				file += "\n" + tc.body
			}

			// Act
			ae := decodeExpectErr(t, []byte(file), agentformat.ErrMalformedDeployed)

			// Assert
			if ae.Key != tc.key {
				t.Errorf("ArtifactError.Key = %q, want %q", ae.Key, tc.key)
			}
			if !errors.Is(ae, agentformat.ErrMalformedDeployed) {
				t.Errorf("error %v does not wrap ErrMalformedDeployed", ae)
			}
		})
	}
}

func TestOwnership_CapabilitySettings_AreMosaicOwnedNotStamps(t *testing.T) {
	for _, key := range []string{"web_search", "sandbox_workspace_write"} {
		if !codextoml.IsMosaicOwned(key) {
			t.Errorf("IsMosaicOwned(%q) = false, want true", key)
		}
		if codextoml.IsStampKey(key) {
			t.Errorf("IsStampKey(%q) = true, want false (Codex-native key, not a stamp)", key)
		}
	}
}

// mosaicText flattens a field value into text for substring checks.
func mosaicText(v mosaic.FieldValue) string {
	out := v.Scalar
	for _, p := range v.Pairs {
		out += " " + p.Key + " " + mosaicText(p.Value)
	}
	for _, it := range v.Items {
		out += " " + mosaicText(it)
	}
	return out
}
