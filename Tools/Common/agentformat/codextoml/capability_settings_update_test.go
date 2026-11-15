package codextoml_test

// capability_settings_update_test.go covers the capability settings on an existing deployed
// Codex file: update recomputes them from the canonical document (including for a file
// deployed before the settings existed), and harness-only refresh keeps the deployed values
// untouched.

import (
	"strings"
	"testing"

	"mosaic-common/mosaic"
)

func TestUpdate_PreChangeFile_GainsTheSettingsAndKeepsUserContent(t *testing.T) {
	// Arrange: a file written before the settings existed, updated with the settings Tools
	// now computes for the agent.
	canonical := capCanonicalWith(t, capPreChangeFile, map[string]*mosaic.FieldValue{
		"web_search":              capScalarValue("live"),
		"sandbox_workspace_write": capNetworkTable(capBool("network_access", true)),
	})

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(capPreChangeFile), false))

	// Assert
	m := capToml(t, file)
	if m["web_search"] != "live" {
		t.Errorf("web_search = %#v, want live\n%s", m["web_search"], file)
	}
	if tbl, _ := m["sandbox_workspace_write"].(map[string]any); tbl["network_access"] != true {
		t.Errorf("sandbox_workspace_write = %#v, want network_access true\n%s", tbl, file)
	}
	if m["user_key"] != "keep-me" {
		t.Errorf("user_key = %#v, want keep-me", m["user_key"])
	}
	if _, ok := m["mcp_servers"]; !ok {
		t.Errorf("carried mcp_servers table lost\n%s", file)
	}
}

func TestUpdate_PreChangeFile_DecodedFirstIsNotTreatedAsUserContent(t *testing.T) {
	// Arrange: decode the pre-change file and encode it back with the settings added; the
	// settings must be ordinary owned keys, not carried user keys that collide on write.
	canonical := capCanonicalWith(t, capPreChangeFile, map[string]*mosaic.FieldValue{
		"web_search": capScalarValue("disabled"),
	})

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(capPreChangeFile), false))

	// Assert
	m := capToml(t, file)
	if m["web_search"] != "disabled" {
		t.Errorf("web_search = %#v, want disabled\n%s", m["web_search"], file)
	}
	if _, ok := m["sandbox_workspace_write"]; ok {
		t.Errorf("file has a sandbox_workspace_write table nothing granted\n%s", file)
	}
	if n := countLines(file, "web_search"); n != 1 {
		t.Errorf("web_search appears %d times, want once\n%s", n, file)
	}
}

func TestUpdate_AgentNoLongerGrantedNetwork_DropsTheTable(t *testing.T) {
	// Arrange: the deployed file has the table; the recomputed settings no longer grant it.
	canonical := capCanonicalWith(t, capCurrentFile, map[string]*mosaic.FieldValue{
		"web_search":              capScalarValue("disabled"),
		"sandbox_workspace_write": nil,
	})

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(capCurrentFile), false))

	// Assert
	m := capToml(t, file)
	if _, ok := m["sandbox_workspace_write"]; ok {
		t.Errorf("sandbox_workspace_write still present after the grant was withdrawn\n%s", file)
	}
	if m["web_search"] != "disabled" {
		t.Errorf("web_search = %#v, want disabled", m["web_search"])
	}
}

func TestUpdate_UserAddedLeafInsideTable_IsReplacedByComputedTable(t *testing.T) {
	// Arrange: the user added a leaf to the owned table; the computed mapping has only the grant.
	deployed := `name = "cap-agent"
description = "A capability agent."
sandbox_mode = "workspace-write"
web_search = "live"
developer_instructions = "Prior body.\n"

[sandbox_workspace_write]
network_access = true
writable_roots = ["/user/added"]
`
	canonical := capCanonicalWith(t, deployed, map[string]*mosaic.FieldValue{
		"sandbox_workspace_write": capNetworkTable(capBool("network_access", true)),
	})

	// Act
	file := capEncode(t, canonical, capUpdateCtx([]byte(deployed), false))

	// Assert
	tbl, _ := capToml(t, file)["sandbox_workspace_write"].(map[string]any)
	if tbl["network_access"] != true {
		t.Errorf("network_access = %#v, want true", tbl["network_access"])
	}
	if _, ok := tbl["writable_roots"]; ok {
		t.Errorf("user-added leaf writable_roots survived; the computed table replaces the owned table\n%s", file)
	}
}

func TestRefresh_FileWithSettings_KeepsDeployedValues(t *testing.T) {
	// Arrange: refresh re-encodes the decoded deployed file with harness-only changes; the
	// deployed settings are canonical content and must come back unchanged.
	deployed := `name = "cap-agent"
description = "A capability agent."
sandbox_mode = "workspace-write"
web_search = "disabled"
developer_instructions = "Prior body.\n"

[sandbox_workspace_write]
network_access = false
`

	// Act
	file := capEncode(t, capDecode(t, deployed), capUpdateCtx([]byte(deployed), true))

	// Assert
	m := capToml(t, file)
	if m["web_search"] != "disabled" {
		t.Errorf("web_search = %#v, want disabled (deployed value kept)\n%s", m["web_search"], file)
	}
	if tbl, _ := m["sandbox_workspace_write"].(map[string]any); tbl["network_access"] != false {
		t.Errorf("sandbox_workspace_write = %#v, want network_access false (deployed value kept)\n%s", tbl, file)
	}
}

func TestRefresh_PreChangeFile_AddsNothing(t *testing.T) {
	// Arrange / Act
	file := capEncode(t, capDecode(t, capPreChangeFile), capUpdateCtx([]byte(capPreChangeFile), true))

	// Assert
	m := capToml(t, file)
	if _, ok := m["web_search"]; ok {
		t.Errorf("refresh added web_search to a file that lacked it\n%s", file)
	}
	if _, ok := m["sandbox_workspace_write"]; ok {
		t.Errorf("refresh added sandbox_workspace_write to a file that lacked it\n%s", file)
	}
	if m["user_key"] != "keep-me" {
		t.Errorf("user_key = %#v, want keep-me", m["user_key"])
	}
}

// countLines counts the lines that start with prefix.
func countLines(file, prefix string) int {
	n := 0
	for _, l := range strings.Split(file, "\n") {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}
