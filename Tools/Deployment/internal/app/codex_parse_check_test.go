package app

// codex_parse_check_test.go specifies what probeDeployedStateWithIndex reports for a Codex
// agent file at the planned path of an id-bearing agent that no indexed file resolved to.
//
//   - A valid Codex file with no id stamp is reported present with its versions as decoded and
//     is not parse-failed (it is the file a first deploy wrote).
//   - A Codex file that cannot be decoded is reported present and parse-failed, with the decode
//     error recorded as the parse problem and the version fields cleared, so the planner raises
//     a conflict instead of planning an overwrite.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	codexParseAgentsDir = ".codex/agents"
	codexParseAgentKey  = "target-agent"
	codexParseTarget    = codexParseAgentsDir + "/" + codexParseAgentKey + ".toml"
)

// probeCodexPlannedPath writes content at the agent's planned Codex path and probes the agent
// through the id-aware probe with an active, empty-for-this-id index.
func probeCodexPlannedPath(t *testing.T, content []byte) domain.DeployedArtifactState {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, filepath.FromSlash(codexParseAgentsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, codexParseAgentKey+".toml", content)
	desc := codexProbeDesc()
	agent := domain.Agent{Key: codexParseAgentKey, NumericID: "9", Role: domain.RoleSubagent}
	paths := plan.PlannedPaths{
		{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: agent.Key}, TargetPath: codexParseTarget},
	}
	index, _ := buildDeployedAgentIndex(ws, codexParseAgentsDir, desc)
	if index == nil {
		t.Fatal("buildDeployedAgentIndex returned a nil index; the id-aware probe needs an active index")
	}

	result, err := probeDeployedStateWithIndex(ws, paths, "model", nil, index,
		map[string]domain.Agent{agent.Key: agent}, nil, desc)
	if err != nil {
		t.Fatalf("probeDeployedStateWithIndex: %v", err)
	}
	return result[codexParseTarget]
}

func TestProbeDeployedStateWithIndex_CodexValidFileWithoutIdStamp_PresentAndNotParseFailed(t *testing.T) {
	// Arrange
	content := codexTomlBytes("2.0", "1.5", "gpt-6-astra")

	// Act
	state := probeCodexPlannedPath(t, content)

	// Assert
	if !state.Present {
		t.Fatal("Present = false; a readable Codex file at the planned path must be reported present")
	}
	if state.ParseFailed {
		t.Errorf("ParseFailed = true (%q); a valid Codex file without an id stamp must not be parse-failed", state.ParseProblem)
	}
	if state.Version != "2.0" || state.HarnessVersion != "1.5" {
		t.Errorf("versions = %q/%q, want 2.0/1.5 as decoded", state.Version, state.HarnessVersion)
	}
}

func TestProbeDeployedStateWithIndex_CodexUndecodableFileAtPlannedPath_ReportedParseFailed(t *testing.T) {
	// Arrange: duplicate keys make the TOML undecodable.
	content := []byte("# mosaic_version: 2.0\n\nmodel = \"a\"\nmodel = \"b\"\n")

	// Act
	state := probeCodexPlannedPath(t, content)

	// Assert
	if !state.Present {
		t.Fatal("Present = false; an existing undecodable file must be reported present")
	}
	if !state.ParseFailed {
		t.Error("ParseFailed = false; an undecodable Codex file must be reported parse-failed, not planned as current")
	}
	if state.ParseProblem == "" {
		t.Error("ParseProblem is empty; the decode error must be recorded")
	}
	if state.Version != "" || state.HarnessVersion != "" {
		t.Errorf("versions = %q/%q, want both cleared for a parse-failed state", state.Version, state.HarnessVersion)
	}
}
