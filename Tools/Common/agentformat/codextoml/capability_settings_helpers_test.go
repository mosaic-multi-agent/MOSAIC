package codextoml_test

// capability_settings_helpers_test.go holds the shared helpers of the capability-settings
// tests (web_search and the sandbox_workspace_write table): fixture files, canonical
// documents built from a deployed file, and TOML inspection.

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"mosaic-common/agentformat"
	"mosaic-common/docformat"
	"mosaic-common/mosaic"
)

const capAgent = "cap-agent"

// capPreChangeFile is a Codex agent file as Deploy wrote it before the capability settings
// existed: no web_search key and no sandbox_workspace_write table. It carries a user root
// key and a user table so layout and carriage are observable.
const capPreChangeFile = `name = "cap-agent"
description = "A capability agent."
model = "gpt-5.6-sol"
sandbox_mode = "workspace-write"
developer_instructions = "Prior body.\n"
user_key = "keep-me"

[mcp_servers.demo]
command = "demo"
`

// capCurrentFile is the same agent as Deploy writes it now, with both settings.
const capCurrentFile = `name = "cap-agent"
description = "A capability agent."
model = "gpt-5.6-sol"
sandbox_mode = "workspace-write"
web_search = "live"
developer_instructions = "Prior body.\n"
user_key = "keep-me"

[sandbox_workspace_write]
network_access = true

[mcp_servers.demo]
command = "demo"
`

func capUpdateCtx(prior []byte, refresh bool) agentformat.ArtifactContext {
	return agentformat.ArtifactContext{
		AgentKey:      capAgent,
		Op:            agentformat.OpUpdate,
		PriorDeployed: prior,
		RefreshMode:   refresh,
	}
}

// capDecode decodes a deployed file to canonical bytes.
func capDecode(t *testing.T, file string) []byte {
	t.Helper()
	out, _, err := codexTranslator(t).Decode([]byte(file), agentformat.ArtifactContext{AgentKey: capAgent})
	if err != nil {
		t.Fatalf("Decode: %v\n%s", err, file)
	}
	return out
}

// capFrontmatter parses canonical bytes and returns the frontmatter.
func capFrontmatter(t *testing.T, canonical []byte) *docformat.Frontmatter {
	t.Helper()
	doc, err := docformat.Parse(canonical)
	if err != nil {
		t.Fatalf("docformat.Parse: %v\n%s", err, canonical)
	}
	return doc.Frontmatter()
}

// capScalar returns the scalar text of a frontmatter key, failing when absent or not scalar.
func capScalar(t *testing.T, fm *docformat.Frontmatter, key string) string {
	t.Helper()
	v, ok := fm.Get(key)
	if !ok {
		t.Fatalf("frontmatter has no %q; keys: %v", key, fm.Keys())
	}
	if v.Kind != mosaic.KindScalar {
		t.Fatalf("frontmatter %q kind = %v, want scalar", key, v.Kind)
	}
	return v.Scalar
}

// capPair returns the value of a leaf in the sandbox_workspace_write mapping.
func capPair(t *testing.T, fm *docformat.Frontmatter, leaf string) (mosaic.FieldValue, bool) {
	t.Helper()
	v, ok := fm.Get("sandbox_workspace_write")
	if !ok {
		t.Fatalf("frontmatter has no sandbox_workspace_write; keys: %v", fm.Keys())
	}
	if v.Kind != mosaic.KindMapping {
		t.Fatalf("sandbox_workspace_write kind = %v, want mapping", v.Kind)
	}
	for _, p := range v.Pairs {
		if p.Key == leaf {
			return p.Value, true
		}
	}
	return mosaic.FieldValue{}, false
}

// capCanonicalWith decodes file, then sets the given frontmatter entries on the canonical
// document (a nil value removes the key). It models the canonical document Transform
// hands to Encode: the decoded deployed form with the freshly computed settings applied.
func capCanonicalWith(t *testing.T, file string, set map[string]*mosaic.FieldValue) []byte {
	t.Helper()
	doc, err := docformat.Parse(capDecode(t, file))
	if err != nil {
		t.Fatalf("docformat.Parse: %v", err)
	}
	fm := doc.Frontmatter()
	for k, v := range set {
		if v == nil {
			fm.Remove(k)
			continue
		}
		if err := fm.Set(k, *v); err != nil {
			t.Fatalf("Set(%q): %v", k, err)
		}
	}
	return doc.Bytes()
}

func capScalarValue(text string) *mosaic.FieldValue {
	v := mosaic.ScalarValue(text, mosaic.QuotePlain)
	return &v
}

func capNetworkTable(leaves ...mosaic.FieldPair) *mosaic.FieldValue {
	v := mosaic.MappingValue(leaves)
	return &v
}

func capBool(key string, on bool) mosaic.FieldPair {
	text := "false"
	if on {
		text = "true"
	}
	return mosaic.FieldPair{Key: key, Value: mosaic.ScalarValue(text, mosaic.QuotePlain)}
}

// capEncode encodes canonical bytes and returns the file text.
func capEncode(t *testing.T, canonical []byte, ctx agentformat.ArtifactContext) string {
	t.Helper()
	out, _, err := codexTranslator(t).Encode(canonical, ctx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return string(out)
}

// capToml parses a file as TOML.
func capToml(t *testing.T, file string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := toml.Unmarshal([]byte(file), &m); err != nil {
		t.Fatalf("not valid TOML: %v\n%s", err, file)
	}
	return m
}

// capLine returns the index of the first line that equals want (trimmed), or -1.
func capLine(file, want string) int {
	for i, l := range strings.Split(file, "\n") {
		if strings.TrimSpace(l) == want {
			return i
		}
	}
	return -1
}

// capLinePrefix returns the index of the first line starting with prefix, or -1.
func capLinePrefix(file, prefix string) int {
	for i, l := range strings.Split(file, "\n") {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}
