package app_test

// codex_capability_output_test.go covers the deployed Codex file produced through
// transform.Apply with the real Codex module: web search level, network access for fetching,
// and the update cycle (including over a file deployed before these settings existed).

import (
	"strings"
	"testing"

	_ "mosaic-common/agentformat/all"
	"mosaic-common/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const capOutKey = "cap-out-agent"

func capOutSource(tools string) []byte {
	return []byte("---\nid: 100\nversion: 1.0.0\ndescription: Capability output agent.\ntools: [" + tools + "]\n---\nAgent body.\n")
}

// capOutApply runs transform.Apply for the Codex module.
func capOutApply(t *testing.T, tools string, op agentformat.Operation, prior []byte, origin transform.OriginConfidence) transform.Result {
	t.Helper()
	req := transform.Request{
		Source: capOutSource(tools),
		Kind:   domain.ArtifactAgent,
		Key:    capOutKey,
		Module: codexAppModule(t),
		Model:  codexAppModel,
		Op:     op,
		Scope:  domain.ScopeProject,
		Origin: origin,
	}
	if prior != nil {
		tr, err := agentformat.LookupString("codex-toml")
		if err != nil {
			t.Fatalf("LookupString: %v", err)
		}
		canonical, _, decErr := tr.Decode(prior, agentformat.ArtifactContext{AgentKey: capOutKey})
		if decErr != nil {
			t.Fatalf("decode prior: %v\n%s", decErr, prior)
		}
		req.Deployed = canonical
		req.DeployedRaw = prior
	}
	res, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("transform.Apply(%s): %v", tools, err)
	}
	return res
}

func capOutCreate(t *testing.T, tools string) map[string]interface{} {
	t.Helper()
	return parseTomlMap(t, capOutApply(t, tools, agentformat.OpCreate, nil, "").Output)
}

func capOutNetwork(m map[string]interface{}) (bool, bool) {
	tbl, ok := m["sandbox_workspace_write"].(map[string]interface{})
	if !ok {
		return false, false
	}
	v, _ := tbl["network_access"].(bool)
	return v, true
}

func TestCodexOutput_WebSearch_LiveWithToolDisabledWithout(t *testing.T) {
	if got := capOutCreate(t, "file_read, web_search")["web_search"]; got != "live" {
		t.Errorf("with web_search: web_search = %#v, want live", got)
	}
	if got := capOutCreate(t, "file_read")["web_search"]; got != "disabled" {
		t.Errorf("without web_search: web_search = %#v, want disabled", got)
	}
}

func TestCodexOutput_WriteCapableAgentWithWebFetch_GetsNetworkAccess(t *testing.T) {
	for _, tools := range []string{"file_write, web_fetch", "terminal, web_fetch"} {
		m := capOutCreate(t, tools)
		if on, present := capOutNetwork(m); !present || !on {
			t.Errorf("tools %q: sandbox_workspace_write = %#v, want network_access true", tools, m["sandbox_workspace_write"])
		}
	}
}

func TestCodexOutput_ReadOnlyAgentWithWebFetch_GetsNoNetworkAccess(t *testing.T) {
	m := capOutCreate(t, "file_read, web_fetch")
	if _, present := capOutNetwork(m); present {
		t.Errorf("read-only agent got sandbox_workspace_write = %#v, want none", m["sandbox_workspace_write"])
	}
}

func TestCodexOutput_TerminalAlone_GrantsNoNetworkAccess(t *testing.T) {
	m := capOutCreate(t, "terminal")
	if _, present := capOutNetwork(m); present {
		t.Errorf("terminal alone got sandbox_workspace_write = %#v, want none", m["sandbox_workspace_write"])
	}
}

