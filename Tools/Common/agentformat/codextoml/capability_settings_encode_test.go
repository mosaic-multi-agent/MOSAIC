package codextoml_test

// capability_settings_encode_test.go covers encoding the web_search key and the
// sandbox_workspace_write table, and the encode-decode round trip. Layout contract:
// web_search sits immediately after sandbox_mode (before developer_instructions); the table
// comes after every root-level key and before any carried table.

import (
	"testing"

	"mosaic-common/agentformat"
	"mosaic-common/mosaic"
)

// capCreateCanonical is a canonical document for a write-capable agent with both settings.
func capCreateCanonical(search string, table *mosaic.FieldValue) []byte {
	out := "---\nname: cap-agent\ndescription: A capability agent.\nmodel: gpt-5.6-sol\nsandbox_mode: workspace-write\n"
	out += "web_search: " + search + "\n"
	if table != nil {
		out += "sandbox_workspace_write:\n"
		for _, p := range table.Pairs {
			out += "  " + p.Key + ": " + p.Value.Scalar + "\n"
		}
	}
	return []byte(out + "---\nBody.\n")
}

var capCreateCtx = agentformat.ArtifactContext{AgentKey: capAgent, Op: agentformat.OpCreate}

func TestEncode_WebSearch_IsEmittedAsStringValue(t *testing.T) {
	for _, level := range []string{"live", "disabled"} {
		// Arrange
		canonical := capCreateCanonical(level, nil)

		// Act
		file := capEncode(t, canonical, capCreateCtx)

		// Assert
		if got := capToml(t, file)["web_search"]; got != level {
			t.Errorf("web_search = %#v, want %q\n%s", got, level, file)
		}
	}
}

func TestEncode_WebSearch_SitsImmediatelyAfterSandboxMode(t *testing.T) {
	// Arrange
	canonical := capCreateCanonical("live", nil)

	// Act
	file := capEncode(t, canonical, capCreateCtx)

	// Assert
	sandbox := capLinePrefix(file, "sandbox_mode")
	search := capLinePrefix(file, "web_search")
	instr := capLinePrefix(file, "developer_instructions")
	if sandbox < 0 || search != sandbox+1 || instr <= search {
		t.Errorf("line order sandbox_mode=%d web_search=%d developer_instructions=%d, want web_search directly after sandbox_mode and before developer_instructions\n%s",
			sandbox, search, instr, file)
	}
}

func TestEncode_NetworkTable_IsEmittedWithGrantedValue(t *testing.T) {
	// Arrange
	table := capNetworkTable(capBool("network_access", true))

	// Act
	file := capEncode(t, capCreateCanonical("disabled", table), capCreateCtx)

	// Assert
	tbl, ok := capToml(t, file)["sandbox_workspace_write"].(map[string]any)
	if !ok || tbl["network_access"] != true {
		t.Errorf("sandbox_workspace_write = %#v, want table with network_access = true\n%s", tbl, file)
	}
}

func TestEncode_NoNetworkTable_EmitsNoTable(t *testing.T) {
	// Arrange / Act
	file := capEncode(t, capCreateCanonical("live", nil), capCreateCtx)

	// Assert
	if _, ok := capToml(t, file)["sandbox_workspace_write"]; ok {
		t.Errorf("file has a sandbox_workspace_write table the canonical document did not carry\n%s", file)
	}
}

func TestEncode_NetworkTable_FollowsEveryRootKeyAndPrecedesCarriedTables(t *testing.T) {
	// Arrange: update over a file with a user root key and a user table.
	canonical := capCanonicalWith(t, capPreChangeFile, map[string]*mosaic.FieldValue{
		"web_search":              capScalarValue("live"),
		"sandbox_workspace_write": capNetworkTable(capBool("network_access", true)),
	})

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(capPreChangeFile), false))

	// Assert
	userKey := capLine(file, `user_key = "keep-me"`)
	header := capLine(file, "[sandbox_workspace_write]")
	carried := capLine(file, "[mcp_servers.demo]")
	if userKey < 0 || header <= userKey || carried <= header {
		t.Errorf("line order user_key=%d [sandbox_workspace_write]=%d [mcp_servers.demo]=%d, want root keys, then the owned table, then carried tables\n%s",
			userKey, header, carried, file)
	}
	if m := capToml(t, file); m["user_key"] != "keep-me" {
		t.Errorf("user_key = %#v, want keep-me", m["user_key"])
	}
}

func TestEncode_NetworkTable_KeepsMappingOrderAndLeafTypes(t *testing.T) {
	// Arrange
	table := capNetworkTable(
		capBool("exclude_slash_tmp", true),
		capBool("network_access", true),
	)

	// Act
	file := capEncode(t, capCreateCanonical("live", table), capCreateCtx)

	// Assert
	first := capLinePrefix(file, "exclude_slash_tmp")
	second := capLinePrefix(file, "network_access")
	if first < 0 || second <= first {
		t.Errorf("leaf order exclude_slash_tmp=%d network_access=%d, want mapping order\n%s", first, second, file)
	}
	tbl, ok := capToml(t, file)["sandbox_workspace_write"].(map[string]any)
	if !ok {
		t.Fatalf("sandbox_workspace_write table missing from output\n%s", file)
	}
	if tbl["exclude_slash_tmp"] != true || tbl["network_access"] != true {
		t.Errorf("table = %#v, want booleans true", tbl)
	}
}

func TestRoundTrip_EncodeThenDecode_ReturnsSameSettings(t *testing.T) {
	// Arrange
	table := capNetworkTable(capBool("network_access", true))
	file := capEncode(t, capCreateCanonical("live", table), capCreateCtx)

	// Act
	fm := capFrontmatter(t, capDecode(t, file))

	// Assert
	if got := capScalar(t, fm, "web_search"); got != "live" {
		t.Errorf("web_search after round trip = %q, want live", got)
	}
	if v, ok := capPair(t, fm, "network_access"); !ok || v.Scalar != "true" {
		t.Errorf("network_access after round trip = %+v (present=%v), want true", v, ok)
	}
}

func TestRoundTrip_DecodeThenEncodeInRefreshMode_KeepsFileSemantics(t *testing.T) {
	// Arrange
	canonical := capDecode(t, capCurrentFile)

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(capCurrentFile), true))

	// Assert
	before, after := capToml(t, capCurrentFile), capToml(t, file)
	if after["web_search"] != before["web_search"] {
		t.Errorf("web_search = %#v, want %#v\n%s", after["web_search"], before["web_search"], file)
	}
	tbl, _ := after["sandbox_workspace_write"].(map[string]any)
	if tbl["network_access"] != true {
		t.Errorf("sandbox_workspace_write = %#v, want network_access true\n%s", tbl, file)
	}
	if after["user_key"] != "keep-me" {
		t.Errorf("user_key = %#v, want keep-me", after["user_key"])
	}
	if _, ok := after["mcp_servers"]; !ok {
		t.Errorf("carried mcp_servers table lost\n%s", file)
	}
}