func TestCodexOutput_PlaceholderOrchestrator_GetsDisabledSearchAndNoNetwork(t *testing.T) {
	// Arrange
	req := transform.Request{
		Source: []byte("---\nid: 1\nversion: 1.0.0\ndescription: Orchestrator.\ntools: {tool-permissions}\n---\nBody.\n"),
		Kind:   domain.ArtifactAgent,
		Key:    "cap-orch",
		Module: codexAppModule(t),
		Model:  codexAppModel,
		Op:     agentformat.OpCreate,
		Scope:  domain.ScopeProject,
		Role:   domain.RoleOrchestrator,
	}

	// Act
	res, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("transform.Apply: %v", err)
	}

	// Assert
	m := parseTomlMap(t, res.Output)
	if m["web_search"] != "disabled" {
		t.Errorf("web_search = %#v, want disabled", m["web_search"])
	}
	if _, present := capOutNetwork(m); present {
		t.Errorf("placeholder orchestrator got sandbox_workspace_write = %#v, want none", m["sandbox_workspace_write"])
	}
}

func TestCodexOutput_UpdateOverPreChangeFile_AddsSettingsOnceAndKeepsUserKey(t *testing.T) {
	// Arrange: a file as Deploy wrote it before the settings existed.
	prior := []byte("name = \"" + capOutKey + "\"\ndescription = \"Old.\"\nmodel = \"gpt-5.6-sol\"\nsandbox_mode = \"workspace-write\"\n" +
		"developer_instructions = \"Old body.\\n\"\nuser_key = \"keep-me\"\n")

	// Act
	res := capOutApply(t, "file_write, web_fetch, web_search", agentformat.OpUpdate, prior, "")

	// Assert
	m := parseTomlMap(t, res.Output)
	if m["web_search"] != "live" {
		t.Errorf("web_search = %#v, want live\n%s", m["web_search"], res.Output)
	}
	if on, present := capOutNetwork(m); !present || !on {
		t.Errorf("sandbox_workspace_write = %#v, want network_access true\n%s", m["sandbox_workspace_write"], res.Output)
	}
	if m["user_key"] != "keep-me" {
		t.Errorf("user_key = %#v, want keep-me", m["user_key"])
	}
	if n := strings.Count(string(res.Output), "web_search"); n != 1 {
		t.Errorf("web_search occurs %d times in the file, want once\n%s", n, res.Output)
	}
}

func TestCodexOutput_UpdateOverOwnOutput_ReportsNoOwnedKeyDifferences(t *testing.T) {
	// Arrange: the file this tool wrote, redeployed with an unconfirmed origin. The new
	// settings must decode back to what Tools computes, so no difference is reported.
	first := capOutApply(t, "file_write, web_fetch, web_search", agentformat.OpCreate, nil, "")

	// Act
	second := capOutApply(t, "file_write, web_fetch, web_search", agentformat.OpUpdate, first.Output, transform.OriginUnconfirmed)

	// Assert: only the new settings are in scope here; other owned keys (such as the
	// Codex name, which has no incoming value) keep their existing reporting.
	for _, d := range second.Report.OwnedKeyDifferences {
		if d.Key != "web_search" && d.Key != "sandbox_workspace_write" {
			continue
		}
		t.Errorf("owned-key difference reported for %q (deployed %q, incoming %q), want none for the tool's own output",
			d.Key, d.Deployed, d.Incoming)
	}
	m := parseTomlMap(t, second.Output)
	if m["web_search"] != "live" {
		t.Errorf("web_search after update = %#v, want live", m["web_search"])
	}
}

func TestCodexOutput_UserChangedNetworkGrant_IsReportedAsOwnedKeyDifference(t *testing.T) {
	// Arrange: the user turned the grant off in the deployed file.
	first := capOutApply(t, "file_write, web_fetch", agentformat.OpCreate, nil, "")
	edited := []byte(strings.Replace(string(first.Output), "network_access = true", "network_access = false", 1))

	// Act
	second := capOutApply(t, "file_write, web_fetch", agentformat.OpUpdate, edited, transform.OriginUnconfirmed)

	// Assert
	found := false
	for _, d := range second.Report.OwnedKeyDifferences {
		if d.Key == "sandbox_workspace_write" {
			found = true
		}
	}
	if !found {
		t.Errorf("no owned-key difference for sandbox_workspace_write; got %+v", second.Report.OwnedKeyDifferences)
	}
}
